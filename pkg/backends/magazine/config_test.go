package magazine

import (
	"encoding/base64"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Config_StoreConfigUpload(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	_, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Tests
	buf := []byte(`{
		"author": "dnIc4tc80O5Lp598",
		"architecture": "amd64",
		"os": "linux",
		"rootfs": {
			"type": "layers",
			"diff_ids": []
		}
	}`)
	bufEnc := base64.StdEncoding.EncodeToString(buf)

	bufDig := digest.FromBytes(buf)
	bu := types.OCIBlobUpload{
		Digest: bufDig.String(),
		Data:   []byte(bufEnc),
	}
	// Check if config is NOT in DB
	_, found, err := mag.SQlite.Config.ConfigGetByDigest(bufDig.String())
	assert.NoError(t, err)
	assert.False(t, found)
	// Check if config is NOT in Minio
	_, found, err = mag.Minio.GetConfig(ctx, bufDig.String())
	assert.NoError(t, err)
	assert.False(t, found)
	// Upload Config
	err = mag.StoreConfigUpload(ctx, &bu, buf)
	assert.NoError(t, err)
	// Check if config is in DB
	cfg, found, err := mag.SQlite.Config.ConfigGetByDigest(bufDig.String())
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", cfg.MediaType)
	// Check if config is in Minio
	_, found, err = mag.Minio.GetConfig(ctx, bufDig.String())
	assert.NoError(t, err)
	assert.True(t, found)

}
