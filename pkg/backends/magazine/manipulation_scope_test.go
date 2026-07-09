package magazine

import (
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

// TestScopeManipulationsToModel covers the slice filter that drops other models'
// manipulations from a model-centric compose: empty reqModel is a no-op; a set
// reqModel keeps only its own model/config plus non-model overlays.
func TestScopeManipulationsToModel(t *testing.T) {
	mps := []types.Manipulation{
		{Label: "model/qnib/minicpm5"},
		{Label: "config/qnib/minicpm5"},
		{Label: "model/e2e-gguf116"},
		{Label: "config/e2e-gguf116"},
		{Label: "glow"},
	}
	if got := scopeManipulationsToModel(mps, ""); len(got) != len(mps) {
		t.Fatalf("empty reqModel must keep all %d, got %d", len(mps), len(got))
	}
	got := scopeManipulationsToModel(mps, "qnib/minicpm5")
	labels := map[string]bool{}
	for _, m := range got {
		labels[m.Label] = true
	}
	for _, want := range []string{"model/qnib/minicpm5", "config/qnib/minicpm5", "glow"} {
		if !labels[want] {
			t.Errorf("scoped set missing %q (got %v)", want, labels)
		}
	}
	if labels["model/e2e-gguf116"] || labels["config/e2e-gguf116"] {
		t.Errorf("another model leaked into the scoped set: %v", labels)
	}
	if len(got) != 3 {
		t.Fatalf("scoped len = %d, want 3 (%v)", len(got), labels)
	}
}

// TestManipulationInScopeForModel is the model-scoping decision that stops a
// profile hosting several models from stacking the wrong one: model/<name> and
// config/<name> manipulations are in scope only for their own model; anything
// else is unaffected; and an empty requested model disables scoping entirely
// (pre-model-centric behaviour, zero regression).
func TestManipulationInScopeForModel(t *testing.T) {
	cases := []struct {
		name     string
		label    string
		reqModel string
		want     bool
	}{
		{"no requested model → model in scope", "model/qnib/minicpm5", "", true},
		{"no requested model → config in scope", "config/e2e-gguf116", "", true},
		{"model matches requested", "model/qnib/minicpm5", "qnib/minicpm5", true},
		{"config matches requested", "config/qnib/minicpm5", "qnib/minicpm5", true},
		{"other model's weights excluded", "model/e2e-gguf116", "qnib/minicpm5", false},
		{"other model's config excluded", "config/e2e-gguf116", "qnib/minicpm5", false},
		{"non-model/config label always in scope", "glow", "qnib/minicpm5", true},
		{"prefix-collision is not a match", "model/qnib/minicpm5-xl", "qnib/minicpm5", false},
	}
	for _, tc := range cases {
		if got := manipulationInScopeForModel(tc.label, tc.reqModel); got != tc.want {
			t.Errorf("%s: manipulationInScopeForModel(%q, %q) = %v, want %v",
				tc.name, tc.label, tc.reqModel, got, tc.want)
		}
	}
}
