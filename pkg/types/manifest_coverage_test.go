package types

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file covers the Manifest serialisation and HTTP-header helpers
// in pkg/types/manifest.go. Before this file existed, `pkg/types/manifest.go`
// was at 6.7% coverage (3 covered ranges, 28 uncovered ranges) despite
// being one of the most heavily-used types in metalib — the content-leaf
// Manifest from Path Z task 2.1. The functions here are all small,
// pure, and trivial to exercise; the low coverage was an oversight
// from the rename session, not a signal of anything risky.
//
// Each test constructs a realistic Manifest fixture (amd64 linux image
// with a config, two layers, a platform, a few annotations, upstream
// provenance) and walks it through the various serialisation + HTTP
// helpers, asserting on the output shape.

// buildTestManifest returns a fully-populated Manifest fixture for the
// serialisation tests. Factored out so every test starts from the same
// baseline and individual tests only mutate what they care about.
func buildTestManifest() Manifest {
	cfgDigest := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	layer1Digest := "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	layer2Digest := "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	manifestDigest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	annos, err := NewAnnotationsFromMap(map[string]string{
		"org.opencontainers.image.source":  "https://example.com/src",
		"org.opencontainers.image.version": "v1.2.3",
	})
	if err != nil {
		panic(err) // test-fixture helper, should never fail
	}

	return Manifest{
		ID:        42,
		Type:      "manifest",
		Origin:    "client-pushed",
		Digest:    manifestDigest,
		Data:      []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json"}`),
		MediaType: imgspecv1.MediaTypeImageManifest,
		Size:      1024,
		Platform: Platform{
			Arch:    "amd64",
			OS:      "linux",
			Variant: "",
		},
		Upstream: Upstream{
			Reg:    "docker.io",
			Repo:   "library/alpine",
			Tag:    "3.19",
			Digest: digest.Digest(manifestDigest),
		},
		Config: Config{
			Digest:    cfgDigest,
			MediaType: imgspecv1.MediaTypeImageConfig,
			Size:      512,
			Data:      []byte(`{"os":"linux","architecture":"amd64"}`),
		},
		Layers: []Layer{
			{
				Digest:    layer1Digest,
				MediaType: imgspecv1.MediaTypeImageLayerGzip,
				Size:      2048,
			},
			{
				Digest:    layer2Digest,
				MediaType: imgspecv1.MediaTypeImageLayerGzip,
				Size:      4096,
			},
		},
		Label:       "alpine:3.19",
		Profile:     "cpu#generic",
		Annotations: annos,
	}
}

// TestManifestToBytes exercises Manifest.ToBytes which is the simple
// JSON marshal helper. Asserts the output round-trips and includes the
// key fields.
func TestManifestToBytes(t *testing.T) {
	m := buildTestManifest()
	data, err := m.ToBytes()
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Round-trip check: the JSON output should re-parse back to a
	// Manifest with the same digest, mediaType, and size.
	var decoded Manifest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, m.Digest, decoded.Digest)
	assert.Equal(t, m.MediaType, decoded.MediaType)
	assert.Equal(t, m.Size, decoded.Size)
	assert.Equal(t, m.Platform.Arch, decoded.Platform.Arch)
	assert.Equal(t, m.Platform.OS, decoded.Platform.OS)
}

// TestManifestToOCIManifestDescriptor exercises the conversion from
// a metalib Manifest to the docker/distribution ManifestDescriptor
// used in manifest-list responses. Asserts the descriptor carries the
// correct digest, mediaType, size, and platform spec.
func TestManifestToOCIManifestDescriptor(t *testing.T) {
	m := buildTestManifest()
	desc := m.ToOCIManifestDescriptor()
	assert.Equal(t, m.Digest, desc.Digest.String())
	assert.Equal(t, m.MediaType, desc.MediaType)
	assert.Equal(t, m.Size, desc.Size)
	assert.Equal(t, m.Platform.Arch, desc.Platform.Architecture)
	assert.Equal(t, m.Platform.OS, desc.Platform.OS)
	assert.Equal(t, m.Platform.Variant, desc.Platform.Variant)
	assert.NotNil(t, desc.Platform.Features,
		"Features must be initialised as an empty slice, not nil, so the JSON output stays `[]` not `null`")
}

