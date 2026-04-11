package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameRule                 = "neo/rule.go"
	ruleReturn                  = `id(r) as ruleID, r.op as ruleOp, r.name as ruleName, r.type AS ruleType, r.tag as ruleTag, r.key as ruleKey, r.value as ruleValue, r.label as ruleLabel`
	FailRulesGetByManipuLayerID = "failed to get rules by ManipuLayer ID"
	FailRulesGetByManifestID    = "failed to get rules by Manifest ID"
	FailRuleGetByLabel          = "failed to get Rule by label"
)

func (n Neo) RuleCreate(ctx context.Context, r types.Rule) (ruleID int64, err error) {
	if r.Type == "" {
		err = errors.New("rule.Type is empty")
		return
	}
	label := fmt.Sprintf("%s:%s", r.Name, r.Tag)
	if r.Tag == "" {
		label = r.Name
	}
	q := `CREATE (r:Rule {name: "%s", tag: "%s", label: "%s", type: "%s", op: "%s", key: "%s", value: "%s"}) RETURN id(r) AS ID`
	query := fmt.Sprintf(q, r.Name, r.Tag, label, r.Type, r.Op, r.Key, r.Value)
	res, err := n.Execute(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	return res.GetInt64("ID")
}

func (n Neo) RuleGet(ctx context.Context, r types.Rule) (ruleID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "RuleGet",
	})
	_ = l
	defer span.End()
	q := `MATCH (r:Rule) WHERE r.name = "%s" AND r.tag = "%s" AND r.op = "%s" AND r.key = "%s" AND r.value = "%s" RETURN id(r) AS ID`
	query := fmt.Sprintf(q, r.Name, r.Tag, r.Op, r.Key, r.Value)
	utils.Debug(l, span, query)
	res, found, err := n.QueryOne(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !found {
		return
	}
	ruleID, err = res.GetInt64("ID")
	return
}

func (n Neo) RuleGetCreate(ctx context.Context, r types.Rule) (ruleID int64, err error) {
	ruleID, found, err := n.RuleGet(ctx, r)
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		return
	}
	return n.RuleCreate(ctx, r)
}

func RuleFromResult(res types.Result) (r types.Rule, err error) {
	err = res.CheckKeys("ruleID", "ruleName", "ruleType", "ruleTag", "ruleKey", "ruleLabel", "ruleValue")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("ruleID")
	name, _ := res.GetStr("ruleName")
	tag, _ := res.GetStr("ruleTag")
	typ, _ := res.GetStr("ruleType")
	op, _ := res.GetStr("ruleOp")
	key, _ := res.GetStr("ruleKey")
	val, _ := res.GetStr("ruleValue")
	label, _ := res.GetStr("ruleLabel")
	r = types.Rule{
		ID:    id,
		Label: label,
		Name:  name,
		Tag:   tag,
		Type:  typ,
		Op:    op,
		Key:   key,
		Value: val,
	}

	return
}

func (n Neo) RulesGetByProfileID(ctx context.Context, profileID int64) (rules types.Rules, err error) {
	q := fmt.Sprintf(`MATCH (p:Profile)-[]->(r:Rule) WHERE id(p) = %d`, profileID)
	q += fmt.Sprintf(` RETURN %s`, ruleReturn)
	res, err := n.Query(q)
	if err != nil {
		return
	}
	rules = types.NewRules()
	for _, r := range res {
		rule, e := RuleFromResult(r)
		if e != nil {
			err = e
			return
		}
		rules.Add(rule)
	}

	return
}

