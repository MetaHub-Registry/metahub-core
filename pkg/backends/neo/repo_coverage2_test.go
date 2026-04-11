package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestCollectionGetByRepoTag covers the full collection-with-manifests lookup path.
func TestCollectionGetByRepoTag(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// The collection exists but may not have Index/Filter edges in this setup.
	// The path requires (Tag)-[:REF]->(Index)-[:IMPLEMENTS]->(Filter) which
	// only exists after the compute pipeline. This test exercises the session.Run
	// path and verifies no panics on an empty result set.
	_, found, err := db.CollectionGetByRepoTag(ctx, "qnib/featuretest", "arm64")
	require.NoError(t, err)
	// found may be false since the compute pipeline hasn't run, but we
	// exercise the code path.
	t.Logf("CollectionGetByRepoTag found=%v", found)

	// Not found case
	_, found, err = db.CollectionGetByRepoTag(ctx, "nonexistent/repo", "missing")
	require.NoError(t, err)
	assert.False(t, found)

	_ = br
}

// TestCollectionGetAll covers GetAllCollections / CollectionGetAll.
func TestCollectionGetAll(t *testing.T) {
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

	// Empty graph — should return empty, no errors
	cols, err := db.CollectionGetAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, cols)
}

// TestCollectionCreateByRequest covers CollectionCreateByRequest.
func TestCollectionCreateByRequest(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	req := types.Request{
		Repo: "qnib/reqtest",
		Tag:  "v1",
	}
	col, err := db.CollectionCreateByRequest(req)
	require.NoError(t, err)
	assert.Equal(t, "qnib/reqtest", col.Repo)
	assert.NotZero(t, col.ID)
}

// TestCollectionGetInsertByRequest covers the get-or-insert by request.
func TestCollectionGetInsertByRequest(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	req := types.Request{
		Repo: "qnib/getinsert",
		Tag:  "latest",
	}
	// First call creates
	col1, err := db.CollectionGetInsertByRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "qnib/getinsert", col1.Repo)

	// Second call gets existing (the query path won't find Index edges,
	// so !found triggers the create-by-request branch which is idempotent)
	col2, err := db.CollectionGetInsertByRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, col1.Repo, col2.Repo)
}

// TestCollectionListingFromResult covers the CollectionListingFromResult helper.
func TestCollectionListingFromResult(t *testing.T) {
	// Build a synthetic Result with the keys the function expects.
	res := types.NewResult()
	res.Add("tagID", int64(42))
	res.Add("tagName", "latest")
	res.Add("collectionID", int64(7))
	res.Add("collectionRepo", "qnib/listing")

	col, err := CollectionListingFromResult(res)
	require.NoError(t, err)
	assert.Equal(t, int64(7), col.ID)
	assert.Equal(t, "qnib/listing", col.Repo)
	assert.Equal(t, "latest", col.Tag)
	assert.Equal(t, int64(42), col.ColTag.ID)

	// Missing key should fail
	resBad := types.NewResult()
	resBad.Add("tagID", int64(1))
	_, err = CollectionListingFromResult(resBad)
	assert.Error(t, err)
}

// TestCollectionGetIDByRepo covers found and not-found.
func TestCollectionGetIDByRepo(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// Not found
	_, found, err := db.CollectionGetIDByRepo(ctx, "nonexistent/repo")
	require.NoError(t, err)
	assert.False(t, found)

	// Create then find
	_, _, err = db.CollectionGetCreateByRepoTag(ctx, "qnib/idtest", "v1")
	require.NoError(t, err)

	colID, found, err := db.CollectionGetIDByRepo(ctx, "qnib/idtest")
	require.NoError(t, err)
	assert.True(t, found)
	assert.GreaterOrEqual(t, colID, int64(0))
}
