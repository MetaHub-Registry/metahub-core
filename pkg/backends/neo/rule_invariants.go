// Package neo rule_invariants.go implements the four invariants from
// Path Z design decision D5 (rule shortcut taxonomy) as a set of
// cypher queries that can be run after RuleEvalAll to catch regressions
// in the rule evaluator.
//
// Four rule flavours produce four distinct :TRUE edge shapes:
//
//	plat eq/ne   → (Rule)-[:TRUE]->(Platform)      shortcut via Platform node
//	anno eq/ne/re→ (Rule)-[:TRUE]->(Annotation)    shortcut via Annotation node
//	anno null    → (Rule)-[:TRUE]->(Manifest)  direct (no shortcut possible)
//
// AssertRuleShortcutInvariants runs four cypher queries to catch every
// way the rule evaluator could drift from this taxonomy. Each query
// returns zero rows when the invariant holds and non-zero rows when it
// fails; failures are collected into a []string of human-readable
// violation messages that the caller can assert on.
//
// Usage from tests:
//
//	err := db.RuleEvalAll(ctx)
//	require.NoError(t, err)
//	violations, err := db.AssertRuleShortcutInvariants(ctx)
//	require.NoError(t, err)
//	assert.Empty(t, violations, "rule shortcut taxonomy violated")
//
// See design decision D5 in
// the MetaHub-Registry/openspec change refactor-graph-model-slot-centred
// (design.md, D5)
// and [[docs/path-z-design-decisions#D5]] in the Obsidian vault.
package neo

