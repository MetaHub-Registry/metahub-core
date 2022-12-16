package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_CollectionTag_CRUD(t *testing.T) {
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
	_, found, err := db.CollectionTag_GetByTag("test")
	assert.NoError(t, err)
	assert.False(t, found)
	ct_got, err := db.CollectionTag_GetCreate("test")
	assert.NoError(t, err)
	assert.NotNil(t, ct_got.ID)
	ct_got2, err := db.CollectionTag_GetCreate("test")
	assert.NoError(t, err)
	assert.Equal(t, ct_got.ID, ct_got2.ID)
}

func Test_CollectionTag_LinkCollection(t *testing.T) {
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
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 90
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	_, found, err := db.CollectionTag_GetByTag("test")
	assert.NoError(t, err)
	assert.False(t, found)
	ct_got, err := db.CollectionTag_GetCreate("test")
	assert.NoError(t, err)
	assert.NotNil(t, ct_got.ID)
	// CT
	mani_amd_id, err := br.GetInt64("manifest:ft_amd64#id")
	assert.NoError(t, err)
	err = db.LinkCollectionTagToManifest(ctx, ct_got.ID, mani_amd_id)
	assert.NoError(t, err)

}
