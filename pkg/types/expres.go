package types

import (
	"fmt"
	"strconv"

	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/sirupsen/logrus"
)

type ExprRes struct {
	Expr        set.StringSet
	Data        map[int64]map[string]ManifestData
	ManiMap     map[int64]ManifestData
	RuleManiMap map[string]set.StringSet
}

func NewExprRes() *ExprRes {
	return &ExprRes{
		Expr:        set.NewStringSet(),
		Data:        make(map[int64]map[string]ManifestData),
		ManiMap:     make(map[int64]ManifestData),
		RuleManiMap: make(map[string]set.StringSet),
	}
}

func (r *ExprRes) Add(rule string, md ManifestData) {
	r.Expr.Add(rule)
	if _, ok := r.ManiMap[md.ID]; !ok {
		r.ManiMap[md.ID] = md
	}
	if _, ok := r.RuleManiMap[rule]; !ok {
		r.RuleManiMap[rule] = set.NewStringSet()
	}
	r.RuleManiMap[rule].Add(fmt.Sprintf("%d", md.ID))

	value, found := r.Data[md.ID]
	if !found {
		value = make(map[string]ManifestData)
		r.Data[md.ID] = value
	} else {
		value[rule] = md
	}
	r.Data[md.ID] = value
}

func (er ExprRes) GetTrueIDs() []int64 {
	ids := make([]int64, 0)
	for expr := range er.Expr {
		for mdID, rules := range er.Data {
			if _, found := rules[expr]; found {
				ids = append(ids, mdID)
			}
		}
	}
	return ids
}

func (er ExprRes) GetTrueManifestData() []ManifestData {
	mds := make([]ManifestData, 0)
	for expr := range er.Expr {
		for _, rules := range er.Data {
			if md, found := rules[expr]; found {
				mds = append(mds, md)
			}
		}
	}
	return mds
}

func (er *ExprRes) GetManifestdataForRules(rules ...string) []ManifestData {
	res := set.NewStringSet()
	for _, rule := range rules {
		if er.Expr.Contains(rule) {
			for mdID := range er.RuleManiMap[rule] {
				res.Add(mdID)
			}
		}
	}
	mds := make([]ManifestData, 0)
	for _, mdID := range res.ToSlice() {
		id, err := strconv.ParseInt(mdID, 10, 64)
		if err != nil {
			logrus.Errorf("error parsing manifest data id: %v", err)
			continue
		}
		if md, ok := er.ManiMap[id]; ok {
			mds = append(mds, md)
			er.ManiMap[id] = md
		}
	}
	return mds
}

func (er ExprRes) FilterByCTPP(ctpp CollectionTagProfilePair) (mds []ManifestData) {
	res := set.NewStringSet()
	for _, rule := range ctpp.Rules.ToSlice() {
		if er.Expr.Contains(rule) {
			for mdID := range er.RuleManiMap[rule] {
				res.Add(mdID)
			}
		}
	}
	mds = make([]ManifestData, 0)
	for _, mdID := range res.ToSlice() {
		id, err := strconv.ParseInt(mdID, 10, 64)
		if err != nil {
			logrus.Errorf("error parsing manifest data id: %v", err)
			continue
		}
		if md, ok := er.ManiMap[id]; ok && ctpp.ManifestIDs.Contains(mdID) {
			mds = append(mds, md)
		}
	}
	return
}
