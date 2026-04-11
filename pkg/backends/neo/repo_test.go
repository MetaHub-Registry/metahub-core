package neo

import (
	"context"
	"testing"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func TestCollectionCRUD(t *testing.T) {
	ctx := context.Background()
	debug := 0
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
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	col1 := yamlCfg.ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "latest",
	}
	// Get Repo
	_, found, err := db.GetCollection(ctx, col1)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	// Add Repo
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
	// Remove
	ups := types.Upstream{
		Repo: "qnib/featuretest",
		Tag:  "latest",
	}
	col, found, e := db.CollectionGetByRepo(ctx, ups.Repo)
	if e != nil {
		t.Fatal(e)
	}
	assert.True(t, found)
	assert.Equal(t, ups.Repo, col.Repo)
	col.Tag = ups.Tag
	found, e = db.CollectionUpdate(ctx, &col)
	if e != nil {
		t.Fatal(e)
	}
	assert.True(t, found)
	assert.Equal(t, ups.Tag, col.ColTag.Tag)
	// CollectionExists
	upt := upstream.UpstreamTriple{
		Reg:  ups.Reg,
		Repo: ups.Repo,
		Tag:  ups.Tag,
	}
	found, e = db.CollectionExists(ctx, upt)
	if e != nil {
		t.Fatal(e)
	}
	assert.True(t, found)
	found, err = db.CollectionRemove(ctx, col)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
}

func TestCollectionGetCreateUpstreamTriple(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	ups := types.Upstream{
		Reg:     "localhost:5000",
		Repo:    "qnib/test",
		Tag:     "latest",
		Digest:  "sha256:1234567890",
		Profile: "generic",
	}
	// Repo Does not exist
	upt := upstream.UpstreamTriple{
		Reg:  ups.Reg,
		Repo: ups.Repo,
		Tag:  ups.Tag,
	}
	found, e := db.CollectionExists(ctx, upt)
	if e != nil {
		t.Fatal(e)
	}
	assert.False(t, found)
	///
	_, found, e = db.CollectionGetByRepo(ctx, ups.Repo)
	if e != nil {
		t.Fatal(e)
	}
	assert.False(t, found)
	maniIn := manifestlist.ManifestDescriptor{
		Platform: manifestlist.PlatformSpec{
			Architecture: "amd64",
			OS:           "linux",
			Variant:      "",
		},
	}
	upID, _, prf, e := db.CollectionGetCreateUpstreamTriple(ctx, ups, maniIn)
	if e != nil {
		t.Error(e)
		return
	}
	// Test Upstream
	gotUpID, upFound, err := db.UpstreamGet(ctx, ups)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, upFound)
	assert.Equal(t, upID, gotUpID)
	expPrfId, _ := br.GetInt64("profile:generic#id")
	assert.Equal(t, expPrfId, prf.ID)
	// Test Repo
	cCreate, found, e := db.CollectionGetCreateByRepoTag(ctx, ups.Repo, ups.Tag)
	if e != nil {
		t.Fatal(e)
	}
	assert.False(t, found)
	assert.Equal(t, ups.Repo, cCreate.Repo)
	col, found, e := db.CollectionGetByRepo(ctx, ups.Repo)
	if e != nil {
		t.Fatal(e)
	}
	assert.True(t, found)
	assert.Equal(t, ups.Repo, col.Repo)
	col.Tag = ups.Tag
	found, e = db.CollectionUpdate(ctx, &col)
	if e != nil {
		t.Fatal(e)
	}
	assert.True(t, found)
	assert.Equal(t, ups.Tag, col.ColTag.Tag)
	// CollectionExists
	found, e = db.CollectionExists(ctx, upt)
	if e != nil {
		t.Fatal(e)
	}
	assert.True(t, found)
	// Repo Remove
	found, err = db.CollectionRemove(ctx, col)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)

}
