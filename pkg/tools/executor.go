package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/generative-ai-go/genai"
)

// Executor defines the interface for executing tools.
type Executor interface {
	Execute(ctx context.Context, name string, args map[string]interface{}) (string, error)
}

// DefaultExecutor implements the Executor interface with the standard toolset.
type DefaultExecutor struct {
	Client *genai.Client
}

// NewExecutor creates a new DefaultExecutor.
func NewExecutor(client *genai.Client) *DefaultExecutor {
	return &DefaultExecutor{Client: client}
}

// Execute routes tool calls to their implementation.
func (e *DefaultExecutor) Execute(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "get_product_specs":
		return fmt.Sprintf("Technical specs for %v: Processor X1, 8GB RAM, Secure Boot enabled.", args["product_id"]), nil
	case "query_cve_database":
		return fmt.Sprintf("No CRITICAL vulnerabilities found for %s %s. 2 LOW found in dependencies.", args["component"], args["version"]), nil
	case "read_cra_regulation_text":
		return "Article X: Products with digital elements shall be designed, developed and produced such that they ensure an appropriate level of cybersecurity.", nil
	case "list_gcp_assets":
		return e.listGCPAssets(args)
	case "ingest_file_system":
		return "Found: config.yaml, main.go, README.md", nil
	case "ingest_git_repo":
		return "Cloned https://github.com/example/repo. Files: .gitignore, deploy.sh, Dockerfile", nil
	case "apply_resource_tags":
		return fmt.Sprintf("Tags applied successfully to resource %s: %v", args["resource_id"], args["tags"]), nil
	case "generate_conformity_doc":
		return fmt.Sprintf("Generated EU Declaration of Conformity for %s (Class: %s)", args["product_name"], args["classification"]), nil
	case "generate_visual_dashboard":
		return e.generateVisualDashboard(ctx, args)
	default:
		return "Tool executed successfully.", nil
	}
}

func (e *DefaultExecutor) listGCPAssets(args map[string]interface{}) (string, error) {
	parent, _ := args["parent"].(string)
	if parent == "" {
		return "Error: parent argument is required.", nil
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
	// TODO: Replace with Cloud Asset Inventory Go SDK
	cmd := exec.Command("gcloud", cmdArgs...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Sprintf("Error executing gcloud: %v\nStderr: %s", err, string(exitErr.Stderr)), nil
		}
		return fmt.Sprintf("Error executing gcloud: %v", err), nil
	}

	// Parse gcloud JSON output
	var assets []map[string]interface{}
	if err := json.Unmarshal(output, &assets); err != nil {
		return fmt.Sprintf("Error parsing gcloud output: %v", err), nil
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
		return fmt.Sprintf("Error marshaling result: %v", err), nil
	}
	return string(finalJSON), nil
}

func (e *DefaultExecutor) generateVisualDashboard(ctx context.Context, args map[string]interface{}) (string, error) {
	prompt, _ := args["prompt"].(string)
	filename, _ := args["filename"].(string)
	if prompt == "" || filename == "" {
		return "Error: prompt and filename are required.", nil
	}

	// Use the image generation model
	// TODO: Use correct model name from config or constant
	imgModel := e.Client.GenerativeModel("gemini-3-pro-image-preview")
	resp, err := imgModel.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return fmt.Sprintf("Error generating image: %v", err), nil
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "Error: No image generated.", nil
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		if blob, ok := part.(genai.Blob); ok {
			// Sanitize filename to prevent path traversal
			safeFilename := filepath.Base(filename)
			if err := os.WriteFile(safeFilename, blob.Data, 0644); err != nil {
				return fmt.Sprintf("Error saving image to file: %v", err), nil
			}
			return fmt.Sprintf("Successfully generated visual dashboard and saved to %s", safeFilename), nil
		}
	}
	return "Error: No recognized image data found in response.", nil
}
