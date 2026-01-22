package magazine

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManifestDataGetByID(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
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
	// Actual test
	mani, err := mag.getManiData(ctx, br, "manifestdata:ft_amd64")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", mani.MediaType)
	assert.NotEqual(t, 0, mani.Config.Size)
	assert.Equal(t, mani.Config.Size, int64(len(mani.Config.Data)))
	expEnv := []string{"PLATFORM_FEATURES=amd64", "ORG_SUPERCONTAINERS_HARDWARE_CPU_TARGET=platform"}
	assert.Equal(t, expEnv, mani.Config.Cfg.Config.Env)
}

func TestStoreManifestArtifactUpload(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
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
	inMani := types.FTamd64OCIManifest()
	b, err := inMani.ToBytes()
	if err != nil {
		t.Fatal(err)
	}
	dig := digest.FromBytes(b)
	// Actual test
	req := types.Request{
		Repo:    "qnib/plain-featuretest",
		Tag:     "20220715-arm64-generic",
		User:    "qnib",
		Profile: "cpu#generic",
		Digest:  dig.String(),
	}
	mag.StoreManifestArtifactUpload(ctx, req, inMani)
}
