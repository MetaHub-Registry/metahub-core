package magazine

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestGetManiData(t *testing.T) {
	ctx := context.Background()
	l, c, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestGenerateOCIv1ConfigPrep.Setup",
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
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")

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
	span.End()
	///
	_, newCtx, newSpan := utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestGenerateOCIv1ConfigPrep.Run",
	})
	defer newSpan.End()
	mani, err := mag.getManiData(newCtx, br, "manifestdata:ft_amd64")
	if err != nil {
		t.Fatal(err)
	}
	if mani.ID <= 0 {
		t.Fatalf("expected manifestdata ID > 0, got %d", mani.ID)
	}
}
