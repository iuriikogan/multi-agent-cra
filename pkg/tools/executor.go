// Package tools provides the logic for executing agent tools against real or simulated environments.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	asset "cloud.google.com/go/asset/apiv1"
	"cloud.google.com/go/asset/apiv1/assetpb"
	"github.com/google/generative-ai-go/genai"
	"github.com/iuriikogan/multi-agent-cra/pkg/knowledge"
	"google.golang.org/api/iterator"
)

// Executor defines the interface for running tool logic and returning results as strings.
type Executor interface {
	Execute(ctx context.Context, name string, args map[string]interface{}) (string, error)
}

// DefaultExecutor implements various ecosystem tools for the compliance agents.
type DefaultExecutor struct {
	Client      *genai.Client
	AssetClient *asset.Client
}

// NewExecutor initializes and returns a DefaultExecutor.
func NewExecutor(client *genai.Client) *DefaultExecutor {
	return &DefaultExecutor{Client: client}
}

// SetAssetClient configures a specific Google Cloud Asset client for the executor.
func (e *DefaultExecutor) SetAssetClient(client *asset.Client) {
	e.AssetClient = client
}

// Close releases resources and clients held by the executor.
func (e *DefaultExecutor) Close() error {
	if e.AssetClient != nil {
		return e.AssetClient.Close()
	}
	return nil
}

// Execute dispatches the tool call to its respective internal handler.
func (e *DefaultExecutor) Execute(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "get_product_specs":
		return fmt.Sprintf("Technical specs for %v: Processor X1, 8GB RAM, Secure Boot enabled.", args["product_id"]), nil
	case "query_cve_database":
		return fmt.Sprintf("No CRITICAL vulnerabilities found for %s %s. 2 LOW found in dependencies.", args["component"], args["version"]), nil
	case "search_cra_knowledge":
		query, _ := args["query"].(string)
		if query == "" {
			return "Error: query argument is required.", nil
		}
		reg := knowledge.RegulationCRA
		if r, ok := args["regulation"].(string); ok && r == string(knowledge.RegulationDORA) {
			reg = knowledge.RegulationDORA
		}
		chunks, err := knowledge.Search(ctx, e.Client, query, reg, 3)
		if err != nil {
			return fmt.Sprintf("Error searching knowledge base: %v", err), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Relevant %s Information:\n", reg))
		for _, c := range chunks {
			fmt.Fprintf(&sb, "- %s (Relevance: %.2f)\n", c.Text, c.Score)
		}
		return sb.String(), nil
	case "list_gcp_assets":
		return e.listGCPAssets(ctx, args)
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

// listGCPAssets uses the Cloud Asset API to search for resources across the project or organization.
func (e *DefaultExecutor) listGCPAssets(ctx context.Context, args map[string]interface{}) (string, error) {
	parent, _ := args["parent"].(string)
	if parent == "" {
		return "Error: parent argument is required.", nil
	}

	scope := parent
	if !strings.HasPrefix(scope, "projects/") && !strings.HasPrefix(scope, "folders/") && !strings.HasPrefix(scope, "organizations/") {
		scope = "projects/" + parent
	}

	if e.AssetClient == nil {
		var err error
		e.AssetClient, err = asset.NewClient(ctx)
		if err != nil {
			return fmt.Sprintf("Error creating asset client: %v", err), nil
		}
	}
	client := e.AssetClient

	req := &assetpb.SearchAllResourcesRequest{
		Scope:      scope,
		AssetTypes: []string{},
	}

	if types, ok := args["asset_types"].([]interface{}); ok && len(types) > 0 {
		for _, t := range types {
			if s, ok := t.(string); ok {
				req.AssetTypes = append(req.AssetTypes, s)
			}
		}
	}

	it := client.SearchAllResources(ctx, req)
	var result []map[string]interface{}
	count := 0

	for {
		asset, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Sprintf("Error listing assets: %v", err), nil
		}

		entry := map[string]interface{}{
			"name":       asset.Name,
			"asset_type": asset.AssetType,
			"location":   asset.Location,
		}
		result = append(result, entry)

		count++
		if count >= 5 {
			break
		}
	}
	finalJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("Error marshaling result: %v", err), nil
	}
	return string(finalJSON), nil
}

// generateVisualDashboard utilizes the image generation capabilities of the model to create a compliance visual.
func (e *DefaultExecutor) generateVisualDashboard(ctx context.Context, args map[string]interface{}) (string, error) {
	prompt, _ := args["prompt"].(string)
	filename, _ := args["filename"].(string)
	if prompt == "" || filename == "" {
		return "Error: prompt and filename are required.", nil
	}

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
			safeFilename := filepath.Base(filename)
			if err := os.WriteFile(safeFilename, blob.Data, 0644); err != nil {
				return fmt.Sprintf("Error saving image to file: %v", err), nil
			}
			return fmt.Sprintf("Successfully generated visual dashboard and saved to %s", safeFilename), nil
		}
	}
	return "Error: No recognized image data found in response.", nil
}
