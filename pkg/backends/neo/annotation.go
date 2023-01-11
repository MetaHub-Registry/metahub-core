package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) Config_AddLabels(ctx context.Context, cfgID int64, a *types.Annotations) (err error) {
	for k, v := range a.Map {
		aID, e := n.AddAnnotation(ctx, k, v)
		if e != nil {
			err = errors.Wrap(e, "failed to add annotation")
			return
		}
		a.ID = aID
		err = n.LinkConfigToAnnotation(ctx, cfgID, aID)
	}
	return
}

func (n Neo) Manifest_AddAnnotations(ctx context.Context, maniID int64, a *types.Annotations) (err error) {
	for k, v := range a.Map {
		aID, e := n.AddAnnotation(ctx, k, v)
		if e != nil {
			err = errors.Wrap(e, "failed to add annotation")
			return
		}
		a.ID = aID
		err = n.LinkManifestToAnnotation(ctx, maniID, aID)
	}
	return
}

func (n Neo) AddAnnotation(ctx context.Context, k, v string) (aID int64, err error) {
	q := fmt.Sprintf(`MATCH (a:Annotation {key: '%s', value: '%s'}) RETURN id(a) as ID`, k, v)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return
	}
	if found {
		return res.GetInt64("ID")
	}

	q = fmt.Sprintf(`CREATE (a:Annotation {key: '%s', value: '%s'}) RETURN id(a) as ID`, k, v)
	res, err = n.Execute(q)
	if err != nil {
		return
	}
	return res.GetInt64("ID")
}

func (n Neo) GetAnnotationsByConfigID(ctx context.Context, cfgID int64) (a types.Annotations, err error) {
	q := fmt.Sprintf(`MATCH (cfg:Config)-[:HAS_ANNOTATION]->(a:Annotation) WHERE id(cfg) = %d`, cfgID)
	q += ` RETURN id(a) AS annotationID, a.key AS annotationKey, a.value AS annotationValue`
	res, err := n.Query(q)
	if err != nil {
		return
	}
	a = types.NewAnnotations()
	for _, r := range res {
		aTmp, e := AnnotationFromResult(r)
		if e != nil {
			err = e
			return
		}
		for k, v := range aTmp.Map {
			a.AddAnnotation(k, v)
		}
	}
	return
}

func AnnotationFromResult(res Result) (a types.Annotations, err error) {
	err = res.CheckKeys("annotationID", "annotationKey", "annotationValue")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("annotationID")
	k, _ := res.GetStr("annotationKey")
	v, _ := res.GetStr("annotationValue")
	a = types.NewAnnotations()
	a.AddAnnotation(k, v)
	a.ID = id
	return
}
