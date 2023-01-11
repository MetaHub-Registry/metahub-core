package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Layer_StoreBlobUpload(t *testing.T) {
	bu, b := types.FT_arm_BlobUpload_Layer0()
	err := mag.StoreBlobUpload(ctx, &bu, b)
	assert.NoError(t, err)
	_, found, err := mag.Database.LayerGetByDigest(bu.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
}

func Test_Config_StoreBlobUpload(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.SetEmptyAll()
	_, err := mag.SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Tests
	bu, b := types.FT_arm_BlobUpload_Config()
	_ = b
	_, found, err := mag.Database.ConfigGetByDigest(bu.Digest)
	assert.NoError(t, err)
	assert.False(t, found)
	err = mag.StoreBlobUpload(ctx, &bu, b)
	assert.NoError(t, err)
	err = mag.StoreBlobUpload(ctx, &bu, []byte(`{"foo":"bar"}`))
	assert.Error(t, err)
}
