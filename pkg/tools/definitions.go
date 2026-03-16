// Package tools defines the functional capabilities accessible to agents.
package tools

import "google.golang.org/genai"

// Tool sets are organized by agent responsibility to enforce the principle of least privilege.

// ScopeTools provides capabilities for agents to retrieve product-specific technical data.
var ScopeTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "get_product_specs",
				Description: "Retrieves technical specifications of a product ID.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"product_id": {Type: genai.TypeString},
					},
					Required: []string{"product_id"},
				},
			},
		},
	},
}

// VulnTools defines capabilities for searching known vulnerability databases.
var VulnTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "query_cve_database",
				Description: "Checks public CVE databases for a given component and version.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"component": {Type: genai.TypeString},
						"version":   {Type: genai.TypeString},
					},
					Required: []string{"component", "version"},
				},
			},
		},
	},
}

// IngestionTools provides capabilities for cloud asset discovery and enumeration.
var IngestionTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "list_gcp_assets",
				Description: "Lists GCP assets within a given scope (project, folder, or organization).",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"parent": {
							Type:        genai.TypeString,
							Description: "The parent resource name, e.g., 'projects/my-project', 'folders/123', 'organizations/456'.",
						},
						"asset_types": {
							Type:        genai.TypeArray,
							Items:       &genai.Schema{Type: genai.TypeString},
							Description: "Optional list of asset types to filter by, e.g., ['compute.googleapis.com/Instance', 'storage.googleapis.com/Bucket'].",
						},
					},
					Required: []string{"parent"},
				},
			},
		},
	},
}

// TaggingTools defines capabilities for applying governance tags to cloud resources.
var TaggingTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "apply_resource_tags",
				Description: "Applies a set of key-value tags to a specified cloud resource.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"resource_id": {Type: genai.TypeString},
						"tags": {
							Type:        genai.TypeObject,
							Description: "Key-value map of tags to apply.",
						},
					},
					Required: []string{"resource_id", "tags"},
				},
			},
		},
	},
}

// ComplianceTools facilitates the generation of formal compliance documentation.
var ComplianceTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "generate_conformity_doc",
				Description: "Generates the official EU Declaration of Conformity PDF.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"classification": {Type: genai.TypeString},
						"product_name":   {Type: genai.TypeString},
					},
					Required: []string{"classification", "product_name"},
				},
			},
		},
	},
}

// RegulatoryCheckerTools provides read-only access to relevant regulatory frameworks.
var RegulatoryCheckerTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "search_knowledge_base",
				Description: "Performs a semantic search against the selected regulatory framework (CRA or DORA) to find relevant articles, requirements, and best practices.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query":      {Type: genai.TypeString, Description: "The search query or concept to look up."},
						"regulation": {Type: genai.TypeString, Description: "The regulation to search in (CRA or DORA)."},
					},
					Required: []string{"query"},
				},
			},
		},
	},
}

// VisualTools allows for the generation of graphical reports and dashboards.
var VisualTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "generate_visual_dashboard",
				Description: "Generates a visual compliance dashboard image based on a descriptive prompt.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"prompt": {
							Type:        genai.TypeString,
							Description: "A detailed description of the dashboard to generate.",
						},
						"filename": {
							Type:        genai.TypeString,
							Description: "The output filename (e.g., 'dashboard.png').",
						},
					},
					Required: []string{"prompt", "filename"},
				},
			},
		},
	},
}
