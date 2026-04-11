package types

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type ExprRes struct {
	Expr        set.StringSet
	Data        map[int64]map[string]Manifest
	ManiMap     map[int64]Manifest
	RuleManiMap map[string]set.StringSet
}

func NewExprRes() *ExprRes {
	return &ExprRes{
		Expr:        set.NewStringSet(),
		Data:        make(map[int64]map[string]Manifest),
		ManiMap:     make(map[int64]Manifest),
		RuleManiMap: make(map[string]set.StringSet),
	}
}

func (r *ExprRes) IsEmpty() bool {
	return len(r.Expr) == 0
}

func (r *ExprRes) Add(rule string, md Manifest) {
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
		value = make(map[string]Manifest)
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

func (er ExprRes) GetTrueManifest() []Manifest {
	mds := make([]Manifest, 0)
	for expr := range er.Expr {
		for _, rules := range er.Data {
			if md, found := rules[expr]; found {
				mds = append(mds, md)
			}
		}
	}
	return mds
}

func (er *ExprRes) GetManifestdataForRules(rules ...string) []Manifest {
	res := set.NewStringSet()
	for _, rule := range rules {
		if er.Expr.Contains(rule) {
			for mdID := range er.RuleManiMap[rule] {
				res.Add(mdID)
			}
		}
	}
	mds := make([]Manifest, 0)
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

func (er ExprRes) FilterByCTPP(ctx context.Context, ctpp CollectionTagProfilePair) (mds []Manifest) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":        "types/expres.go",
		"func":           "FilterByCTPP",
		"ctpp":           ctpp.String(),
		"cppt.manifests": len(er.ManiMap),
	})
	defer span.End()
	res := set.NewStringSet()
	for _, rule := range ctpp.Rules.ToSlice() {
		utils.Debug(l, span, "Checking rule: %s", rule)
		if er.Expr.Contains(rule) {
			utils.Debug(l, span, "  Rule matched, adding manifestdata")
			for mdID := range er.RuleManiMap[rule] {
				utils.Debug(l, span, "    Adding manifestdata id: %s which matched rule '%s'", mdID, rule)
				res.Add(mdID)
			}
		}
	}
	mds = make([]Manifest, 0)
	for _, mdID := range res.ToSlice() {
		utils.Debug(l, span, "Processing manifestdata id: %s", mdID)
		time.Sleep(5 * time.Millisecond)
		id, err := strconv.ParseInt(mdID, 10, 64)
		if err != nil {
			utils.ErrorHandler(l, span, err, "error parsing manifest data id: %s", mdID)
			continue
		}
		utils.Debug(l, span, "  Parsed manifestdata id: %d", id)
		if md, ok := er.ManiMap[id]; ok {
			if ctpp.ManifestIDs.Contains(mdID) {
				utils.Debug(l, span, "  Adding manifestdata: %s", md.Label)
				mds = append(mds, md)
			} else {
				utils.Debug(l, span, "  Skipping manifestdata: %s not in CTTP manifestIDs", md.Label)
			}
		} else {
			utils.Debug(l, span, "  Manifestdata id: %d not found in ManiMap", id)
		}
	}
	utils.Info(l, span, "Filtered manifestdata count: %d", len(mds))
	return mds
}
