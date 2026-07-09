package magazine

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// setupBlobTest stands up an empty magazine test env (no fixtures needed — these
// tests seed blobs directly through the Minio + Database backends).
func setupBlobTest(ctx context.Context, t *testing.T) (utils.Bootstrap, utils.BootstrapResult) {
	t.Helper()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	return bs, br
}

// seedComposedBlob stores buf in Minio and creates its :Blob node (via a
// single-chunk compose), returning its digest — a blob BlobDelete can find on
// the layer path.
func seedComposedBlob(ctx context.Context, t *testing.T, repo string, buf []byte) string {
	t.Helper()
	bu, _ := types.FTarmBlobUploadConfig()
	bu.Repo = repo
	bu.BlobType = "layer"
	bu.Digest = digest.FromBytes(buf).String()
	require.NoError(t, mag.Database.BlobUploadCreateByOCI(ctx, &bu))
	chunk, err := mag.Database.UploadChunkCreate(ctx, &bu, 0, int64(len(buf)))
	require.NoError(t, err)
	require.NoError(t, mag.StoreBlobUpload(ctx, &bu, buf))
	require.NoError(t, mag.ComposeFromChunks(ctx, &bu, []types.BlobChunk{chunk}))
	return bu.Digest
}

// TestBlobDelete_LayerAndNotFound: deleting a stored blob reports found and
// removes it from the graph + Minio; a second delete (and an unknown digest)
// reports not-found without error.
func TestBlobDelete_LayerAndNotFound(t *testing.T) {
	ctx := context.Background()
	bs, br := setupBlobTest(ctx, t)
	defer TeardownTest(bs, br)

	dig := seedComposedBlob(ctx, t, "qnib/del", []byte("delete-me-blob-payload"))

	// Present → deleted.
	found, err := mag.BlobDelete(ctx, types.Request{Repo: "qnib/del", Digest: dig})
	require.NoError(t, err)
	require.True(t, found, "stored blob should be found + deleted")

	// Gone from the graph.
	_, stillThere, err := mag.Database.BlobGetByDigest(ctx, dig)
	require.NoError(t, err)
	require.False(t, stillThere, "blob node should be gone after delete")

	// Gone from Minio.
	_, statErr := mag.Minio.BlobStatObj(ctx, dig)
	require.Error(t, statErr, "blob object should be gone from Minio")

	// Second delete → not found, no error.
	found, err = mag.BlobDelete(ctx, types.Request{Repo: "qnib/del", Digest: dig})
	require.NoError(t, err)
	require.False(t, found)

	// Unknown digest → not found, no error.
	found, err = mag.BlobDelete(ctx, types.Request{Repo: "qnib/del", Digest: "sha256:" + strings.Repeat("a", 64)})
	require.NoError(t, err)
	require.False(t, found)
}

// TestBlobDelete_ConfigPath seeds a config-only node (no :Blob) so BlobDelete
// falls through to the config branch and deletes it.
func TestBlobDelete_ConfigPath(t *testing.T) {
	ctx := context.Background()
	bs, br := setupBlobTest(ctx, t)
	defer TeardownTest(bs, br)

	cfgBytes := []byte(`{"architecture":"amd64","os":"linux"}`)
	cfgDig := digest.FromBytes(cfgBytes).String()
	require.NoError(t, mag.Minio.PutBlob(ctx, cfgBytes, cfgDig))
	_, err := mag.Database.ConfigInsert(ctx, types.Config{
		Digest:    cfgDig,
		MediaType: types.MediaTypeImageConfig,
		Size:      int64(len(cfgBytes)),
	})
	require.NoError(t, err)

	found, err := mag.BlobDelete(ctx, types.Request{Repo: "qnib/cfg", Digest: cfgDig})
	require.NoError(t, err)
	require.True(t, found, "config blob should be found + deleted")

	_, statErr := mag.Minio.BlobStatObj(ctx, cfgDig)
	require.Error(t, statErr, "config object should be gone from Minio")
}

