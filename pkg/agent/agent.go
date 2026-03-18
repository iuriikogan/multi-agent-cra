// Package agent provides an abstraction for interacting with AI models.
//
// Rationale: Isolating the LLM interaction allows the system to seamlessly switch
// between models or API versions. The agent handles context management and tool
// execution locally, feeding results back to the LLM.
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
// It mandates a standard interface for interacting with different specialized agents.
type Agent interface {
	Name() string                                           // Name returns the identifier of the agent.
	Role() string                                           // Role returns the designated function of the agent.
	Chat(ctx context.Context, input string) (string, error) // Chat facilitates a conversational interaction.
	Close() error                                           // Close cleans up resources tied to the agent.
}

// GeminiAgent implements the Agent interface using the Google GenAI SDK.
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

// Option allows for functional configuration of a GeminiAgent instance.
type Option func(*GeminiAgent)

// WithTools attaches tools (functions) to the agent for autonomous execution.
func WithTools(tools ...*genai.Tool) Option {
	return func(a *GeminiAgent) {
		if len(tools) > 0 {
			a.tools = append(a.tools, tools...)
		}
	}
}

// WithSystemInstruction defines the primary behavioral prompt (persona) for the agent.
func WithSystemInstruction(instruction string) Option {
	return func(a *GeminiAgent) {
		a.systemInstruction = instruction
	}
}

// WithExecutor configures a custom tool execution engine.
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

	// Initialize default executor if none provided
	if agent.executor == nil {
		agent.executor = tools.NewExecutor(client)
	}

	return agent
}

// Name returns the configured name of the agent.
func (a *GeminiAgent) Name() string { return a.name }

// Role returns the designated role of the agent.
func (a *GeminiAgent) Role() string { return a.role }

// Close releases resources associated with the agent's executor, if it implements io.Closer.
func (a *GeminiAgent) Close() error {
	if closer, ok := a.executor.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Chat facilitates a multi-turn conversation, handling autonomous tool execution.
// It intercepts tool calls (FunctionCalling) requested by the LLM, executes them locally,
// and feeds the results back into the model to generate the final response.
func (a *GeminiAgent) Chat(ctx context.Context, input string) (string, error) {
	slog.Debug("Agent interaction started", "agent", a.name, "role", a.role, "input_length", len(input))

	if a.client == nil {
		return "", fmt.Errorf("agent client is not initialized")
	}

	temp := float32(0.2)
	config := &genai.GenerateContentConfig{
		Temperature: &temp,
		Tools:       a.tools,
	}

	// Set System Instruction if configured
	if a.systemInstruction != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{
				{Text: a.systemInstruction},
			},
		}
	}

	// Create a chat session with the SDK
	chat, err := a.client.Chats.Create(ctx, a.modelName, config, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create chat session: %w", err)
	}

	// Send initial prompt
	res, err := chat.SendMessage(ctx, genai.Part{Text: input})
	if err != nil {
		return "", fmt.Errorf("failed to send initial message: %w", err)
	}

	// Recursive tool execution loop
	for {
		fcs := res.FunctionCalls()
		if len(fcs) == 0 {
			// No function calls requested, return the textual response
			return res.Text(), nil
		}

		var toolResponses []genai.Part
		for _, fc := range fcs {
			slog.Info("Executing tool", "agent", a.name, "tool", fc.Name)

			// Execute tool logic locally
			result, err := a.executor.Execute(ctx, fc.Name, fc.Args)
			if err != nil {
				slog.Error("Tool execution failed", "tool", fc.Name, "error", err)
				result = fmt.Sprintf("System Error executing tool '%s': %v", fc.Name, err)
			}

			// Format the tool execution results back to the model
			toolResponses = append(toolResponses, genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name: fc.Name,
					Response: map[string]any{
						"output": result,
					},
				},
			})
		}

		// Send tool responses back to continue the conversation
		res, err = chat.SendMessage(ctx, toolResponses...)
		if err != nil {
			return "", fmt.Errorf("failed to send tool responses: %w", err)
		}
	}
}
