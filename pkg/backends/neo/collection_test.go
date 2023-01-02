package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func Test_Collection_CRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer0:ft_arm64")
	bs.AddBootstrap("layer1:ft_arm64")
	bs.AddBootstrap("layer0:ft_amd64")
	bs.AddBootstrap("layer1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifestlist:ft_generic")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	col1 := yamlCfg.ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "latest",
	}
	// Get Collection
	_, found, err := db.GetCollection(col1)
	assert.NoError(t, err)
	assert.False(t, found)
	// Add Collection
	col1_id, colTag1_ID, err := db.CreateCollection(col1)
	_ = colTag1_ID
	assert.NoError(t, err)
	assert.NotNil(t, col1_id)
	col1_got, found, err := db.GetCollection(col1)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, col1_id, col1_got.ID)
	/// LINKS
	/// To Manifest
	man1_id, err := br.GetInt64("manifest:ft_arm64#id")
	assert.NoError(t, err)
	err = db.LinkCollectionTagToManifest(ctx, col1_got.ColTag.ID, man1_id)
	assert.NoError(t, err)
	/// To Manifest
	mani_amd_id, err := br.GetInt64("manifest:ft_amd64#id")
	assert.NoError(t, err)
	err = db.LinkCollectionTagToManifest(ctx, col1_got.ColTag.ID, mani_amd_id)
	assert.NoError(t, err)
}

func Test_Collection_ComputeML(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer0:ft_arm64")
	bs.AddBootstrap("layer1:ft_arm64")
	bs.AddBootstrap("layer0:ft_amd64")
	bs.AddBootstrap("layer1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	err = db.ComputeManifestlists()
	assert.NoError(t, err)
	cols, err := db.GetAllCollections()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cols))
	col, found, err := db.Collection_Get_ByRepoTag("qnib/featuretest", "generic")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "qnib/featuretest", col.Repo)

}

func Test_Collection_CRUD2(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	col1, found, err := db.Collection_GetCreate_ByRepo("qnib/featuretest")
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, "qnib/featuretest", col1.Repo)
	col2, found, err := db.Collection_GetCreate_ByRepo("qnib/featuretest")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, col1.ID, col2.ID)
	//
	_, found, err = db.CollectionTag_GetByTag("latest")
	assert.NoError(t, err)
	assert.False(t, found)
	col3, found, err := db.Collection_GetCreate_ByRepoTag(ctx, "qnib/featuretest", "latest")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, col1.ID, col3.ID)
	assert.Equal(t, "latest", col3.Tag)
	assert.Equal(t, "latest", col3.ColTag.Tag)
}

func Test_Collection_ByRequest(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	req := types.Request{
		Repo: "qnib/featuretest",
		Tag:  "tag1",
	}
	////// By Upstream
	mani := types.FT_amd_Manifest()
	err = db.CollectionManifest_ByRequest(req, mani)
	assert.NoError(t, err)
	/// By DIGEST
	m, found, err := db.Manifest_Get_ByDigest(mani.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "qnib-pub-containers/qnib/plain-featuretest", m.Upstream.Repo)
	// More tests
}

func Test_Collection_ByRequest_MetahubUpstream(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	reqCol := types.Request{
		Host:    "metahub-registry.org",
		Repo:    "qnib/featuretest",
		Tag:     "tag1",
		Profile: "generic",
	}
	reqMani := types.Request{
		Host:    "metahub-registry.org",
		Repo:    "qnib/featuretest",
		Tag:     "tag1-linux.amd64",
		Profile: "generic",
	}
	////// With Metahub Upstream
	mani := types.FT_amd_Manifest()
	up := types.Upstream{
		Reg:  "metahub-registry.org",
		Repo: "qnib/featuretest",
		Tag:  "tag1-linux.amd64",
	}
	mani.Upstream = up
	err = db.CollectionManifest_ByRequest(reqCol, mani)
	assert.NoError(t, err)
	/// By DIGEST
	m, found, err := db.Manifest_Get_ByDigest(mani.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "qnib/featuretest", m.Upstream.Repo)
	/// Via Upstream
	upMani1, found, err := db.Manifest_Get_ByUpstream(up)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, mani.Digest, upMani1.Digest)
	assert.Equal(t, "qnib/featuretest", upMani1.Upstream.Repo)
	// More tests
	reqMani1, found, err := db.Manifest_GetTypes_ByRequest(reqMani)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "qnib/featuretest", reqMani1.Upstream.Repo)
	/// Second Manifest
	maniArm := types.FT_arm_Manifest()
	maniArm.Upstream = up
	err = db.CollectionManifest_ByRequest(reqCol, maniArm)
	assert.NoError(t, err)
	// ManifestList?
	ml, found, err := db.ManifestList_Get_ByRequest(reqCol)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 2, len(ml.Manifests))

}

func Test_Collection_GetCollectionFilterPairs_ForCollection(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("cypher:collection:ft_tag1")
	bs.AddBootstrap("cypher:collection_only:ft_generic")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual Test
	//// Generic
	col := types.Collection{
		Repo: "qnib/featuretest",
		Tag:  "tag1",
		ColTag: types.CollectionTag{
			Tag: "tag1",
		},
	}
	cfps, err := db.GetCollectionFilterPairs_ForCollection(col)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cfps), cfps)
	assert.Equal(t, 2, len(cfps[0].Manifests))
	// Compute Manifestlist
	err = db.computeManifestlists(cfps)
	assert.NoError(t, err)
	/// Check ManifestList
	req := types.Request{
		Repo:    "qnib/featuretest",
		Tag:     "tag1",
		Profile: "generic",
	}
	ml, found, err := db.ManifestList_Get_ByRequest(req)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 2, len(ml.Manifests))

}
