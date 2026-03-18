// Package tools provides the logic for executing agent-driven actions against external environments.
//
// Rationale: This module implements the actual side-effects of "Function Calling".
// It bridges the gap between AI reasoning and the Google Cloud ecosystem (Asset Inventory, AI Image generation, etc.).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	asset "cloud.google.com/go/asset/apiv1"
	"cloud.google.com/go/asset/apiv1/assetpb"
	"github.com/iuriikogan/Audit-Agent/pkg/knowledge"
	"google.golang.org/api/iterator"
	"google.golang.org/genai"
)

// Executor defines the interface for mapping function names to Go logic.
type Executor interface {
	// Execute dispatches the request to the correct tool handler and returns the result as a serializable string.
	Execute(ctx context.Context, name string, args map[string]interface{}) (string, error)
}

// DefaultExecutor provides the production implementation of various ecosystem tools.
type DefaultExecutor struct {
	Client      *genai.Client
	AssetClient *asset.Client
}

// NewExecutor initializes a new DefaultExecutor with a provided GenAI client.
func NewExecutor(client *genai.Client) *DefaultExecutor {
	return &DefaultExecutor{Client: client}
}

// SetAssetClient manually configures the Asset API client (useful for dependency injection in testing).
func (e *DefaultExecutor) SetAssetClient(client *asset.Client) {
	e.AssetClient = client
}

// Close ensures all initialized API clients are gracefully disconnected.
func (e *DefaultExecutor) Close() error {
	if e.AssetClient != nil {
		return e.AssetClient.Close()
	}
	return nil
}

// Execute evaluates the requested function name and routes it to the corresponding logic handler.
func (e *DefaultExecutor) Execute(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	slog.Info("Executing tool", "tool_name", name)

	switch name {
	case "get_product_specs":
		return e.handleGetProductSpecs(args)
	case "query_cve_database":
		return e.handleQueryCVEDatabase(args)
	case "search_knowledge_base":
		return e.handleSearchKnowledgeBase(ctx, args)
	case "list_gcp_assets":
		return e.listGCPAssets(ctx, args)
	case "ingest_file_system":
		return "Simulation: Recursive scan of file system complete. Files: config.yaml, main.go, README.md", nil
	case "ingest_git_repo":
		return fmt.Sprintf("Simulation: Repository %v successfully cloned and parsed.", args["repo_url"]), nil
	case "apply_resource_tags":
		return e.handleApplyResourceTags(args)
	case "generate_conformity_doc":
		return e.handleGenerateConformityDoc(args)
	case "generate_visual_dashboard":
		return e.generateVisualDashboard(ctx, args)
	default:
		slog.Warn("Unknown tool called", "tool_name", name)
		return fmt.Sprintf("System: Tool '%s' is not implemented on this executor.", name), nil
	}
}

// handleGetProductSpecs simulates the retrieval of hardware/software specifications.
func (e *DefaultExecutor) handleGetProductSpecs(args map[string]interface{}) (string, error) {
	productID, _ := args["product_id"].(string)
	if productID == "" {
		return "Error: product_id is required.", nil
	}
	return fmt.Sprintf("Technical specs for %s: ARM v9 Processor, 16GB ECC RAM, TPM 2.0 Module enabled.", productID), nil
}

// handleQueryCVEDatabase simulates a lookup in vulnerability databases like NVD.
func (e *DefaultExecutor) handleQueryCVEDatabase(args map[string]interface{}) (string, error) {
	component, _ := args["component"].(string)
	version, _ := args["version"].(string)
	if component == "" || version == "" {
		return "Error: component and version are required for CVE lookup.", nil
	}
	return fmt.Sprintf("CVE Analysis for %s %s: 0 Critical, 1 High (fixed in %s.1), 3 Medium findings.", component, version, version), nil
}

