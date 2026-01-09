package magazine

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestCollectionHeadManifestNotFoundManifest(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
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
	br, err := mag.SetupTest(ctx, bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		mag.Database.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs, br)
	// Actual test
	reqGen := types.Request{
		Host:    "localhost",
		Repo:    "alpine",
		Tag:     "manifest",
		Profile: "generic",
	}
	_, _, e := mag.Database.CollectionGetCreateByRepoTag(ctx, reqGen.Repo, reqGen.Tag)
	if e != nil {
		t.Fatal(e)
	}
	_, _, err = mag.HeadManifestNotFound(ctx, reqGen)
	if err != nil {
		t.Fatal(err)
	}

	req3 := types.Request{
		Host:    "localhost",
		Repo:    "alpine",
		Tag:     "manifest",
		Profile: "cpu#zen3",
	}
	_, _, err = mag.HeadManifestNotFound(ctx, req3)
	if err != nil {
		t.Fatal(err)
	}

}
