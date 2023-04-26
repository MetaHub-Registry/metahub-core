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
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")

	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
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
	_, found, err := mag.Database.Config_GetByDigest(ctx, bufDig.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	// Check if config is NOT in Minio
	_, found, err = mag.Minio.GetConfig(ctx, bufDig.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	// Upload Config
	err = mag.StoreConfigUpload(ctx, &bu, buf)
	if err != nil {
		t.Fatal(err)
	}
	// Check if config is in DB
	cfg, found, err := mag.Database.Config_GetByDigest(ctx, bufDig.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	// TODO: check if unknown is correct
	assert.Equal(t, "unknownConfig", cfg.MediaType)
	//assert.Equal(t, 2, len(cfg.Labels.Map))
	// Check if config is in Minio
	_, found, err = mag.Minio.GetConfig(ctx, bufDig.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
}
