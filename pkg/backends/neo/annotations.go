package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (n Neo) ConfigAddLabels(ctx context.Context, cfgID int64, as *types.Annotations) (err error) {
	for _, a := range as.Data {
		aID, e := n.LabelGetInsert(ctx, a)
		if e != nil {
			err = errors.Wrap(e, "failed to add annotation")
			return
		}
		a.ID = aID
		as.Data[a.Key] = a
		err = n.LinkConfigToAnnotation(ctx, cfgID, aID)
	}
	return
}

func (n Neo) LabelsGetByConfigDigest(ctx context.Context, cfgDig string) (as types.Annotations, err error) {
	q := fmt.Sprintf(`MATCH (cfg:Blob)-[:HAS_LABEL]->(l:Label) WHERE cfg.digest = "%s"`, cfgDig)
	q += ` RETURN id(l) AS labelID, l.key AS labelKey, l.value AS labelValue`
	res, err := n.Query(q)
	if err != nil {
		return
	}
	as = types.NewAnnotations()
	for _, r := range res {
		aTmp, e := AnnotationsFromLabelsResult(r)
		if e != nil {
			err = e
			return
		}
		for _, v := range aTmp.Data {
			as.AddAnnotation(v)
		}
	}
	return
}
func (n Neo) LabelsGetByConfigID(ctx context.Context, cfgID int64) (as types.Annotations, err error) {
	q := fmt.Sprintf(`MATCH (cfg:Blob)-[:HAS_LABEL]->(l:Label) WHERE id(cfg) = %d`, cfgID)
	q += ` RETURN id(l) AS labelID, l.key AS labelKey, l.value AS labelValue`
	res, err := n.Query(q)
	if err != nil {
		return
	}
	as = types.NewAnnotations()
	for _, r := range res {
		aTmp, e := AnnotationsFromLabelsResult(r)
		if e != nil {
			err = e
			return
		}
		for _, v := range aTmp.Data {
			as.AddAnnotation(v)
		}
	}
	return
}

func (n Neo) GetAnnotationsByManifestDataID(ctx context.Context, msID int64) (as types.Annotations, err error) {
	_, span := tracer.Start(ctx, "GetAnnotationsByManifestDataID")
	defer span.End()
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[:HAS_ANNOTATION]->(a:Annotation) WHERE id(md) = %d`, msID)
	q += ` RETURN id(a) AS annotationID, a.key AS annotationKey, a.value AS annotationValue`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Query(q)
	if err != nil {
		return
	}

	as = types.NewAnnotations()
	for _, r := range res {
		aTmp, e := AnnotationsFromResult(r)
		if e != nil {
			err = e
			return
		}
		for _, v := range aTmp.Data {
			as.AddAnnotation(v)
		}
	}
	return
}

func (n Neo) GetAnnotationsByManifestID(ctx context.Context, msID int64) (as types.Annotations, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "neo.GetAnnotationsByManifestID",
		"mani.ID": msID,
	})
	defer span.End()
	_ = l
	q := fmt.Sprintf(`MATCH (ms:Manifest)-[:HAS_ANNOTATION]->(a:Annotation) WHERE id(ms) = %d`, msID)
	q += ` RETURN id(a) AS annotationID, a.key AS annotationKey, a.value AS annotationValue`
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return
	}
	as = types.NewAnnotations()
	for _, r := range res {
		aTmp, e := AnnotationsFromResult(r)
		if e != nil {
			err = e
			return
		}
		for _, v := range aTmp.Data {
			utils.Debug(l, span, "found annotation: %v", v)
			as.AddAnnotation(v)
		}
	}

	return
}

func AnnotationsFromLabelsResult(res types.Result) (as types.Annotations, err error) {
	err = res.CheckKeys("labelID", "labelKey", "labelValue")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("labelID")
	k, _ := res.GetStr("labelKey")
	v, _ := res.GetStr("labelValue")
	as = types.NewAnnotations()
	a := types.NewAnnotation(k, v)
	a.ID = id
	as.AddAnnotation(a)

	return
}

func AnnotationsFromResult(res types.Result) (as types.Annotations, err error) {
	err = res.CheckKeys("annotationID", "annotationKey", "annotationValue")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("annotationID")
	k, _ := res.GetStr("annotationKey")
	v, _ := res.GetStr("annotationValue")
	as = types.NewAnnotations()
	a := types.NewAnnotation(k, v)
	a.ID = id
	as.AddAnnotation(a)
	return
}
