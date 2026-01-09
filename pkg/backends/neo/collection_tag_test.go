package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestCollectionTagCRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	_, found, err := db.CollectionTag_GetByTag("test")
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	ct_got, err := db.CollectionTag_GetCreate(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, ct_got.ID)
	ct_got2, err := db.CollectionTag_GetCreate(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, ct_got.ID, ct_got2.ID)
}

func TestCollectionTagLinkCollection(t *testing.T) {
	ctx := context.Background()
	debug := 0
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
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_amd64")
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
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	_, found, err := db.CollectionTag_GetByTag("test")
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	ct_got, err := db.CollectionTag_GetCreate(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, ct_got.ID)
	// CT
	mani_amd_id, err := br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	err = db.LinkCollectionTagToManifest(ctx, ct_got.ID, mani_amd_id)
	if err != nil {
		t.Fatal(err)
	}
	tags, e := db.CollectionGetTagsListByRepo(ctx, "qnib/featuretest")
	if e != nil {
		t.Fatal(e)
	}
	assert.Equal(t, 2, len(tags.Tags))
	// Remove AMD64 ManifestData should remove everything below
	//sha256:5bf8eee20fde14eb6e3e924c18fafa4d816e131f315810875136f0438ff7beed
	//db.ManifestData_Delete(ctx, maniAMD64)
	if err = db.CollectionTagCleanupByID(ctx, ct_got.ID); err != nil {
		t.Fatal(err)
	}
}
