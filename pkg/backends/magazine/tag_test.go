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
	//bs.SetSqlite("Desktop/metahub.db")
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
	// Remove Manifests and expect the collection to stick around with 0 tags
	m := types.Manifest{
		Upstream: types.Upstream{
			Repo: "qnib-pub-containers/qnib/plain-featuretest",
			Tag:  "20220715-arm64-generic",
		},
		Digest: "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5",
	}
	_, _, err = mag.SQlite.Manifest.Remove(m)
	assert.NoError(t, err)
	res, err = mag.Tags_GetList(req)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(res.Tags))

}
