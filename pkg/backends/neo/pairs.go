package neo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (n Neo) CollectionTagProfilePair_GetAll(ctx context.Context) (ctpps []types.CollectionTagProfilePair, err error) {
	ctx, span := tracer.Start(ctx, "CollectionTagProfilePair_GetAll")
	defer span.End()
	ctpps, e := n.CollectionTagProfilePair_Annotation_GetAll(ctx)
	if e != nil {
		err = errors.Wrap(e, "failed to get CollectionTagProfilePairs with annotations")
		return
	}
	ctpps2, e := n.CollectionTagProfilePair_Platform_GetAll(ctx)
	if e != nil {
		err = errors.Wrap(e, "failed to get CollectionTagProfilePairs with platforms")
		return
	}
	ctpps = append(ctpps, ctpps2...)
	return
}

func (n Neo) CollectionTagProfilePair_GetRegexFiltered(ctx context.Context, repoRegex, tagRegex, prfRegex string) (ctpps []types.CollectionTagProfilePair, err error) {
	ctx, span := tracer.Start(ctx, "CollectionTagProfilePair_GetRegexFiltered")
	defer span.End()
	ctpps, e := n.CollectionTagProfilePair_Annotation_GetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
	if e != nil {
		err = errors.Wrap(e, "failed to get CollectionTagProfilePairs with annotations")
		return
	}
	msg := fmt.Sprintf("Got %d ctpps from CollectionTagProfilePair_Annotation_GetRegexFiltered", len(ctpps))
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	ctpps2, e := n.CollectionTagProfilePair_Platform_GetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
	if e != nil {
		err = errors.Wrap(e, "failed to get CollectionTagProfilePairs with platforms")
		return
	}
	msg = fmt.Sprintf("Got %d ctpps from CollectionTagProfilePair_Platform_GetRegexFiltered", len(ctpps2))
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	ctpps = append(ctpps, ctpps2...)
	return
}

func (n Neo) CollectionTagProfilePair_Platform_GetRegexFiltered(ctx context.Context, repoRegex, tagRegex, prfRegex string) (ctpps []types.CollectionTagProfilePair, err error) {
	ctx, span := tracer.Start(ctx, "CollectionTagProfilePair_Platform_GetAll")
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(Rule)-[%s]->(p:Platform)<-[:IS]-(m:Manifest)-[:CREATED]-(md:ManifestData)`, rel)
	q += fmt.Sprintf(` WHERE prf.label =~ "%s"`, prfRegex)
	q += ` MATCH (c:Collection)-[:UPSTREAM]->(m)<-[:UPSTREAM]-(t:Tag)`
	q += fmt.Sprintf(` WHERE c.repo =~ "%s" AND t.name =~ "%s"`, repoRegex, tagRegex)
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule,`
	q += ` id(c) AS collectionID, c.repo AS collectionRepo,`
	q += ` id(t) AS tagID, t.name AS tagName`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	return n.collectionTagProfilePair_FromResult(ctx, res)
}

func (n Neo) CollectionTagProfilePair_Platform_GetAll(ctx context.Context) (ctpps []types.CollectionTagProfilePair, err error) {
	ctx, span := tracer.Start(ctx, "CollectionTagProfilePair_Platform_GetAll")
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(Rule)-[%s]->(p:Platform)<-[:IS]-(m:Manifest)-[:CREATED]-(md:ManifestData)`, rel)
	q += ` MATCH (c:Collection)-[:UPSTREAM]->(m)<-[:UPSTREAM]-(t:Tag)`
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule,`
	q += ` id(c) AS collectionID, c.repo AS collectionRepo,`
	q += ` id(t) AS tagID, t.name AS tagName`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	return n.collectionTagProfilePair_FromResult(ctx, res)
}

func (n Neo) CollectionTagProfilePair_Annotation_GetRegexFiltered(ctx context.Context, repoRegex, tagRegex, profileRegex string) (ctpps []types.CollectionTagProfilePair, err error) {
	ctx, span := tracer.Start(ctx, "CollectionTagProfilePair_Annotation_GetRegexFiltered")
	defer span.End()
	q := `MATCH (prf:Profile)-[:USES]->(Rule)-[:TRUE]->(Annotation)<-[:HAS_ANNOTATION]-(:ManifestData)<-[:CREATED]-(m:Manifest)`
	q += fmt.Sprintf(` WHERE prf.label =~ "%s"`, profileRegex)
	q += ` MATCH (c:Collection)-[:UPSTREAM]->(m)<-[:UPSTREAM]-(t:Tag)`
	q += fmt.Sprintf(` WHERE c.repo =~ "%s" AND t.name =~ "%s"`, repoRegex, tagRegex)
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule,`
	q += ` id(c) AS collectionID, c.repo AS collectionRepo,`
	q += ` id(t) AS tagID, t.name AS tagName`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	ctpps, err = n.collectionTagProfilePair_FromResult(ctx, res)
	return
}

func (n Neo) CollectionTagProfilePair_Annotation_GetAll(ctx context.Context) (ctpps []types.CollectionTagProfilePair, err error) {
	ctx, span := tracer.Start(ctx, "CollectionTagProfilePair_Annotation_GetAll")
	defer span.End()
	q := `MATCH (prf:Profile)-[:USES]->(Rule)-[:TRUE]->(Annotation)<-[:HAS_ANNOTATION]-(:ManifestData)<-[:CREATED]-(m:Manifest)`
	q += ` MATCH (c:Collection)-[:UPSTREAM]->(m)<-[:UPSTREAM]-(t:Tag)`
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule,`
	q += ` id(c) AS collectionID, c.repo AS collectionRepo,`
	q += ` id(t) AS tagID, t.name AS tagName`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	return n.collectionTagProfilePair_FromResult(ctx, res)
}

func (n Neo) collectionTagProfilePair_FromResult(ctx context.Context, res []types.Result) (ctpps []types.CollectionTagProfilePair, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/pairs.go",
		"func":    "collectionTagProfilePair_FromResult",
	})

	ctx, span := tracer.Start(ctx, "collectionTagProfilePair_FromResult")
	defer span.End()
	ctpps = []types.CollectionTagProfilePair{}
	for _, r := range res {
		p, err := n.ProfileFromResult(r)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			continue
		}
		c, err := CollectionFromResult(r)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			continue
		}
		t, err := CollectionTagFromResult(r)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			continue
		}
		c.ColTag = t
		msg := fmt.Sprintf("Collection %s:%s", c.Repo, t.Tag)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		ctpp, err := types.NewCollectionTagProfilePair(c, t, p)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return ctpps, err
		}
		/// Get Manifests for this Collection/Tag
		mds, err := n.ManifestData_GetByCollectionTag(ctx, c.ID, t.ID)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return ctpps, err
		}
		for _, md := range mds {
			msg := fmt.Sprintf("Adding ManifestData to pair '%s'", md.Label)
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			l.Debug(msg)
			err = ctpp.AddManifest(md)
			if err != nil {
				span.SetStatus(codes.Error, err.Error())
				span.AddEvent("log", trace.WithAttributes(
					attribute.String("log.severity", "error"),
					attribute.String("log.message", err.Error()),
				))
				span.RecordError(err)
				l.Error(err.Error())
				return ctpps, err
			}
		}
		ctpps = append(ctpps, ctpp)
	}
	return
}
