package types

import (
	"github.com/opencontainers/go-digest"
)

const (
	// MediaTypeCycloneDX is the ORAS artifact config media type for CycloneDX SBOMs.
	// This is intentionally NOT in the container-image config allowlist, so manifests
	// using this media type are classified as ORAS artifacts.
	MediaTypeCycloneDX = "application/vnd.cyclonedx"
)

// FTOrasUpstream returns an Upstream for the ORAS CycloneDX featuretest artifact.
func FTOrasUpstream() Upstream {
	return Upstream{
		Reg:    reg5050,
		Repo:   "qnib/featuretest",
		Tag:    "cyclonedx",
		Digest: digest.Digest("sha256:0408af498cb876a98e041b4f93470a3a0b5e1ba587d4edf3be0b20b32073bc75"),
	}
}

// FTOrasConfig returns a Config representing an empty CycloneDX ORAS artifact config blob.
// The mediaType is "application/vnd.cyclonedx" — not a container image config — so manifests
// linked to this config are returned by ORAS artifact queries.
func FTOrasConfig() Config {
	data := []byte(`{}`)
	return Config{
		Digest:    digest.FromBytes(data).String(),
		MediaType: MediaTypeCycloneDX,
		Size:      int64(len(data)),
		Data:      data,
	}
}

// FTOrasManifest returns a Manifest representing an ORAS CycloneDX artifact.
// The config mediaType is "application/vnd.cyclonedx", which causes ORAS artifact queries
// to include this manifest (it is excluded from the container-image config allowlist).
// Platform is intentionally empty: ORAS artifacts are not platform-specific.
func FTOrasManifest() Manifest {
	orasManifest := NewDataTuple("data/manifests/oras/0408af498cb876a98e041b4f93470a3a0b5e1ba587d4edf3be0b20b32073bc75.json")
	cfg := FTOrasConfig()
	up := FTOrasUpstream()
	return Manifest{
		Digest:       string(orasManifest.Digest),
		Size:         int64(len(orasManifest.Data)),
		MediaType:    "application/vnd.oci.image.manifest.v1+json",
		ArtifactType: MediaTypeCycloneDX,
		Data:         orasManifest.Data,
		Config:       cfg,
		Type:         "ORIGINAL",
		Upstream:     up,
	}
}
