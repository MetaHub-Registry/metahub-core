package magazine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestCacheLayers(t *testing.T) {
	bs := utils.NewBootstrap()
	_, err := mag.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Test
	mani := types.FT_arm_ManifestData()
	err = mag.CacheLayers(ctx, mani)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mani.Digest)
	// Check if layers are cached
	for _, l := range mani.Layers {
		psUrl, err := mag.Minio.LayerGetPreSignedUrl(ctx, l.Digest)
		if err != nil {
			t.Fatal(err)
		}
		assert.NotEmpty(t, psUrl)
	}
}

func TestBaseLayerEval(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_zen")
	bs.AddBootstrap("upstream:ft_zen")
	bs.AddBootstrap("manifest:ft_zen")
	bs.AddBootstrap("layer:0:ft_zen_f144")
	bs.AddBootstrap("layer:1:ft_zen_4c85")
	bs.AddBootstrap("manifestdata:ft_zen_5c99")
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
	// Actual Test
	md, e := mag.getManiData(ctx, br, "manifestdata:ft_zen_5c99")
	if e != nil {
		t.Fatal(e)
	}
	if len(md.Layers) != 2 {
		t.Fatalf("Expected 2 layers, but got %d", len(md.Layers))
	}
	for idx, lyr := range md.Layers {
		if idx != 0 {
			continue
		}
		t.Logf("Eval layer: %s", lyr.Digest)
		os, err := mag.BaseLayerEvalOS(ctx, lyr)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "alpine", os.ID)
	}

}
