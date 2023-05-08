package magazine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManifestlistCompute(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
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
		TeardownTest(bs)
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Actual Test
	err = mag.Database.Rule_EvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	maniID, err := br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	prfID, err := br.GetInt64("profile:cpu:generic#id")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Profile ID: %d // ManiID: %d", prfID, maniID)
	mani, found, err := mag.Database.Manifest_GetByID(ctx, maniID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Manifest not found")
	}
	// Actual test
	err = mag.ComputeManifestForProfile(ctx, mani, prfID)
	if err != nil {
		t.Fatal(err)
	}
	err = mag.Database.Rule_EvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if e := mag.ComputeManifestlists(ctx); e != nil {
		t.Fatal(e)
	}
	// TODO: Remove Collection w/ Manifestlist
	req := types.Request{
		Repo: "qnib/featuretest",
		Tag:  "generic",
	}
	found, err = mag.RemoveCollectionByRequest(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
}