import (
	"context"
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// AssertRuleShortcutInvariants runs the four D5 invariant queries and
// returns a slice of human-readable violation messages, one per
// violation found. An empty slice means the rule evaluator is
// producing :TRUE edges that conform to the taxonomy.
//
// The four invariants are:
//
//  1. No `:Rule` has `:TRUE` edges to more than one node type. A Rule
//     is either a Platform rule, an Annotation rule, or a direct-
//     Manifest rule — never mixed. Mixed edges indicate the evaluator
//     mis-classified the rule's op field and wrote to the wrong target.
//
//  2. `anno null` rules have `:TRUE` edges only to `:Manifest` (the
//     content-leaf, currently still labelled :Manifest in cypher
//     until Phase C3 renames it to :ManifestWrapper). Absence rules have no
//     shortcut target — there's no `:Annotation {key: X}` node for
//     "manifests where key X is absent", so the evaluator must write
//     directly to the content-leaf. A `:TRUE` edge from an `anno null`
//     rule to a `:Platform` or `:Annotation` is a bug.
//
//  3. `plat eq`/`plat ne` rules have `:TRUE` edges only to `:Platform`.
//     The evaluator's Platform-rule code path walks
//     `(r:Rule)-[:TRUE]->(p:Platform)<-[:HAS_PLATFORM]-(md)` to get
//     the matching manifests, and that walk depends on the edge
//     landing on the Platform node, not on the content-leaf.
//
//  4. `anno eq`/`anno ne`/`anno re` rules have `:TRUE` edges only to
//     `:Annotation`. Same rationale as #3 — the evaluator's
//     Annotation-rule walk depends on the shortcut target.
//
// Implementation note: the queries walk `:Manifest` (the pre-Phase-C3
// label name) not `:ManifestWrapper`. Phase C3 will rename this in lockstep
// with the wrapper deletion.
func (n Neo) AssertRuleShortcutInvariants(ctx context.Context) (violations []string, err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/rule_invariants.go",
		"func":    "AssertRuleShortcutInvariants",
	})
	defer span.End()

	// Invariant 1: no Rule has :TRUE edges to more than one node type.
	//
	// Classify each Rule's TRUE-edge targets by label and count the
	// distinct labels per Rule. A correctly-classified Rule has exactly
	// 1 distinct label; a Rule with 2+ distinct labels is mixed.
	q1 := `
		MATCH (r:Rule)-[:TRUE]->(target)
		WITH r, collect(DISTINCT labels(target)[0]) AS targetLabels
		WHERE size(targetLabels) > 1
		RETURN r.label AS ruleLabel, r.type AS ruleType, r.op AS ruleOp, targetLabels
	`
	utils.Debug(l, span, "AssertRuleShortcutInvariants invariant 1: mixed targets")
	mixedRs, err := n.Query(q1)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "invariant 1 query failed")
	}
	for _, r := range mixedRs {
		ruleLabel, _ := r.GetStr("ruleLabel")
		ruleType, _ := r.GetStr("ruleType")
		ruleOp, _ := r.GetStr("ruleOp")
		violations = append(violations,
			fmt.Sprintf("invariant 1 (no mixed targets): Rule %q (type=%s, op=%s) has :TRUE edges to multiple node types",
				ruleLabel, ruleType, ruleOp))
	}

	// Invariant 2: `anno null` rules point ONLY at :Manifest.
	//
	// Find any :TRUE edge from an anno-null rule to a non-Manifest
	// target. The evaluator should never produce these; if it does,
	// the shortcut classification code has a bug.
	q2 := `
		MATCH (r:Rule {type: "anno", op: "null"})-[:TRUE]->(target)
		WHERE NOT target:Manifest
		RETURN r.label AS ruleLabel, labels(target) AS targetLabels, id(target) AS targetID
	`
	utils.Debug(l, span, "AssertRuleShortcutInvariants invariant 2: anno-null targets")
	nullMisses, err := n.Query(q2)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "invariant 2 query failed")
	}
	for _, r := range nullMisses {
		ruleLabel, _ := r.GetStr("ruleLabel")
		targetID, _ := r.GetInt64("targetID")
		violations = append(violations,
			fmt.Sprintf("invariant 2 (anno null → :Manifest only): Rule %q has :TRUE edge to non-Manifest node id=%d",
				ruleLabel, targetID))
	}

	// Invariant 3: `plat eq`/`plat ne` rules point ONLY at :Platform.
	q3 := `
		MATCH (r:Rule {type: "plat"})-[:TRUE]->(target)
		WHERE NOT target:Platform
		RETURN r.label AS ruleLabel, r.op AS ruleOp, labels(target) AS targetLabels, id(target) AS targetID
	`
	utils.Debug(l, span, "AssertRuleShortcutInvariants invariant 3: plat rule targets")
	platMisses, err := n.Query(q3)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "invariant 3 query failed")
	}
	for _, r := range platMisses {
		ruleLabel, _ := r.GetStr("ruleLabel")
		ruleOp, _ := r.GetStr("ruleOp")
		targetID, _ := r.GetInt64("targetID")
		violations = append(violations,
			fmt.Sprintf("invariant 3 (plat %s → :Platform only): Rule %q has :TRUE edge to non-Platform node id=%d",
				ruleOp, ruleLabel, targetID))
	}

	// Invariant 4: `anno eq`/`ne`/`re` rules point ONLY at :Annotation.
	//
	// Note: the cypher WHERE clause excludes op="null" explicitly because
	// anno-null is covered by invariant 2 above and has its own target
	// rules (direct to :Manifest).
	q4 := `
		MATCH (r:Rule {type: "anno"})-[:TRUE]->(target)
		WHERE r.op <> "null" AND NOT target:Annotation
		RETURN r.label AS ruleLabel, r.op AS ruleOp, labels(target) AS targetLabels, id(target) AS targetID
	`
	utils.Debug(l, span, "AssertRuleShortcutInvariants invariant 4: anno (non-null) rule targets")
	annoMisses, err := n.Query(q4)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "invariant 4 query failed")
	}
	for _, r := range annoMisses {
		ruleLabel, _ := r.GetStr("ruleLabel")
		ruleOp, _ := r.GetStr("ruleOp")
		targetID, _ := r.GetInt64("targetID")
		violations = append(violations,
			fmt.Sprintf("invariant 4 (anno %s → :Annotation only): Rule %q has :TRUE edge to non-Annotation node id=%d",
				ruleOp, ruleLabel, targetID))
	}

	if len(violations) == 0 {
		utils.Debug(l, span, "AssertRuleShortcutInvariants: all four invariants hold")
	} else {
		utils.Debug(l, span, "AssertRuleShortcutInvariants: %d violations", len(violations))
	}
	return violations, nil
}
