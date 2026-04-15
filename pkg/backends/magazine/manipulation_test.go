package magazine

import (
	"context"
	"testing"

	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManipulationCreateOCI(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
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
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		t.Logf("Error while SetupTest: %v", err)
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Actual test
	mani, err := mag.getMani(ctx, br, "manifest:ft_arm64")
	if err != nil {
		t.Fatal(err)
	}
	if len(mani.Manifests) != 1 {
		t.Fatalf("Expected 1 Manifest, but got %d", len(mani.Manifests))
	}
	maniData := mani.Manifests[0]
	if maniData.Type == "" {
		t.Fatal("maniData.Type is empty")
	}
	// Glow
	prfLabel := "glow#1.5.0-arm64"
	prf, err := mag.getProfile(ctx, br, prfLabel)
	if err != nil {
		t.Fatal(err)
	}
	mps, err := mag.Database.ManipulationsGetByProfileID(ctx, prf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mps) != 1 {
		t.Fatalf("Expected 1 manipulation, but got %d", len(mps))
	}
	// Unwind
	if cfgDig, e := mag.ManipulationUnwind(ctx, mani.ID, maniData.ID, prf.ID); e != nil {
		t.Fatal(e)
	} else {
		assert.Equal(t, "sha256:26608a4ddd90f25b3854b5d5f2543b158f79c6e6bdb4099b65b63eb0dcf9d45d", cfgDig)
	}
	maniDataUnwound, mdFound, e := mag.Database.ManifestGetByDigest(ctx, maniData.Digest)
	if e != nil {
		t.Fatal(e)
	}
	if !mdFound {
		t.Fatal("Did not find manifestdata")
	}
	assert.Equal(t, 5, len(maniDataUnwound.Layers))

	cfgGen, e := mag.GenerateOCIv1Config(ctx, mani, maniData)
	if e != nil {
		t.Fatal(e)
	}
	exp := set.CreateStringSet("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "PLATFORM_FEATURES=arm64", "SKIP_ENTRYPOINTS=true")
	assert.True(t, exp.Equals(set.CreateStringSet(cfgGen.Config.Env...)))
	maniDataUnwoundID, e := mag.CreateOCIv1ForProfile(ctx, mani, maniDataUnwound, prf)
	if e != nil {
		t.Fatal(e)
	}
	ociManiData, glowFound, e := mag.ManifestGetByID(ctx, maniDataUnwoundID)
	if e != nil {
		t.Fatal(e)
	}
	if !glowFound {
		t.Fatal("Did not find manifestdata")
	}
	if len(ociManiData.Layers) != 5 {
		t.Fatalf("Expected 5 layers got %d", len(ociManiData.Layers))
	}
	expEnv := set.CreateStringSet("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "PLATFORM_FEATURES=arm64", "SKIP_ENTRYPOINTS=true")
	assert.True(t, expEnv.Equals(set.CreateStringSet(ociManiData.Config.Cfg.Config.Env...)))
}

func TestManipulationParse(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Actual test
	manipuGlow := types.ManipulationGlow150
	mpID, err := mag.ManipulationGetCreate(ctx, manipuGlow)
	if err != nil {
		t.Fatal(err)
	}
	manipuGlow.ID = mpID
	// Check if Upstream was really cached
	expMDDig := types.ManifestDTGlow150Arm64().Digest
	if md, found, err := mag.Database.ManifestGetByDigest(ctx, expMDDig.String()); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Did not find manifestdata")
	} else {
		assert.Equal(t, expMDDig.String(), md.Digest)
		// Fetch MD from Minio
		if gotManiData, err := mag.Minio.ManifestGetObj(ctx, md.Digest); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 3, len(gotManiData.Layers))
		}
		// Fetch Config from Minio
		if gotCfg, err := mag.Minio.ConfigGetObj(ctx, md.Config.Digest); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 1, len(gotCfg.Cfg.Config.Env))
		}
	}
}

// TestManipulationParseExtended tests the manipulation parsing with environment and cmd changes
func TestManipulationParseExtended(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Actual test
	manipuGlow := types.ManipulationGlow151
	mpID, err := mag.ManipulationGetCreate(ctx, manipuGlow)
	if err != nil {
		t.Fatal(err)
	}
	manipuGlow.ID = mpID
	// Check if Upstream was really cached
	expMDDig := types.ManifestDTGlow151Arm64().Digest
	if md, found, err := mag.Database.ManifestGetByDigest(ctx, expMDDig.String()); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Did not find manifestdata")
	} else {
		assert.Equal(t, expMDDig.String(), md.Digest)
		// Fetch MD from Minio
		if gotManiData, err := mag.Minio.ManifestGetObj(ctx, md.Digest); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 3, len(gotManiData.Layers))
		}
		// Fetch Config from Minio
		if gotCfg, err := mag.Minio.ConfigGetObj(ctx, md.Config.Digest); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 2, len(gotCfg.Cfg.Config.Env))
		}
	}
}

