package neo

import (
	"context"
	"testing"
)

// assertRuleShortcutInvariantsOrFatal is the test-layer shim around
// Neo.AssertRuleShortcutInvariants. It runs the four D5 invariants as
// cypher queries against the test-scoped db, and calls t.Fatalf with a
// per-violation diagnostic if any invariant is breached.
//
// Path Z task 7.7 follow-up: this helper is the canary that every
// rule-eval test runs immediately after RuleEvalAll. A regression in
// the evaluator (mis-classifying a rule op, writing to the wrong
// shortcut target, producing mixed TRUE edges) surfaces as a fatal
// failure in whichever test was running when the bad edge was
// materialised, rather than silently flowing through to a downstream
// assertion that only catches the symptom.
//
// Usage (after any call that materialises :TRUE edges — typically
// `db.RuleEvalAll(ctx)` but also direct RuleEvalAnnotation* / Platform
// calls from the internal test bypass):
//
//	err := db.RuleEvalAll(ctx)
//	require.NoError(t, err)
//	assertRuleShortcutInvariantsOrFatal(t, ctx)
//
// The helper is deliberately void-return + fatal-on-violation so call
// sites stay a single line. Tests that want to inspect the violation
// strings directly (or assert there should be N violations for a
// negative test) should call db.AssertRuleShortcutInvariants directly
// instead.
func assertRuleShortcutInvariantsOrFatal(t *testing.T, ctx context.Context) {
	t.Helper()
	violations, err := db.AssertRuleShortcutInvariants(ctx)
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
