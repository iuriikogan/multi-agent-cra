package core

// GCPResource represents a GCP resource to be assessed.
type GCPResource struct {
	ID        string
	Name      string
	Type      string // e.g., "compute.googleapis.com/Instance"
	ProjectID string
	Region    string
}

// AssessmentResult holds the outcome of the multi-agent workflow.
type AssessmentResult struct {
	ResourceID   string
	ResourceName string
	ResourceType string
	ProjectID    string

	Classification string // Kept for backward compatibility (Scope)
	AuditStatus    string // Kept for backward compatibility (Audit)
	VulnReport     string // Kept for backward compatibility (Vuln)

	// New Pipeline Fields
	ComplianceModel  string
	ComplianceReport string
	ApprovalStatus   string
	Tags             string
	Status           string // Overall status (Compliant/Non-Compliant)

	Error error
}