// handleSearchKnowledgeBase performs a vector search against the embedded regulatory knowledge base (CRA/DORA).
func (e *DefaultExecutor) handleSearchKnowledgeBase(ctx context.Context, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "Error: query argument is required.", nil
	}

	reg := knowledge.RegulationCRA
	if r, ok := args["regulation"].(string); ok && strings.ToUpper(r) == string(knowledge.RegulationDORA) {
		reg = knowledge.RegulationDORA
	}

	chunks, err := knowledge.Search(ctx, e.Client, query, reg, 3)
	if err != nil {
		return fmt.Sprintf("Internal Error searching knowledge base: %v", err), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Relevant %s Regulatory Context:\n", reg)
	for _, c := range chunks {
		fmt.Fprintf(&sb, "- %s (Confidence: %.2f)\n", c.Text, c.Score)
	}
	return sb.String(), nil
}

// handleApplyResourceTags simulates tagging a resource in GCP with compliance metadata.
func (e *DefaultExecutor) handleApplyResourceTags(args map[string]interface{}) (string, error) {
	resID, _ := args["resource_id"].(string)
	tags, _ := args["tags"].(map[string]interface{})
	if resID == "" {
		return "Error: resource_id is required for tagging.", nil
	}
	return fmt.Sprintf("Success: Resource %s tagged with %v", resID, tags), nil
}

// handleGenerateConformityDoc simulates the generation of legal compliance documents.
func (e *DefaultExecutor) handleGenerateConformityDoc(args map[string]interface{}) (string, error) {
	name, _ := args["product_name"].(string)
	class, _ := args["classification"].(string)
	return fmt.Sprintf("Success: EU Declaration of Conformity generated for %s (Risk Classification: %s)", name, class), nil
}

// listGCPAssets uses the Cloud Asset API to search for resources across the project or organization.
func (e *DefaultExecutor) listGCPAssets(ctx context.Context, args map[string]interface{}) (string, error) {
	parent, _ := args["parent"].(string)
	if parent == "" {
		return "Error: parent argument (e.g., 'projects/my-id') is required.", nil
	}

	scope := parent
	if !strings.Contains(scope, "/") {
		scope = "projects/" + parent
	}

	if e.AssetClient == nil {
		var err error
		e.AssetClient, err = asset.NewClient(ctx)
		if err != nil {
			return fmt.Sprintf("Internal Error initializing GCP Asset client: %v", err), nil
		}
	}

	req := &assetpb.SearchAllResourcesRequest{
		Scope:      scope,
		AssetTypes: []string{},
	}

	if types, ok := args["asset_types"].([]interface{}); ok {
		for _, t := range types {
			if s, ok := t.(string); ok {
				req.AssetTypes = append(req.AssetTypes, s)
			}
		}
	}

	it := e.AssetClient.SearchAllResources(ctx, req)
	var result []map[string]string
	count := 0

	for {
		asset, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Sprintf("Internal Error querying GCP Assets: %v", err), nil
		}

		result = append(result, map[string]string{
			"name":       asset.Name,
			"asset_type": asset.AssetType,
			"location":   asset.Location,
		})

		count++
		if count >= 10 { // Optimization: limit results to prevent LLM context overflow
			break
		}
	}

	finalJSON, _ := json.Marshal(result)
	return string(finalJSON), nil
}

// generateVisualDashboard utilizes the image generation capabilities of Gemini 2.5 Flash Image.
func (e *DefaultExecutor) generateVisualDashboard(ctx context.Context, args map[string]interface{}) (string, error) {
	prompt, _ := args["prompt"].(string)
	filename, _ := args["filename"].(string)
	if prompt == "" || filename == "" {
		return "Error: both prompt and filename are required for dashboard generation.", nil
	}

	// Using strictly the correct GenAI SDK image generation call
	resp, err := e.Client.Models.GenerateImages(ctx, "gemini-2.5-flash-image", prompt, nil)
	if err != nil {
		return fmt.Sprintf("Internal Error generating dashboard image: %v", err), nil
	}

	if len(resp.GeneratedImages) == 0 {
		return "Error: No image was generated by the model.", nil
	}

	safeFilename := filepath.Base(filename)
	if err := os.WriteFile(safeFilename, resp.GeneratedImages[0].Image.ImageBytes, 0644); err != nil {
		return fmt.Sprintf("Internal Error saving image: %v", err), nil
	}

	return fmt.Sprintf("Success: Visual compliance report generated and saved to %s", safeFilename), nil
}
