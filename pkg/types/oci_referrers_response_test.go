package types

import "testing"

func TestNewReferrersResponse(t *testing.T) {
	r := NewReferrersResponse()
	if r.SchemaVersion != 2 {
		t.Fatalf("Expected SchemaVersion 2, got %d", r.SchemaVersion)
	}
	if r.MediaType != "application/vnd.oci.image.index.v1+json" {
		t.Fatalf("Unexpected MediaType: %s", r.MediaType)
	}
	if len(r.Manifests) != 0 {
		t.Fatalf("Expected empty Manifests, got %d entries", len(r.Manifests))
	}
}
func TestRefferrersResponseFromBytes(t *testing.T) {
	original := NewReferrersResponse()
	data, err := original.ToBytes()
	if err != nil {
		t.Fatalf("Failed to marshal ReferrersResponse: %v", err)
	}
	parsed, err := RefferrersResponseFromBytes(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal ReferrersResponse: %v", err)
	}
	if parsed.SchemaVersion != original.SchemaVersion {
		t.Fatalf("SchemaVersion mismatch: expected %d, got %d", original.SchemaVersion, parsed.SchemaVersion)
	}
	if parsed.MediaType != original.MediaType {
		t.Fatalf("MediaType mismatch: expected %s, got %s", original.MediaType, parsed.MediaType)
	}
	if len(parsed.Manifests) != len(original.Manifests) {
		t.Fatalf("Manifests length mismatch: expected %d, got %d", len(original.Manifests), len(parsed.Manifests))
	}
}
