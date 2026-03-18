// Package core provides domain.go defines core entities for the audit agent.
package core

// GCPResource represents a GCP resource to be assessed by the agents.
// It maps directly to Google Cloud Asset Inventory structures.
type GCPResource struct {
	ID        string `json:"id"`         // Unique identifier for the resource.
	Name      string `json:"name"`       // Display name of the resource.
	Type      string `json:"type"`       // GCP resource type (e.g., "compute.googleapis.com/Instance").
	ProjectID string `json:"project_id"` // ID of the GCP project containing the resource.
	Region    string `json:"region"`     // GCP region where the resource is deployed.
}

// AssessmentResult holds the comprehensive outcome of the multi-agent workflow.
// It tracks findings across various stages: Modeling, Validation, Review, and Tagging.
type AssessmentResult struct {
	// Resource Identification
	ResourceID   string `json:"resource_id"`   // ID of the evaluated resource.
	ResourceName string `json:"resource_name"` // Name of the evaluated resource.
	ResourceType string `json:"resource_type"` // Type of the evaluated resource.
	ProjectID    string `json:"project_id"`    // Associated project ID.

	// Legacy Pipeline Fields (Maintained for backward compatibility)
	Classification string `json:"classification,omitempty"` // Legacy scope classification.
	AuditStatus    string `json:"audit_status,omitempty"`   // Legacy audit status.
	VulnReport     string `json:"vuln_report,omitempty"`    // Legacy vulnerability report.

	// Modern Pipeline Fields
	ComplianceModel  string `json:"compliance_model"`  // The regulatory model applied (e.g., CRA or DORA specifics).
	ComplianceReport string `json:"compliance_report"` // Detailed findings from the Validator agent.
	ApprovalStatus   string `json:"approval_status"`   // Status from the Reviewer agent (e.g., Approved, Rejected).
	Tags             string `json:"tags"`              // JSON string or comma-separated list of applied tags.
	Status           string `json:"status"`            // Overall compliance status (e.g., "Compliant", "Non-Compliant").
	Regulation       string `json:"regulation"`        // The regulation assessed against ("CRA" or "DORA").

	Error error `json:"-"` // Internal error tracking during pipeline execution (not serialized).
}
