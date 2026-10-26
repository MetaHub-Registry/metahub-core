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
	// Path Z Phase C3: walk via Slot instead of the :ManifestWrapper wrapper.
	// The Slot model provides the (repo, tag) → Manifest association
	// that previously went through (:Repo)-[:UPSTREAM]->(:ManifestWrapper).
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(Rule)-[%s]->(p:Platform)<-[:HAS_PLATFORM]-(md:Manifest)`, rel)
	q += fmt.Sprintf(` WHERE prf.label =~ "%s"`, prfRegex)
	q += ` MATCH (s:Slot)-[:HAS_PUSHED]->(md)`
	q += fmt.Sprintf(` WHERE s.repo =~ "%s" AND s.tag =~ "%s"`, repoRegex, tagRegex)
	q += ` MATCH (c:Repo {repo: s.repo})-[:HAS_TAG]->(t:Tag {name: s.tag})`
	utils.Debug(l, span, "%s", q+" RETURN *")
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule, prf.label AS profileLabel,`
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
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(Rule)-[%s]->(p:Platform)<-[:HAS_PLATFORM]-(md:Manifest)`, rel)
	q += ` MATCH (s:Slot)-[:HAS_PUSHED]->(md)`
	q += ` MATCH (c:Repo {repo: s.repo})-[:HAS_TAG]->(t:Tag {name: s.tag})`
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule, prf.label AS profileLabel,`
	q += ` id(c) AS collectionID, c.repo AS collectionRepo,`
	q += ` id(t) AS tagID, t.name AS tagName`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
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
	// Path Z Phase C3: walk via Slot instead of :ManifestWrapper wrapper.
	q := `MATCH (prf:Profile)-[:USES]->(Rule)-[:TRUE]->(Annotation)<-[:HAS_ANNOTATION]-(md:Manifest)`
	q += fmt.Sprintf(` WHERE prf.label =~ "%s"`, profileRegex)
	q += ` MATCH (s:Slot)-[:HAS_PUSHED]->(md)`
	q += fmt.Sprintf(` WHERE s.repo =~ "%s" AND s.tag =~ "%s"`, repoRegex, tagRegex)
	q += ` MATCH (c:Repo {repo: s.repo})-[:HAS_TAG]->(t:Tag {name: s.tag})`
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule, prf.label AS profileLabel,`
	q += ` id(c) AS collectionID, c.repo AS collectionRepo,`
	q += ` id(t) AS tagID, t.name AS tagName`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
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
		"package": pkgNamePairs,
		"func":    "CollectionTagProfilePairAnnotationGetAll",
	})
	defer span.End()
	// Path Z Phase C3: walk via Slot instead of :ManifestWrapper wrapper.
	q := `MATCH (prf:Profile)-[:USES]->(Rule)-[:TRUE]->(Annotation)<-[:HAS_ANNOTATION]-(md:Manifest)`
	q += ` MATCH (s:Slot)-[:HAS_PUSHED]->(md)`
	q += ` MATCH (c:Repo {repo: s.repo})-[:HAS_TAG]->(t:Tag {name: s.tag})`
	utils.Debug(l, span, "%s", q+" RETURN *")
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule, prf.label AS profileLabel,`
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
		"package":     pkgNamePairs,
		"func":        "collectionTagProfilePairFromResult",
		"resultCount": len(res),
	})
	defer span.End()
	ctpps = []types.CollectionTagProfilePair{}
	for _, r := range res {
		p, err := n.ProfileFromResult(r)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			_ = utils.ErrorHandler(l, span, err, "while getting profile from result")
			continue
		}
		c, err := CollectionFromResult(r)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			_ = utils.ErrorHandler(l, span, err, "while getting collection from result")
			continue
		}
		t, err := CollectionTagFromResult(r)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			_ = utils.ErrorHandler(l, span, err, "while getting collection tag from result")
			continue
		}
		c.ColTag = t
		utils.Debug(l, span, "Repo %s:%s", c.Repo, t.Tag)
		ctpp, err := types.NewCollectionTagProfilePair(c, t, p)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			err = utils.ErrorHandler(l, span, err, "while creating new CollectionTagProfilePair")
			return ctpps, err
		}
		/// Get Manifests for this Repo/Tag
		mds, err := n.ManifestGetByCollectionTag(ctx, c.ID, t.ID)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			err = utils.ErrorHandler(l, span, err, "while getting manifestdata by collection tag")
			return ctpps, err
		}
		for _, md := range mds {
			utils.Debug(l, span, "Adding Manifest to pair '%s'", md.Label)
			err = ctpp.AddManifest(md)
			if err != nil {
				span.SetStatus(codes.Error, err.Error())
				err = utils.ErrorHandler(l, span, err, "while adding manifestdata to pair")
				return ctpps, err
			}
		}
		// Explicit-pin exclusivity: record every non-generic `profile:`
		// binding for this collection so FilterByCTPP drops manifests
		// pinned to another profile from this profile's computed index.
		n.applyCollectionPins(ctx, l, span, &ctpp, c.Repo, t.Tag)
		utils.Debug(l, span, "add ctpp: %s", ctpp.String())
		ctpps = append(ctpps, ctpp)
	}
	utils.Debug(l, span, "returned %d ctpps", len(ctpps))
	return
}

// applyCollectionPins records, on the CTPP, every explicit (non-generic)
// profile a manifest is pinned to within the (repo, tag) collection —
// derived from Slot{repo,tag,profile≠generic}-[:HAS_PUSHED]->md edges. The
// pins drive types.CollectionTagProfilePair pin-exclusivity in FilterByCTPP.
// Best-effort: any failure is warned and leaves the pin set empty/partial,
// degrading to the historical rule-union membership rather than erroring.
func (n Neo) applyCollectionPins(ctx context.Context, l *logrus.Entry, span trace.Span, ctpp *types.CollectionTagProfilePair, repo, tag string) {
	slots, err := n.SlotGetAllForRepoTag(ctx, repo, tag)
	if err != nil {
		utils.Warn(l, span, "applyCollectionPins: SlotGetAllForRepoTag(%s:%s) failed (non-fatal, pin-exclusivity off): %v", repo, tag, err)
		return
	}
	for _, slot := range slots {
		// "generic"/"" is the non-exclusive multi-arch default;
		// AddPinnedProfile ignores it, and we skip the HAS_PUSHED walk here.
		if slot.Profile == "" || slot.Profile == "generic" {
			continue
		}
		slotMds, e := n.ManifestGetAllForSlot(ctx, slot.ID)
		if e != nil {
			utils.Warn(l, span, "applyCollectionPins: ManifestGetAllForSlot(%d) failed (non-fatal): %v", slot.ID, e)
			continue
		}
		for _, md := range slotMds {
			ctpp.AddPinnedProfile(fmt.Sprintf("%d", md.ID), slot.Profile)
		}
	}
}
