package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// func TestCollectionTagProfilePairGetAll(t *testing.T) {
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("upstream:glow150arm64")
// 	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.AddBootstrap("layer:0:glow150arm64_2660")
// 	bs.AddBootstrap("layer:1:glow150arm64_3571")
// 	bs.AddBootstrap("layer:2:glow150arm64_d54e")
// 	bs.AddBootstrap("config:glow150arm64")
// 	bs.AddBootstrap("manifestdata:glow150arm64")
// 	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
// 	//bs.AddBootstrap("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#glow150arm64_.*")
// 	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
// 	// ManiData Setup
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	_, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{
// 		"func": "TestCollectionTagProfilePairGetAll.Setup",
// 	})
// 	defer span.End()
// 	br, err := SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		db.TeardownTest(bs, br)
// 		span.End()
// 		t.Fatal(err)
// 	}
// 	span.End()
// 	defer db.TeardownTest(bs, br)
// 	// Actual test
// 	_, ctx, span = utils.InitLogging(context.Background(), tracer, map[string]any{
// 		"func": "TestCollectionTagProfilePairGetAll.Run",
// 	})
// 	defer span.End()
// 	ctpps, err := db.CollectionTagProfilePairGetAll(ctx)
// 	if err != nil {
// 		span.End()
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 3, len(ctpps))
// 	span.End()
// }

func TestCollectionTagProfilePairAnnotationGetAll(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("empty:all")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.AddBootstrap("config:ft_zen3")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_zen3")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_zen3")
	bs.AddBootstrap("layer:0:ft_zen3")
	bs.AddBootstrap("layer:1:ft_zen3")
	bs.AddBootstrap("manifestdata:ft_zen3")
	bs.AddBootstrap("collection:ft_zen3")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	_, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestCollectionTagProfilePairAnnotationGetAll.Setup",
	})
	defer span.End()
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		db.TeardownTest(bs, br)
		span.End()
		t.Fatal(err)
	}
	err = db.RuleEvalAll(ctx)
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	// Path Z task 7.7: D5 invariant canary after every RuleEvalAll.
	assertRuleShortcutInvariantsOrFatal(t, ctx)
	// Path Z Phase C3: CTPP queries now walk via Slot instead of the
	// :ManifestWrapper wrapper. Create a Slot + HAS_PUSHED edge for the
	// bootstrapped manifest data so the Slot-based walk can find it.
	mdID, err := br.GetInt64("manifestdata:ft_zen3#id")
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	slot, err := db.SlotGetCreate(ctx, "qnib/featuretest", "zen3", "cpu#zen3")
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	err = db.SlotAppendPushedManifest(ctx, slot.ID, mdID)
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	span.End()
	defer db.TeardownTest(bs, br)
	// Actual test
	_, ctx, span = utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestCollectionTagProfilePairAnnotationGetAll.Run",
	})
	defer span.End()
	ctpps, err := db.CollectionTagProfilePairAnnotationGetAll(ctx)
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(ctpps))
	// End
	span.End()
}

// func TestCollectionTagProfilePairPlatformGetAll(t *testing.T) {
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("empty:all")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("upstream:glow150arm64")
// 	bs.AddBootstrap("layer:0:glow150arm64_2660")
// 	bs.AddBootstrap("layer:1:glow150arm64_3571")
// 	bs.AddBootstrap("layer:2:glow150arm64_d54e")
// 	bs.AddBootstrap("config:glow150arm64")
// 	bs.AddBootstrap("manifestdata:glow150arm64")
// 	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
// 	//bs.AddBootstrap("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#glow150arm64_.*")
// 	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
// 	// ManiData Setup
// 	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("collection:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("manifest:ft_amd64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
// 	bs.AddBootstrap("manifestdata:ft_amd64")

// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	_, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{
// 		"func": "TestCollectionTagProfilePairPlatformGetAll.Setup",
// 	})
// 	defer span.End()
// 	br, err := SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		db.TeardownTest(bs, br)
// 		span.End()
// 		t.Fatal(err)
// 	}
// 	err = db.RuleEvalAll(ctx)
// 	if err != nil {
// 		span.End()
// 		t.Fatal(err)
// 	}
// 	defer db.TeardownTest(bs, br)

// 	if err := db.RuleEvalAll(ctx); err != nil {
// 		span.End()
// 		t.Fatal(err)
// 	}
// 	span.End()
// 	// Actual test
// 	_, ctx, span = utils.InitLogging(context.Background(), tracer, map[string]any{
// 		"func": "TestCollectionTagProfilePairPlatformGetAll.Run",
// 	})
// 	defer span.End()
// 	ctpps, err := db.CollectionTagProfilePairPlatformGetAll(ctx)
// 	if err != nil {
// 		span.End()
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 1, len(ctpps))
// 	// End
// 	span.End()
// }
