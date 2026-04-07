package neo

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNamePairs = "neo/pairs.go"
)

func (n Neo) CollectionTagProfilePairGetAll(ctx context.Context) (ctpps []types.CollectionTagProfilePair, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNamePairs,
		"func":    "CollectionTagProfilePairGetAll",
	})
	defer span.End()
	ctpps, e := n.CollectionTagProfilePairAnnotationGetAll(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get CollectionTagProfilePairs with annotations")
		return
	}
	utils.Debug(l, span, "Got %d ctpps from CollectionTagProfilePairAnnotationGetAll", len(ctpps))
	ctpps2, e := n.CollectionTagProfilePairPlatformGetAll(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get CollectionTagProfilePairs with platforms")
		return
	}
	utils.Debug(l, span, "Got %d ctpps from CollectionTagProfilePairPlatformGetAll", len(ctpps2))
	ctpps = append(ctpps, ctpps2...)
	return
}

func (n Neo) CollectionTagProfilePairGetRegexFiltered(ctx context.Context, repoRegex, tagRegex, prfRegex string) (ctpps []types.CollectionTagProfilePair, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNamePairs,
		"func":      "CollectionTagProfilePairGetRegexFiltered",
		"repoRegex": repoRegex,
		"tagRegex":  tagRegex,
		"prfRegex":  prfRegex,
	})
	defer span.End()
	ctpps, e := n.CollectionTagProfilePairAnnotationGetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get CollectionTagProfilePairs with annotations")
		return
	}
	utils.Debug(l, span, "Got %d ctpps from CollectionTagProfilePairAnnotationGetRegexFiltered", len(ctpps))
	ctpps2, e := n.CollectionTagProfilePairPlatformGetRegexFiltered(ctx, repoRegex, tagRegex, prfRegex)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get CollectionTagProfilePairs with platforms")
		return
	}
	utils.Debug(l, span, "Got %d ctpps from CollectionTagProfilePairPlatformGetRegexFiltered", len(ctpps2))
	ctpps = append(ctpps, ctpps2...)
	return
}

func (n Neo) CollectionTagProfilePairPlatformGetRegexFiltered(ctx context.Context, repoRegex, tagRegex, prfRegex string) (ctpps []types.CollectionTagProfilePair, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNamePairs,
		"func":      "CollectionTagProfilePairPlatformGetRegexFiltered",
		"repoRegex": repoRegex,
		"tagRegex":  tagRegex,
		"prfRegex":  prfRegex,
	})
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(Rule)-[%s]->(p:Platform)<-[:IS]-(md:ManifestData)-[]-(m:Manifest)`, rel)
	q += fmt.Sprintf(` WHERE prf.label =~ "%s"`, prfRegex)
	q += ` MATCH (c:Collection)-[:UPSTREAM]->(m)<-[:UPSTREAM]-(t:Tag)`
	q += fmt.Sprintf(` WHERE c.repo =~ "%s" AND t.name =~ "%s"`, repoRegex, tagRegex)
	utils.Debug(l, span, q+" RETURN *")
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule,`
	q += ` id(c) AS collectionID, c.repo AS collectionRepo,`
	q += ` id(t) AS tagID, t.name AS tagName`
	res, err := n.Query(q)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "while executing query")
		return
	}
	return n.collectionTagProfilePairFromResult(ctx, res)
}

func (n Neo) CollectionTagProfilePairPlatformGetAll(ctx context.Context) (ctpps []types.CollectionTagProfilePair, err error) {
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "CollectionTagProfilePairPlatformGetAll",
	})
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(Rule)-[%s]->(p:Platform)<-[:IS]-(md:ManifestData)-[]-(m:Manifest)`, rel)
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
	return n.collectionTagProfilePairFromResult(ctx, res)
}

func (n Neo) CollectionTagProfilePairAnnotationGetRegexFiltered(ctx context.Context, repoRegex, tagRegex, profileRegex string) (ctpps []types.CollectionTagProfilePair, err error) {
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "CollectionTagProfilePairAnnotationGetRegexFiltered",
	})
	defer span.End()
	q := `MATCH (prf:Profile)-[:USES]->(Rule)-[:TRUE]->(Annotation)<-[:HAS_ANNOTATION]-(:ManifestData)<-[]-(m:Manifest)`
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
	ctpps, err = n.collectionTagProfilePairFromResult(ctx, res)
	return
}

func (n Neo) CollectionTagProfilePairAnnotationGetAll(ctx context.Context) (ctpps []types.CollectionTagProfilePair, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/pairs.go",
		"func":    "CollectionTagProfilePairAnnotationGetAll",
	})
	defer span.End()
	q := `MATCH (prf:Profile)-[:USES]->(Rule)-[:TRUE]->(Annotation)<-[:HAS_ANNOTATION]-(:ManifestData)<-[]-(m:Manifest)`
	q += ` MATCH (c:Collection)-[:UPSTREAM]->(m)<-[:UPSTREAM]-(t:Tag)`
	utils.Debug(l, span, q+" RETURN *")
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule,`
	q += ` id(c) AS collectionID, c.repo AS collectionRepo,`
	q += ` id(t) AS tagID, t.name AS tagName`
	res, err := n.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	return n.collectionTagProfilePairFromResult(ctx, res)
}

func (n Neo) collectionTagProfilePairFromResult(ctx context.Context, res []types.Result) (ctpps []types.CollectionTagProfilePair, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":     "neo/pairs.go",
		"func":        "collectionTagProfilePairFromResult",
		"resultCount": len(res),
	})
	defer span.End()
	ctpps = []types.CollectionTagProfilePair{}
	for _, r := range res {
		p, err := n.ProfileFromResult(r)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			err = utils.ErrorHandler(l, span, err, "while getting profile from result")
			continue
		}
		c, err := CollectionFromResult(r)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			err = utils.ErrorHandler(l, span, err, "while getting collection from result")
			continue
		}
		t, err := CollectionTagFromResult(r)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			err = utils.ErrorHandler(l, span, err, "while getting collection tag from result")
			continue
		}
		c.ColTag = t
		utils.Debug(l, span, "Collection %s:%s", c.Repo, t.Tag)
		ctpp, err := types.NewCollectionTagProfilePair(c, t, p)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			err = utils.ErrorHandler(l, span, err, "while creating new CollectionTagProfilePair")
			return ctpps, err
		}
		/// Get Manifests for this Collection/Tag
		mds, err := n.ManifestDataGetByCollectionTag(ctx, c.ID, t.ID)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			err = utils.ErrorHandler(l, span, err, "while getting manifestdata by collection tag")
			return ctpps, err
		}
		for _, md := range mds {
			utils.Debug(l, span, "Adding ManifestData to pair '%s'", md.Label)
			err = ctpp.AddManifest(md)
			if err != nil {
				span.SetStatus(codes.Error, err.Error())
				err = utils.ErrorHandler(l, span, err, "while adding manifestdata to pair")
				return ctpps, err
			}
		}
		utils.Debug(l, span, "add ctpp: %s", ctpp.String())
		ctpps = append(ctpps, ctpp)
	}
	utils.Debug(l, span, "returned %d ctpps", len(ctpps))
	return
}
