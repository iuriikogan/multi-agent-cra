package batch

import (
	"os"
	"testing"

	"github.com/iuriikogan/multi-agent-cra/pkg/core"
)

func TestGenerateCSV(t *testing.T) {
	// Setup
	results := []core.AssessmentResult{
		{
			ResourceID:       "1",
			ResourceName:     "Test Resource",
			ResourceType:     "compute",
			ComplianceReport: "Compliant",
			Tags:             "",
		},
	}

	// Execution
	GenerateCSV(results)

	// Validation
	if _, err := os.Stat("compliance_report.csv"); os.IsNotExist(err) {
		t.Fatalf("CSV file was not created")
	}

	// Cleanup
	_ = os.Remove("compliance_report.csv")
}

func TestGenerateTaggingInstructions(t *testing.T) {
	// Setup
	results := []core.AssessmentResult{
		{
			ResourceID:       "1",
			ResourceName:     "Test Resource",
			ResourceType:     "compute",
			ComplianceReport: "NON-COMPLIANT",
			Tags:             "APPLIED_TAGS: status=non_compliant",
		},
	}

	// Execution (no panic check)
	GenerateTaggingInstructions(results)
}
