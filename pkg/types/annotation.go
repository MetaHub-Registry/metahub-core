package types

import (
	"regexp"

	"github.com/sirupsen/logrus"
)

type Annotation struct {
	ID    int64  `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NewAnnotation(k, v string) Annotation {
	a := Annotation{
		Key:   k,
		Value: v,
	}
	return a
}

func (a Annotation) EvalRule(r Rule) bool {
	return a.evalRule(r.Op, r.Value)
}

// TODO: Needs to become an interface with more complex annotations
func (a Annotation) evalRule(op, v string) bool {
	logrus.Warnf("evaluating rule %s %s %s", a.Value, op, v)
	switch op {
	case "eq":
		return a.Value == v
	case "ne":
		return a.Value != v
	case "gt":
		return a.Value > v
	case "lt":
		return a.Value < v
	case "ge":
		return a.Value >= v
	case "le":
		return a.Value <= v
	case "null":
		return a.Value == ""
	case "re":
		// regex
		re := regexp.MustCompile(v)
		return re.MatchString(a.Value)
	}
	return false
}
