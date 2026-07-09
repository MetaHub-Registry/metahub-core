package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestTagIsPerCollection guards the model-centric-pull invariant that a Tag node
// is per-collection: its identity is {name, repo}, NOT the bare tag name. A
// single global `latest` node shared by every collection made
// Repo→HAS_TAG→Tag→REF→Index fan every collection's manifests into every other
// collection's listing ("all manifests under each collection"). Both collections
// are created through the production code path (RepoLinkTag), so this fails on the
// pre-fix shared-Tag code and passes once Tag identity carries the repo.
func TestTagIsPerCollection(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	require.NoError(t, db.RepoLinkTag(ctx, "qnib/colA", "latest"))
	require.NoError(t, db.RepoLinkTag(ctx, "qnib/colB", "latest"))

	ctA, foundA, err := db.CollectionTagGetByTag("qnib/colA", "latest")
	require.NoError(t, err)
	require.True(t, foundA)
	ctB, foundB, err := db.CollectionTagGetByTag("qnib/colB", "latest")
	require.NoError(t, err)
	require.True(t, foundB)

	require.NotEqual(t, ctA.ID, ctB.ID,
		"each collection must own its `latest` Tag node; a shared global tag cross-links collections")

	// Each Repo resolves to exactly one Tag node (its own).
	res, err := db.Query(`MATCH (c:Repo {repo:"qnib/colA"})-[:HAS_TAG]->(t:Tag) RETURN count(t) AS n`)
	require.NoError(t, err)
	require.Len(t, res, 1)
	nA, err := res[0].GetInt64("n")
	require.NoError(t, err)
	require.Equal(t, int64(1), nA, "qnib/colA must HAS_TAG exactly one tag node")
}

// TestCollectionTagRefScopedToOwnIndexes reproduces the read fan-out directly:
// two collections share the tag name `latest`, each with its own compute Index
// linked to its tag (as LinkManifestListToCollectionTag does). Traversing the
// path the collection-manifest read uses (Repo→HAS_TAG→Tag→REF→Index) from
// collection A must reach ONLY A's index. On the pre-fix shared-Tag graph the
// single `latest` node REFs both indexes, so A leaks B's index (count 2) — which
// is exactly why the frontend showed every collection's manifests under each
// collection.
func TestCollectionTagRefScopedToOwnIndexes(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	require.NoError(t, db.RepoLinkTag(ctx, "qnib/colA", "latest"))
	require.NoError(t, db.RepoLinkTag(ctx, "qnib/colB", "latest"))

	_, err = db.Execute(`MATCH (:Repo {repo:"qnib/colA"})-[:HAS_TAG]->(t:Tag {name:"latest", repo:"qnib/colA"})
		CREATE (t)-[:REF]->(:Index {digest:"sha256:idxA", mediaType:"application/vnd.oci.image.index.v1+json", size:1})`)
	require.NoError(t, err)
	_, err = db.Execute(`MATCH (:Repo {repo:"qnib/colB"})-[:HAS_TAG]->(t:Tag {name:"latest", repo:"qnib/colB"})
		CREATE (t)-[:REF]->(:Index {digest:"sha256:idxB", mediaType:"application/vnd.oci.image.index.v1+json", size:1})`)
	require.NoError(t, err)

	res, err := db.Query(`MATCH (c:Repo {repo:"qnib/colA"})-[:HAS_TAG]->(:Tag)-[:REF]->(idx:Index) RETURN count(idx) AS n`)
	require.NoError(t, err)
	require.Len(t, res, 1)
	n, err := res[0].GetInt64("n")
	require.NoError(t, err)
	require.Equal(t, int64(1), n,
		"collection A must REF only its own index via HAS_TAG→REF; a shared `latest` tag leaks other collections' indexes")
}
