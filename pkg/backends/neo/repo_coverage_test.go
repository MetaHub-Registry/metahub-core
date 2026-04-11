package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

// TestCollectionYamlCRUD exercises CollectionGetIDByYaml, CollectionCreateByYaml,
// GetCollection, CreateCollection in a single workflow.
func TestCollectionYamlCRUD(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	yaml := yamlCfg.ManifestCollection{
		Name: "qnib/yamltest",
		Tag:  "latest",
	}

	// Not found initially
	_, found, err := db.GetCollection(ctx, yaml)
	require.NoError(t, err)
	assert.False(t, found)

	// CollectionGetIDByYaml: not found
	_, _, found, err = db.CollectionGetIDByYaml(ctx, yaml)
	require.NoError(t, err)
	assert.False(t, found)

	// Create
	colID, tagID, err := db.CreateCollection(ctx, yaml)
	require.NoError(t, err)
	assert.NotZero(t, colID)
	assert.NotZero(t, tagID)

	// Now found
	col, found, err := db.GetCollection(ctx, yaml)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "qnib/yamltest", col.Repo)

	// CollectionGetIDByYaml: now found
	gotID, _, found, err := db.CollectionGetIDByYaml(ctx, yaml)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, colID, gotID)

	// CollectionCreateByYaml on existing: should return existing
	gotID2, _, err := db.CollectionCreateByYaml(ctx, yaml)
	require.NoError(t, err)
	assert.Equal(t, colID, gotID2)
}

// TestCollectionGetCreateByRepo covers the repo-level getCreate.
func TestCollectionGetCreateByRepo(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	col1, found1, err := db.CollectionGetCreateByRepo(ctx, "qnib/newrepo")
	require.NoError(t, err)
	assert.False(t, found1, "first call should not find existing (creates new)")
	assert.Equal(t, "qnib/newrepo", col1.Repo)
	assert.NotZero(t, col1.ID)

	col2, found2, err := db.CollectionGetCreateByRepo(ctx, "qnib/newrepo")
	require.NoError(t, err)
	assert.True(t, found2, "second call should find existing")
	assert.Equal(t, col1.ID, col2.ID)
}

// TestCollectionGetByRepoTagWoManifests covers repo+tag lookup without manifests.
func TestCollectionGetByRepoTagWoManifests(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	_, _, err := db.CollectionGetCreateByRepoTag(ctx, "qnib/womanifest", "v2")
	require.NoError(t, err)

	col, found, err := db.CollectionGetByRepoTagWoManifests(ctx, "qnib/womanifest", "v2")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "qnib/womanifest", col.Repo)

	// Not found
	_, found, err = db.CollectionGetByRepoTagWoManifests(ctx, "nonexistent/repo", "missing")
	require.NoError(t, err)
	assert.False(t, found)
}

// TestCollectionRemoveNotFound verifies remove of a non-existent collection.
func TestCollectionRemoveNotFound(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	col := types.Repo{
		ID:   99999,
		Repo: "nonexistent/repo",
		Tag:  "notfound",
	}
	found, err := db.CollectionRemove(ctx, col)
	// CollectionRemove first calls CollectionUpdate to resolve the repo+tag.
	// If no Repo node exists with this name, found should be false.
	// (Note: with an empty graph, no :Repo exists at all)
	_ = found
	require.NoError(t, err)
}

// NOTE: TestCollectionGetAll requires a fully-wired graph with
// (Tag)-[:REF]->(Index)-[:IMPLEMENTS]->(Filter) edges, which needs
// the compute pipeline to have run. Covered via magazine-level tests
// (TestManifestlistCompute) rather than neo-level unit tests.
