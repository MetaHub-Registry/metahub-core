package magazine

import (
	"context"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestCreateOCIv1PulledByPrf(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
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
	mani, err := mag.getMani(ctx, br, "manifest:ft_amd64")
	if err != nil {
		t.Fatal(err)
	}
	maniData, err := mag.getManiData(ctx, br, "manifestdata:ft_amd64")
	if err != nil {
		t.Fatal(err)
	}
	// Generic
	cfgGen, e := mag.GenerateOCIv1Config(ctx, mani, maniData)
	if e != nil {
		t.Fatal(e)
	}
	assert.Equal(t, []string{"ORG_SUPERCONTAINERS_HARDWARE_CPU_TARGET=platform", "PLATFORM_FEATURES=amd64"}, cfgGen.Config.Env)
	// Zen3
	prfLabel := "cpu#zen3"
	prf, err := mag.getProfile(ctx, br, prfLabel)
	if err != nil {
		t.Fatal(err)
	}
	ociManiDataID, _, e := mag.CreateOCIv1PulledByPrf(ctx, mani, maniData, prf)
	if e != nil {
		t.Fatal(e)
	}
	ociManiData, zen3Found, e := mag.ManifestGetByID(ctx, ociManiDataID)
	if e != nil {
		t.Fatal(e)
	}
	if !zen3Found {
		t.Fatal("Did not find manifestdata")
	}
	expEnv := []string{
		"ORG_SUPERCONTAINERS_HARDWARE_CPU_TARGET=platform",
		"PLATFORM_FEATURES=amd64",
		"ORG_SUPERCONTAINERS_HARDWARE_CPU_TARGET_MICROARCH=zen3",
	}
	assert.Equal(t, expEnv, ociManiData.Config.Cfg.Config.Env)
}

func TestGenerateOCIv1Config(t *testing.T) {
	ctx := context.Background()
	l, c, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestGenerateOCIv1Config",
	})
	defer span.End()
	ctx = c
	_ = l
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
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
	mani, err := mag.getMani(ctx, br, "manifest:ft_amd64")
	if err != nil {
		t.Fatal(err)
	}
	maniData, err := mag.getManiData(ctx, br, "manifestdata:ft_amd64")
	if err != nil {
		t.Fatal(err)
	}
	cfgGen, e := mag.GenerateOCIv1Config(ctx, mani, maniData)
	if e != nil {
		t.Fatal(e)
	}
	assert.Equal(t, []string{"ORG_SUPERCONTAINERS_HARDWARE_CPU_TARGET=platform", "PLATFORM_FEATURES=amd64"}, cfgGen.Config.Env)
}

func TestOCICreateBaseManifestForGlow(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:glow151arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("layer:0:glow151arm64_7c57")
	bs.AddBootstrap("layer:1:glow151arm64_8a17")
	bs.AddBootstrap("layer:2:glow151arm64_a24c")
	bs.AddBootstrap("config:glow151arm64")
	bs.AddBootstrap("manifestdata:glow151arm64")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.1-arm64.yaml")
	bs.AddBootstrap("profile:profiles/glow/1.5.1-arm64.yaml")
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
		t.Logf("Error while SetupTest: %v", err)
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Prepare
	if e := mag.Database.RuleEvalAll(ctx); e != nil {
		t.Fatal(e)
	}
	// Path Z task 7.7: D5 invariant canary after every RuleEvalAll.
	assertRuleShortcutInvariantsOrFatal(t, ctx)
	// Actual test
	prf, err := mag.getProfile(ctx, br, "glow#1.5.1-arm64")
	if err != nil {
		t.Fatal(err)
	}
	origMani, err := mag.getManiData(ctx, br, "manifestdata:ft_arm64")
	if err != nil {
		t.Fatal(err)
	}
	mani, err := mag.getMani(ctx, br, "manifest:ft_arm64")
	if err != nil {
		t.Fatal(err)
	}
	ociManiDataID, err := mag.OCICreateBaseManifest(ctx, mani, origMani, prf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ociManiData, found, e := mag.ManifestGetByID(ctx, ociManiDataID); e != nil {
		t.Fatal(e)
	} else if !found {
		t.Fatal("Did not find OCI manifestdata")
	} else {
		assert.Equal(t, v1.MediaTypeImageManifest, ociManiData.MediaType)
		assert.Equal(t, 5, len(ociManiData.Layers))
	}
}
