package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestLayerCRUD(t *testing.T) {
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
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	arm_l0 := types.FT_arm_Layer0Reference()
	_, found, err := db.LayerGetByDigest(arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	arm_l0_id, err := db.Layer_Create_ByDescriptor(arm_l0)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, arm_l0_id)
	arm_l0_got, found, err := db.LayerGetByDigest(arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, arm_l0_id, arm_l0_got.ID)
	arm_l0_got_id, found, err := db.GetLayer(arm_l0_got)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, arm_l0_id, arm_l0_got_id)
	// Delete
	err = db.LayerDeleteByDigest(arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	_, found, err = db.LayerGetByDigest(arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	// Create Layer
	arm_l0_id, err = db.CreateLayer(arm_l0_got)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, arm_l0_id)
	l0_id_got, err := db.Layer_PlainInsert(ctx, arm_l0_got)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, arm_l0_id, l0_id_got)
	err = db.LayerDeleteByDigest(arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	l0_id_got, err = db.Layer_PlainInsert(ctx, arm_l0_got)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, arm_l0_id, l0_id_got)
}