func TestManipulationUnwind(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
	//bs.AddBootstrap("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#glow150arm64_.*")
	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
	// ManiData Setup
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
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Actual test

	mani, err := mag.getMani(ctx, br, "manifest:ft_arm64")
	if err != nil {
		t.Fatal(err)
	}
	if len(mani.Manifests) != 1 {
		t.Fatalf("Expected 1 manifestData, but got %d", len(mani.Manifests))
	}
	maniData := mani.Manifests[0]
	prfLabel := "glow#1.5.0-arm64"
	prf, err := mag.getProfile(ctx, br, prfLabel)
	if err != nil {
		t.Fatal(err)
	}
	mps, err := mag.Database.ManipulationsGetByProfileID(ctx, prf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mps) != 1 {
		t.Fatalf("Expected 1 manipulation, but got %d", len(mps))
	}
	// Prepare (eval rules)
	if err = mag.Database.RuleEvalAll(ctx); err != nil {
		t.Fatal(err)
	}
	// Path Z task 7.7: D5 invariant canary after every RuleEvalAll.
	assertRuleShortcutInvariantsOrFatal(t, ctx)
	/// Unwind Rules
	if cfgDigest, e := mag.ManipulationUnwind(ctx, mani.ID, maniData.ID, prf.ID); e != nil {
		t.Fatal(e)
	} else {
		assert.Equal(t, "sha256:26608a4ddd90f25b3854b5d5f2543b158f79c6e6bdb4099b65b63eb0dcf9d45d", cfgDigest)
	}
	ociManiData, mdFound, e := mag.Database.ManifestGetByDigest(ctx, mani.Manifests[0].Digest)
	if e != nil {
		t.Fatal(e)
	}
	if !mdFound {
		t.Fatal("Did not find manifestdata")
	}
	//t.Log(ociManiData.Config)
	assert.Equal(t, 5, len(ociManiData.Layers))
}

// TestApplyManipulationsToManifests tests the new pre-compute manipulation path.
// It bootstraps a glow profile with a manipulation, an arm64 manifest, evaluates rules,
// then verifies that ApplyManipulationsToManifests creates a manipulated manifest
// with the correct number of layers and a MANIPULATED_FROM edge.
func TestApplyManipulationsToManifests(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
	// Original manifest (featuretest arm64 = 2 layers)
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
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	// Eval rules so ManipuLayer matching works
	if err = mag.Database.RuleEvalAll(ctx); err != nil {
		t.Fatal(err)
	}
	assertRuleShortcutInvariantsOrFatal(t, ctx)

	// Get the arm64 manifest and populate its platform
	origMdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	origMd, found, err := mag.Database.ManifestGetByID(ctx, origMdID)
	if err != nil || !found {
		t.Fatalf("Original manifest not found: %v", err)
	}
	// ManifestGetByID does not populate Platform; set it from bootstrap
	platID, err := br.GetInt64("platform:linux/arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	origMd.Platform = types.Platform{ID: platID, OS: "linux", Arch: "arm64"}
	t.Logf("Original manifest: ID=%d, Digest=%s, Platform=%s", origMd.ID, origMd.Digest, origMd.Platform.GetLabel())

	// Get the glow profile
	prfLabel := "glow#1.5.0-arm64"
	prf, err := mag.getProfile(ctx, br, prfLabel)
	if err != nil {
		t.Fatal(err)
	}

	// Apply manipulations — should create a manipulated manifest with 5 layers (2 original + 3 glow)
	candidates := []types.Manifest{origMd}
	result, err := mag.ApplyManipulationsToManifests(ctx, prf, candidates)
	if err != nil {
		t.Fatalf("ApplyManipulationsToManifests failed: %v", err)
	}
	assert.Equal(t, 1, len(result), "Should still have 1 manifest in result")
	manipMd := result[0]
	// The manipulated manifest should be different from the original
	assert.NotEqual(t, origMd.ID, manipMd.ID, "Manipulated manifest should have a different ID")
	assert.NotEqual(t, origMd.Digest, manipMd.Digest, "Manipulated manifest should have a different digest")

	// Verify layer count: original has 2 layers, glow has 3 layers = 5 total
	manipLayers, err := mag.Database.GetLayersForManifestID(ctx, manipMd.ID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, len(manipLayers), "Manipulated manifest should have 5 layers (2+3)")

	// Verify MANIPULATED_FROM edge exists via the manipulated manifest's ID
	t.Logf("Manipulated manifest: ID=%d, Digest=%s, Label=%s", manipMd.ID, manipMd.Digest, manipMd.Label)

	// Verify layer count directly on the returned manifest
	assert.Equal(t, 5, len(manipMd.Layers), "Returned manipulated manifest should have 5 layers")

	// Verify MANIPULATED_FROM edge by looking up manipulated manifest from original digest
	manipMdFromGraph, fromFound, err := mag.Database.ManifestGetManipulatedByOriginalAndLabel(ctx, origMd.Digest, "layer/glow#1.5.0-arm64")
	assert.NoError(t, err)
	assert.True(t, fromFound, "MANIPULATED_FROM edge should exist from manipulated to original")
	if fromFound {
		assert.Equal(t, manipMd.ID, manipMdFromGraph.ID, "Found manifest via MANIPULATED_FROM should match")
	}

	// Get the generic profile (no manipulations) and verify no manipulation is applied
	genPrf, err := mag.getProfile(ctx, br, "cpu#generic")
	if err != nil {
		t.Fatal(err)
	}
	genResult, err := mag.ApplyManipulationsToManifests(ctx, genPrf, candidates)
	if err != nil {
		t.Fatalf("ApplyManipulationsToManifests for generic failed: %v", err)
	}
	assert.Equal(t, 1, len(genResult), "Generic profile should have 1 manifest")
	assert.Equal(t, origMd.ID, genResult[0].ID, "Generic profile should use the original unmanipulated manifest")

	// Dedup test: calling again should reuse the existing manipulated manifest
	result2, err := mag.ApplyManipulationsToManifests(ctx, prf, candidates)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, manipMd.ID, result2[0].ID, "Second call should reuse existing manipulated manifest (dedup)")
}
