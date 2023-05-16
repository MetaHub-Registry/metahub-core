package neo

import (
	"context"
	"fmt"

	lep "github.com/mgudov/logic-expression-parser"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (n Neo) TraverseExpr(er *types.ExprRes, expr lep.Expression) (bool, error) {
	ctx := context.Background()
	switch e := expr.(type) {
	default:
		return false, fmt.Errorf("not implemented: %T", e)
	case *lep.OrX:
		res := false
		for _, disjunction := range e.Disjunctions {
			direct, err := n.TraverseExpr(er, disjunction)
			if err != nil {
				return false, err
			}
			res = res || direct
		}
		return res, nil
	case *lep.AndX:
		res := true
		for _, conjunct := range e.Conjuncts {
			direct, err := n.TraverseExpr(er, conjunct)
			if err != nil {
				return false, err
			}
			res = res && direct
		}
		return res, nil
	case *lep.EqualsX:
		value := e.Value.Value()
		if value == nil {
			return false, nil
		}
		para := e.Param.String()
		switch value.(type) {
		case string:
			logrus.Warnf("Param: '%s' Value: %s", para, value)
		case int:
			logrus.Warnf("Param: '%s' Value: %d", para, value)
		case bool:
			// For bool we need to check is the rule evaluates to true
			rLabel, err := utils.DecodeKey(para)
			if err != nil {
				return false, err
			}
			r, found, err := n.RuleGetByLabel(ctx, rLabel)
			if err != nil {
				return false, err
			}
			if !found {
				return false, fmt.Errorf("rule with label '%s' not found", rLabel)
			}
			// TODO: We need to find the manifestData that matches the rule
			// TODO: The Result needs to keep track of all matching manifestData ids for all rules
			/*

			 */
			mds, err := n.ManifestData_GetByRule(ctx, r)
			if err != nil {
				return false, err
			}
			for _, md := range mds {
				er.Add(rLabel, md)
			}
			if len(mds) > 0 {
				return true, nil
			}
		}
		return false, nil
	case *lep.NotEqualsX:
		value := e.Value.Value()
		if value == nil {
			return false, nil
		}
		return false, nil
	case *lep.GreaterThanX:
		return false, nil
	}
}