// TestManifestFromOciDescriptor is the reverse direction — build a
// metalib Manifest from an OCI v1 Descriptor. Covers the
// Annotations-from-map path and the optional platform hydration.
func TestManifestFromOciDescriptor(t *testing.T) {
	srcDigest := "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	desc := imgspecv1.Descriptor{
		Digest:    "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		MediaType: imgspecv1.MediaTypeImageManifest,
		Size:      2048,
		Annotations: map[string]string{
			"org.metahub.test": "yes",
		},
		Platform: &imgspecv1.Platform{
			Architecture: "arm64",
			OS:           "linux",
			Variant:      "v8",
		},
	}
	m := ManifestFromOciDescriptor(desc)
	assert.Equal(t, srcDigest, m.Digest)
	assert.Equal(t, desc.MediaType, m.MediaType)
	assert.Equal(t, desc.Size, m.Size)
	assert.Equal(t, "arm64", m.Platform.Arch)
	assert.Equal(t, "linux", m.Platform.OS)
	assert.Equal(t, "v8", m.Platform.Variant)
	annoMap := m.Annotations.ToMap()
	assert.Equal(t, "yes", annoMap["org.metahub.test"])
}

// TestManifestFromOciDescriptor_NoPlatform covers the branch where
// the source OCI Descriptor has no Platform field — common for ORAS
// artifacts and subject references.
func TestManifestFromOciDescriptor_NoPlatform(t *testing.T) {
	desc := imgspecv1.Descriptor{
		Digest:    "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		MediaType: "application/vnd.cncf.oras.artifact.manifest.v1+json",
		Size:      256,
		// Platform deliberately nil
	}
	m := ManifestFromOciDescriptor(desc)
	assert.Equal(t, desc.Size, m.Size)
	assert.Equal(t, "", m.Platform.Arch, "no source platform → zero-value Platform.Arch")
	assert.Equal(t, "", m.Platform.OS)
}

// TestManifestToOCIDescriptor is the round-trip going back out as an
// OCI v1 Descriptor. Asserts both the populated-platform branch (lines
// 96-102) and the annotation-map serialisation.
func TestManifestToOCIDescriptor(t *testing.T) {
	m := buildTestManifest()
	desc := m.ToOCIDescriptor()
	assert.Equal(t, m.Digest, desc.Digest.String())
	assert.Equal(t, m.MediaType, desc.MediaType)
	assert.Equal(t, m.Size, desc.Size)
	require.NotNil(t, desc.Platform, "fixture has non-zero platform, descriptor.Platform must be populated")
	assert.Equal(t, m.Platform.Arch, desc.Platform.Architecture)
	assert.Equal(t, m.Platform.OS, desc.Platform.OS)
	// Annotations are passed through the map.
	assert.Equal(t, "https://example.com/src", desc.Annotations["org.opencontainers.image.source"])
	assert.Equal(t, "v1.2.3", desc.Annotations["org.opencontainers.image.version"])
}

// TestManifestToOCIDescriptor_EmptyPlatform covers the branch where
// the source Manifest has an all-zero Platform (no Arch, no OS, no
// Variant). The resulting Descriptor should have Platform == nil —
// not an empty Platform struct — because that's what ORAS / artifact
// pullers expect.
func TestManifestToOCIDescriptor_EmptyPlatform(t *testing.T) {
	m := Manifest{
		Digest:    "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		MediaType: "application/vnd.cncf.oras.artifact.manifest.v1+json",
		Size:      256,
		// Platform deliberately zero-value
	}
	desc := m.ToOCIDescriptor()
	assert.Nil(t, desc.Platform,
		"all-zero Platform should serialise to nil Descriptor.Platform, not an empty Platform struct")
}

// TestManifestToManifestDescriptor exercises the in-house
// ManifestDescriptor projection used by the Index serialisation.
func TestManifestToManifestDescriptor(t *testing.T) {
	m := buildTestManifest()
	md := m.ToManifestDescriptor()
	assert.Equal(t, m.Digest, md.Digest)
	assert.Equal(t, m.MediaType, md.MediaType)
	assert.Equal(t, int(m.Size), md.Size)
	assert.Equal(t, m.Platform.Arch, md.Platform.Architecture)
	assert.Equal(t, m.Platform.OS, md.Platform.OS)
	assert.NotNil(t, md.Platform.Features, "Features must be initialised as an empty slice")
}

