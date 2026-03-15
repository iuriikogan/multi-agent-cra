// Package core provides domain.go implementation.
//
// Rationale: This module is designed to encapsulate domain-specific logic,
// ensuring strict separation of concerns within the multi-agent CRA architecture.
// Terminology: CRA (Cyber Resilience Act), GCP (Google Cloud Platform), Agent (Autonomous AI actor).
// Measurability: Ensures code maintainability and testability by isolating discrete workflow steps.
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
	Regulation       string // CRA or DORA

	Error error
}
