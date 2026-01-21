package types

import "testing"

func TestFTOCIManifestlist(t *testing.T) {
	ml := FTOCIManifestlist()
	if ml.MediaType != "application/vnd.oci.image.index.v1+json" {
		t.Fatalf("Unexpected media type: %s", ml.MediaType)
	}
}
