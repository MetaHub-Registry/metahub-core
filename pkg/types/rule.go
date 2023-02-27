package types

import (
	"fmt"
	"strings"

	"github.com/minio/minio-go/pkg/set"
)

const (
	RuleTypeAnnotation = "anno"
	RelationSuccess    = "TRUE"
	RelationFailure    = "FALSE"
)

var (
	RuleTypes = set.CreateStringSet(
		RuleTypeAnnotation,
	)
)

type Rule struct {
	ID    int64  `yaml:"-"`
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
