// Package agent provides an abstraction for interacting with AI models.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/iuriikogan/multi-agent-cra/pkg/tools"

	"github.com/google/generative-ai-go/genai"
)

// Agent defines the behavior for an autonomous actor in the compliance system.
type Agent interface {
	Name() string
	Role() string
	Chat(ctx context.Context, input string) (string, error)
	Close() error
}

// GeminiAgent implements the Agent interface using Google's GenAI API.
type GeminiAgent struct {
	name              string
	role              string
	client            *genai.Client
	model             *genai.GenerativeModel
	apiKey            string
	modelName         string
	systemInstruction string
	executor          tools.Executor
}

// Option allows for functional configuration of a GeminiAgent.
type Option func(*GeminiAgent)

// WithTools attaches tools to the agent for function calling.
func WithTools(tools ...*genai.Tool) Option {
	return func(a *GeminiAgent) {
		if len(tools) > 0 {
			a.model.Tools = append(a.model.Tools, tools...)
		}
	}
}

// WithSystemInstruction defines the primary behavioral prompt for the agent.
func WithSystemInstruction(instruction string) Option {
	return func(a *GeminiAgent) {
		a.systemInstruction = instruction
		a.model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(instruction)},
		}
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
	model := client.GenerativeModel(modelName)
	model.SetTemperature(0.2)

	agent := &GeminiAgent{
		name:      name,
		role:      role,
		client:    client,
		model:     model,
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

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", a.modelName, a.apiKey)

	var contents []map[string]interface{}

	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []map[string]interface{}{
			{"text": input},
		},
	})

	httpClient := &http.Client{}

	for {
		reqBody := map[string]interface{}{
			"contents": contents,
		}

		if a.systemInstruction != "" {
			reqBody["system_instruction"] = map[string]interface{}{
				"parts": []map[string]interface{}{
					{"text": a.systemInstruction},
				},
			}
		}

		if len(a.model.Tools) > 0 {
			reqBody["tools"] = convertToolsToJSON(a.model.Tools)
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return "", fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return "", fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(bodyBytes))
		}

		var respData map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&respData)
		_ = resp.Body.Close()
		
		if err != nil {
			return "", fmt.Errorf("failed to decode response: %w", err)
		}

		candidates, ok := respData["candidates"].([]interface{})
		if !ok || len(candidates) == 0 {
			return "", fmt.Errorf("no candidates in response")
		}
		candidate, ok := candidates[0].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("invalid candidate format")
		}
		content, ok := candidate["content"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("missing content in candidate")
		}
		parts, ok := content["parts"].([]interface{})
		if !ok {
			return "", fmt.Errorf("missing parts in content")
		}

		contents = append(contents, content)

		var functionCalls []map[string]interface{}
		for _, p := range parts {
			part := p.(map[string]interface{})
			if fc, ok := part["functionCall"].(map[string]interface{}); ok {
				functionCalls = append(functionCalls, fc)
			}
		}

		if len(functionCalls) == 0 {
			for _, p := range parts {
				part := p.(map[string]interface{})
				if text, ok := part["text"].(string); ok {
					slog.Debug("Agent interaction completed", "agent", a.name, "response_length", len(text))
					return text, nil
				}
			}
			return "", fmt.Errorf("no text or function calls in response")
		}

		var responseParts []map[string]interface{}
		for _, fc := range functionCalls {
			name := fc["name"].(string)
			args := fc["args"].(map[string]interface{})

			if args == nil {
				args = make(map[string]interface{})
			}

			slog.Info("Executing tool", "agent", a.name, "tool", name)
			result, err := a.executor.Execute(ctx, name, args)
			if err != nil {
				slog.Error("Tool execution failed", "tool", name, "error", err)
				result = fmt.Sprintf("System Error executing tool '%s': %v", name, err)
			}

			responseParts = append(responseParts, map[string]interface{}{
				"functionResponse": map[string]interface{}{
					"name":     name,
					"response": map[string]interface{}{"result": result},
				},
			})
		}

		contents = append(contents, map[string]interface{}{
			"role":  "function",
			"parts": responseParts,
		})
	}
}
