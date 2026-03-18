// Package tools defines the functional capabilities (toolsets) exposed to specialized agents.
//
// Rationale: Organizing tools into logical sets ensures the Principle of Least Privilege.
// Agents only receive the tool definitions necessary for their specific compliance role.
package tools

import "google.golang.org/genai"

// ScopeTools provides capabilities for retrieving granular technical data about a product's hardware or software.
var ScopeTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "get_product_specs",
				Description: "Retrieves low-level technical specifications (CPU, RAM, TPM, etc.) for a given product ID.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"product_id": {Type: genai.TypeString, Description: "Unique identifier for the product/device."},
					},
					Required: []string{"product_id"},
				},
			},
		},
	},
}

// VulnTools defines capabilities for performing automated vulnerability lookups.
var VulnTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "query_cve_database",
				Description: "Checks public vulnerability databases (e.g., NVD) for known security flaws in a component.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"component": {Type: genai.TypeString, Description: "Name of the software or hardware component."},
						"version":   {Type: genai.TypeString, Description: "Specific version string to check."},
					},
					Required: []string{"component", "version"},
				},
			},
		},
	},
}

// IngestionTools provides capabilities for recursive discovery of cloud assets and source code.
var IngestionTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "list_gcp_assets",
				Description: "Performs a cloud asset search across a GCP project, folder, or organization scope.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"parent": {
							Type:        genai.TypeString,
							Description: "The parent resource name (e.g., 'projects/my-id', 'folders/123').",
						},
						"asset_types": {
							Type:        genai.TypeArray,
							Items:       &genai.Schema{Type: genai.TypeString},
							Description: "Optional filter for specific resource types (e.g., ['compute.googleapis.com/Instance']).",
						},
					},
					Required: []string{"parent"},
				},
			},
		},
	},
}

// TaggingTools enables the automated enforcement of governance metadata on cloud resources.
var TaggingTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "apply_resource_tags",
				Description: "Attaches compliance-specific metadata tags to a live GCP resource.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"resource_id": {Type: genai.TypeString, Description: "Full GCP resource name or ID."},
						"tags": {
							Type:        genai.TypeObject,
							Description: "Map of key-value pairs representing the governance tags.",
						},
					},
					Required: []string{"resource_id", "tags"},
				},
			},
		},
	},
}

// ComplianceTools facilitates the creation of formal regulatory artifacts.
var ComplianceTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "generate_conformity_doc",
				Description: "Produces a formal EU Declaration of Conformity based on the assessment findings.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"classification": {Type: genai.TypeString, Description: "The risk classification (e.g., Class I, II)."},
						"product_name":   {Type: genai.TypeString, Description: "The commercial name of the product."},
					},
					Required: []string{"classification", "product_name"},
				},
			},
		},
	},
}

// RegulatoryCheckerTools connects the agents to the embedded regulatory knowledge base.
var RegulatoryCheckerTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "search_knowledge_base",
				Description: "Queries the EU Cyber Resilience Act (CRA) or DORA for specific compliance requirements.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query":      {Type: genai.TypeString, Description: "The topic or requirement to search for semantically."},
						"regulation": {Type: genai.TypeString, Description: "The regulation framework (CRA or DORA)."},
					},
					Required: []string{"query"},
				},
			},
		},
	},
}

// VisualTools allows agents to generate graphical artifacts for human-in-the-loop reporting.
var VisualTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "generate_visual_dashboard",
				Description: "Uses AI imaging to create a summary dashboard of compliance health.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"prompt":   {Type: genai.TypeString, Description: "Detailed description of the chart/visual to generate."},
						"filename": {Type: genai.TypeString, Description: "Desired filename (e.g., 'audit_summary.png')."},
					},
					Required: []string{"prompt", "filename"},
				},
			},
		},
	},
}
