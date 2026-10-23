package magazine

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestCollectionHeadManifestNotFoundManifest(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	_, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestCollectionHeadManifestNotFoundManifest.Setup",
	})
	defer span.End()
	br, err := mag.SetupTest(ctx, bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		mag.Database.TeardownTest(bs, br)
		span.End()
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs, br)
	// Actual test
	_, ctx, span = utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestCollectionHeadManifestNotFoundManifest.Run",
	})
	defer span.End()
	reqGen := types.Request{
		Host:    "localhost",
		Repo:    "alpine",
		Tag:     "manifest",
		Profile: "generic",
	}
	_, _, e := mag.Database.CollectionGetCreateByRepoTag(ctx, reqGen.Repo, reqGen.Tag)
	if e != nil {
		span.End()
		t.Fatal(e)
	}

	_, _, err = mag.HeadManifestNotFound(ctx, reqGen)
	if err != nil {
		span.End()
		t.Fatal(err)
	}

	// req3 := types.Request{
	// 	Host:    "localhost",
	// 	Repo:    "alpine",
	// 	Tag:     "manifest",
	// 	Profile: "cpu#zen3",
	// }
	// _, _, err = mag.HeadManifestNotFound(ctx, req3)
	// if err != nil {
	// 	span.End()
	// 	t.Fatal(err)
	// }
	span.End()
}
