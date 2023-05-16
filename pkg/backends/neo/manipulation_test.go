package neo

import (
	"context"
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
	mpGlow := types.ManipulationGlow150
	/// Check if manipulation exists
	_, found, err := db.ManipulationGetIDByLabel(ctx, mpGlow.Label)
	if err != nil {
		t.Fatalf("Failed to query manipulation: %v", err)
	}
	assert.False(t, found, "Manipulation should not be found")
	// Insert Manipulation
	mpIns, err := db.ManipulationCreate(ctx, mpGlow)
	if err != nil {
		t.Fatalf("Failed to create manipulation: %v", err)
	}
	assert.NotEqual(t, int64(0), mpIns, "Manipulation ID should be 1")
	// Get Manipulation by ID
	mpGet, found, err := db.ManipulationGetByID(ctx, mpIns)
	if err != nil {
		t.Fatalf("Failed to query manipulation: %v", err)
	}
	assert.True(t, found, "Manipulation should be found")
	assert.Equal(t, mpIns, mpGet.ID)
	//assert.Equal(t, 1, len(mpGet.Layers), "Manipulation should have one rule")
	//
	mpGetCreateID, err := db.ManipulationGetCreate(ctx, mpGlow)
	if err != nil {
		t.Errorf("Failed to create manipulation: %v", err)
	}
	assert.Equal(t, mpIns, mpGetCreateID, "Manipulation ID should be 1")
	// Delete Manipulation
	found, err = db.ManipulationDeleteByID(ctx, mpIns)
	if err != nil {
		t.Fatalf("Failed to delete manipulation: %v", err)
	}
	assert.True(t, found, "Manipulation should be found")
	found, err = db.ManipulationDeleteByID(ctx, mpIns)
	if err != nil {
		t.Fatalf("Failed to delete manipulation: %v", err)
	}
	assert.False(t, found, "Second delte should not find manipulation")
	// Create Manipulation again
	mpGetCreateID2, err := db.ManipulationGetCreate(ctx, mpGlow)
	if err != nil {
		t.Errorf("Failed to create manipulation: %v", err)
	}
	assert.Greater(t, mpGetCreateID2, mpIns, "Manipulation ID should be greater than first insert")

}

func TestUpstreamGetByManipuLayerID(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("upstream:glow150amd64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
	bs.AddBootstrap("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#glow150arm64_.*")
	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
	// ManiData Setup
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
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
	// Eval RUles
	if err := db.Rule_EvalAll(ctx); err != nil {
		t.Fatal(err)
	}
	// Actual tests
	mplArm64ID, err := br.GetInt64("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#id")
	if err != nil {
		t.Fatalf("Failed to get ManipuLayer ID: %v", err)
	}
	if up, upFound, err := db.UpstreamGetByManipuLayerID(ctx, mplArm64ID); err != nil {
		t.Fatal(err)
	} else if !upFound {
		t.Fatal("Upstream should be found")
	} else {
		assert.Equal(t, "cqnib/metahub-layers/hpc3/glow", up.Repo)
	}
	if rls, err := db.RulesGetByManipuLayerID(ctx, mplArm64ID); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 1, len(rls.Rules), "ManipuLayer should have 3 rules")
	}
}

func TestManipulationFromConfigManipulation(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/glow/1.5.0.yaml#pass")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("upstream:glow150amd64")
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
	//
	cfgMp := config.Manipulation{
		Label: "glow150arm64",
		ManipuLayers: []config.ManipuLayer{
			config.ManipuLayerGlow150ARM64,
			config.ManipuLayerGlow150AMD64,
		},
	}
	mp, err := db.ManipulationFromConfigManipulation(ctx, cfgMp)
	if err != nil {
		t.Fatalf("Failed to create manipulation: %v", err)
	}
	assert.Equal(t, "glow150arm64", mp.Label, "Manipulation label should be glow150arm64")
	assert.Equal(t, 2, len(mp.ManipuLayers), "Manipulation should have one ManipuLayer")
}

func TestManipulationUnwind(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("upstream:glow150amd64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
	bs.AddBootstrap("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#glow150arm64_.*")
	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
	// ManiData Setup
	// ManiData Setup
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
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
	mani, err := db.getMani(ctx, br, "manifest:ft_arm64")
	if err != nil {
		t.Fatal(err)
	}
	if len(mani.Manifests) != 1 {
		t.Fatalf("Expected 1 manifestData, but got %d", len(mani.Manifests))
	}
	maniData := mani.Manifests[0]
	prfLabel := "glow#1.5.0-arm64"
	prf, err := db.getProfile(ctx, br, prfLabel)
	if err != nil {
		t.Fatal(err)
	}
	mps, err := db.ManipulationsGetByProfileID(ctx, prf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mps) != 1 {
		t.Fatalf("Expected 1 manipulation, but got %d", len(mps))
	}
	/// Unwind Rules
	if e := db.ManipulationUnwind(ctx, mani.ID, maniData.ID, prf.ID); e != nil {
		t.Fatal(e)
	}
	ociManiData, mdFound, e := db.ManifestDataGetByDigest(ctx, mani.Manifests[0].Digest)
	if e != nil {
		t.Fatal(e)
	}
	if !mdFound {
		t.Fatal("Did not find manifestdata")
	}
	//t.Log(ociManiData.Config)
	assert.Equal(t, 5, len(ociManiData.Layers))

}
