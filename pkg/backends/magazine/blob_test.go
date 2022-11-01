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
	_, found, err := mag.SQlite.Layer.LayerGetByDigest(bu.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
}

func Test_Config_StoreBlobUpload(t *testing.T) {
	bs := utils.NewBootstrap()
	err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	bu, b := types.FT_arm_BlobUpload_Config()
	_ = b
	cfg := types.Config{
		Digest: bu.Digest,
	}
	found, err := mag.SQlite.Config.GetByRequest(&cfg)
	assert.NoError(t, err)
	assert.False(t, found)
	err = mag.StoreBlobUpload(ctx, &bu, b)
	assert.NoError(t, err)
	cfgId, err := mag.SQlite.Config.GetID(bu.Digest)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), cfgId)
	found, err = mag.SQlite.Config.GetByRequest(&cfg)
	assert.NoError(t, err)
	assert.True(t, found)
	err = mag.StoreBlobUpload(ctx, &bu, []byte(`{"foo":"bar"}`))
	assert.Error(t, err)
}
