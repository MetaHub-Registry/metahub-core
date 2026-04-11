package neo

import (
	"context"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestManifestGetRawByRepoTag_BootstrappedFtArm64 verifies that the raw
// helper returns the manifestdata bootstrapped via the standard
// `upstream:ft_arm64` + `manifestdata:ft_arm64` chain by walking via the
// `(Upstream {repo, tag})-[:ORIGINAL]->(Manifest)` edge.
//
// The ft_arm64 fixture creates an Upstream with
//
//	repo = "qnib-pub-containers/qnib/plain-featuretest"
//	tag  = "3.17.3-arm64"
//
// (NOT the Repo's "qnib/featuretest:arm64" — Upstream carries
// provenance, Repo carries user-facing identity). The raw helper
// queries via Upstream, so the test uses the Upstream's repo/tag values.
//
// The contrast with ManifestGetByCollectionTag (which returns empty
// for the same fixture because no Profile is linked yet) is preserved as
// a sanity check in a separate call site.
func TestManifestGetRawByRepoTag_BootstrappedFtArm64(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("empty:all")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	wantMdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	// Upstream-side coordinates — these are what the raw helper queries on.
	const upstreamRepo = "qnib-pub-containers/qnib/plain-featuretest"
	const upstreamTag = "3.17.3-arm64"

	mds, err := db.ManifestGetRawByRepoTag(ctx, upstreamRepo, upstreamTag)
	require.NoError(t, err)
	require.Len(t, mds, 1, "raw lookup should return the bootstrapped manifestdata")
	assert.Equal(t, wantMdID, mds[0].ID)
	assert.NotEmpty(t, mds[0].Digest, "manifestdata must have a digest")
	assert.Equal(t, "ORIGINAL", mds[0].Type, "raw helper reports the Upstream [:ORIGINAL] relationship type")
}

// TestManifestGetRawByRepoTag_UnknownRepoTagReturnsEmpty verifies the
// helper returns an empty slice (and no error) when (repo, tag) does not
// match any Upstream node in the graph. The caller is responsible for
// treating empty as "nothing to compute".
func TestManifestGetRawByRepoTag_UnknownRepoTagReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	mds, err := db.ManifestGetRawByRepoTag(ctx, "does/not/exist", "nope")
	require.NoError(t, err)
	assert.Empty(t, mds, "unknown (repo, tag) must return an empty slice")
}

// TestManifestGetRawByRepoTag_MultipleUpstreamsSameRepoTag verifies that
// when two Manifest have been pushed to the same (repo, tag) — i.e.
// two Upstream nodes share the compound key, each linked from its own
// Manifest via (md)-[:ORIGINAL]->(u) — the helper returns BOTH. This
// is the multi-arch aggregation case: each push creates its own Upstream,
// so the per-(repo, tag) walk preserves the full push history regardless
// of which :ManifestWrapper wrapper they currently hang off.
func TestManifestGetRawByRepoTag_MultipleUpstreamsSameRepoTag(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("empty:all")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	firstMdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	// Create a SECOND Upstream + Manifest pair with the same
	// (repo, tag) but a different digest, simulating a second push to
	// the same tag (e.g. the amd64 variant after the arm64 one).
	const sharedRepo = "qnib-pub-containers/qnib/plain-featuretest"
	const sharedTag = "3.17.3-arm64"
	const fakeDigest2 = "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

	// Directly insert a second (Upstream)-[:ORIGINAL]->(Manifest)
	// pair via cypher. We don't need a full manifestdata shape for the
	// raw helper — just enough fields to satisfy manifestDataReturn.
	session := db.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	createQ := `CREATE (md:Manifest {digest: $digest, mediaType: "application/vnd.oci.image.manifest.v1+json", size: 0, label: "test:second"})
	            CREATE (u:Upstream {reg: "registry:5050", repo: $repo, tag: $tag, digest: $digest, label: "second"})
	            CREATE (md)-[:ORIGINAL]->(u)
	            RETURN id(md) AS id`
	params := map[string]interface{}{
		"digest": fakeDigest2,
		"repo":   sharedRepo,
		"tag":    sharedTag,
	}
	createRes, err := session.Run(createQ, params)
	require.NoError(t, err)
	require.True(t, createRes.Next(), "CREATE must return an id")
	secondMdID, _ := createRes.Record().Get("id")

	mds, err := db.ManifestGetRawByRepoTag(ctx, sharedRepo, sharedTag)
	require.NoError(t, err)
	require.Len(t, mds, 2, "raw lookup should return BOTH manifestdata for the same (repo, tag)")

	ids := []int64{mds[0].ID, mds[1].ID}
	assert.Contains(t, ids, firstMdID, "original ft_arm64 manifestdata must be in result")
	assert.Contains(t, ids, secondMdID.(int64), "second manifestdata (same repo+tag, different digest) must be in result")
}
