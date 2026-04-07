package types

// ProfileRule holds a single rule linked to a Profile node in the graph.
type ProfileRule struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Profile struct {
	ID            int64          `json:"id"`
	Label         string         `json:"label"`
	Name          string         `json:"name"`
	Tag           string         `json:"tag"`
	Rule          string         `json:"rule"`
	Manipulations []Manipulation `json:"manipulations"`
	Rules         []ProfileRule  `json:"rules"`
}

const (
	AnnotationRuleEq    = "eq"
	AnnotationRuleNeq   = "neq"
	AnnotationRuleGt    = "gt"
	AnnotationRuleGte   = "gte"
	AnnotationRuleLt    = "lt"
	AnnotationRuleLte   = "lte"
	AnnotationRuleRegex = "re"
)

var (
	AnnotationsRules = []string{
		AnnotationRuleEq,
		AnnotationRuleNeq,
		AnnotationRuleGt,
		AnnotationRuleGte,
		AnnotationRuleLt,
		AnnotationRuleLte,
		AnnotationRuleRegex,
	}
)
