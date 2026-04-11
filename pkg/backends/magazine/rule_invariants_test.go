package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestRuleShortcutInvariants_MixedRuleTypes is the Path Z task 7.8
// regression test for the rule-shortcut taxonomy codified in design
// decision D5.
//
// It seeds **one rule of each flavour** (plat eq, anno eq, anno ne,
// anno re, anno null) against a manifestdata fixture, runs the full
// rule evaluator (RuleEvalAll), and then asserts that
// AssertRuleShortcutInvariants returns an empty violations slice —
// proving that:
//
//  1. No rule crosses the taxonomy boundaries (no rule has TRUE
//     edges to more than one node type).
//  2. The anno-null rule's TRUE edge lands directly on :Manifest
//     and NOT on any shortcut node.
//  3. The plat eq rule's TRUE edge lands on :Platform.
//  4. The three annotation-shortcut rules (eq, ne, re) all land
//     on :Annotation.
//
// If the rule evaluator ever drifts from this taxonomy (e.g. by
// mis-classifying an op and writing to the wrong target), this test
// fails with a detailed per-violation message from
// AssertRuleShortcutInvariants, which makes regressions trivial to
// diagnose.
//
// This test complements the existing per-flavour rule tests in
// pkg/backends/neo/rule_test.go and pkg/backends/magazine/compute_manifest_test.go
// which only exercise one flavour at a time.
func TestRuleShortcutInvariants_MixedRuleTypes(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("empty:all")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.SetEmptyAll()

	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	// Seed a profile with one rule of each shortcut flavour:
	//
	//   plat eq   → Platform shortcut
	//   anno eq   → Annotation shortcut
	//   anno ne   → Annotation shortcut
	//   anno re   → Annotation shortcut
	//   anno null → Manifest direct (no shortcut possible)
	//
	// The exact values don't matter for the invariant assertion —
	// the test only cares that RuleEvalAll writes TRUE edges to
	// the correct target type, not whether the predicates actually
	// match the seeded manifestdata.
	prf := createTestProfileWithRules(t, "test-mixed-rules", []types.Rule{
		{
			Type:  types.RuleTypePlatform,
			Op:    "eq",
			Key:   "arch",
			Value: "arm64",
		},
		{
			Type:  types.RuleTypeAnnotation,
			Op:    "eq",
			Key:   "org.metahub.invariant-eq",
			Value: "yes",
		},
		{
			Type:  types.RuleTypeAnnotation,
			Op:    "ne",
			Key:   "org.metahub.invariant-ne",
			Value: "no",
		},
		{
			Type:  types.RuleTypeAnnotation,
			Op:    "re",
			Key:   "org.metahub.invariant-re",
			Value: "^v.*$",
		},
		{
			Type: types.RuleTypeAnnotation,
			Op:   "null",
			Key:  "org.metahub.invariant-null",
		},
	})
	_ = prf

	// Run the rule evaluator against the whole graph so all five
	// rules get their TRUE edges materialised.
	err = mag.Database.RuleEvalAll(ctx)
	require.NoError(t, err, "RuleEvalAll must succeed before asserting the rule-shortcut invariants")

	// Now the actual assertion: AssertRuleShortcutInvariants must
	// return an empty violations slice. If it doesn't, the error
	// message includes a per-violation breakdown of which rule
	// drifted and where its TRUE edge landed, so the developer
	// can diagnose the regression without writing a cypher query.
	violations, err := mag.Database.AssertRuleShortcutInvariants(ctx)
	require.NoError(t, err, "AssertRuleShortcutInvariants must not fail")
	assert.Empty(t, violations,
		"rule shortcut taxonomy violated after RuleEvalAll with mixed rule types — see design decision D5 for the expected shape")
	for _, v := range violations {
		t.Errorf("violation: %s", v)
	}
}

// TestAssertRuleShortcutInvariants_EmptyGraph is a sanity-check that
// the invariants return zero violations on a graph with no :Rule
// nodes at all. Catches the edge case where the helper's queries
// might spuriously report violations on empty input.
func TestAssertRuleShortcutInvariants_EmptyGraph(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("empty:all")
	bs.SetEmptyAll()

	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	violations, err := mag.Database.AssertRuleShortcutInvariants(ctx)
	require.NoError(t, err)
	assert.Empty(t, violations, "empty graph should report zero rule-shortcut violations")
}