// TestComposeFromChunks_MultiChunk exercises the streaming multi-chunk compose:
// out-of-order chunks are sorted, streamed through chunkSeqReader (open/close one
// at a time) into a single verified blob. Also covers the zero-chunk guard and
// the digest-mismatch rejection (assembled blob removed).
func TestComposeFromChunks_MultiChunk(t *testing.T) {
	ctx := context.Background()
	bs, br := setupBlobTest(ctx, t)
	defer TeardownTest(bs, br)

	c0 := []byte(strings.Repeat("A", 2048))
	c1 := []byte(strings.Repeat("B", 1500))
	full := append(append([]byte{}, c0...), c1...)
	d0 := digest.FromBytes(c0).String()
	d1 := digest.FromBytes(c1).String()
	require.NoError(t, mag.Minio.PutBlob(ctx, c0, d0))
	require.NoError(t, mag.Minio.PutBlob(ctx, c1, d1))

	bu, _ := types.FTarmBlobUploadConfig()
	bu.Repo = "qnib/mc"
	bu.BlobType = "layer"
	bu.Digest = digest.FromBytes(full).String()
	require.NoError(t, mag.Database.BlobUploadCreateByOCI(ctx, &bu))

	// Deliberately out of start_byte order → ComposeFromChunks must sort.
	chunks := []types.BlobChunk{
		{Digest: d1, StartByte: 2048, Bytes: int64(len(full) - 1)},
		{Digest: d0, StartByte: 0, Bytes: 2047},
	}
	require.NoError(t, mag.ComposeFromChunks(ctx, &bu, chunks))

	got, err := mag.Minio.BlobGetBytes(ctx, bu.Digest)
	require.NoError(t, err)
	require.Equal(t, full, got, "composed blob must equal the concatenated chunks in order")

	// Zero chunks → guard error.
	require.Error(t, mag.ComposeFromChunks(ctx, &bu, nil))

	// Digest mismatch → rejected, assembled object removed.
	bad, _ := types.FTarmBlobUploadConfig()
	bad.Repo = "qnib/mc"
	bad.BlobType = "layer"
	bad.Digest = "sha256:" + strings.Repeat("0", 64)
	require.Error(t, mag.ComposeFromChunks(ctx, &bad, chunks))
	_, statErr := mag.Minio.BlobStatObj(ctx, bad.Digest)
	require.Error(t, statErr, "mismatched compose must not leave the object behind")
}

// TestBlobGetByHTTPRequest_Edges covers the not-found (unknown digest) and the
// blob-in-graph-but-missing-in-Minio branches — both must report found=false
// without a hard error.
func TestBlobGetByHTTPRequest_Edges(t *testing.T) {
	ctx := context.Background()
	bs, br := setupBlobTest(ctx, t)
	defer TeardownTest(bs, br)

	// Unknown digest → found=false.
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"repo":      "qnib",
		"image":     "featuretest",
		"reference": "sha256:" + strings.Repeat("b", 64),
	}, map[string]string{}, map[string]string{}, "/{repo}/{image}/blobs/{reference}", nil)
	_, found, err := mag.BlobGetByHTTPRequest(ctx, r)
	require.NoError(t, err)
	require.False(t, found, "unknown digest must report not-found")

	// Blob present in the graph but its object removed from Minio → found=false.
	buf := []byte("blob-with-object-later-removed")
	dig := seedComposedBlob(ctx, t, "qnib", buf)
	require.NoError(t, mag.Minio.BlobRemoveObj(ctx, dig))

	_, r2 := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"repo":      "qnib",
		"image":     "featuretest",
		"reference": dig,
	}, map[string]string{}, map[string]string{}, "/{repo}/{image}/blobs/{reference}", nil)
	blob, found, err := mag.BlobGetByHTTPRequest(ctx, r2)
	require.NoError(t, err)
	require.False(t, found, "blob missing from Minio must report not-found")
	require.NotNil(t, blob, "the graph blob is still returned for the caller's inspection")
}
