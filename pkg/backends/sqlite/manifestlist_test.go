package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Manifestlist_Get(t *testing.T) {
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
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Tests
	req := types.Request{
		Repo:    "qnib/featuretest",
		Tag:     "latest",
		Profile: "generic",
	}
	ml, found, err := sq.GetTypesManifestListByRequest(req)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 0, len(ml.Manifests))

}
