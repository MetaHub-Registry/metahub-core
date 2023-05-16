package magazine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManipulationCreateOCI(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("upstream:glow150amd64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/platform/arch/amd64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0.yaml")
	bs.AddBootstrap("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#glow150arm64_.*")
	bs.AddBootstrap("profile:profiles/glow/1.5.0.yaml")
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
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		t.Logf("Error while SetupTest: %v", err)
		TeardownTest(bs)
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Actual test
	mani, err := mag.getMani(ctx, br, "manifest:ft_arm64")
	if err != nil {
		t.Fatal(err)
	}
	if len(mani.Manifests) != 1 {
		t.Fatalf("Expected 1 ManifestData, but got %d", len(mani.Manifests))
	}
	maniData := mani.Manifests[0]
	if maniData.Type == "" {
		t.Fatal("maniData.Type is empty")
	}
	// Glow
	prfLabel := "glow#1.5.0"
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
	if e := mag.Database.ManipulationUnwind(ctx, mani.ID, maniData.ID, prf.ID); e != nil {
		t.Fatal(e)
	}
	maniDataUnwound, mdFound, e := mag.Database.ManifestDataGetByDigest(ctx, maniData.Digest)
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
	assert.Equal(t, []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "PLATFORM_FEATURES=arm64", "SKIP_ENTRYPOINTS=true"}, cfgGen.Config.Env)
	maniDataUnwoundID, e := mag.CreateOCIv1ForProfile(ctx, mani, maniDataUnwound, prf)
	if e != nil {
		t.Fatal(e)
	}
	ociManiData, glowFound, e := mag.ManifestData_Get_ByID(ctx, maniDataUnwoundID)
	if e != nil {
		t.Fatal(e)
	}
	if !glowFound {
		t.Fatal("Did not find manifestdata")
	}
	if len(ociManiData.Layers) != 5 {
		t.Fatal("Expected 5 layers")
	}
	expEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"PLATFORM_FEATURES=arm64", "SKIP_ENTRYPOINTS=true",
	}
	assert.Equal(t, expEnv, ociManiData.Config.Cfg.Config.Env)
}
