package core

import (
	"encoding/json"
	"testing"
)

func TestGCPResource_JSON(t *testing.T) {
	r := GCPResource{
		ID:        "r1",
		Name:      "test-instance",
		Type:      "compute.googleapis.com/Instance",
		ProjectID: "test-project",
		Region:    "us-central1",
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal resource: %v", err)
	}

	var r2 GCPResource
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatalf("failed to unmarshal resource: %v", err)
	}

	if r.ID != r2.ID {
		t.Errorf("expected ID %s, got %s", r.ID, r2.ID)
	}
	if r.Name != r2.Name {
		t.Errorf("expected Name %s, got %s", r.Name, r2.Name)
	}
}

func TestAssessmentResult_JSON(t *testing.T) {
	ar := AssessmentResult{
		ResourceID:       "r1",
		ResourceName:     "test-instance",
		ResourceType:     "compute.googleapis.com/Instance",
		ComplianceReport: "Pass",
		Status:           "Compliant",
	}

	data, err := json.Marshal(ar)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var ar2 AssessmentResult
	if err := json.Unmarshal(data, &ar2); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if ar.ResourceID != ar2.ResourceID {
		t.Errorf("expected ResourceID %s, got %s", ar.ResourceID, ar2.ResourceID)
	}
	if ar.Status != ar2.Status {
		t.Errorf("expected Status %s, got %s", ar.Status, ar2.Status)
	}
}
