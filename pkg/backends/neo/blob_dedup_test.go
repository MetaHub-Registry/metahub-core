package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// bootstrapDedup spins up the minimal fixture the dedup tests share.
func bootstrapDedup(t *testing.T) (utils.Bootstrap, utils.BootstrapResult) {
	t.Helper()
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	return bs, br
}

// TestCreateBlobFromChunk_IdempotentOnDigest is the regression guard for the
// duplicate-:Blob-node bug: pushing identical content twice (e.g. the same
// model-layer image seeded under two profiles) must converge on ONE
// content-addressed :Blob node. Before the MERGE-on-digest fix each push did a
// bare CREATE, minting a second node whose digest then fanned out HAS_LAYER
// edges during compose → "number of layers and diffIDs don't match".
func TestCreateBlobFromChunk_IdempotentOnDigest(t *testing.T) {
	ctx := context.Background()
	bs, br := bootstrapDedup(t)
	defer db.TeardownTest(bs, br)

	dig := "sha256:dedupe00000000000000000000000000000000000000000000000000000001"
	// Two independent uploads of byte-identical content.
	for i := 0; i < 2; i++ {
		bu := types.OCIBlobUpload{
			UUID: uuid.New(), Repo: "qnib/dedupe", UploadUser: "test",
			UploadProfile: "generic", BlobType: "layer", Digest: dig,
		}
		require.NoError(t, db.BlobUploadCreateByOCI(ctx, &bu))
		require.NoError(t, db.CreateBlobFromChunk(ctx, &bu, types.BlobChunk{Digest: dig, Bytes: 1024}))
	}

	res, _, err := db.QueryOne(fmt.Sprintf(`MATCH (b:Blob {digest: "%s"}) RETURN count(b) AS n`, dig))
	require.NoError(t, err)
	n, err := res.GetInt64("n")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n,
		"re-pushing an identical digest must MERGE onto one :Blob node, not create a duplicate")
}

// TestManifestLinkToLayer_NoFanOutOnDuplicateBlobs guards the compose-time half
// of the fix: even if a graph STILL carries duplicate :Blob nodes for one digest
// (seeded before the CreateBlobFromChunk MERGE fix), ManifestLinkToLayer must
// bind exactly ONE canonical node, creating a single HAS_LAYER edge. Without the
// `ORDER BY id(l) LIMIT 1` guard the unconstrained match fanned out to every
// copy, inflating the manifest's layer count above its config DiffIDs.
func TestManifestLinkToLayer_NoFanOutOnDuplicateBlobs(t *testing.T) {
	ctx := context.Background()
	bs, br := bootstrapDedup(t)
	defer db.TeardownTest(bs, br)

	dig := "sha256:fan0ut000000000000000000000000000000000000000000000000000002"
	mt := "application/vnd.oci.image.layer.v1.tar"
	// Simulate two legacy duplicate :Blob nodes for the same digest.
	for i := 0; i < 2; i++ {
		_, err := db.Execute(fmt.Sprintf(
			`CREATE (:Blob {digest: "%s", size: 10, label: "dup", type: "layer", mediaType: "%s"})`, dig, mt))
		require.NoError(t, err)
	}
	res, err := db.Execute(
		`CREATE (m:Manifest {digest: "sha256:manifanout", mediaType: "application/vnd.oci.image.manifest.v1+json"}) RETURN id(m) AS ID`)
	require.NoError(t, err)
	maniID, err := res.GetInt64("ID")
	require.NoError(t, err)

	layer := types.Layer{Digest: dig, MediaType: mt, Size: 10}
	require.NoError(t, db.ManifestLinkToLayer(ctx, maniID, layer, 0))

	res, _, err = db.QueryOne(fmt.Sprintf(
		`MATCH (m:Manifest)-[r:HAS_LAYER]->(:Blob) WHERE id(m) = %d RETURN count(r) AS n`, maniID))
	require.NoError(t, err)
	n, err := res.GetInt64("n")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n,
		"ManifestLinkToLayer must create exactly one HAS_LAYER edge even with duplicate :Blob nodes sharing the digest")
}
