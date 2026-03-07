package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	pb "cloud.google.com/go/ai/generativelanguage/apiv1beta/generativelanguagepb"
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
	if a.systemInstruction != "" {
		// System instructions are passed in 'system_instruction' field, not 'contents' usually,
		// but let's stick to the generateContent body structure.
		// Actually, system_instruction is a top-level field.
	}

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
		defer resp.Body.Close()

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
			result := a.executeMockTool(ctx, name, args)

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

func convertToolsToJSON(tools []*genai.Tool) []map[string]interface{} {
	var finalResult []map[string]interface{}
	for _, t := range tools {
		if len(t.FunctionDeclarations) > 0 {
			var funcs []map[string]interface{}
			for _, fd := range t.FunctionDeclarations {
				funcMap := map[string]interface{}{
					"name":        fd.Name,
					"description": fd.Description,
				}
				if fd.Parameters != nil {
					funcMap["parameters"] = convertSchemaToJSON(fd.Parameters)
				}
				funcs = append(funcs, funcMap)
			}
			finalResult = append(finalResult, map[string]interface{}{
				"function_declarations": funcs,
			})
		}
	}
	return finalResult
}

func convertSchemaToJSON(s *genai.Schema) map[string]interface{} {
	if s == nil {
		return nil
	}
	
	// Use the protobuf Type enum's String() method to get the correct API representation (e.g., "STRING", "OBJECT")
	t := pb.Type(s.Type).String()
	
	m := map[string]interface{}{
		"type": t,
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if len(s.Properties) > 0 {
		props := make(map[string]interface{})
		for k, v := range s.Properties {
			props[k] = convertSchemaToJSON(v)
		}
		m["properties"] = props
	}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	if s.Items != nil {
		m["items"] = convertSchemaToJSON(s.Items)
	}
	return m
}


// executeMockTool routes tool calls to their implementations (real or mock).
func (a *GeminiAgent) executeMockTool(ctx context.Context, name string, args map[string]interface{}) string {
	switch name {
	case "get_product_specs":
		return fmt.Sprintf("Technical specs for %v: Processor X1, 8GB RAM, Secure Boot enabled.", args["product_id"])
	case "query_cve_database":
		return fmt.Sprintf("No CRITICAL vulnerabilities found for %s %s. 2 LOW found in dependencies.", args["component"], args["version"])
	case "read_cra_regulation_text":
		return "Article X: Products with digital elements shall be designed, developed and produced such that they ensure an appropriate level of cybersecurity."
	case "list_gcp_assets":
		parent, _ := args["parent"].(string)
		if parent == "" {
			return "Error: parent argument is required."
		}

		// Prepare gcloud arguments
		cmdArgs := []string{"asset", "list", "--format=json"}

		if strings.HasPrefix(parent, "projects/") {
			projectID := strings.TrimPrefix(parent, "projects/")
			cmdArgs = append(cmdArgs, "--project="+projectID)
		} else if strings.HasPrefix(parent, "folders/") {
			folderID := strings.TrimPrefix(parent, "folders/")
			cmdArgs = append(cmdArgs, "--folder="+folderID)
		} else if strings.HasPrefix(parent, "organizations/") {
			orgID := strings.TrimPrefix(parent, "organizations/")
			cmdArgs = append(cmdArgs, "--organization="+orgID)
		} else {
			// Fallback: assume it's a project ID if no prefix
			cmdArgs = append(cmdArgs, "--project="+parent)
		}

		// Handle optional asset_types filtering
		if types, ok := args["asset_types"].([]interface{}); ok && len(types) > 0 {
			var typeList []string
			for _, t := range types {
				if s, ok := t.(string); ok {
					typeList = append(typeList, s)
				}
			}
			if len(typeList) > 0 {
				cmdArgs = append(cmdArgs, "--asset-types="+strings.Join(typeList, ","))
			}
		}

		// Execute gcloud command
		cmd := exec.Command("gcloud", cmdArgs...)
		output, err := cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return fmt.Sprintf("Error executing gcloud: %v\nStderr: %s", err, string(exitErr.Stderr))
			}
			return fmt.Sprintf("Error executing gcloud: %v", err)
		}

		// Parse gcloud JSON output
		var assets []map[string]interface{}
		if err := json.Unmarshal(output, &assets); err != nil {
			return fmt.Sprintf("Error parsing gcloud output: %v", err)
		}

		// Transform to expected format (snake_case for main.go compatibility)
		var result []map[string]interface{}
		locationRegex := regexp.MustCompile(`/(locations|zones|regions)/([^/]+)`)

		for _, asset := range assets {
			entry := map[string]interface{}{
				"name":       asset["name"],
				"asset_type": asset["assetType"],
				"location":   "", // Location isn't consistently available in top-level asset object
			}
			// Attempt to extract location from name if possible
			nameStr, _ := asset["name"].(string)
			if match := locationRegex.FindStringSubmatch(nameStr); len(match) == 3 {
				entry["location"] = match[2]
			}

			result = append(result, entry)
		}

		finalJSON, err := json.Marshal(result)
		if err != nil {
			return fmt.Sprintf("Error marshaling result: %v", err)
		}
		return string(finalJSON)
	case "ingest_file_system":
		return "Found: config.yaml, main.go, README.md"
	case "ingest_git_repo":
		return "Cloned https://github.com/example/repo. Files: .gitignore, deploy.sh, Dockerfile"
	case "apply_resource_tags":
		return fmt.Sprintf("Tags applied successfully to resource %s: %v", args["resource_id"], args["tags"])
	case "generate_conformity_doc":
		return fmt.Sprintf("Generated EU Declaration of Conformity for %s (Class: %s)", args["product_name"], args["classification"])
	case "generate_visual_dashboard":
		prompt, _ := args["prompt"].(string)
		filename, _ := args["filename"].(string)
		if prompt == "" || filename == "" {
			return "Error: prompt and filename are required."
		}

		// Use the image generation model
		imgModel := a.client.GenerativeModel("gemini-3-pro-image-preview")
		resp, err := imgModel.GenerateContent(ctx, genai.Text(prompt))
		if err != nil {
			return fmt.Sprintf("Error generating image: %v", err)
		}

		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			return "Error: No image generated."
		}

		for _, part := range resp.Candidates[0].Content.Parts {
			if blob, ok := part.(genai.Blob); ok {
				// Sanitize filename to prevent path traversal
				safeFilename := filepath.Base(filename)
				if err := os.WriteFile(safeFilename, blob.Data, 0644); err != nil {
					return fmt.Sprintf("Error saving image to file: %v", err)
				}
				return fmt.Sprintf("Successfully generated visual dashboard and saved to %s", safeFilename)
			}
		}
		return "Error: No recognized image data found in response."
	default:
		return "Tool executed successfully."
	}
}
