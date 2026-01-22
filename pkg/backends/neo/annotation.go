package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	AnnotationType = "Annotation"
	LabelType      = "Label"
	pkgNameAnno    = "neo/annotation.go"
)

func (n Neo) LabelGetInsert(ctx context.Context, a types.Annotation) (aID int64, err error) {
	k := a.Key
	v := a.Value
	q := fmt.Sprintf(`MATCH (l:Label {key: '%s', value: '%s'}) RETURN id(l) as ID`, k, v)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return
	}
	if found {
		return res.GetInt64("ID")
	}
	return n.AnnotationCreate(ctx, LabelType, a)
}

func (n Neo) AnnotationCreate(ctx context.Context, typ string, a types.Annotation) (aID int64, err error) {
	q := fmt.Sprintf(`CREATE (n:%s {key: '%s', value: '%s'}) RETURN id(n) as ID`, typ, a.Key, a.Value)
	res, err := n.Execute(q)
	if err != nil {
		return
	}
	aID, err = res.GetInt64("ID")
	if err != nil {
		return
	}
	if typ == AnnotationType {
		err = n.AnnotationEvalRule(ctx, a)
		if err != nil {
			err = errors.Wrap(err, "failed to eval rule")
			return
		} else {
			logrus.Warnf("evaluated rule for annotation %s=%s", a.Key, a.Value)
		}
	}
	return
}

func (n Neo) AnnotationUpdate(ctx context.Context, a types.Annotation) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameAnno,
		"func":    "upstreamInsertGeneric",
		"annoID":  a.ID,
		"key":     a.Key,
		"value":   a.Value,
	})
	defer span.End()
	curA, found, e := n.AnnotationGetByID(ctx, a.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "error while getting current annotation")
	}
	if !found {
		return utils.NewError(l, span, "annotation with ID %d not found", a.ID)
	}
	if curA.Key != a.Key {
		return utils.NewError(l, span, "annotation with ID %d has different key (%s != %s)", a.ID, curA.Key, a.Key)
	}
	q := fmt.Sprintf(`MATCH (n:Annotation) WHERE id(n) = %d SET n.value = "%s"`, a.ID, a.Value)
	utils.Debug(l, span, q)
	_, e = n.Execute(q)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "error while executing query")
	}
	return
}

func (n Neo) AnnotationGetInsert(ctx context.Context, in types.Annotation) (aID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "AnnotationGetInsert",
	})
	_ = l
	defer span.End()
	a, found, err := n.AnnotationGetByKeyValue(ctx, in.Key, in.Value)
	if err != nil {
		return
	}
	if found {
		return a.ID, nil
	}
	return n.AnnotationCreate(ctx, AnnotationType, in)
}

func (n Neo) AnnotationEvalRule(ctx context.Context, a types.Annotation) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func":  "AnnotationEvalRule",
		"key":   a.Key,
		"value": a.Value,
	})
	_ = l
	defer span.End()
	q := fmt.Sprintf(`MATCH (r:Rule) WHERE r.key = "%s" AND r.type = "%s"`, a.Key, types.RuleTypeAnnotation)
	q += fmt.Sprintf(` RETURN %s`, ruleReturn)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Query(q)
	if err != nil {
		return
	}
	aGot, found, err := n.AnnotationGetByKeyValue(ctx, a.Key, a.Value)
	if err != nil {
		err = errors.Wrap(err, "failed to get annotation from graph")
		return
	}
	if !found {
		err = fmt.Errorf("annotation not found: %s=%s", a.Key, a.Value)
		return
	}
	a.ID = aGot.ID
	for _, ele := range res {
		r, err := RuleFromResult(ele)
		if err != nil {
			return err
		}

		if a.EvalRule(r) {
			msg := fmt.Sprintf("rule[%d] %s=%s matched annotation[%d] %s=%s", r.ID, r.Key, r.Value, a.ID, a.Key, a.Value)
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			err = n.RuleAnnotationSuccess(ctx, r.ID, a.ID)
			if err != nil {
				span.AddEvent("log", trace.WithAttributes(
					attribute.String("log.severity", "error"),
					attribute.String("log.error", err.Error()),
				))
				return err
			}
		}
	}
	return
}

func (n Neo) AnnotationGetByKeyValue(ctx context.Context, k, v string) (a types.Annotation, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func":  "AnnotationGetByKeyValue",
		"key":   k,
		"value": v,
	})
	_ = l
	defer span.End()
	q := fmt.Sprintf(`MATCH (a:Annotation {key: '%s', value: '%s'})`, k, v)
	q += ` RETURN id(a) as annotationID, a.key as annotationKey, a.value as annotationValue`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	a, err = AnnotationFromResult(res)
	if err != nil {
		return
	}
	return
}

func (n Neo) AnnotationGetByID(ctx context.Context, annoID int64) (a types.Annotation, found bool, err error) {
	q := fmt.Sprintf(`MATCH (a:Annotation) WHERE id(a) = %d`, annoID)
	q += ` RETURN id(a) as annotationID, a.key as annotationKey, a.value as annotationValue`
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	a, err = AnnotationFromResult(res)
	if err != nil {
		return
	}
	return
}

func AnnotationFromResult(res types.Result) (a types.Annotation, err error) {
	err = res.CheckKeys("annotationID", "annotationKey", "annotationValue")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("annotationID")
	key, _ := res.GetStr("annotationKey")
	val, _ := res.GetStr("annotationValue")
	a = types.Annotation{
		ID:    id,
		Key:   key,
		Value: val,
	}

	return
}
