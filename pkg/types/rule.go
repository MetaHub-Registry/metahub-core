package types

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/minio/minio-go/pkg/set"
	"github.com/sirupsen/logrus"
)

const (
	RuleTypeAnnotation = "anno"
	RuleTypePlatform   = "plat"
	RelationSuccess    = "TRUE"
	RelationFailure    = "FALSE"
)

var (
	RuleTypes = set.CreateStringSet(
		RuleTypeAnnotation,
		RuleTypePlatform,
	)
)

type Rule struct {
	ID    int64  `yaml:"-"`
	Label string `yaml:"label"`
	Name  string `yaml:"name"`
	Tag   string `yaml:"tag"`
	Type  string `yaml:"type"`
	Op    string `yaml:"op"`
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

func NewRule(typ, op, k, v string) (r Rule, err error) {
	if !RuleTypes.Contains(typ) {
		err = fmt.Errorf("type '%s' is not supported (%s)", typ, strings.Join(RuleTypes.ToSlice(), ", "))
		return
	}
	r = Rule{
		Type:  typ,
		Op:    op,
		Key:   k,
		Value: v,
	}
	return
}

func (r Rule) ToString() string {
	return fmt.Sprintf("%6s > %-45s | %-3s | %s", r.Type, r.Key, r.Op, r.Value)
}

func (r Rule) evalRule(op, v string) bool {
	logrus.Debugf("evaluating rule %s %s %s", r.Value, op, v)
	switch op {
	case "eq":
		return r.Value == v
	case "ne":
		return r.Value != v
	case "null":
		return true
	case "re":
		// regex
		re := regexp.MustCompile(v)
		return re.MatchString(r.Value)
	}
	return false
}
