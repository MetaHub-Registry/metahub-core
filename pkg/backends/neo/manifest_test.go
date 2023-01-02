package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Manifest_CRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	arm_mani := types.FT_arm_Manifest()
	_, found, err := db.GetManifest(arm_mani)
	assert.NoError(t, err)
	assert.False(t, found)
	arm_mani_id, err := db.createManifest(arm_mani)
	assert.NoError(t, err)
	arm_mani_id_got1, found, err := db.GetManifest(arm_mani)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, arm_mani_id_got1)
	/// Add Upstream
	up := types.FT_arm_Upstream()
	found, err = db.Upstream_GetInsert(&up)
	assert.NoError(t, err)
	assert.False(t, found)
	err = db.LinkManifestToUpstream(ctx, arm_mani_id_got1, up.ID)
	assert.NoError(t, err)
	// Manifest_Get_ByDigest
	arm_mani_got3, found, err := db.Manifest_Get_ByDigest(arm_mani.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, arm_mani_got3.ID)
	// Manifest_Remove
	reqGot3 := types.Request{
		Repo:   arm_mani_got3.Upstream.Repo,
		Tag:    arm_mani_got3.Upstream.Tag,
		Host:   arm_mani_got3.Upstream.Reg,
		Digest: arm_mani_got3.Digest,
	}
	found, err = db.Manifest_Remove(ctx, reqGot3)
	assert.NoError(t, err)
	assert.True(t, found)
	_, found, err = db.Manifest_Get_ByDigest(arm_mani_got3.Digest)
	assert.NoError(t, err)
	assert.False(t, found)
	// Manifest_GetInsert
	err = db.Manifest_GetInsert(ctx, &arm_mani)
	assert.NoError(t, err)
	/// Add Upstream
	found, err = db.Upstream_GetInsert(&up)
	assert.NoError(t, err)
	assert.True(t, found)
	err = db.LinkManifestToUpstream(ctx, arm_mani.ID, up.ID)
	assert.NoError(t, err)
	arm_mani_got4, found, err := db.Manifest_Get_ByDigest(arm_mani.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, arm_mani.ID, arm_mani_got4.ID)
	arm_mani.ID = -1
	err = db.Manifest_GetInsert(ctx, &arm_mani)
	assert.NoError(t, err)
	assert.Equal(t, arm_mani_got4.ID, arm_mani.ID)
	arm_mani_got5, err := db.Manifest_Insert(ctx, arm_mani)
	assert.NoError(t, err)
	assert.Equal(t, arm_mani_got4.ID, arm_mani_got5)
	reqArm := types.Request{
		Repo:   arm_mani.Upstream.Repo,
		Tag:    arm_mani.Upstream.Tag,
		Host:   arm_mani.Upstream.Reg,
		Digest: arm_mani.Digest,
	}
	found, err = db.Manifest_Remove(ctx, reqArm)
	assert.NoError(t, err)
	assert.True(t, found)
	arm_mani_got6, err := db.Manifest_Insert(ctx, arm_mani)
	assert.NoError(t, err)
	assert.NotEqual(t, arm_mani_got4.ID, arm_mani_got6)
}

func Test_Manifest_Links(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("layer0:ft_arm64")
	bs.AddBootstrap("layer1:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	arm_mani := types.FT_arm_Manifest()
	arm_mani_id, err := db.createManifest(arm_mani)
	assert.NoError(t, err)
	assert.NotNil(t, arm_mani_id)
	// Link Platform
	arm64_platform_id, err := br.GetInt64("platform:linux/arm64#id")
	assert.NoError(t, err)
	err = db.LinkManifestToPlatform(ctx, arm_mani_id, arm64_platform_id)
	assert.NoError(t, err)
	// Link Config
	arm_config_id, err := br.GetInt64("config:ft_arm64#id")
	assert.NoError(t, err)
	err = db.LinkManifestToConfig(ctx, arm_mani_id, arm_config_id)
	assert.NoError(t, err)
	// Link Upstream
	arm_upstream_id, err := br.GetInt64("upstream:ft_arm64#id")
	assert.NoError(t, err)
	err = db.LinkManifestToUpstream(ctx, arm_mani_id, arm_upstream_id)
	assert.NoError(t, err)
	// Query all links?
	manis, err := db.GetAllManifests()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(manis))
	// Add Layers
	arm_layer0_id, err := br.GetInt64("layer0:ft_arm64#id")
	assert.NoError(t, err)
	err = db.LinkManifestToLayer(ctx, arm_mani_id, arm_layer0_id, 0)
	assert.NoError(t, err)
	arm_layer1_id, err := br.GetInt64("layer1:ft_arm64#id")
	assert.NoError(t, err)
	err = db.LinkManifestToLayer(ctx, arm_mani_id, arm_layer1_id, 1)
	assert.NoError(t, err)
	// GET By Upstream
	arm_up, err := db.Upstream_Get_ByID(arm_upstream_id)
	assert.NoError(t, err)
	arm_mani_got1, found, err := db.Manifest_Get_ByUpstream(arm_up)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, arm_mani_got1.ID)
	arm_mani_got2, found, err := db.Manifest_GetID_ByDigest(arm_mani_got1.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, arm_mani_got2)
}

func Test_Manifest_MultiManifest(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer0:ft_arm64")
	bs.AddBootstrap("layer1:ft_arm64")
	bs.AddBootstrap("layer0:ft_amd64")
	bs.AddBootstrap("layer1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 90
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	// Query all links?
	manis, err := db.GetAllManifests()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(manis))
}