// TestOCIManifestToBytes exercises the OCIManifest JSON marshal helper.
func TestOCIManifestToBytes(t *testing.T) {
	om := OCIManifest{
		SchemaVersion: 2,
		MediaType:     imgspecv1.MediaTypeImageManifest,
		Config: imgspecv1.Descriptor{
			Digest:    "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			MediaType: imgspecv1.MediaTypeImageConfig,
			Size:      512,
		},
	}
	b, err := om.ToBytes()
	require.NoError(t, err)
	require.NotEmpty(t, b)

	// Round-trip
	var decoded OCIManifest
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, 2, decoded.SchemaVersion)
	assert.Equal(t, om.MediaType, decoded.MediaType)
	assert.Equal(t, om.Config.Digest, decoded.Config.Digest)
}

// TestManifestToOCIBytes exercises the projection from a metalib
// Manifest (with Config + Layers) into OCI wire-format bytes. Covers
// the Layers loop and the Config projection.
func TestManifestToOCIBytes(t *testing.T) {
	m := buildTestManifest()
	b := m.ToOCIBytes()
	require.NotEmpty(t, b)

	// Parse the output and validate it's a well-formed OCI manifest
	// with the expected layer count and config digest.
	var parsed OCIManifest
	require.NoError(t, json.Unmarshal(b, &parsed))
	assert.Equal(t, 2, parsed.SchemaVersion)
	assert.Equal(t, m.MediaType, parsed.MediaType)
	assert.Equal(t, m.Config.Digest, parsed.Config.Digest.String())
	assert.Len(t, parsed.Layers, 2, "both layers must be in the output")
	assert.Equal(t, m.Layers[0].Digest, parsed.Layers[0].Digest.String())
	assert.Equal(t, m.Layers[1].Digest, parsed.Layers[1].Digest.String())
}

// TestManifestToOCIBytes_NoLayers covers the zero-layer branch (e.g.
// an artifact manifest with only a config). The Layers field must
// serialise as `[]`, not `null`.
func TestManifestToOCIBytes_NoLayers(t *testing.T) {
	m := Manifest{
		MediaType: imgspecv1.MediaTypeImageManifest,
		Config: Config{
			Digest:    "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			MediaType: imgspecv1.MediaTypeImageConfig,
			Size:      128,
		},
		Layers: nil,
	}
	b := m.ToOCIBytes()
	var parsed OCIManifest
	require.NoError(t, json.Unmarshal(b, &parsed))
	assert.NotNil(t, parsed.Layers, "Layers must not be nil in the wire output")
	assert.Len(t, parsed.Layers, 0, "zero-layer manifest must serialise with Layers=[]")
}

// TestManifestWriteWWWHead exercises the HTTP HEAD header writer which
// the pull handler uses to respond to HEAD /v2/<name>/manifests/<ref>.
// Asserts all four required OCI headers are set.
func TestManifestWriteWWWHead(t *testing.T) {
	m := buildTestManifest()
	rec := httptest.NewRecorder()
	err := m.WriteWWWHead(rec)
	require.NoError(t, err)

	h := rec.Header()
	assert.Equal(t, m.MediaType, h.Get("Content-Type"))
	assert.Equal(t, m.Digest, h.Get("Docker-Content-Digest"))
	assert.Equal(t, m.Digest, h.Get("Etag"))
	// Content-Length should match the byte length of m.Data
	assert.Equal(t, "76", h.Get("Content-Length"),
		"Content-Length must reflect len(m.Data) = 76")
}

