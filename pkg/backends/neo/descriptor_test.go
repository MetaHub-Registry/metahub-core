package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestLinkSubject(t *testing.T) {
	ctx := context.Background()
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
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	// Actual test
	refID, err := br.GetInt64(`manifestdata:ft_arm64#id`)
	if err != nil {
		t.Fatal(err)
	}
	subDigest, err := br.Get(`manifestdata:ft_amd64#digest`)
	if err != nil {
		t.Fatal(err)
	}
	subMediaType, err := br.Get(`manifestdata:ft_amd64#mediaType`)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.LinkSubjectByDigestMediaType(ctx, refID, subDigest, subMediaType); err != nil {
		t.Fatal(err)
	}
	res, err := db.ReferrersGetBySubjectDigest(ctx, subDigest)
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, res.Manifests, 1)
}
