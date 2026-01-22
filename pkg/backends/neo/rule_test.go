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
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
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

func TestRulesGetByManipuLayerID(t *testing.T) {
	//
}
