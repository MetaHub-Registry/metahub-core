package magazine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestComputeManifestlistWithManipulation(t *testing.T) {
	// Replicate the following test:
	/// ./misc/bin/parse-glow.sh ./misc/yaml/collections/qnib/featuretest/arm64.yaml
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
	// ManiData Setup
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
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
	// Connect Collection/Tag
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatalf("Failed to get manifest:ft_arm64#id: %v", err)
	}
	colID, err := br.GetInt64("collection:ft#id")
	if err != nil {
		t.Fatalf("Failed to get collection:ft#id: %v", err)
	}
	tagID, err := br.GetInt64("tag:arm64#id")
	if err != nil {
		t.Fatalf("Failed to get tag:arm64#id: %v", err)
	}
	if e := mag.Database.LinkCollectionTagToManifest(ctx, tagID, maniID); e != nil {
		t.Fatalf("Failed to LinkCollectionTagToManifest: %v", e)
	}
	if e := mag.Database.LinkCollectionToManifest(ctx, colID, maniID); e != nil {
		t.Fatalf("Failed to LinkCollectionToManifest: %v", e)
	}
	// Compute Manifest is done in the TestComputeManifestWithManipulation
	if e := mag.ComputeAllManifests(ctx); e != nil {
		t.Fatalf("Failed to ComputeAllManifests: %v", e)
	}
	// Actual test
	if e := mag.ComputeManifestlists(ctx); e != nil {
		t.Fatalf("Failed to ComputeManifestlists: %v", e)
	}
	q := `MATCH (ml:ManifestList) RETURN count(ml) as count`
	if res, _, err := mag.Database.QueryOne(q); err != nil {
		t.Fatalf("Failed to QueryOne: %v", err)
	} else {
		count, err := res.GetInt("count")
		if err != nil {
			t.Fatalf("Failed to GetInt: %v", err)
		}
		// TODO: computeManifestList should yield 2 manifest lists.
		// one for each profile and not a combined one
		assert.Equal(t, 0, count)
	}
	// ManifestDataGetByProfileMediaType
	mArm64ID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatalf("Failed to get manifest:ft_arm64#id: %v", err)
	}
	prfID, err := br.GetInt64("profile:glow:1.5.0-arm64#id")
	if err != nil {
		t.Fatalf("Failed to get profile:glow:1.5.0-arm64#id: %v", err)
	}
	mdArm64ID, err := br.GetInt64("manifestdata:ft_arm64#id")
	if err != nil {
		t.Fatalf("Failed to get manifestdata:ft_arm64#id: %v", err)
	}
	if gotMdID, found, err := mag.Database.ManifestDataGetByProfileMediaType(ctx, mArm64ID, prfID, []string{"application/vnd.docker.distribution.manifest.v2+json"}); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatalf("Should be found got:%d", gotMdID)
	} else {
		assert.Equal(t, mdArm64ID, gotMdID)
	}
}
