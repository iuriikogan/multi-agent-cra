// Package store provides testing for the GCSStore implementation.
package store

import (
	"testing"
)

// TestGCS_Paths verifies that the generated object paths for metadata and findings are correctly formatted.
func TestGCS_Paths(t *testing.T) {
	s := &GCSStore{} // Shallow initialization for path tests.
	jobID := "test-job-uuid"
	resource := "test-compute-instance"

	t.Run("MetadataPath", func(t *testing.T) {
		gotMetadata := s.metadataPath(jobID)
		wantMetadata := "scans/test-job-uuid/metadata.json"
		if gotMetadata != wantMetadata {
			t.Errorf("metadataPath() = %q, want %q", gotMetadata, wantMetadata)
		}
	})

	t.Run("FindingPath", func(t *testing.T) {
		gotFinding := s.findingPath(jobID, resource)
		wantFinding := "scans/test-job-uuid/findings/test-compute-instance.json"
		if gotFinding != wantFinding {
			t.Errorf("findingPath() = %q, want %q", gotFinding, wantFinding)
		}
	})
}

// TestGCSStore_Compilation verifies that GCSStore correctly fulfills the Store interface.
func TestGCSStore_Compilation(t *testing.T) {
	// A compilation check for the GCSStore struct.
	var _ Store = (*GCSStore)(nil)
}
