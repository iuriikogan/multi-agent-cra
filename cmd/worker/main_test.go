package main

import (
	"testing"
)

func TestWorker_CompileCheck(t *testing.T) {
	// Similar to batch, we verify runScan exists and compiles.
	var _ = runScan
}
