// Package agent provides an abstraction for interacting with AI models.
package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/iuriikogan/Audit-Agent/pkg/tools"

	"google.golang.org/genai"
)

// Agent defines the behavior for an autonomous actor in the compliance system.
type Agent interface {
	Name() string
	Role() string
	Chat(ctx context.Context, input string) (string, error)
	Close() error
}

// GeminiAgent implements the Agent interface using Google's GenAI SDK.
type GeminiAgent struct {
	name              string
	role              string
	client            *genai.Client
	apiKey            string
	modelName         string
	systemInstruction string
	executor          tools.Executor
	tools             []*genai.Tool
}

// Option allows for functional configuration of a GeminiAgent.
type Option func(*GeminiAgent)

// WithTools attaches tools to the agent for function calling.
func WithTools(tools ...*genai.Tool) Option {
	return func(a *GeminiAgent) {
		if len(tools) > 0 {
			a.tools = append(a.tools, tools...)
		}
	}
}

// WithSystemInstruction defines the primary behavioral prompt for the agent.
func WithSystemInstruction(instruction string) Option {
	return func(a *GeminiAgent) {
		a.systemInstruction = instruction
	}
}

// WithExecutor configures a custom tool execution engine for the agent.
func WithExecutor(executor tools.Executor) Option {
	return func(a *GeminiAgent) {
		a.executor = executor
	}
}

// New initializes and returns a GeminiAgent with the specified identity and options.
func New(client *genai.Client, apiKey, name, role, modelName string, opts ...Option) *GeminiAgent {
	if modelName == "" {
		modelName = "gemini-3.1-flash-lite-preview"
	}

	agent := &GeminiAgent{
		name:      name,
		role:      role,
		client:    client,
		apiKey:    apiKey,
		modelName: modelName,
	}

	for _, opt := range opts {
		opt(agent)
	}

	if agent.executor == nil {
		agent.executor = tools.NewExecutor(client)
	}

	return agent
}

func (a *GeminiAgent) Name() string { return a.name }
func (a *GeminiAgent) Role() string { return a.role }

// Close releases resources associated with the agent's executor.
func (a *GeminiAgent) Close() error {
	if closer, ok := a.executor.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Chat facilitates a multi-turn conversation, handling autonomous tool execution.
func (a *GeminiAgent) Chat(ctx context.Context, input string) (string, error) {
	slog.Debug("Agent interaction started", "agent", a.name, "role", a.role, "input_length", len(input))

	temp := float32(0.2)
	config := &genai.GenerateContentConfig{
		Temperature: &temp,
		Tools:       a.tools,
	}

	if a.systemInstruction != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{
				{Text: a.systemInstruction},
			},
		}
	}

	chat, err := a.client.Chats.Create(ctx, a.modelName, config, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create chat session: %w", err)
	}

	res, err := chat.SendMessage(ctx, genai.Part{Text: input})
	if err != nil {
		return "", fmt.Errorf("failed to send initial message: %w", err)
	}

	for {
		fcs := res.FunctionCalls()
		if len(fcs) == 0 {
			return res.Text(), nil
		}

		var toolResponses []genai.Part
		for _, fc := range fcs {
			slog.Info("Executing tool", "agent", a.name, "tool", fc.Name)
			result, err := a.executor.Execute(ctx, fc.Name, fc.Args)
			if err != nil {
				slog.Error("Tool execution failed", "tool", fc.Name, "error", err)
				result = fmt.Sprintf("System Error executing tool '%s': %v", fc.Name, err)
			}

			toolResponses = append(toolResponses, genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name: fc.Name,
					Response: map[string]any{
						"output": result,
					},
				},
			})
		}

		res, err = chat.SendMessage(ctx, toolResponses...)
		if err != nil {
			return "", fmt.Errorf("failed to send tool responses: %w", err)
		}
	}
}
