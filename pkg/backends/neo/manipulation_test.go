package neo

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func TestManipulationCRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/platform/arch/amd64.yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
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
	mpGlow := types.ManipulationGlow150
	/// Check if manipulation exists
	_, found, err := db.ManipulationGetIDByLabel(ctx, mpGlow.Label)
	if err != nil {
		t.Fatalf("Failed to query manipulation: %v", err)
	}
	assert.False(t, found, "Manipulation should not be found")
	// Insert Manipulation
	err = db.ManipulationCreate(ctx, &mpGlow)
	if err != nil {
		t.Fatalf("Failed to create manipulation: %v", err)
	}
	assert.GreaterOrEqual(t, mpGlow.ID, int64(0), "Manipulation ID should be a valid neo4j node ID")
	// Get Manipulation by ID
	mpGet, found, err := db.ManipulationGetByID(ctx, mpGlow.ID)
	if err != nil {
		t.Fatalf("Failed to query manipulation: %v", err)
	}
	assert.True(t, found, "Manipulation should be found")
	assert.Equal(t, mpGlow.ID, mpGet.ID)
	mpInsID := mpGlow.ID
	//assert.Equal(t, 1, len(mpGet.Layers), "Manipulation should have one rule")
	mpGlow2 := types.ManipulationGlow150
	err = db.ManipulationGetCreate(ctx, &mpGlow2)
	if err != nil {
		t.Errorf("Failed to create manipulation: %v", err)
	}
	assert.Equal(t, mpInsID, mpGlow2.ID, "Manipulation ID should match the previously inserted ID")
	// Delete Manipulation
	found, err = db.ManipulationDeleteByID(ctx, mpInsID)
	if err != nil {
		t.Fatalf("Failed to delete manipulation: %v", err)
	}
	assert.True(t, found, "Manipulation should be found")
	found, err = db.ManipulationDeleteByID(ctx, mpInsID)
	if err != nil {
		t.Fatalf("Failed to delete manipulation: %v", err)
	}
	assert.False(t, found, "Second delte should not find manipulation")
	// Create Manipulation again
	mpGlow3 := types.ManipulationGlow150
	mpGlow3.ID = int64(-1)
	err = db.ManipulationGetCreate(ctx, &mpGlow3)
	if err != nil {
		t.Errorf("Failed to create manipulation: %v", err)
	}
	assert.NotEqual(t, mpGlow3.ID, int64(-1), "Manipulation ID should be greater than first insert")
}

func TestUpstreamGetByManipuLayerID(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
	//bs.AddBootstrap("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#glow150arm64_.*")
	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
	// ManiData Setup
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
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
	_, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestUpstreamGetByManipuLayerID.Setup",
	})
	defer span.End()
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
	// Eval RUles
	if err := db.RuleEvalAll(ctx); err != nil {
		t.Fatal(err)
	}
	// Path Z task 7.7: D5 invariant canary after every RuleEvalAll.
	assertRuleShortcutInvariantsOrFatal(t, ctx)
	span.End()
	// Actual tests
	_, ctx, span = utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestUpstreamGetByManipuLayerID.Run",
	})
	defer span.End()
	mplArm64ID, err := br.GetInt64("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#id")
	if err != nil {
		t.Fatalf("Failed to get ManipuLayer ID: %v", err)
	}
	if up, upFound, err := db.UpstreamGetByManipuLayerID(ctx, mplArm64ID); err != nil {
		span.End()
		t.Fatal(err)
	} else if !upFound {
		span.End()
		t.Fatal("Upstream should be found")
	} else {
		assert.Equal(t, "cqnib/metahub-layers/hpc3/glow", up.Repo)
	}
	if rls, err := db.RulesGetByManipuLayerID(ctx, mplArm64ID); err != nil {
		span.End()
		t.Fatal(err)
	} else {
		assert.Equal(t, 1, len(rls.Rules), "ManipuLayer should have 1 rules")
	}
	// Test ManinpulationCheckRules
	prfId, err := br.GetInt64("profile:glow:1.5.0-arm64#id")
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	/// ARM64
	maniArm64ID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	if ok, err := db.ManinpulationCheckRules(ctx, prfId, maniArm64ID); err != nil {
		span.End()
		t.Fatal(err)
	} else {
		assert.True(t, ok, "Arm64 manifest should ge through")
	}
	/// ARM64
	maniAmd64ID, err := br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	if ok, err := db.ManinpulationCheckRules(ctx, prfId, maniAmd64ID); err != nil {
		span.End()
		t.Fatal(err)
	} else {
		assert.False(t, ok, "AMD64 manifest should NOT go through")
	}
	span.End()
}

func TestManipulationFromConfigManipulation(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("profile:profiles/glow/1.5.0.yaml#pass")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
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
	//
	cfgMp := config.Manipulation{
		Label: "glow150arm64",
		ManipuLayers: []config.ManipuLayer{
			config.ManipuLayerGlow150ARM64,
		},
	}
	mp, err := db.ManipulationFromConfigManipulation(ctx, cfgMp)
	if err != nil {
		t.Fatalf("Failed to create manipulation: %v", err)
	}
	assert.Equal(t, "glow150arm64", mp.Label, "Manipulation label should be glow150arm64")
	assert.Equal(t, 1, len(mp.ManipuLayers), "Manipulation should have one ManipuLayer")
}

func TestManipulationFromYAML(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("manifestdata:glow150arm64")
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
	cfgMp, err := config.LoadManipulation(ctx, filepath.Join("../../../misc/yaml", "manipulations/layer/glow/1.5.0-arm64.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "layer/glow#1.5.0-arm64", cfgMp.Label)
	mp, err := db.ManipulationFromConfigManipulation(ctx, cfgMp)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "layer/glow#1.5.0-arm64", mp.Label)
}
