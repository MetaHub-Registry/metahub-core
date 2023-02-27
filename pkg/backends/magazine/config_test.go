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
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	//bs.PostTestSleep = 300
	_, err := mag.SetupTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Tests
	buf := []byte(`{
		"author": "dnIc4tc80O5Lp598",
		"architecture": "amd64",
		"os": "linux",
		"rootfs": {
			"type": "layers",
			"diff_ids": []
		},
		"config": {
			"Labels": {
			   "org.supercontainers.hardware.cpu.optimized.mode": "generic",
			   "org.supercontainers.hardware.cpu.optimized.version": "x86_64_v4"
		 }
	   }
	}`)
	bufEnc := base64.StdEncoding.EncodeToString(buf)

	bufDig := digest.FromBytes(buf)
	bu := types.OCIBlobUpload{
		Digest:    bufDig.String(),
		Data:      []byte(bufEnc),
		MediaType: "application/vnd.docker.container.image.v1+json",
	}
	// Check if config is NOT in DB
	_, found, err := mag.Database.Config_GetByDigest(bufDig.String())
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
	cfgs, found, err := mag.Database.Config_GetByDigest(bufDig.String())
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 1, len(cfgs))
	cfg := cfgs[0]
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", cfg.MediaType)
	//assert.Equal(t, 2, len(cfg.Labels.Map))
	// Check if config is in Minio
	_, found, err = mag.Minio.GetConfig(ctx, bufDig.String())
	assert.NoError(t, err)
	assert.True(t, found)
}
