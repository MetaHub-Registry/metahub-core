package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestManifestGetByCollectionTag_SlotCentred verifies that ManifestGetByCollectionTag
// returns manifests via the slot-centred graph path (Repo -> Tag -> Index -> Manifest)
// when no legacy ManifestWrapper/UPSTREAM relationships exist. It also verifies that
// ContentSize is computed as the sum of config + layer blob sizes.
func TestManifestGetByCollectionTag_SlotCentred(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// Build the slot-centred graph directly via Cypher:
	//   (Repo)-[:HAS_TAG]->(Tag)-[:REF]->(Index)-[:REFERENCES]->(Manifest)
	//   (Manifest)-[:HAS_PLATFORM]->(Platform)
	//   (Manifest)-[:oci_image_config_v1JSON]->(Blob{type:"config"})
	//   (Manifest)-[:USES]->(Blob{type:"layer"})
	q := `
		CREATE (c:Repo {repo: "test/slot-image"})
		CREATE (t:Tag {name: "v1"})
		CREATE (i:Index {digest: "sha256:idx0001", mediaType: "application/vnd.oci.image.index.v1+json", size: 100, label: "generic"})
		CREATE (md:Manifest {digest: "sha256:mani0001", mediaType: "application/vnd.oci.image.manifest.v1+json", size: 200, label: "test/slot-image:v1-amd64"})
		CREATE (p:Platform {arch: "amd64", os: "linux", variant: ""})
		CREATE (cfg:Blob {type: "config", digest: "sha256:cfg0001", mediaType: "application/vnd.oci.image.config.v1+json", size: 611})
		CREATE (lyr:Blob {type: "layer", digest: "sha256:lyr0001", mediaType: "application/vnd.oci.image.layer.v1.tar+gzip", size: 3643742})
		CREATE (c)-[:HAS_TAG]->(t)
		CREATE (t)-[:REF]->(i)
		CREATE (i)-[:REFERENCES]->(md)
		CREATE (md)-[:HAS_PLATFORM]->(p)
		CREATE (md)-[:oci_image_config_v1JSON]->(cfg)
		CREATE (md)-[:USES]->(lyr)
		RETURN id(c) AS repoID, id(t) AS tagID
	`
	res, err := db.Execute(q)
	require.NoError(t, err)
	colID, err := res.GetInt64("repoID")
	require.NoError(t, err)
	colTagID, err := res.GetInt64("tagID")
	require.NoError(t, err)

	t.Run("slot_path_returns_manifests", func(t *testing.T) {
		manis, err := db.ManifestGetByCollectionTag(ctx, colID, colTagID)
		require.NoError(t, err)
		assert.Len(t, manis, 1, "expected 1 manifest via slot-centred path")
		if len(manis) > 0 {
			m := manis[0]
			assert.Equal(t, "sha256:mani0001", m.Digest)
			assert.Equal(t, "amd64", m.Platform.Arch)
			assert.Equal(t, "linux", m.Platform.OS)
			assert.Equal(t, "sha256:cfg0001", m.Config.Digest)
			// Upstream is synthesized from Repo/Tag metadata
			assert.Equal(t, "test/slot-image", m.Upstream.Repo)
			assert.Equal(t, "v1", m.Upstream.Tag)
			// ContentSize = config (611) + layer (3643742) = 3644353
			assert.Equal(t, int64(3644353), m.ContentSize, "ContentSize should be sum of config + layer blob sizes")
		}
	})
}
