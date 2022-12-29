package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_CollectionListing_Get_ByID(t *testing.T) {
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
	colID, err := br.GetInt64("collection:ft#id")
	assert.NoError(t, err)
	colTagID, err := br.GetInt64("tag:generic#id")
	assert.NoError(t, err)
	cl, err := db.CollectionListing_Get_ByID(colID, colTagID)
	assert.NoError(t, err)
	assert.Equal(t, colID, cl.ID)
	assert.Equal(t, colTagID, cl.ColTag.ID)
	assert.Equal(t, "generic", cl.ColTag.Tag)
	assert.Equal(t, "qnib/featuretest", cl.Repo)
	// Like
	cls, err := db.CollectionListing_Get_ByRepoLike("")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cls))
	cls, err = db.CollectionListing_Get_ByRepoLike("qnib")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cls))
	cls, err = db.CollectionListing_Get_ByRepoLike("qnib/nope")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(cls))
}
