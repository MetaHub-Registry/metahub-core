package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Basic(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("cypher:collection:ft_tag1")
	bs.AddBootstrap("cypher:collection_only:ft_generic")
	bs.AddBootstrap("ComputeManifestlists")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	_, err := mag.SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	cols, err := mag.Database.CollectionGetAll()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cols), cols)
	req := types.Request{
		Repo: "qnib",
	}
	res, err := mag.Collections_GetCatalog(req)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res.Repos))
	// TODO: add setup collection to make this work
	//assert.Equal(t, 3, len(res.Repos))
}

func Test_Catalog_ARM64(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("layer0:ft_arm64")
	bs.AddBootstrap("layer1:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
	bs.AddBootstrap("ComputeManifestlists")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	_, err := mag.SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	cols, err := mag.Database.CollectionGetAll()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cols), cols)
	req := types.Request{
		Repo: "qnib",
	}
	res, err := mag.Collections_GetCatalog(req)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res.Repos))

}
