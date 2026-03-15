package main

import (
	"testing"

	"github.com/iuriikogan/Audit-Agent/internal/batch"
)

func TestRunBatch_CompileCheck(t *testing.T) {
	// This test ensures that the batch.Run function signature matches expectations
	// and the package compiles. We can't easily run it without mocking everything.
	// So we just reference it to ensure it exists.
	var _ = batch.Run
}
