package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (n Neo) Rule_Create(ctx context.Context, r types.Rule) (ruleID int64, err error) {
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

func (n Neo) Rule_Get(ctx context.Context, r types.Rule) (ruleID int64, found bool, err error) {
	q := `MATCH (r:Rule) WHERE r.name = "%s" AND r.tag = "%s" AND r.op = "%s" AND r.key = "%s" AND r.value = "%s" RETURN id(r) AS ID`
	query := fmt.Sprintf(q, r.Name, r.Tag, r.Op, r.Key, r.Value)
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

func (n Neo) Rule_GetCreate(ctx context.Context, r types.Rule) (ruleID int64, err error) {
	ruleID, found, err := n.Rule_Get(ctx, r)
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		return
	}
	return n.Rule_Create(ctx, r)
}

func RuleFromResult(res types.Result) (r types.Rule, err error) {
	err = res.CheckKeys("ruleID", "ruleName", "ruleType", "ruleTag", "ruleKey", "ruleValue")
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
	r = types.Rule{
		ID:    id,
		Name:  name,
		Tag:   tag,
		Type:  typ,
		Op:    op,
		Key:   key,
		Value: val,
	}

	return
}

func (n Neo) Rules_GetByProfileID(ctx context.Context, profileID int64) (rules []types.Rule, err error) {
	q := fmt.Sprintf(`MATCH (p:Profile)-[]->(r:Rule) WHERE id(p) = %d`, profileID)
	q += ` RETURN`
	q += ` id(r) as ruleID, r.op as ruleOp, r.name as ruleName, r.tag as ruleTag, r.key as ruleKey, r.value as ruleValue`
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

func (n Neo) Rule_GetByLabel(ctx context.Context, label string) (rule types.Rule, found bool, err error) {
	q := fmt.Sprintf(`MATCH (r:Rule) WHERE r.label = "%s" RETURN`, label)
	q += ` id(r) as ruleID, r.op as ruleOp, r.name as ruleName, r.type AS ruleType, r.tag as ruleTag, r.key as ruleKey, r.value as ruleValue`
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

func (n Neo) Rules_GetAll(ctx context.Context) (rules []types.Rule, err error) {
	ctx, span := tracer.Start(ctx, "Rules_GetAll")
	_ = ctx
	defer span.End()
	q := `MATCH (r:Rule) RETURN`
	q += ` id(r) as ruleID, r.op as ruleOp, r.name as ruleName, r.type AS ruleType, r.tag as ruleTag, r.key as ruleKey, r.value as ruleValue`
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

func (n Neo) Rule_EvalAll(ctx context.Context) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/rule.go",
		"func":    "Rule_EvalAll",
	})
	ctx, span := tracer.Start(ctx, "Rule_EvalAll")
	defer span.End()
	rls, err := n.Rules_GetAll(ctx)
	if err != nil {
		return
	}
	for _, r := range rls {
		msg := fmt.Sprintf("Eval rule: %s:%s [%s]", r.Name, r.Tag, r.Type)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		switch r.Type {
		case types.RuleTypeAnnotation:
			err = n.Rule_EvalAnnotationRule(ctx, r)
			if err != nil {
				err = errors.Wrap(err, fmt.Sprintf("failed to eval rule: %s:%s", r.Name, r.Tag))
				return
			}
		case types.RuleTypePlatform:
			err = n.Rule_EvalPlatformRule(ctx, r)
			if err != nil {
				err = errors.Wrap(err, fmt.Sprintf("failed to eval rule: %s:%s", r.Name, r.Tag))
				return
			}
		default:
			err = fmt.Errorf("unknown rule type: %s", r.Type)
			return
		}

	}
	return
}

func (n Neo) Rule_EvalPlatformRule(ctx context.Context, r types.Rule) (err error) {
	q := fmt.Sprintf(`MATCH (p:Platform) WHERE p.%s = "%s"`, r.Key, r.Value)
	q += ` RETURN id(p) as platformID, p.os as platformOS, p.arch as platformArch, p.variant as platformVariant`
	//fmt.Printf("Query: \n%s\n", q)
	res, err := n.Query(q)
	if err != nil {
		return
	}
	for _, ele := range res {
		p, err := PlatformFromResult(ele)
		if err != nil {
			return err
		}
		if p.EvalRule(r) {
			err = n.Rule_Platform_Success(ctx, r.ID, p.ID)
			if err != nil {
				return err
			}
		}
	}
	return
}

func (n Neo) Rule_EvalAnnotationRule_NULL(ctx context.Context, r types.Rule) (err error) {
	if r.Op != "null" {
		err = fmt.Errorf("invalid op for annotation rule: %s", r.Op)
		return
	}
	q := `MATCH (a:Annotation) WHERE a.key =~ "%s"`
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
		err = n.Rule_Annotation_Failure(ctx, r.ID, a.ID)
		if err != nil {
			return err
		}
	}
	return
}

func (n Neo) Rule_EvalAnnotationRule(ctx context.Context, r types.Rule) (err error) {
	ctx, span := tracer.Start(ctx, "Rule_EvalAnnotationRule")
	defer span.End()
	if r.Op == "null" {
		return n.Rule_EvalAnnotationRule_NULL(ctx, r)
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
			err = n.Rule_Annotation_Success(ctx, r.ID, a.ID)
			if err != nil {
				return err
			}
		}
	}
	return
}

func (n Neo) Rule_GetSuccessAnnotations(ctx context.Context, r types.Rule) (as types.Annotations, err error) {
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

func (n Neo) Rule_Annotation_Success(ctx context.Context, ruleID, annotationID int64) (err error) {
	q := `MATCH (r:Rule), (a:Annotation)`
	q += ` WHERE id(r) = %d AND id(a) = %d`
	q += ` MERGE (r)-[:%s]->(a)`
	query := fmt.Sprintf(q, ruleID, annotationID, types.RelationSuccess)
	_, err = n.Execute(query)
	return
}

func (n Neo) Rule_Annotation_Failure(ctx context.Context, ruleID, annotationID int64) (err error) {
	q := `MATCH (r:Rule), (a:Annotation)`
	q += ` WHERE id(r) = %d AND id(a) = %d`
	q += ` MERGE (r)-[:%s]->(a)`
	query := fmt.Sprintf(q, ruleID, annotationID, types.RelationFailure)
	_, err = n.Execute(query)
	return
}

func (n Neo) Rule_GetSuccessPlatform(ctx context.Context, r types.Rule) (ps []types.Platform, err error) {
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

func (n Neo) Rule_Platform_Success(ctx context.Context, ruleID, platformID int64) (err error) {
	q := `MATCH (r:Rule), (p:Platform)`
	q += ` WHERE id(r) = %d AND id(p) = %d`
	q += ` MERGE (r)-[:%s]->(p)`
	query := fmt.Sprintf(q, ruleID, platformID, types.RelationSuccess)
	_, err = n.Execute(query)
	return
}
