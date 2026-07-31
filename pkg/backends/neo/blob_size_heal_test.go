package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestBlobUpdateSizeByDigest_GrowOnlyHealsZero guards the storage-quota
// under-count fix: a :Blob node minted as a size-0 placeholder by
// BlobEnsureByDigest (client deduped the upload, "Layer already exists") must be
// healed to the authoritative descriptor size, and a real size must never be
// shrunk. Before the fix BlobUpdateSizeByDigest guarded on `size < 0`, so a
// size-0 node stayed 0 and ProjectUsageBytes summed zeros.
func TestBlobUpdateSizeByDigest_GrowOnlyHealsZero(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	const dig = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

	// Placeholder minted with size 0 (the HEAD-fallback dedup case).
	require.NoError(t, db.BlobEnsureByDigest(ctx, dig, 0))
	b, found, err := db.BlobGetByDigest(ctx, dig)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int64(0), b.Size, "placeholder starts at size 0")

	// Authoritative descriptor size heals the 0.
	require.NoError(t, db.BlobUpdateSizeByDigest(ctx, dig, 500))
	b, _, err = db.BlobGetByDigest(ctx, dig)
	require.NoError(t, err)
	assert.Equal(t, int64(500), b.Size, "size-0 node healed to descriptor size")

	// A subsequent unknown (0) size must not shrink the real value.
	require.NoError(t, db.BlobUpdateSizeByDigest(ctx, dig, 0))
	b, _, err = db.BlobGetByDigest(ctx, dig)
	require.NoError(t, err)
	assert.Equal(t, int64(500), b.Size, "grow-only: size 0 is a no-op, never shrinks")

	// Grows if a larger authoritative size ever arrives (idempotent otherwise).
	require.NoError(t, db.BlobUpdateSizeByDigest(ctx, dig, 900))
	b, _, err = db.BlobGetByDigest(ctx, dig)
	require.NoError(t, err)
	assert.Equal(t, int64(900), b.Size)
}
