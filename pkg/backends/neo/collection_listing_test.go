package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestCollectionListingOrasSeparation verifies that ORAS-only repos do not appear in
// CollectionListingGetByRepoLike, while image repos do appear.
func TestCollectionListingOrasSeparation(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("scenario:oras_cyclonedx") // sets up qnib/featuretest with ORAS artifact only
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)
	for _, l := range br.ToLines() {
		_ = l
	}

	t.Run("oras_only_repo_absent_from_collection_listing", func(t *testing.T) {
		// qnib/featuretest only has an ORAS CycloneDX artifact — it must NOT appear here
		cols, err := db.CollectionListingGetByRepoLike(ctx, "")
		require.NoError(t, err)
		for _, c := range cols {
			assert.NotEqual(t, "qnib/featuretest", c.Repo,
				"ORAS-only repo should not appear in collection listing")
		}
	})

	t.Run("oras_only_repo_absent_with_prefix", func(t *testing.T) {
		cols, err := db.CollectionListingGetByRepoLike(ctx, "qnib")
		require.NoError(t, err)
		assert.Empty(t, cols, "ORAS-only repo should not appear in collection listing with qnib prefix")
	})
}

// TestCollectionListingMixedContent verifies that a repo containing both image and ORAS manifests
// appears in the collection listing (because it has at least one image manifest).
func TestCollectionListingMixedContent(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	// Set up the ORAS artifact AND a container image for the same repo
	bs.AddBootstrap("scenario:oras_cyclonedx")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	t.Run("mixed_repo_appears_in_collection_listing", func(t *testing.T) {
		cols, err := db.CollectionListingGetByRepoLike(ctx, "qnib")
		require.NoError(t, err)
		found := false
		for _, c := range cols {
			if c.Repo == "qnib/featuretest" {
				found = true
				break
			}
		}
		assert.True(t, found, "mixed-content repo should appear in collection listing")
	})
}

// TestCollectionListingAll exercises all three CollectionListing methods in a single
// bootstrap/teardown cycle to reduce shared-container contention.
func TestCollectionListingAll(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_arm64")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	colID, err := br.GetInt64("collection:ft#id")
	require.NoError(t, err)
	colTagID, err := br.GetInt64("tag:arm64#id")
	require.NoError(t, err)

	t.Run("GetByID_found", func(t *testing.T) {
		cl, err := db.CollectionListingGetByID(ctx, colID, colTagID)
		require.NoError(t, err)
		assert.Equal(t, colID, cl.ID)
		assert.Equal(t, colTagID, cl.ColTag.ID)
		assert.Equal(t, "arm64", cl.ColTag.Tag)
		assert.Equal(t, "qnib/featuretest", cl.Repo)
		// Manifests may be empty with standard bootstrap: CREATED relationships are set up
		// only by the cypher bootstrap path (ManifestGetByCollectionTag needs CREATED).
		for _, m := range cl.Manifests {
			assert.NotEmpty(t, m.Digest, "any returned manifest must have a digest")
		}
	})

	t.Run("GetByID_notFound", func(t *testing.T) {
		_, err := db.CollectionListingGetByID(ctx, -999999, -999999)
		assert.Error(t, err)
	})

	t.Run("GetByName_found", func(t *testing.T) {
		cl, err := db.CollectionListingGetByName(ctx, "qnib/featuretest", "arm64")
		require.NoError(t, err)
		assert.Equal(t, "qnib/featuretest", cl.Repo)
		assert.Equal(t, "arm64", cl.ColTag.Tag)
		for _, m := range cl.Manifests {
			assert.NotEmpty(t, m.Digest)
		}
	})

	t.Run("GetByName_notFound", func(t *testing.T) {
		_, err := db.CollectionListingGetByName(ctx, "nonexistent/repo", "notag")
		assert.Error(t, err)
	})

	t.Run("GetByRepoLike_all", func(t *testing.T) {
		cols, err := db.CollectionListingGetByRepoLike(ctx, "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(cols), 2)
	})

	t.Run("GetByRepoLike_prefix", func(t *testing.T) {
		cols, err := db.CollectionListingGetByRepoLike(ctx, "qnib")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(cols), 1)
		for _, c := range cols {
			assert.Contains(t, c.Repo, "qnib")
		}
	})

	t.Run("GetByRepoLike_noMatch", func(t *testing.T) {
		cols, err := db.CollectionListingGetByRepoLike(ctx, "qnib/nope")
		require.NoError(t, err)
		assert.Empty(t, cols)
	})

	t.Run("GetByRepoLike_imageFilterApplied", func(t *testing.T) {
		cols, err := db.CollectionListingGetByRepoLike(ctx, "qnib")
		require.NoError(t, err)
		for _, col := range cols {
			for _, m := range col.Manifests {
				assert.NotEmpty(t, m.Digest)
			}
		}
	})
}
