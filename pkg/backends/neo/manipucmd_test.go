package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManipuCmdCRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
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
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	mplID, err := br.GetInt64("manipulation:layer/glow#1.5.0-arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	var mplyrID int64
	if mpls, err := db.ManipuLayersGetByManipulationID(ctx, mplID); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 1, len(mpls), "Expected 1 ManipuLayer")
		mplyrID = mpls[0].ID
	}

	mpc := types.ManipuCmdPrependHelp
	if ok, e := db.ManipuCmdGet(ctx, &mpc); e != nil {
		t.Fatal(e)
	} else {
		assert.False(t, ok, "Should not have found ManipuCmd before creation")
	}
	if e := db.ManipuCmdCreate(ctx, mplyrID, &mpc); e != nil {
		t.Fatal(e)
	}
	if ok, e := db.ManipuCmdGet(ctx, &mpc); e != nil {
		t.Fatal(e)
	} else {
		assert.True(t, ok, "Should have found ManipuCmd after creation")
	}
	// Insert (GetCreate)
	mpc2 := types.ManipuCmdAppendHelp
	if e := db.ManipuCmdInsert(ctx, mplyrID, &mpc2); e != nil {
		t.Fatal(e)
	}
	if ok, e := db.ManipuCmdGet(ctx, &mpc2); e != nil {
		t.Fatal(e)
	} else {
		assert.True(t, ok, "Should have found ManipuCmd after inserting")
	}
	// Idempotent insert
	if e := db.ManipuCmdInsert(ctx, mplyrID, &mpc2); e != nil {
		t.Fatal(e)
	}
	// ManipuCmdsGetByManipulayerID
	if mpcs, err := db.ManipuCmdsGetByManipulayerID(ctx, mplyrID); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 2, len(mpcs), "Expected 2 ManipuCmds")
	}
}
