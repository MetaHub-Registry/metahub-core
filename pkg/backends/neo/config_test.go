package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestConfigCRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(bs)
	if err != nil {
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	// Actual test
	arm_cfg := types.FT_arm_Config()

	_, found, err := db.Config_GetByDigest(ctx, arm_cfg.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	_, found, err = db.Config_GetByDigest(ctx, arm_cfg.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	arm_cfg_id, err := db.createConfig(ctx, arm_cfg)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, arm_cfg_id)
	// AMD
	amd64_ocimani := types.FT_amd64_OCIManifest()
	amd64_up := types.FT_amd64_Upstream()
	amd64_label := fmt.Sprintf("%s/%s:%s", amd64_up.Reg, amd64_up.Repo, amd64_up.Tag)
	amd64_cfg_got, found, err := db.Config_GetInsert_ByOCIManifest(ctx, amd64_ocimani, amd64_label)
	assert.NoError(t, err)
	assert.False(t, found)
	assert.NotNil(t, amd64_cfg_got.ID)
	amd64_cfg_got2, found, err := db.Config_GetInsert_ByOCIManifest(ctx, amd64_ocimani, amd64_label)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, amd64_cfg_got.ID, amd64_cfg_got2.ID)
	amd64_cfg_got3, found, err := db.Config_GetByDigest(ctx, amd64_cfg_got.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found, fmt.Sprintf("expected to find config with digest %s", amd64_cfg_got.Digest))
	assert.Equal(t, amd64_cfg_got.ID, amd64_cfg_got3.ID)
	// Delete
	err = db.ConfigDeleteByDigest(amd64_cfg_got.Digest)
	if err != nil {
		t.Fatal(err)
	}
	_, found, err = db.Config_GetByDigest(ctx, amd64_cfg_got.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)

}

func TestConfigMediaType(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	cfgID, err := br.GetInt64("config:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	cfg, found, err := db.Config_GetByID(ctx, cfgID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Config not found")
	}
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", cfg.MediaType)

	// Get MediaType
	mediaType, err := db.Config_GetMediaType_ByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, cfg.MediaType, mediaType)
}
