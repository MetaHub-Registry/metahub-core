package neo

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_OCIv1Config_GetInsertDescriptor(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
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
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(bs)
	if err != nil {
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer db.TeardownTest(bs)
	// Actual test
	// Find config
	desc := v1.Descriptor{
		MediaType: "application/vnd.docker.container.image.v1+json",
		Digest:    digest.Digest("sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3"),
		Size:      2191,
	}
	cfg, found, err := db.OCIv1Config_GetInsertDescriptor(ctx, desc)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Config not found")
	}
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", cfg.MediaType)
	// Insert Config
	desc2 := v1.Descriptor{
		MediaType: "application/vnd.docker.container.image.v1+json",
		Digest:    digest.Digest("sha256:affe26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3"),
		Size:      1234,
	}
	cfg2, found2, err := db.OCIv1Config_GetInsertDescriptor(ctx, desc2)
	if err != nil {
		t.Fatal(err)
	}
	if found2 {
		t.Fatal("Config already found")
	}
	assert.Equal(t, int64(1234), cfg2.Size)
}
