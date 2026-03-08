package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"multi-agent-cra/pkg/tools"

	"github.com/google/generative-ai-go/genai"
)

// Agent represents the behavioral contract for any AI agent in the system.
type Agent interface {
	Name() string
	Role() string
	Chat(ctx context.Context, input string) (string, error)
}

// GeminiAgent is the concrete implementation using Google's GenAI SDK.
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

// Option defines a functional option for configuring an agent.
type Option func(*GeminiAgent)

// WithTools adds tools to the agent configuration.
func WithTools(tools ...*genai.Tool) Option {
	return func(a *GeminiAgent) {
		if len(tools) > 0 {
			a.model.Tools = append(a.model.Tools, tools...)
		}
	}
}

// WithSystemInstruction sets the system instruction.
func WithSystemInstruction(instruction string) Option {
	return func(a *GeminiAgent) {
		a.systemInstruction = instruction
		a.model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(instruction)},
		}
	}
}

// WithExecutor sets a custom tool executor.
func WithExecutor(executor tools.Executor) Option {
	return func(a *GeminiAgent) {
		a.executor = executor
	}
}

// New creates a new GeminiAgent with the given options.
// Configures the model, API key, and system instructions.
func New(client *genai.Client, apiKey, name, role, modelName string, opts ...Option) *GeminiAgent {
	if modelName == "" {
		modelName = "gemini-3.1-flash-lite-preview"
	}
	model := client.GenerativeModel(modelName)
	model.SetTemperature(0.2) // Low temperature for deterministic/regulatory tasks

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

	// Default executor if not provided
	if agent.executor == nil {
		agent.executor = tools.NewExecutor(client)
	}

	return agent
}

func (a *GeminiAgent) Name() string { return a.name }
func (a *GeminiAgent) Role() string { return a.role }

// Chat executes a single interaction loop.
// It handles potential tool calls by the model, executes them, and returns the final text response.
func (a *GeminiAgent) Chat(ctx context.Context, input string) (string, error) {
	slog.Debug("Agent interaction started", "agent", a.name, "role", a.role, "input_length", len(input))
	// We implement manual REST calls here because the current Go SDK (v0.20.1)
	// does not support 'thought_signature' required by Gemini 3.1 models.

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", a.modelName, a.apiKey)

	// Initialize history with system instruction if present
	var contents []map[string]interface{}

	// Prepare the initial user message
	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []map[string]interface{}{
			{"text": input},
		},
	})

	httpClient := &http.Client{}

	for {
		// Construct Request Body
		reqBody := map[string]interface{}{
			"contents": contents,
		}

		// Add System Instruction if present
		if a.systemInstruction != "" {
			reqBody["system_instruction"] = map[string]interface{}{
				"parts": []map[string]interface{}{
					{"text": a.systemInstruction},
				},
			}
		}

		// Add Tools if present
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
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("Failed to close response body", "error", err)
			}
		}()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(bodyBytes))
		}

		var respData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
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

		// Append model response to history immediately
		// IMPORTANT: This preserves the 'thought_signature' received in 'parts'
		contents = append(contents, content)

		// Check for function calls
		var functionCalls []map[string]interface{}
		for _, p := range parts {
			part := p.(map[string]interface{})
			if fc, ok := part["functionCall"].(map[string]interface{}); ok {
				functionCalls = append(functionCalls, fc)
			}
		}

		if len(functionCalls) == 0 {
			// No function calls, return text
			for _, p := range parts {
				part := p.(map[string]interface{})
				if text, ok := part["text"].(string); ok {
					slog.Debug("Agent interaction completed", "agent", a.name, "response_length", len(text))
					return text, nil
				}
			}
			return "", fmt.Errorf("no text or function calls in response")
		}

		// Execute tools
		var responseParts []map[string]interface{}
		for _, fc := range functionCalls {
			name := fc["name"].(string)
			args := fc["args"].(map[string]interface{})

			// Handle args which might be nil or empty
			if args == nil {
				args = make(map[string]interface{})
			}

			slog.Info("Executing tool", "agent", a.name, "tool", name)
			// Delegate execution to the configured executor
			result, err := a.executor.Execute(ctx, name, args)
			if err != nil {
				slog.Error("Tool execution failed", "tool", name, "error", err)
				// We still return the error string to the model so it can try to recover or report it
				result = fmt.Sprintf("System Error executing tool '%s': %v", name, err)
			}

			responseParts = append(responseParts, map[string]interface{}{
				"functionResponse": map[string]interface{}{
					"name":     name,
					"response": map[string]interface{}{"result": result},
				},
			})
		}

		// Append function responses to history
		contents = append(contents, map[string]interface{}{
			"role":  "function",
			"parts": responseParts,
		})
	}
}