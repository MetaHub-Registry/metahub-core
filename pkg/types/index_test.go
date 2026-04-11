package types

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func twoArchIndex() Index {
	return Index{
		SchemaVersion: 2,
		MediaType:     v1.MediaTypeImageIndex,
		Manifests: []Manifest{
			{
				Digest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Size:      200,
				MediaType: v1.MediaTypeImageManifest,
				Platform:  Platform{Arch: "arm64", OS: "linux", Variant: "v8"},
			},
			{
				Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Size:      100,
				MediaType: v1.MediaTypeImageManifest,
				Platform:  Platform{Arch: "amd64", OS: "linux"},
			},
		},
	}
}

func TestToOCIBytes_SortsByDigest(t *testing.T) {
	idx := twoArchIndex()
	b := idx.ToOCIBytes()
	require.NotEmpty(t, b)

	var parsed OCIManifestList
	err := json.Unmarshal(b, &parsed)
	require.NoError(t, err)
	assert.Equal(t, 2, parsed.SchemaVersion)
	assert.Equal(t, string(v1.MediaTypeImageIndex), parsed.MediaType)
	require.Len(t, parsed.Manifests, 2)

	// Must be sorted by digest (amd64 "aaa..." < arm64 "bbb...")
	assert.Equal(t, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		parsed.Manifests[0].Digest.String(), "first descriptor should be amd64 (lower digest)")
	assert.Equal(t, "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		parsed.Manifests[1].Digest.String(), "second descriptor should be arm64 (higher digest)")
}

func TestToOCIBytes_PlatformMetadata(t *testing.T) {
	idx := twoArchIndex()
	b := idx.ToOCIBytes()
	var parsed OCIManifestList
	err := json.Unmarshal(b, &parsed)
	require.NoError(t, err)

	// amd64 (sorted first)
	assert.Equal(t, "amd64", parsed.Manifests[0].Platform.Architecture)
	assert.Equal(t, "linux", parsed.Manifests[0].Platform.OS)
	// arm64
	assert.Equal(t, "arm64", parsed.Manifests[1].Platform.Architecture)
	assert.Equal(t, "linux", parsed.Manifests[1].Platform.OS)
	assert.Equal(t, "v8", parsed.Manifests[1].Platform.Variant)
}

func TestToOCIBytes_EmptyManifests(t *testing.T) {
	idx := Index{
		SchemaVersion: 2,
		MediaType:     v1.MediaTypeImageIndex,
		Manifests:     []Manifest{},
	}
	b := idx.ToOCIBytes()
	var parsed OCIManifestList
	err := json.Unmarshal(b, &parsed)
	require.NoError(t, err)
	assert.Nil(t, parsed.Manifests, "empty input should produce null manifests array")
}

func TestToOCIDescriptor(t *testing.T) {
	idx := Index{
		MediaType: v1.MediaTypeImageIndex,
		Digest:    "sha256:abc123",
		Size:      512,
		Annotations: map[string]string{
			"foo": "bar",
		},
	}
	desc := idx.ToOCIDescriptor()
	assert.Equal(t, v1.MediaTypeImageIndex, desc.MediaType)
	assert.Equal(t, "sha256:abc123", desc.Digest.String())
	assert.Equal(t, int64(512), desc.Size)
	assert.Equal(t, "bar", desc.Annotations["foo"])
}

func TestToOCIDescriptor_WithArtifactType(t *testing.T) {
	idx := Index{
		MediaType:    v1.MediaTypeImageIndex,
		ArtifactType: "application/vnd.example.custom+type",
		Digest:       "sha256:def456",
		Size:         256,
	}
	desc := idx.ToOCIDescriptor()
	assert.Equal(t, "application/vnd.example.custom+type", desc.ArtifactType)
}

func TestWriteWWW(t *testing.T) {
	idx := twoArchIndex()
	w := httptest.NewRecorder()
	idx.WriteWWW(w)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, string(v1.MediaTypeImageIndex), w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Header().Get("Docker-Content-Digest"))
	assert.NotEmpty(t, w.Header().Get("Content-Length"))
	assert.NotEmpty(t, w.Header().Get("Etag"))

	var parsed OCIManifestList
	err := json.Unmarshal(w.Body.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Len(t, parsed.Manifests, 2)
}

func TestWriteWWWHeader(t *testing.T) {
	idx := twoArchIndex()
	w := httptest.NewRecorder()
	idx.WriteWWWHeader(w)

	assert.Equal(t, string(v1.MediaTypeImageIndex), w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Header().Get("Docker-Content-Digest"))
	assert.Empty(t, w.Body.Bytes(), "HEAD response must have no body")
}

func TestNewManifestListFromData(t *testing.T) {
	idx := twoArchIndex()
	b := idx.ToOCIBytes()

	parsed, err := NewManifestListFromData(b)
	require.NoError(t, err)
	assert.Equal(t, string(v1.MediaTypeImageIndex), parsed.MediaType)
	assert.NotEmpty(t, parsed.Digest, "digest must be computed from bytes")
	// Note: SchemaVersion doesn't round-trip because Index uses
	// json:"schema_version" but OCIManifestList uses json:"schemaVersion".
	// This is a pre-existing type mismatch; the Index.SchemaVersion field
	// is populated by the database hydration path, not by JSON round-trip.
}

func TestNewManifestListFromData_InvalidJSON(t *testing.T) {
	_, err := NewManifestListFromData([]byte(`{invalid}`))
	assert.Error(t, err)
}
