package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/google/generative-ai-go/genai"
)

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
			for i, tool := range tools {
				if len(tool.FunctionDeclarations) == 0 {
					t.Errorf("%s[%d] has no function declarations", name, i)
				}
				for j, fn := range tool.FunctionDeclarations {
					if fn.Name == "" {
						t.Errorf("%s[%d].FunctionDeclarations[%d] has no name", name, i, j)
					}
					if fn.Description == "" {
						t.Errorf("%s[%d].FunctionDeclarations[%d] has no description", name, i, j)
					}
					if fn.Parameters == nil {
						t.Errorf("%s[%d].FunctionDeclarations[%d] has no parameters schema", name, i, j)
					}
				}
			}
		})
	}
}

func TestDefaultExecutor_Execute(t *testing.T) {
	executor := NewExecutor(nil) // Client not needed for these tests
	ctx := context.Background()

	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
		want     string
		contains string
	}{
		{
			name:     "get_product_specs",
			toolName: "get_product_specs",
			args:     map[string]interface{}{"product_id": "123"},
			want:     "Technical specs for 123: Processor X1, 8GB RAM, Secure Boot enabled.",
		},
		{
			name:     "read_cra_regulation_text",
			toolName: "read_cra_regulation_text",
			args:     map[string]interface{}{"article_number": "10"},
			contains: "Article X",
		},
		{
			name:     "ingest_file_system",
			toolName: "ingest_file_system",
			args:     map[string]interface{}{"path": "/tmp/project"},
			want:     "Found: config.yaml, main.go, README.md",
		},
		{
			name:     "ingest_git_repo",
			toolName: "ingest_git_repo",
			args:     map[string]interface{}{"repo_url": "https://github.com/example/repo"},
			contains: "Cloned https://github.com/example/repo",
		},
		{
			name:     "apply_resource_tags",
			toolName: "apply_resource_tags",
			args:     map[string]interface{}{"resource_id": "res-1", "tags": map[string]string{"env": "prod"}},
			contains: "Tags applied successfully",
		},
		{
			name:     "generate_conformity_doc",
			toolName: "generate_conformity_doc",
			args:     map[string]interface{}{"product_name": "Widget", "classification": "Class I"},
			want:     "Generated EU Declaration of Conformity for Widget (Class: Class I)",
		},
		{
			name:     "query_cve_database",
			toolName: "query_cve_database",
			args:     map[string]interface{}{"component": "LibX", "version": "1.0"},
			contains: "No CRITICAL vulnerabilities found",
		},
		{
			name:     "default",
			toolName: "unknown_tool",
			args:     nil,
			want:     "Tool executed successfully.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executor.Execute(ctx, tt.toolName, tt.args)
			if err != nil {
				t.Errorf("executor.Execute() error = %v", err)
				return
			}
			if tt.contains != "" {
				if !strings.Contains(got, tt.contains) {
					t.Errorf("executor.Execute() = %q, expected to contain %q", got, tt.contains)
				}
			} else {
				if got != tt.want {
					t.Errorf("executor.Execute() = %q, want %q", got, tt.want)
				}
			}
		})
	}
}
