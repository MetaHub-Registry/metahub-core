package types

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

/*
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
*/
type Filter struct {
	ID    int64
	Name  string
	Rules []Rule
}

func NewEmptyFilter(name string) (f Filter) {
	f = Filter{
		Name:  name,
		Rules: make([]Rule, 0),
	}
	return
}

func NewFilter(name, typ, op, k, v string) (f Filter, err error) {
	f = NewEmptyFilter(name)
	if err != nil {
		return
	}
	r, err := NewRule(typ, op, k, v)
	if err != nil {
		return
	}
	err = f.AddRule(r)
	return
}

func (f *Filter) AddRule(r Rule) (err error) {
	if ValidateRule(r.Op) {
		f.Rules = append(f.Rules, r)
	} else {
		return fmt.Errorf("rule '%s' is not supported (%s)", r.Op, strings.Join(AnnotationsRules, ", "))
	}
	return nil
}

func ValidateRule(r string) bool {
	for _, rule := range AnnotationsRules {
		if r == rule {
			return true
		}
	}
	return false
}

func (f Filter) EvalRules(as Annotations) (err error) {
	res := map[int]error{}
	for idx := range f.Rules {
		res[idx] = nil
	}
	for idx, r := range f.Rules {
		op := r.Op

		if !as.Keys.Contains(r.Key) {
			return fmt.Errorf("annotation '%s' does not exist", r.Key)
		}
		a := as.Data[r.Key]
		if !as.Data[a.Key].EvalRule(r) {
			e := fmt.Errorf("annotation '%s=%s' does not match rule '%s' w/ value '%s'", a.Key, a.Value, op, r.Value)
			res[idx] = e
		}
	}
	for _, v := range res {
		if v != nil {
			if err == nil {
				err = fmt.Errorf("filter '%s' failed", f.Name)
			}
			err = errors.Wrap(err, v.Error())
		}
	}
	return err
}
