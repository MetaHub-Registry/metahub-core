package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

const (
	AnnotationType = "Annotation"
	LabelType      = "Label"
)

func (n Neo) Label_GetInsert(ctx context.Context, a types.Annotation) (aID int64, err error) {
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
	return n.Annotation_Create(ctx, LabelType, a)
}

func (n Neo) Annotation_Create(ctx context.Context, typ string, a types.Annotation) (aID int64, err error) {
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
		err = n.Annotation_EvalRule(ctx, a)
		if err != nil {
			err = errors.Wrap(err, "failed to eval rule")
			return
		} else {
			logrus.Warnf("evaluated rule for annotation %s=%s", a.Key, a.Value)
		}
	}
	return
}

func (n Neo) Annotation_GetInsert(ctx context.Context, in types.Annotation) (aID int64, err error) {
	a, found, err := n.Annotation_GetByKeyValue(ctx, in.Key, in.Value)
	if err != nil {
		return
	}
	if found {
		return a.ID, nil
	}
	return n.Annotation_Create(ctx, AnnotationType, in)
}

func (n Neo) Annotation_EvalRule(ctx context.Context, a types.Annotation) (err error) {
	q := `MATCH (r:Rule) WHERE r.key = "%s" AND r.type = "%s"`
	q += ` RETURN id(r) as ruleID, r.op as ruleOp, r.name as ruleName,`
	q += ` r.type AS ruleType, r.tag as ruleTag, r.key as ruleKey, r.value as ruleValue`
	query := fmt.Sprintf(q, a.Key, types.RuleTypeAnnotation)
	res, err := n.Query(query)
	if err != nil {
		return
	}
	aGot, found, err := n.Annotation_GetByKeyValue(ctx, a.Key, a.Value)
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
			logrus.Warnf("rule[%d] %s=%s matched annotation[%d] %s=%s", r.ID, r.Key, r.Value, a.ID, a.Key, a.Value)
			err = n.Rule_Annotation_Success(ctx, r.ID, a.ID)
			if err != nil {
				return err
			}
		}
	}
	return
}

func (n Neo) Annotation_GetByKeyValue(ctx context.Context, k, v string) (a types.Annotation, found bool, err error) {
	q := fmt.Sprintf(`MATCH (a:Annotation {key: '%s', value: '%s'})`, k, v)
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

func (n Neo) Annotation_GetByID(ctx context.Context, annoID int64) (a types.Annotation, found bool, err error) {
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
