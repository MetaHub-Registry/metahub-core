package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestRuleGetCreate(t *testing.T) {
	ctx := context.Background()
	debug := 0
	_, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestRuleGetCreate.Setup",
	})
	defer span.End()

	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer db.TeardownTest(bs, br)
	// Actual Test
	l, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestRuleGetCreate.Run",
	})
	defer span.End()
	_ = l
	r := types.Rule{
		Op:    "eq",
		Type:  "anno",
		Name:  "foo0",
		Tag:   "bar0",
		Key:   "foo0",
		Value: "bar0",
	}
	_, found, err := db.RuleGet(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	// Empty
	if _, e := db.RuleCreate(ctx, types.Rule{Type: ""}); e == nil {
		t.Fatal("Should have failed because type is empty")
	}
	/// create Rule
	ruleID, err := db.RuleCreate(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	gotRuleID, found, err := db.RuleGet(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, ruleID, gotRuleID)
	r0 := types.Rule{
		Op:    "eq",
		Name:  "foo1",
		Type:  "anno",
		Tag:   "bar1",
		Key:   "foo1",
		Value: "bar1",
	}
	ruleID, err = db.RuleGetCreate(ctx, r0)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, ruleID, gotRuleID)
	rls, err := db.RulesGetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(rls))
}

func TestRuleEvalAnnotationRule(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target:platform")
	//bs.PostTestSleep = 120
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual Test
	_, found, err := db.RuleGetByLabel(ctx, "target:platform")
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	r := types.Rule{
		Name:  "target",
		Tag:   "platform",
		Type:  "anno",
		Label: "target:platform",
		Op:    "eq",
		Key:   "org.supercontainers.hardware.cpu.target",
		Value: "platform",
	}
	rID, err := db.RuleGetCreate(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	r.ID = rID
	err = db.RuleEvalAnnotationRule(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	q := fmt.Sprintf(`MATCH (r:Rule) WHERE id(r) = %d RETURN id(r) AS rID`, rID)
	_, found, err = db.QueryOne(q)
	if err != nil || !found {
		t.Fatal("Rule not found")
	}
	gotR, found, err := db.RuleGetByLabel(ctx, "target:platform")
	if err != nil || !found {
		t.Fatal("Rule not found")
	}
	assert.Equal(t, r, gotR)
}

func TestRuleEvalAnnotationRuleNoop(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target:platform")
	//bs.PostTestSleep = 120
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual Test
	_, found, err := db.RuleGetByLabel(ctx, "target:platform")
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	r := types.Rule{
		Name:  "cpu",
		Tag:   "noop",
		Type:  "anno",
		Label: "cpu:noop",
		Op:    "null",
		Key:   "org.supercontainers.hardware.cpu.*",
		Value: "",
	}
	rID, err := db.RuleGetCreate(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	r.ID = rID
	err = db.RuleEvalAnnotationRule(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	q := fmt.Sprintf(`MATCH (r:Rule) WHERE id(r) = %d RETURN id(r) AS rID`, rID)
	_, found, err = db.QueryOne(q)
	if err != nil || !found {
		t.Fatal("Rule not found")
	}
	gotR, found, err := db.RuleGetByLabel(ctx, "cpu:noop")
	if err != nil || !found {
		t.Fatal("Rule not found")
	}
	assert.Equal(t, r, gotR)
}

func TestRuleEvalAll(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("annotation:foo:bar")
	bs.AddBootstrap("rules:rules/foo/bar.yaml")
	bs.AddBootstrap("rules:rules/platform/os/linux.yaml")
	//bs.PostTestSleep = 120
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual Test
	err = db.RuleEvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Check rules
	fbR, found, err := db.RuleGetByLabel(ctx, "foo:bar")
	if err != nil || !found {
		t.Fatal("Rule not found")
	}

	as, err := db.RuleGetSuccessAnnotations(ctx, fbR)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(as.Data))

	osR, found, err := db.RuleGetByLabel(ctx, "platform/os:linux")
	if err != nil || !found {
		t.Fatal("Rule not found")
	}

	ps, err := db.RuleGetSuccessPlatform(ctx, osR)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 6, len(ps))
}

// func TestRulesGetByManipuLayerID(t *testing.T) {
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
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
// 	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
// 	// ManiData Setup
// 	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_arm64")
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
// 		"func": "TestRulesGetByManipuLayerID.Setup",
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
// 		t.Fatal(err)
// 	}
// 	defer db.TeardownTest(bs, br)
// 	// Eval RUles
// 	if err := db.RuleEvalAll(ctx); err != nil {
// 		t.Fatal(err)
// 	}
// 	span.End()
// 	// Actual tests
// 	_, ctx, span = utils.InitLogging(context.Background(), tracer, map[string]any{
// 		"func": "TestRulesGetByManipuLayerID.Run",
// 	})
// 	defer span.End()
// 	maniArm64ID, err := br.GetInt64("manifest:ft_arm64#id")
// 	if err != nil {
// 		span.End()
// 		t.Fatal(err)
// 	}
// 	if rules, err := db.RulesGetByManifestID(ctx, maniArm64ID); err != nil {
// 		span.End()
// 		t.Fatal(err)
// 	} else {
// 		assert.Equal(t, 2, len(rules.Rules), "Manifest should have 3 rules")
// 	}
// }
