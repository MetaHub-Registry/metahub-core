package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Layer_CRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	arm_l0 := types.FT_arm_Layer0Reference()
	_, found, err := db.LayerGetByDigest(arm_l0.Digest.String())
	assert.NoError(t, err)
	assert.False(t, found)
	arm_l0_id, err := db.Layer_Create_ByDescriptor(arm_l0)
	assert.NoError(t, err)
	assert.NotNil(t, arm_l0_id)
	arm_l0_got, found, err := db.LayerGetByDigest(arm_l0.Digest.String())
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, arm_l0_id, arm_l0_got.ID)
	arm_l0_got_id, found, err := db.GetLayer(arm_l0_got)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, arm_l0_id, arm_l0_got_id)
	// Delete
	err = db.LayerDeleteByDigest(arm_l0.Digest.String())
	assert.NoError(t, err)
	_, found, err = db.LayerGetByDigest(arm_l0.Digest.String())
	assert.NoError(t, err)
	assert.False(t, found)
	// Create Layer
	arm_l0_id, err = db.CreateLayer(arm_l0_got)
	assert.NoError(t, err)
	assert.NotNil(t, arm_l0_id)
	l0_id_got, err := db.Layer_PlainInsert(ctx, arm_l0_got)
	assert.NoError(t, err)
	assert.Equal(t, arm_l0_id, l0_id_got)
	err = db.LayerDeleteByDigest(arm_l0.Digest.String())
	assert.NoError(t, err)
	l0_id_got, err = db.Layer_PlainInsert(ctx, arm_l0_got)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(0), l0_id_got)
	assert.NotEqual(t, arm_l0_id, l0_id_got)
}
