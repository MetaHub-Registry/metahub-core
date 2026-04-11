package types

import (
	"testing"

	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestOCIManifestIsArtifact(t *testing.T) {
	tests := []struct {
		name string
		m    OCIManifest
		want bool
	}{
		{
			name: "normal oci manifest",
			m: OCIManifest{
				Config: imgspecv1.Descriptor{
					MediaType: imgspecv1.MediaTypeImageConfig,
				},
			},
			want: false,
		},
		{
			name: "artifact oci manifest",
			m: OCIManifest{
				Config: imgspecv1.Descriptor{
					MediaType: "Fake/Artifact+Type",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.IsArtifact(); got != tt.want {
				t.Errorf("OCIManifest.IsArtifact() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Additional Manifest tests are in manifest_coverage_test.go
