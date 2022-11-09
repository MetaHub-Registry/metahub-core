package magazine

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_(t *testing.T) {
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
	_, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	req := types.Request{
		Repo: "foo",
	}
	res, err := mag.Collections_GetCatalog(req)
	assert.NoError(t, err)
	logrus.Error(res)
	// TODO: add setup collection to make this work
	//assert.Equal(t, 3, len(res.Repos))
}
