package types

import (
	"context"
	"strings"

	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameRules = "types/rules.go"
)

type Rules struct {
	Rules    []Rule
	LabelSet set.StringSet
}

func (rs Rules) ToString() string {
	res := []string{}
	for _, r := range rs.Rules {
		res = append(res, r.Label)
	}
	return strings.Join(res, ", ")
}

func NewRules(rls ...Rule) Rules {
	rs := Rules{
		Rules:    rls,
		LabelSet: set.NewStringSet(),
	}
	for _, r := range rls {
		rs.LabelSet.Add(r.Label)
	}
	return rs
}

// All of the RS rules need to be satisfied by the input rules
func (rs Rules) SatisfiedBy(ctx context.Context, in Rules) bool {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameRules,
		"func":    "SatisfiedBy",
		"base":    rs.ToString(),
		"in":      in.ToString(),
	})
	defer span.End()
	_ = ctx
	// We create a copy of the labels we need to satisfy
	needLabels := set.CopyStringSet(rs.LabelSet)
	utils.Debug(l, span, " %d Rules to be satisfied: %s", len(needLabels.ToSlice()), strings.Join(needLabels.ToSlice(), ", "))
	// We iterate over the rules
	for idx, r := range rs.Rules {
		utils.Debug(l, span, "check baseRule [%d]: %s", idx, r.ToString())
		for inIdx, inR := range in.Rules {
			if r.evalRule(inR.Op, inR.Value) {
				utils.Debug(l, span, " %d > check inRule did MATCH: %s", inIdx, inR.ToString())
				needLabels.Remove(r.Label)
			} else {
				utils.Debug(l, span, " %d > check inRule did NOT match: %s", inIdx, inR.ToString())
			}
		}
	}
	utils.Debug(l, span, "Rules left: %s", strings.Join(needLabels.ToSlice(), ", "))
	return needLabels.IsEmpty()
}

func (rs *Rules) Add(r Rule) {
	rs.Rules = append(rs.Rules, r)
	rs.LabelSet.Add(r.Label)
}

func (rs Rules) GetSet() set.StringSet {
	return set.CopyStringSet(rs.LabelSet)
}
