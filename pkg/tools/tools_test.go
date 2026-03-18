// Package tools provides tests for the functional agent capabilities.
package tools

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/genai"
)

// TestToolDefinitions ensures all toolsets are properly defined with names, descriptions, and schemas.
func TestToolDefinitions(t *testing.T) {
	toolSets := map[string][]*genai.Tool{
		"ScopeTools":             ScopeTools,
		"VulnTools":              VulnTools,
		"IngestionTools":         IngestionTools,
		"TaggingTools":           TaggingTools,
		"ComplianceTools":        ComplianceTools,
		"RegulatoryCheckerTools": RegulatoryCheckerTools,
	}

	for name, tools := range toolSets {
		t.Run(name, func(t *testing.T) {
			if len(tools) == 0 {
				t.Errorf("%s is empty", name)
			}
			for _, tool := range tools {
				if len(tool.FunctionDeclarations) == 0 {
					t.Errorf("%s has no function declarations", name)
				}
				for _, fn := range tool.FunctionDeclarations {
					if fn.Name == "" || fn.Description == "" || fn.Parameters == nil {
						t.Errorf("%s function %q is missing metadata", name, fn.Name)
					}
				}
			}
		})
	}
}

// TestDefaultExecutor_Execute verifies the routing and response formatting of the DefaultExecutor.
func TestDefaultExecutor_Execute(t *testing.T) {
	executor := NewExecutor(nil) // Client not needed for these simulation tests
	ctx := context.Background()

	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
		contains string
	}{
		{
			name:     "get_product_specs",
			toolName: "get_product_specs",
			args:     map[string]interface{}{"product_id": "123"},
			contains: "Technical specs for 123",
		},
		{
			name:     "search_knowledge_base_no_client",
			toolName: "search_knowledge_base",
			args:     map[string]interface{}{"query": "reporting obligations"},
			contains: "knowledge: genai client is required",
		},
		{
			name:     "ingest_file_system",
			toolName: "ingest_file_system",
			args:     map[string]interface{}{"path": "/tmp/project"},
			contains: "Simulation: Recursive scan",
		},
		{
			name:     "ingest_git_repo",
			toolName: "ingest_git_repo",
			args:     map[string]interface{}{"repo_url": "https://github.com/example/repo"},
			contains: "Simulation: Repository https://github.com/example/repo successfully cloned",
		},
		{
			name:     "apply_resource_tags",
			toolName: "apply_resource_tags",
			args:     map[string]interface{}{"resource_id": "res-1", "tags": map[string]interface{}{"env": "prod"}},
			contains: "Success: Resource res-1 tagged",
		},
		{
			name:     "generate_conformity_doc",
			toolName: "generate_conformity_doc",
			args:     map[string]interface{}{"product_name": "Widget", "classification": "Class I"},
			contains: "Success: EU Declaration of Conformity generated for Widget",
		},
		{
			name:     "query_cve_database",
			toolName: "query_cve_database",
			args:     map[string]interface{}{"component": "LibX", "version": "1.0"},
			contains: "CVE Analysis for LibX 1.0",
		},
		{
			name:     "unknown_tool",
			toolName: "unknown_tool",
			args:     nil,
			contains: "System: Tool 'unknown_tool' is not implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executor.Execute(ctx, tt.toolName, tt.args)
			if err != nil {
				t.Fatalf("Execute() unexpected error: %v", err)
			}
			if !strings.Contains(got, tt.contains) {
				t.Errorf("Execute() = %q, want it to contain %q", got, tt.contains)
			}
		})
	}
}
