package utils

import (
	"fmt"

	lep "github.com/mgudov/logic-expression-parser"
)

// RuleSpecificity scores how specific an override rule is, so the most-specific
// matching rule wins (ties broken by declaration order at the call site). The
// score is a leaf count over the parsed AST:
//
//	comparison (leaf) = 1
//	A && B            = specificity(A) + specificity(B)   // more constraints ⇒ more specific
//	A || B            = min(specificity(A), specificity(B)) // an OR is only as specific as its weakest branch
//
// Override rules address a profile's hardware.* fields (e.g. "hardware.gpu.vram_gb >= 100")
// and are parsed raw — they do not use the EncodeExpression label hack.
func RuleSpecificity(rule string) (int, error) {
	expr, err := lep.ParseExpression(rule)
	if err != nil {
		return 0, fmt.Errorf("failed to parse override rule %q: %w", rule, err)
	}
	return ruleSpecificity(expr), nil
}

func ruleSpecificity(expr lep.Expression) int {
	switch e := expr.(type) {
	case *lep.AndX:
		sum := 0
		for _, c := range e.Conjuncts {
			sum += ruleSpecificity(c)
		}
		return sum
	case *lep.OrX:
		min := -1
		for _, d := range e.Disjunctions {
			if s := ruleSpecificity(d); min < 0 || s < min {
				min = s
			}
		}
		if min < 0 {
			return 0
		}
		return min
	default:
		// Any comparison node (EqualsX, GreaterThanX, …) is one constraint.
		return 1
	}
}

// EvalHardwareRule parses a hardware override rule (e.g.
// `hardware.gpu.vram_gb >= 100 && hardware.gpu.vendor = "amd"`) and evaluates it
// against a profile's flattened hardware fields. fields maps a dotted field name
// (exactly as written in the rule) to its value: int / int64 / float64 for numbers,
// string for text, bool for booleans. A comparison against an absent field, a type
// mismatch, or an ordering comparison on a string all evaluate to false.
// Note: lep's equality operator is a single `=` (not `==`).
func EvalHardwareRule(rule string, fields map[string]any) (bool, error) {
	expr, err := lep.ParseExpression(rule)
	if err != nil {
		return false, fmt.Errorf("failed to parse override rule %q: %w", rule, err)
	}
	return evalHardware(expr, fields), nil
}

func evalHardware(expr lep.Expression, f map[string]any) bool {
	switch e := expr.(type) {
	case *lep.AndX:
		for _, c := range e.Conjuncts {
			if !evalHardware(c, f) {
				return false
			}
		}
		return true
	case *lep.OrX:
		for _, d := range e.Disjunctions {
			if evalHardware(d, f) {
				return true
			}
		}
		return false
	case *lep.EqualsX:
		return cmpHardware(e.Param, e.Value, f, "eq")
	case *lep.NotEqualsX:
		return cmpHardware(e.Param, e.Value, f, "neq")
	case *lep.GreaterThanX:
		return cmpHardware(e.Param, e.Value, f, "gt")
	case *lep.GreaterThanEqualX:
		return cmpHardware(e.Param, e.Value, f, "gte")
	case *lep.LessThanX:
		return cmpHardware(e.Param, e.Value, f, "lt")
	case *lep.LessThanEqualX:
		return cmpHardware(e.Param, e.Value, f, "lte")
	default:
		return false
	}
}

func cmpHardware(param *lep.ParamX, val lep.Value, f map[string]any, op string) bool {
	fv, ok := f[param.Name]
	if !ok {
		return false
	}
	switch lit := val.(type) {
	case *lep.StringX:
		s, ok := fv.(string)
		if !ok {
			return false
		}
		switch op {
		case "eq":
			return s == lit.Val
		case "neq":
			return s != lit.Val
		default:
			return false // ordering on strings is unsupported
		}
	case *lep.IntegerX:
		return cmpNumeric(fv, float64(lit.Val), op)
	case *lep.FloatX:
		return cmpNumeric(fv, lit.Val, op)
	case *lep.BooleanX:
		b, ok := fv.(bool)
		if !ok {
			return false
		}
		switch op {
		case "eq":
			return b == lit.Val
		case "neq":
			return b != lit.Val
		default:
			return false
		}
	default:
		return false
	}
}

func cmpNumeric(fv any, lit float64, op string) bool {
	x, ok := toFloat(fv)
	if !ok {
		return false
	}
	switch op {
	case "eq":
		return x == lit
	case "neq":
		return x != lit
	case "gt":
		return x > lit
	case "gte":
		return x >= lit
	case "lt":
		return x < lit
	case "lte":
		return x <= lit
	default:
		return false
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}
