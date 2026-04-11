package magazine

import (
	"context"
	"testing"
)

// assertRuleShortcutInvariantsOrFatal is the magazine-package test-layer
// shim around magazine.Database.AssertRuleShortcutInvariants. Same
// semantics as the neo-package helper: runs the four D5 invariants and
// calls t.Fatalf with per-violation diagnostics if any invariant is
// breached.
//
// Path Z task 7.7 follow-up — rolled out into every magazine test that
// runs RuleEvalAll, giving the rule evaluator the same canary coverage
// across the magazine-layer call graph as it has at the neo-layer.
//
// Usage:
//
//	err := mag.Database.RuleEvalAll(ctx)
//	require.NoError(t, err)
//	assertRuleShortcutInvariantsOrFatal(t, ctx)
//
// The helper is deliberately void-return + fatal-on-violation so call
// sites stay a single line. Tests that want to inspect the violation
// strings directly (or assert there should be N violations for a
// negative test) should call mag.Database.AssertRuleShortcutInvariants
// directly instead.
func assertRuleShortcutInvariantsOrFatal(t *testing.T, ctx context.Context) {
	t.Helper()
	violations, err := mag.Database.AssertRuleShortcutInvariants(ctx)
	if err != nil {
		t.Fatalf("AssertRuleShortcutInvariants failed to run: %v", err)
	}
	if len(violations) == 0 {
		return
	}
	t.Errorf("rule shortcut taxonomy violated (%d violations) — see design decision D5 in path-z-design-decisions.md for the expected shape:", len(violations))
	for _, v := range violations {
		t.Errorf("  violation: %s", v)
	}
	t.FailNow()
}
