// Package agent provides autonomous AI models and abstractions.
package agent

import (
	"context"
	"testing"

	"google.golang.org/genai"
)

// Ensure that GeminiAgent strictly implements the Agent interface.
var _ Agent = (*GeminiAgent)(nil)

// TestGeminiAgent_Name_Role verifies the accessor methods for Identity and Role
// properties of the GeminiAgent object.
func TestGeminiAgent_Name_Role(t *testing.T) {
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

// TestGeminiAgent_UninitializedClient verifies that a nil client triggers the
// appropriate error handling during the Chat sequence.
func TestGeminiAgent_UninitializedClient(t *testing.T) {
	a := &GeminiAgent{
		name: "test-agent",
		role: "tester",
	}

	_, err := a.Chat(context.Background(), "hello")
	if err == nil {
		t.Errorf("expected an error when calling Chat with a nil client, got none")
	}
}

// TestWithSystemInstruction verifies that the Option correctly updates
// the system instruction internal state.
func TestWithSystemInstruction(t *testing.T) {
	instruction := "You are a specialized auditor."
	agent := New(nil, "fake-api-key", "auditor", "audit", "gemini-3.1-flash-lite-preview", WithSystemInstruction(instruction))
	if agent.systemInstruction != instruction {
		t.Errorf("expected system instruction to be %q, got %q", instruction, agent.systemInstruction)
	}
}

// TestWithTools verifies that passing tool definitions appends them to the internal slice.
func TestWithTools(t *testing.T) {
	mockTool := &genai.Tool{}
	agent := New(nil, "fake-api-key", "auditor", "audit", "gemini-3.1-flash-lite-preview", WithTools(mockTool))
	if len(agent.tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(agent.tools))
	}
}