// TestManifestWriteWWWHead_EmptyMediaType covers the error path where
// the manifest has no mediaType. The caller (pull handler) uses this
// as a signal to emit a 500.
func TestManifestWriteWWWHead_EmptyMediaType(t *testing.T) {
	m := Manifest{
		Digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		// MediaType empty
	}
	rec := httptest.NewRecorder()
	err := m.WriteWWWHead(rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MediaType is empty")
}

// TestManifestWriteWWWHead_WithSubject covers the OCI-Subject header
// path for manifests that reference a subject (e.g. ORAS referrers).
func TestManifestWriteWWWHead_WithSubject(t *testing.T) {
	subjectDigest := "sha256:abababababababababababababababababababababababababababababababab"
	m := buildTestManifest()
	m.Subject = &Manifest{
		Digest: subjectDigest,
	}
	rec := httptest.NewRecorder()
	require.NoError(t, m.WriteWWWHead(rec))
	assert.Equal(t, subjectDigest, rec.Header().Get("OCI-Subject"),
		"OCI-Subject header must be set when Manifest.Subject is non-nil")
}

// TestManifestWriteWWWHeader covers the header-only variant which
// always reports Content-Length: 0 (used for HEAD-like responses
// where the body is not yet available).
func TestManifestWriteWWWHeader(t *testing.T) {
	m := buildTestManifest()
	rec := httptest.NewRecorder()
	require.NoError(t, m.WriteWWWHeader(rec))
	h := rec.Header()
	assert.Equal(t, m.MediaType, h.Get("Content-Type"))
	assert.Equal(t, m.Digest, h.Get("Docker-Content-Digest"))
	assert.Equal(t, m.Digest, h.Get("Etag"))
	assert.Equal(t, "0", h.Get("Content-Length"),
		"WriteWWWHeader (no trailing 'd') emits Content-Length: 0 — it's a header-only variant")
}

func TestManifestWriteWWWHeader_EmptyMediaType(t *testing.T) {
	m := Manifest{
		Digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
	}
	err := m.WriteWWWHeader(httptest.NewRecorder())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MediaType is empty")
}

func TestManifestWriteWWWHeader_WithSubject(t *testing.T) {
	m := buildTestManifest()
	m.Subject = &Manifest{
		Digest: "sha256:abababababababababababababababababababababababababababababababab",
	}
	rec := httptest.NewRecorder()
	require.NoError(t, m.WriteWWWHeader(rec))
	assert.NotEmpty(t, rec.Header().Get("OCI-Subject"))
}

// TestManifestWriteWWW exercises the full-body writer which the pull
// handler uses for GET /v2/<name>/manifests/<ref>. Asserts the body
// bytes are written and the Content-Length reflects the full size.
func TestManifestWriteWWW(t *testing.T) {
	m := buildTestManifest()
	rec := httptest.NewRecorder()
	m.WriteWWW(rec)

	h := rec.Header()
	assert.Equal(t, m.MediaType, h.Get("Content-Type"))
	assert.Equal(t, m.Digest, h.Get("Docker-Content-Digest"))
	assert.Equal(t, m.Digest, h.Get("Etag"))
	assert.Equal(t, "76", h.Get("Content-Length"))
	assert.Equal(t, m.Data, rec.Body.Bytes())
}

// TestManifestWriteWWW_DigestMismatch exercises the branch where the
// Data bytes don't hash to m.Digest. The function logs an error but
// still writes the bytes (the digest mismatch is a soft signal, not
// a hard failure, since upstream content may legitimately be
// pre-computed). This test just asserts the function doesn't panic
// and the body is still written.
func TestManifestWriteWWW_DigestMismatch(t *testing.T) {
	m := Manifest{
		Digest:    "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		MediaType: imgspecv1.MediaTypeImageManifest,
		Data:      []byte(`{"actual":"different content"}`),
	}
	rec := httptest.NewRecorder()
	// Should not panic despite the digest mismatch
	m.WriteWWW(rec)
	assert.Equal(t, m.Data, rec.Body.Bytes(), "body should still be written even on digest mismatch")
}

// Sanity check: the Header()-setter helpers don't leak headers set by
// previous calls (each test uses a fresh httptest.ResponseRecorder so
// this is mostly a guard against accidental shared state).
func TestManifestHeaders_NoLeakage(t *testing.T) {
	m := buildTestManifest()
	rec := httptest.NewRecorder()
	require.NoError(t, m.WriteWWWHead(rec))
	// Check that we don't set random unexpected headers.
	forbidden := []string{"Set-Cookie", "Authorization", "Server"}
	for _, h := range forbidden {
		assert.Empty(t, rec.Header().Get(h), "header %s should not be set by WriteWWWHead", h)
	}
	// Sanity: ResponseWriter is a valid interface
	var _ http.ResponseWriter = rec
}
