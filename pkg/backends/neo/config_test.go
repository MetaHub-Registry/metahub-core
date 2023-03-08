package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Config_CRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	//bs.AddBootstrap("platforms:generic")
	//bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	//bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	arm_cfg := types.FT_arm_Config()

	_, found, err := db.Config_GetByDigest(arm_cfg.Digest)
	assert.NoError(t, err)
	assert.False(t, found)
	_, found, err = db.Config_GetByDigest(arm_cfg.Digest)
	assert.NoError(t, err)
	assert.False(t, found)
	arm_cfg_id, err := db.createConfig(ctx, arm_cfg)
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, amd64_cfg_got.ID, amd64_cfg_got2.ID)
	amd64_cfgs_got3, found, err := db.Config_GetByDigest(amd64_cfg_got.Digest)
	assert.Equal(t, 1, len(amd64_cfgs_got3))
	amd64_cfg_got3 := amd64_cfgs_got3[0]
	assert.NoError(t, err)
	assert.True(t, found, fmt.Sprintf("expected to find config with digest %s", amd64_cfg_got.Digest))
	assert.Equal(t, amd64_cfg_got.ID, amd64_cfg_got3.ID)
	// Delete
	err = db.ConfigDeleteByDigest(amd64_cfg_got.Digest)
	assert.NoError(t, err)
	_, found, err = db.Config_GetByDigest(amd64_cfg_got.Digest)
	assert.NoError(t, err)
	assert.False(t, found)
}
