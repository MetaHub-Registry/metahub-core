package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func TestCollectionCRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
	//br.PrintDict()
	if err != nil {
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	col1 := yamlCfg.ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "latest",
	}
	// Get Collection
	_, found, err := db.GetCollection(ctx, col1)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	// Add Collection
	col1_id, colTag1_ID, err := db.CreateCollection(ctx, col1)
	_ = colTag1_ID
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, col1_id)
	col1_got, found, err := db.GetCollection(ctx, col1)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, col1_id, col1_got.ID)
	/// LINKS
	/// To Manifest
	man1_id, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	err = db.LinkCollectionTagToManifest(ctx, col1_got.ColTag.ID, man1_id)
	if err != nil {
		t.Fatal(err)
	}
	err = db.LinkCollectionToManifest(ctx, col1_got.ID, man1_id)
	if err != nil {
		t.Fatal(err)
	}
	/// To Manifest
	mani_amd_id, err := br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	err = db.LinkCollectionTagToManifest(ctx, col1_got.ColTag.ID, mani_amd_id)
	if err != nil {
		t.Fatal(err)
	}
	err = db.LinkCollectionToManifest(ctx, col1_got.ID, mani_amd_id)
	if err != nil {
		t.Fatal(err)
	}
}

/*
func TestCollectionComputeManifest(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("filter:platform-generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		t.Fatal(err)
}
	defer db.TeardownTest(bs)
	// Actual test
}

/*
func TestCollectionComputeML(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
}
	defer db.TeardownTest(bs)
	// Actual test
	err = db.ComputeManifestlists()
	if err != nil {
		t.Fatal(err)
}
	cols, err := db.GetAllCollections()
	if err != nil {
		t.Fatal(err)
}
	assert.Equal(t, 1, len(cols))
	col, found, err := db.Collection_Get_ByRepoTag("qnib/featuretest", "generic")
	if err != nil {
		t.Fatal(err)
}
	assert.True(t, found)
	assert.Equal(t, "qnib/featuretest", col.Repo)

}

func TestCollectionByRequest(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
}
	defer db.TeardownTest(bs)
	// Actual test
	req := types.Request{
		Repo: "qnib/featuretest",
		Tag:  "tag1",
	}
	////// By Upstream
	mani := types.FT_amd_ManifestData()
	err = db.CollectionManifest_ByRequest(req, mani)
	if err != nil {
		t.Fatal(err)
}
	/// By DIGEST
	m, found, err := db.ManifestData_Get_ByDigest(mani.Digest)
	if err != nil {
		t.Fatal(err)
}
	assert.True(t, found)
	assert.Equal(t, "qnib-pub-containers/qnib/plain-featuretest", m.Upstream.Repo)
	// More tests
}

func TestCollectionByRequestMetahubUpstream(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
}
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
	mani := types.FT_amd_ManifestData()
	up := types.Upstream{
		Reg:  "metahub-registry.org",
		Repo: "qnib/featuretest",
		Tag:  "tag1-linux.amd64",
	}
	mani.Upstream = up
	err = db.CollectionManifest_ByRequest(reqCol, mani)
	if err != nil {
		t.Fatal(err)
}
	/// By DIGEST
	m, found, err := db.ManifestData_Get_ByDigest(mani.Digest)
	if err != nil {
		t.Fatal(err)
}
	assert.True(t, found)
	assert.Equal(t, "qnib/featuretest", m.Upstream.Repo)
	/// Via Upstream
	upMani1, found, err := db.ManifestData_Get_ByUpstream(up)
	if err != nil {
		t.Fatal(err)
}
	assert.True(t, found)
	assert.Equal(t, mani.Digest, upMani1.Digest)
	assert.Equal(t, "qnib/featuretest", upMani1.Upstream.Repo)
	// More tests
	reqMani1, found, err := db.ManifestData_GetTypes_ByRequest(reqMani)
	if err != nil {
		t.Fatal(err)
}
	assert.True(t, found)
	assert.Equal(t, "qnib/featuretest", reqMani1.Upstream.Repo)
	/// Second Manifest
	maniArm := types.FT_arm_ManifestData()
	maniArm.Upstream = up
	err = db.CollectionManifest_ByRequest(reqCol, maniArm)
	if err != nil {
		t.Fatal(err)
}
	// ManifestList?
	ml, found, err := db.ManifestList_Get_ByRequest(reqCol)
	if err != nil {
		t.Fatal(err)
}
	assert.True(t, found)
	assert.Equal(t, 2, len(ml.Manifests))

}
*/
/*
func TestCollectionGetCollectionFilterPairsForCollection(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("cypher:collection:ft_tag1")
	bs.AddBootstrap("cypher:collection_only:ft_generic")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
}
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
	if err != nil {
		t.Fatal(err)
}
	assert.Equal(t, 1, len(cfps), cfps)
	assert.Equal(t, 2, len(cfps[0].Manifests))
	// Compute Manifestlist
	err = db.computeManifestlists(cfps)
	if err != nil {
		t.Fatal(err)
}
	/// Check ManifestList
	req := types.Request{
		Repo:    "qnib/featuretest",
		Tag:     "tag1",
		Profile: "generic",
	}
	ml, found, err := db.ManifestList_Get_ByRequest(req)
	if err != nil {
		t.Fatal(err)
}
	assert.True(t, found)
	assert.Equal(t, 2, len(ml.Manifests))

}
*/
