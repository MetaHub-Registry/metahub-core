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
