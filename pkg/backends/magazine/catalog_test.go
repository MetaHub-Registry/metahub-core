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
	bs.AddBootstrap("collection:qnib/featuretest")
	err := SetupTest(bs)
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
