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
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	arm_l0 := types.FT_arm_Layer0Reference()
	_, found, err := db.LayerGetByDigest(ctx, arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	arm_l0_id, err := db.Layer_Create_ByDescriptor(arm_l0)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, arm_l0_id)
	arm_l0_got, found, err := db.LayerGetByDigest(ctx, arm_l0.Digest.String())
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
	err = db.LayerDeleteByDigest(ctx, arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	_, found, err = db.LayerGetByDigest(ctx, arm_l0.Digest.String())
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
	err = db.LayerDeleteByDigest(ctx, arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	l0_id_got, err = db.Layer_PlainInsert(ctx, arm_l0_got)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, arm_l0_id, l0_id_got)
}

func TestLayerGetUpstream(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		db.TeardownTest(bs)
		t.Fatal(err)

	}
	defer db.TeardownTest(bs)
	// Actual test
	lId, e := br.GetInt64("layer:0:ft_amd64#id")
	if e != nil {
		t.Fatal(e)
	}
	up, upFound, e := db.LayerGetUpstream(ctx, lId)
	if e != nil {
		t.Fatal(e)
	}
	if !upFound {
		t.Fatal("Did not find upstream")
	}
	assert.Equal(t, "registry.gitlab.com", up.Reg)
}
