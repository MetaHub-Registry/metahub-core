package types

import "github.com/minio/minio-go/v7/pkg/set"

type Rules struct {
	Rules    []Rule
	LabelSet set.StringSet
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

func (rs Rules) SatisfiedBy(in Rules) bool {
	needLabels := set.CopyStringSet(rs.LabelSet)
	for _, r := range in.Rules {
		if rs.LabelSet.Contains(r.Label) {
			needLabels.Remove(r.Label)
		}
	}
	return needLabels.IsEmpty()
}

func (rs *Rules) Add(r Rule) {
	rs.Rules = append(rs.Rules, r)
	rs.LabelSet.Add(r.Label)
}

func (rs Rules) GetSet() set.StringSet {
	return set.CopyStringSet(rs.LabelSet)
}
