package types

import "github.com/metahub-registry/metahub-core/pkg/utils"

// Override is one entry in a models/ override list (for quant or config): either a
// profile-alias pin or a hardware rule, carrying the value to apply. Exactly one of
// Profile / Rule is set; Profile takes precedence.
type Override[T any] struct {
	Profile string // explicit profile-alias pin (highest precedence)
	Rule    string // hardware.* predicate, evaluated when Profile is empty
	Value   T
}

// ResolveOverride returns the winning override value for a profile, resolved by the
// shared grammar: an exact Profile-alias pin wins outright; otherwise the
// highest-specificity Rule that matches the profile's hardware fields, ties broken
// by declaration order (first declared wins). ok is false when nothing matches, so
// the caller falls back to its default. hwFields comes from Hardware.FieldMap.
func ResolveOverride[T any](ovs []Override[T], profileLabel string, hwFields map[string]any) (value T, ok bool) {
	var zero T
	// 1. Exact profile-alias pin wins outright.
	for _, o := range ovs {
		if o.Profile != "" && o.Profile == profileLabel {
			return o.Value, true
		}
	}
	// 2. Highest-specificity matching rule; ties → first declared.
	best := -1
	bestSpec := -1
	for i := range ovs {
		o := ovs[i]
		if o.Rule == "" {
			continue
		}
		match, err := utils.EvalHardwareRule(o.Rule, hwFields)
		if err != nil || !match {
			continue
		}
		spec, err := utils.RuleSpecificity(o.Rule)
		if err != nil {
			continue
		}
		if spec > bestSpec {
			bestSpec = spec
			best = i
		}
	}
	if best >= 0 {
		return ovs[best].Value, true
	}
	return zero, false
}
