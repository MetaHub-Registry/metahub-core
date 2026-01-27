package neo

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManipuEnvCRUD(t *testing.T) {
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
		if len(mpls) != 1 {
			t.Fatalf("Expected 1 ManipuLayer, but got %d", len(mpls))
		}
		mplyrID = mpls[0].ID
	}

	mpe := types.ManipuEnvIfNullHelp
	if ok, e := db.ManipuEnvGet(ctx, &mpe); e != nil {
		t.Fatal(e)
	} else {
		if ok {
			t.Fatal("Should not have found ManipuEnv")
		}
	}
	if e := db.ManipuEnvCreate(ctx, mplyrID, &mpe); e != nil {
		t.Fatal(e)
	}
	if ok, e := db.ManipuEnvGet(ctx, &mpe); e != nil {
		t.Fatal(e)
	} else {
		if !ok {
			t.Fatal("Should HAVE been found ManipuEnv")
		}
	}
	// GetCreate
	mpe2 := types.ManipuEnvIfNullHelp
	if e := db.ManipuEnvInsert(ctx, mplyrID, &mpe2); e != nil {
		t.Fatal(e)
	}
	if ok, e := db.ManipuEnvGet(ctx, &mpe2); e != nil {
		t.Fatal(e)
	} else {
		if !ok {
			t.Fatal("Should HAVE been found ManipuEnv after inserting")
		}
	}
	if e := db.ManipuEnvInsert(ctx, mplyrID, &mpe2); e != nil {
		t.Fatal(e)
	}
	// ManipuEnvGetByManipulayerID
	if mpes, err := db.ManipuEnvsGetByManipulayerID(ctx, mplyrID); err != nil {
		t.Fatal(err)
	} else {
		if len(mpes) != 1 {
			t.Fatalf("Expected 1 ManipuEnv, but got %d", len(mpes))
		}
	}
}
