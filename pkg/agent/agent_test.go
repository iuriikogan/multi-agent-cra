package agent

import (
	"testing"
)

// Since we cannot easily test the specific GeminiAgent struct without a live client,
// We will create a MockAgent here that can be used by other packages (like workflow)
// if they import it. But usually mocks belong in the test package of the consumer
// or a separate mocks package.
//
// For this file, let's just ensure that GeminiAgent implements the Agent interface.
var _ Agent = (*GeminiAgent)(nil)

func TestGeminiAgent_Name_Role(t *testing.T) {
	// We can construct a struct directly for testing simple getters
	// since we are in the same package.
	a := &GeminiAgent{
		name: "test-agent",
		role: "tester",
	}

	if a.Name() != "test-agent" {
		t.Errorf("expected name 'test-agent', got %s", a.Name())
	}
	if a.Role() != "tester" {
		t.Errorf("expected role 'tester', got %s", a.Role())
	}
}

