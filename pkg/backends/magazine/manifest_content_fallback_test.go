package magazine

import (
	"context"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestEnsureBlobNodeResolvable covers the content-addressed manifest-link fallback
// that fixes the ORAS artifact-push 400 ("config blob not found").
//
// Scenario: the HEAD content-addressed fallback answers a client's blob HEAD 200
// from the surviving object even after the e2e --empty-graph reseed DETACH-DELETEd
// the :Blob node, so the client skips re-uploading and no node is (re)created. The
// subsequent manifest-link (ManifestLinkToArtifactConfig / ManifestLinkToConfig)
// MATCHes (b:Blob {digest}); without this fallback it 400s. Cases:
//  1. object present, :Blob node wiped -> node re-materialized from the bucket;
//  2. OCI empty-config sentinel, no prior object -> object + node materialized;
//  3. genuinely absent (no node, no object, not the sentinel) -> error preserved.
func TestEnsureBlobNodeResolvable(t *testing.T) {
	ctx := context.Background()
	l, c, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestEnsureBlobNodeResolvable",
	})
	ctx = c
	defer span.End()
	bs, br := setupBlobTest(ctx, t)
	defer TeardownTest(bs, br)

	// 1. Object present in the bucket, but no :Blob graph node (the wipe scenario).
	content := []byte("hello-artifact-config")
	dig := digest.FromBytes(content).String()
	require.NoError(t, mag.Minio.PutBlob(ctx, content, dig))
	_ = mag.Database.BlobDeleteByDigest(ctx, dig) // ensure node absent (object survives)
	if _, found, err := mag.Database.BlobGetByDigest(ctx, dig); err == nil {
		require.False(t, found, "precondition: :Blob node must be absent")
	}
	require.NoError(t, mag.ensureBlobNodeResolvable(ctx, l, span, dig, "application/octet-stream"))
	b, found, err := mag.Database.BlobGetByDigest(ctx, dig)
	require.NoError(t, err)
	require.True(t, found, "node must be re-materialized from the surviving object")
	require.Equal(t, int64(len(content)), b.Size, "materialized node size comes from the object stat")

	// 2. OCI empty-config sentinel with NO prior object: server materializes both.
	_ = mag.Minio.BlobRemoveObj(ctx, types.EmptyConfigDigest)
	_ = mag.Database.BlobDeleteByDigest(ctx, types.EmptyConfigDigest)
	require.NoError(t, mag.ensureBlobNodeResolvable(ctx, l, span, types.EmptyConfigDigest, types.MediaTypeOCIEmpty))
	_, statErr := mag.Minio.BlobStatObj(ctx, types.EmptyConfigDigest)
	require.NoError(t, statErr, "empty-config object must be materialized without any client upload")
	eb, found, err := mag.Database.BlobGetByDigest(ctx, types.EmptyConfigDigest)
	require.NoError(t, err)
	require.True(t, found, "empty-config :Blob node must be materialized")
	require.Equal(t, int64(len(types.EmptyConfigBytes)), eb.Size)

	// 3. Genuinely absent: no node, no object, not the empty sentinel -> keep 400.
	absent := "sha256:" + strings.Repeat("0", 64)
	require.Error(t, mag.ensureBlobNodeResolvable(ctx, l, span, absent, "application/octet-stream"),
		"a truly missing blob must still be rejected (real MANIFEST_INVALID)")
}
