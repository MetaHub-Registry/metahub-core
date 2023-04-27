package magazine

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Collection_HeadManifestNotFound_Manifest(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen2.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen2")
	bs.AddBootstrap("profile:profiles/cpu/zen2.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := mag.SetupTest(ctx, bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		mag.Database.TeardownTest(bs)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs)
	// Actual test
	req2 := types.Request{
		Host:    "localhost",
		Repo:    "alpine",
		Tag:     "manifest",
		Profile: "cpu#zen2",
	}
	_, _, err = mag.HeadManifestNotFound(ctx, req2)
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

/*
func Test_Collection_HeadManifestNotFound_ManifestList(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := mag.SetupTest(ctx, bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		mag.Database.TeardownTest(bs)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs)
	// Actual test
	req := types.Request{
		Host:    "localhost",
		Repo:    "alpine",
		Tag:     "3.17.2",
		Profile: "cpu#zen3",
	}
	err = mag.HeadManifestNotFound(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
}
*/