func (n Neo) RulesGetByManipuLayerID(ctx context.Context, mplID int64) (rules types.Rules, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameRule,
		"func":    "RulesGetByManipuLayerID",
		"mplID":   mplID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (mpl:ManipuLayer)-[]->(r:Rule) WHERE id(mpl) = %d`, mplID)
	q += fmt.Sprintf(` RETURN %s`, ruleReturn)
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return
	}
	rules = types.NewRules()
	for _, r := range res {
		rule, e := RuleFromResult(r)
		if e != nil {
			err = e
			return
		}
		rules.Add(rule)
	}
	return
}

func (n Neo) RuleGetByID(ctx context.Context, id int64) (rule types.Rule, found bool, err error) {
	q := fmt.Sprintf(`MATCH (r:Rule) WHERE id(r) = %d RETURN %s`, id, ruleReturn)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	rule, e := RuleFromResult(res)
	if e != nil {
		err = e
		return
	}
	return
}

func (n Neo) RuleGetByLabel(ctx context.Context, label string) (rule types.Rule, found bool, err error) {
	q := fmt.Sprintf(`MATCH (r:Rule) WHERE r.label = "%s" RETURN %s`, label, ruleReturn)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	rule, e := RuleFromResult(res)
	if e != nil {
		err = e
		return
	}
	return
}

func (n Neo) RulesGetAll(ctx context.Context) (rules []types.Rule, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameRule,
		"func":    "RulesGetAll",
	})
	defer span.End()
	_ = ctx
	_ = l
	q := `MATCH (r:Rule) RETURN ` + ruleReturn
	res, err := n.Query(q)
	if err != nil {
		return
	}
	rules = make([]types.Rule, 0)
	for _, r := range res {
		rule, e := RuleFromResult(r)
		if e != nil {
			err = e
			return
		}
		rules = append(rules, rule)
	}
	return
}

func (n Neo) RulesGetByManifestID(ctx context.Context, maniID int64) (rules types.Rules, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameRule,
		"func":    "RulesGetByManifestID",
		"maniID":  maniID,
	})
	defer span.End()
	_ = ctx
	// Platform rules
	q := `MATCH (m:ManifestWrapper)-[]-(md:Manifest)-[:HAS_PLATFORM]->(plat:Platform)<-[:TRUE]-(r:Rule)`
	q += fmt.Sprintf(`WHERE id(m) = %d RETURN %s`, maniID, ruleReturn)
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return
	}
	rules = types.NewRules()
	for _, r := range res {
		rule, e := RuleFromResult(r)
		if e != nil {
			err = e
			return
		}
		rules.Add(rule)
	}
	return

}

func (n Neo) RuleEvalAll(ctx context.Context) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameRule,
		"func":    "RuleEvalAll",
	})
	defer span.End()
	rls, err := n.RulesGetAll(ctx)
	if err != nil {
		return
	}
	for _, r := range rls {
		utils.Debug(l, span, "Eval rule: %s:%s [%s]", r.Name, r.Tag, r.Type)
		switch r.Type {
		case types.RuleTypeAnnotation:
			err = n.RuleEvalAnnotationRule(ctx, r)
			if err != nil {
				return utils.ErrorHandler(l, span, err, "failed to eval rule: %s:%s", r.Name, r.Tag)
			}
		case types.RuleTypePlatform:
			err = n.RuleEvalPlatformRule(ctx, r)
			if err != nil {
				return utils.ErrorHandler(l, span, err, "failed to eval rule: %s:%s", r.Name, r.Tag)
			}
		default:
			return utils.NewError(l, span, "unknown rule type: %s", r.Type)
		}
	}
	return
}

func (n Neo) RuleEvalPlatformRule(ctx context.Context, r types.Rule) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameRule,
		"func":    "RuleEvalPlatformRule",
	})
	defer span.End()

	q := fmt.Sprintf(`MATCH (p:Platform) WHERE p.%s = "%s"`, r.Key, r.Value)
	q += ` RETURN id(p) as platformID, p.os as platformOS, p.arch as platformArch, p.variant as platformVariant`
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return
	}
	for _, ele := range res {
		p, err := PlatformFromResult(ele)
		if err != nil {
			return err
		}
		utils.Debug(l, span, "Within platform evel %s", p.GetLabel())
		if p.EvalRule(r) {
			err = n.RulePlatformSuccess(ctx, r.ID, p.ID)
			if err != nil {
				return err
			}
		}
	}
	return
}

func (n Neo) RuleEvalAnnotationRuleNULL(ctx context.Context, r types.Rule) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameRule,
		"func":    "RuleEvalAnnotationRuleNULL",
	})
	defer span.End()
	if r.Op != "null" {
		err = fmt.Errorf("invalid op for annotation rule: %s", r.Op)
		return
	}
	// Find all annotations matching the key pattern
	q := `MATCH (a:Annotation) WHERE a.key =~ "%s"`
	q += ` RETURN id(a) as annotationID, a.key as annotationKey, a.value as annotationValue`
	query := fmt.Sprintf(q, r.Key)
	utils.Debug(l, span, query)
	res, err := n.Query(query)
	if err != nil {
		return
	}
	for _, ele := range res {
		a, err := AnnotationFromResult(ele)
		if err != nil {
			return err
		}
		err = n.RuleAnnotationFailure(ctx, r.ID, a.ID)
		if err != nil {
			return err
		}
	}
	// Check out all Manifest without an annotation matching the key pattern (regex)
	q = `MATCH (md:Manifest) WHERE NOT EXISTS { MATCH (md)-[:HAS_ANNOTATION]->(a:Annotation) WHERE a.key =~ "%s" }`
	q += ` RETURN id(md) AS manifestDataID`
	query = fmt.Sprintf(q, r.Key)
	utils.Debug(l, span, query)
	res, err = n.Query(query)
	if err != nil {
		return
	}
	for _, ele := range res {
		mdID, err := ele.GetInt64("manifestDataID")
		if err != nil {
			return err
		}
		err = n.RuleManifestSuccess(ctx, r.ID, mdID)
		if err != nil {
			return err
		}
	}
	return
}

func (n Neo) RuleEvalAnnotationRule(ctx context.Context, r types.Rule) (err error) {
	ctx, span := tracer.Start(ctx, "RuleEvalAnnotationRule")
	defer span.End()
	if r.Op == "null" {
		return n.RuleEvalAnnotationRuleNULL(ctx, r)
	}
	q := `MATCH (a:Annotation) WHERE a.key = "%s"`
	q += ` RETURN id(a) as annotationID, a.key as annotationKey, a.value as annotationValue`
	query := fmt.Sprintf(q, r.Key)
	res, err := n.Query(query)
	if err != nil {
		return
	}
	for _, ele := range res {
		a, err := AnnotationFromResult(ele)
		if err != nil {
			return err
		}
		if a.EvalRule(r) {
			err = n.RuleAnnotationSuccess(ctx, r.ID, a.ID)
			if err != nil {
				return err
			}
		}
	}
	return
}

func (n Neo) RuleGetSuccessAnnotations(ctx context.Context, r types.Rule) (as types.Annotations, err error) {
	q := fmt.Sprintf(`MATCH (r:Rule)-[%s]->(a:Annotation) WHERE id(r) = %d`, types.RelationSuccess, r.ID)
	q += ` RETURN id(a) as annotationID, a.key as annotationKey, a.value as annotationValue`
	res, err := n.Query(q)
	if err != nil {
		return
	}
	as = types.NewAnnotations()
	for _, ele := range res {
		a, err := AnnotationFromResult(ele)
		if err != nil {
			return as, err
		}
		err = as.AddAnnotation(a)
		if err != nil {
			return as, err
		}
	}
	return
}

func (n Neo) RuleAnnotationSuccess(ctx context.Context, ruleID, annotationID int64) (err error) {
	q := `MATCH (r:Rule), (a:Annotation)`
	q += ` WHERE id(r) = %d AND id(a) = %d`
	q += ` MERGE (r)-[:%s]->(a)`
	query := fmt.Sprintf(q, ruleID, annotationID, types.RelationSuccess)
	_, err = n.Execute(query)
	return
}

func (n Neo) RuleAnnotationFailure(ctx context.Context, ruleID, annotationID int64) (err error) {
	q := `MATCH (r:Rule), (a:Annotation)`
	q += ` WHERE id(r) = %d AND id(a) = %d`
	q += ` MERGE (r)-[:%s]->(a)`
	query := fmt.Sprintf(q, ruleID, annotationID, types.RelationFailure)
	_, err = n.Execute(query)
	return
}

func (n Neo) RuleGetSuccessPlatform(ctx context.Context, r types.Rule) (ps []types.Platform, err error) {
	q := fmt.Sprintf(`MATCH (r:Rule)-[%s]->(n:Platform) WHERE id(r) = %d`, types.RelationSuccess, r.ID)
	q += ` RETURN id(n) AS platformID, n.arch AS platformArch, n.os AS platformOS, n.variant AS platformVariant`
	res, err := n.Query(q)
	if err != nil {
		return
	}
	ps = make([]types.Platform, 0)
	for _, ele := range res {
		p, err := PlatformFromResult(ele)
		if err != nil {
			return ps, err
		}
		ps = append(ps, p)
	}
	return
}

func (n Neo) RulePlatformSuccess(ctx context.Context, ruleID, platformID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameRule,
		"func":    "RulePlatformSuccess",
	})
	defer span.End()
	q := `MATCH (r:Rule), (p:Platform)`
	q += ` WHERE id(r) = %d AND id(p) = %d`
	q += ` MERGE (r)-[:%s]->(p)`
	query := fmt.Sprintf(q, ruleID, platformID, types.RelationSuccess)
	utils.Debug(l, span, query)
	_, err = n.Execute(query)
	return
}

func (n Neo) RuleManifestSuccess(ctx context.Context, ruleID, mdID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameRule,
		"func":    "RuleManifestSuccess",
	})
	defer span.End()
	q := `MATCH (r:Rule), (md:Manifest)`
	q += ` WHERE id(r) = %d AND id(md) = %d`
	q += ` MERGE (r)-[:%s]->(md)`
	query := fmt.Sprintf(q, ruleID, mdID, types.RelationSuccess)
	utils.Debug(l, span, query)
	_, err = n.Execute(query)
	return
}
