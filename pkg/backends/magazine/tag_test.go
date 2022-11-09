package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_TagList(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	bs.AddBootstrap("upstreams:qnib/featuretest-arm64")
	bs.AddBootstrap("layers:qnib/featuretest-arm64")
	bs.AddBootstrap("config:qnib/featuretest-arm64")
	bs.AddBootstrap("manifest:qnib/featuretest-arm64")
	bs.AddBootstrap("manifestlist:qnib/featuretest-generic-arm64")
	bs.AddBootstrap("collection:qnib/featuretest")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	_ = br
	// Test

	req := types.Request{
		Repo: "qnib/featuretest",
	}
	// pure SQLite
	res, err := mag.SQlite.Collection.GetTagsListByRepo(req.Repo)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res.Tags))
	res, err = mag.Tags_GetList(req)
	assert.NoError(t, err)
	// TODO: add setup collection to make this work
	assert.Equal(t, 1, len(res.Tags))
	reqPage := types.Request{
		Repo:   "qnib/featuretest",
		Number: 1,
		Last:   "latest",
	}
	res, err = mag.Tags_GetList(reqPage)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res.Tags))
}
