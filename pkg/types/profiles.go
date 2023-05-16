package types

type Profile struct {
	ID            int64          `json:"id"`
	Label         string         `json:"label"`
	Name          string         `json:"name"`
	Tag           string         `json:"tag"`
	Rule          string         `json:"rule"`
	Manipulations []Manipulation `json:"manipulations"`
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
