package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameML          = "neo/manifestlist.go"
	manifestListReturn = "id(ml) as manifestlistID, ml.digest as manifestlistDigest, ml.mediaType as manifestlistMediaType, ml.size as manifestlistSize"
	// res.CheckKeys("manifestlistID", "manifestlistDigest", "manifestlistMediaType", "manifestlistSize")
)

func (n Neo) GetManifestList(dig digest.Digest, mt string, size int) (mlID int64, success bool, err error) {
	q := `MATCH (n:Index) WHERE n.digest = "%s" AND n.mediaType = "%s" AND n.size = %d RETURN id(n) AS ID`
	query := fmt.Sprintf(q, dig, mt, size)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf(errFailedRunQueryFmt, err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("GetManifestList: failed to get ID")
			return
		}
		mlID = v.(int64)
		success = true
	}
	return
}

func (n Neo) CreateManifestList(dig digest.Digest, mt string, size int) (mlID int64, err error) {
	q := `CREATE (n:Index {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, dig, mt, size, utils.ShortenDigest(dig.String()))
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create conifg: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("CreateManifestList: failed to get ID")
			return
		}
		mlID = v.(int64)
	}
	return
}

func (n Neo) UnLinkManifestListToCollection(mlID, colID int64) (err error) {
	q := `MATCH (c:Repo)-[r:REF]->(ml:Index) WHERE id(c) = %d AND id(ml) = %d`
	q += ` DELETE r`
	query := fmt.Sprintf(q, colID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:Index)-[:REF]->(c:Repo): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToCollection(mlID, colID int64) (err error) {
	q := `MATCH (c:Repo), (ml:Index) WHERE id(c) = %d AND id(ml) = %d`
	q += ` MERGE (ml)<-[:REF]-(c)`
	query := fmt.Sprintf(q, colID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:Index)-[:REF]->(c:Repo): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToManifest(ctx context.Context, mlID, mdID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "LinkManifestListToManifest",
		"mlID":    mlID,
		"mdID":    mdID,
	})
	defer span.End()
	if mlID < 0 || mdID < 0 {
		err = utils.NewError(l, span, "LinkManifestListToManifest: invalid IDs mlID:%d mdID:%d", mlID, mdID)
		return
	}
	q := `MATCH (ml:Index), (md:Manifest) WHERE id(ml) = %d AND id(md) = %d`
	q += ` MERGE (ml)-[:REFERENCES]-(md)`
	query := fmt.Sprintf(q, mlID, mdID)
	utils.Debug(l, span, query)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml)-[:REFERENCES]-(md): %v", err)
		return
	}
	return
}

func (n Neo) UnLinkManifestListToCollectionTag(mlID, colTagID int64) (err error) {
	q := `MATCH (ct:Tag)-[r:REF]->(ml:Index) WHERE id(ct) = %d AND id(ml) = %d`
	q += ` DELETE r`
	query := fmt.Sprintf(q, colTagID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:Index)-[:REF]->(t:Tag): %v", err)
		return
	}
	return
}

// UnLinkCollectionTagFromOtherIndexes removes every (ct:Tag)-[:REF]->(ml:Index)
// edge where the Tag matches colTagID and the Index is NOT mlID. Call directly
// before LinkManifestListToCollectionTag on every recompute to enforce the
// one-index-per-tag invariant — mirroring what UnLinkProfileFromOtherRepoIndexes
// does for profiles. Without this, superseded computed indexes accumulate stale
// Tag→REF edges that surface as duplicate results in the wrapper-era query layer
// and show stale indexes in the UI.
func (n Neo) UnLinkCollectionTagFromOtherIndexes(mlID, colTagID int64) (err error) {
	q := `MATCH (ct:Tag)-[r:REF]->(ml:Index) WHERE id(ct) = %d AND id(ml) <> %d DELETE r`
	query := fmt.Sprintf(q, colTagID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to unlink tag %d from other indexes: %v", colTagID, err)
		return
	}
	return
}

func (n Neo) LinkManifestListToCollectionTag(mlID, colTagID int64) (err error) {
	q := `MATCH (ct:Tag), (ml:Index) WHERE id(ct) = %d AND id(ml) = %d`
	q += ` MERGE (ml)<-[:REF]-(ct)`
	query := fmt.Sprintf(q, colTagID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:Index)-[:REF]->(t:Tag): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToProfile(mlID, pID int64) (err error) {
	q := `MATCH (p:Profile), (ml:Index) WHERE id(p) = %d AND id(ml) = %d`
	q += ` MERGE (ml)<-[:REF]-(p)`
	query := fmt.Sprintf(q, pID, mlID)
	_, err = n.Execute(query)
	return
}

// UnLinkProfileFromOtherRepoIndexes enforces the one-image-per-profile
// invariant within a collection: it deletes every (p:Profile)-[:REF]->(ml:Index)
// edge that points at an Index of repo repoID other than mlID.
//
// The Profile→Index REF edge is what the serve path joins on
// (ManifestListExistsForTagProfile / ManifestListExists:
// Profile→Index←Repo-HAS_TAG→Tag-REF→Index). If a profile stays attached to
// the index of a previously pushed image, that pattern matches two rows and
// QueryOne serves a stale image nondeterministically — observed as a re-push
// of different content still resolving to the old digest. Called directly
// before LinkManifestListToProfile, the invariant "a profile is attached to at
// most one image per collection" holds after every push: the profile is
// detached from the old image before it is attached to the new one.
func (n Neo) UnLinkProfileFromOtherRepoIndexes(mlID, pID, repoID int64) (err error) {
	q := `MATCH (p:Profile)-[r:REF]->(ml:Index)<-[:REF]-(c:Repo)`
	q += ` WHERE id(p) = %d AND id(c) = %d AND id(ml) <> %d`
	q += ` DELETE r`
	query := fmt.Sprintf(q, pID, repoID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to unlink profile %d from other indexes of repo %d: %v", pID, repoID, err)
		return
	}
	return
}

func (n Neo) LinkManifestListToFilter(mlID, filterID int64) (err error) {
	q := `MATCH (ml:Index),(f:Filter) WHERE id(ml) = %d AND id(f) = %d`
	q += ` MERGE (ml)-[:IMPLEMENTS]->(f)`
	query := fmt.Sprintf(q, mlID, filterID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:Index)-[:IMPLEMENTS]->(f:Filter): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToManifests(mlID int64, manifests []types.Manifest) (err error) {
	for _, m := range manifests {
		q := `MATCH (ml:Index),(m:Manifest) WHERE id(ml) = %d AND id(m) = %d`
		q += ` MERGE (ml)-[:REFERENCES]->(m)`
		query := fmt.Sprintf(q, mlID, m.ID)
		session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		defer session.Close()
		_, err = session.Run(query, map[string]interface{}{})
		if err != nil {
			logrus.Errorf("failed to create link (ml:Index)-[:REFERENCES]->(m:Manifest): %v\n%s", err, query)
			return
		}
	}
	return
}

func (n Neo) ManifestListAddAnnotation(ctx context.Context, mlID int64, a *types.Annotation) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "ManifestListAddAnnotation",
		"mlID":    mlID,
	})
	defer span.End()
	logrus.Warnf("adding annotation %s=%s", a.Key, a.Value)
	aID, e := n.AnnotationGetInsert(ctx, *a)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to add annotation")
	}
	a.ID = aID
	e = n.ManifestListLinkToAnnotation(ctx, mlID, aID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to link annotation")
	}

	return nil
}

func (n Neo) ManifestListLinkToAnnotation(ctx context.Context, mlID, annotationID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgNameML,
		"func":         "ManifestListLinkToAnnotation",
		"mlID":         mlID,
		"annotationID": annotationID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (ml:Index), (a:Annotation) WHERE id(ml) = %d AND id(a) = %d`, mlID, annotationID)
	q += ` MERGE (ml)-[:HAS_ANNOTATION]->(a)`
	utils.Debug(l, span, q)
	_, err = n.Execute(q)
	if err != nil {
		return
	}
	// TODO: Check if annotation is linked to a rule which has all filters rules satisfied
	return
}

func (n Neo) ManifestListInsert(ctx context.Context, ml *types.Index) (err error) {
	dig := digest.Digest(ml.Digest)
	mlID, success, err := n.GetManifestList(dig, ml.MediaType, 0)
	if err != nil {
		logrus.Errorf("failed to get manifest list: %v", err)
		return
	}
	if success {
		ml.ID = mlID
		return
	}
	mlID, err = n.CreateManifestList(dig, ml.MediaType, 0)
	if err != nil {
		logrus.Errorf("failed to create manifest list: %v", err)
		return
	}
	ml.ID = mlID
	return
}

func (n Neo) InsertManifestList(ctx context.Context, dig digest.Digest, mt string, size int) (mlID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNameML,
		"func":      "InsertManifestList",
		"digest":    utils.ShortenDigest(dig.String()),
		"mediaType": mt,
		"size":      size,
	})
	defer span.End()

	mlID, success, err := n.GetManifestList(dig, mt, size)
	if err != nil {
		return
	}
	utils.Debug(l, span, "Inserted Index w/ ID '%d' and success %t", mlID, success)
	if !success {
		mlID, err = n.CreateManifestList(dig, mt, size)
		if err != nil {
			return
		}
	}
	utils.Debug(l, span, "Inserted Index w/ ID '%d'", mlID)
	return
}

func (n Neo) ManifestListGetByDigest(ctx context.Context, dig string) (ml *types.Index, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "ManifestListGetByDigest",
		"digest":  utils.ShortenDigest(dig),
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (ml:Index) WHERE ml.digest = "%s"`, dig)
	utils.Debug(l, span, q+` RETURN *`)
	q += fmt.Sprintf(` RETURN %s`, manifestListReturn)

	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		mlRes, err := n.IndexFromResult(ctx, res)
		if err != nil {
			logrus.Error(err)
			return nil, err
		}
		ml = &mlRes
		return ml, nil
	}
	return nil, nil
}

func (n Neo) IndexFromResult(ctx context.Context, res types.Result) (ml types.Index, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "IndexFromResult",
	})
	defer span.End()
	err = res.CheckKeys("manifestlistID", "manifestlistDigest", "manifestlistMediaType", "manifestlistSize")
	if err != nil {
		return
	}
	_ = l
	id, _ := res.GetInt64("manifestlistID")
	size, _ := res.GetInt("manifestlistSize")
	manifestlistDigest, _ := res.GetStr("manifestlistDigest")
	manifestlistMediaType, _ := res.GetStr("manifestlistMediaType")
	ml = types.Index{
		ID:            id,
		SchemaVersion: 2,
		Digest:        manifestlistDigest,
		MediaType:     manifestlistMediaType,
		Size:          size,
		Annotations:   map[string]string{},
	}
	as, err := n.GetAnnotationsByManifestListID(ctx, ml.ID)
	if err != nil {
		return
	}
	for _, a := range as.Data {
		ml.Annotations[a.Key] = a.Value
	}
	utils.Debug(l, span, "found %d annotations", len(ml.Annotations))
	return
}

func (n Neo) GetAnnotationsByManifestListID(ctx context.Context, mlID int64) (as types.Annotations, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "GetAnnotationsByManifestListID",
		"mlID":    mlID,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (ml:Index)-[:HAS_ANNOTATION]->(a:Annotation) WHERE id(ml) = %d`, mlID)
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
			as.AddAnnotation(v)
		}
	}
	utils.Debug(l, span, "found %d annotations", as.Len())
	return
}

// IndexGetByRequest resolves a client pull request (repo, tag, profile) to
// the Index node currently pointed at by the matching Slot/Profile and
// returns it fully hydrated with its constituent Manifests.
//
// Path Z task 11 — the "manifests:null" bug fix. Historically this function
// had two bugs:
//
//  1. The raw `rs, err := n.Query(q)` result for the descriptor fetch did
//     not check `err` before iterating, silently swallowing any error.
//  2. The descriptor fetch walked `(md:Manifest)<-[]-(m:ManifestWrapper)-[:HAS_PLATFORM]->(p:Platform)`
//     through the legacy :ManifestWrapper wrapper node. Path Z task 5.1 moved the
//     :HAS_PLATFORM edge directly to the content-leaf (:Manifest), so
//     the wrapper hop became unnecessary; worse, post-Phase-A Slot-based
//     compute produces Indexes whose referenced Manifest nodes have NO
//     wrapper at all, so the second MATCH returned zero rows, leaving
//     ml.Manifests empty. The OCI output then serialised as `"manifests": null`,
//     which broke every `docker pull` that went through the compute path.
//
// The fix uses a deliberate two-query approach:
//
//	Query 1: find the Index node by (repo, tag, profile) via the existing
//	         Profile/Repo/Tag :REF edges.
//	Query 2: walk the Index's :REFERENCES edges directly to the content-leaf
//	         :Manifest nodes, reading :HAS_PLATFORM from the content-leaf
//	         itself (NOT through the wrapper), and decode each match to a
//	         types.Manifest with platform metadata attached.
//
// Why two queries instead of one with COLLECT: early Path Z experiments used
// `WITH ml, COLLECT(md) AS mds` to co-return the Index and its descriptors
// in a single round-trip, but COLLECT ordering is unstable in neo4j 4.x and
// the :REFERENCES edge does not yet carry an `order` property (Path Z task
// 8.2 adds it). Splitting into two queries keeps the current code simple
// and leaves a natural insertion point for the `ORDER BY r.order` clause
// when task 8.2 lands. Revisit this once neo4j 5.x + edge-order ships.
func (n Neo) IndexGetByRequest(ctx context.Context, req types.Request) (ml types.Index, found bool, err error) {
	ctx, span := tracer.Start(ctx, "IndexGetByRequest")
	defer span.End()
	span.SetAttributes(
		attribute.Key("req.Digest").String(req.Digest),
		attribute.Key("req.Host").String(req.Host),
		attribute.Key("req.Repo").String(req.Repo),
		attribute.Key("req.Tag").String(req.Tag),
		attribute.Key("req.Profile").String(req.Profile),
	)
	logrus.Infof("Query for ML of name '%s' and tag '%s' / dig:'%s'", req.Repo, req.Tag, utils.ShortenDigest(req.Digest))
	// Query 1: resolve (repo, tag, profile) to the Index node via the
	// Profile/Repo/Tag :REF edges. This is unchanged from pre-Path-Z; it
	// produces the Index node's ID, digest, mediaType, and size.
	q := fmt.Sprintf(`MATCH (c:Repo)-[:REF]->(ml)<-[:REF]-(t:Tag) WHERE c.repo = "%s"`, req.Repo)
	if req.Tag != "" {
		q += fmt.Sprintf(` AND t.name ="%s"`, req.Tag)
		// Exclude indexes that are in any Slot's HAS_HISTORY for this (repo, tag):
		// those are superseded computed indexes whose Tag→REF was not yet pruned
		// by UnLinkCollectionTagFromOtherIndexes. HAS_HISTORY is the authoritative
		// "this index is stale" signal from the Slot-centred model.
		q += fmt.Sprintf(` AND NOT (:Slot {repo: "%s", tag: "%s"})-[:HAS_HISTORY]->(ml)`, req.Repo, req.Tag)
	} else {
		q += fmt.Sprintf(` AND ml.digest ="%s"`, req.Digest)
	}
	q += fmt.Sprintf(` MATCH (ml)<-[:REF]-(prf:Profile) WHERE prf.label = "%s"`, req.Profile)
	q += ` RETURN id(ml) as manifestlistID, ml.digest as manifestlistDigest, ml.mediaType as manifestlistMediaType, ml.size as manifestlistSize`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	res, found, err := n.QueryOne(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !found {
		logrus.Warnf("Index not found for repo=%s tag=%s digest=%s profile=%s", req.Repo, req.Tag, req.Digest, req.Profile)
		return
	}
	ml, err = n.IndexFromResult(ctx, res)
	if err != nil {
		logrus.Error(err)
		return
	}
	// Query 2 (the manifests:null fix): walk the Index's REFERENCES edges
	// directly to the content-leaf Manifest nodes, reading HAS_PLATFORM
	// from the content-leaf (NOT through the legacy wrapper). Post-Path-Z
	// Phase A+B+C this walk works for both wrapper-era seeded fixtures and
	// Slot-era compute outputs because the :REFERENCES edge and the
	// :HAS_PLATFORM edge both live on the content-leaf in both models.
	q = fmt.Sprintf(`MATCH (ml:Index)-[:REFERENCES]->(md:Manifest) WHERE id(ml) = %d`, ml.ID)
	q += ` OPTIONAL MATCH (md)-[:HAS_PLATFORM]->(p:Platform)`
	q += ` RETURN ` + manifestDataReturn + `,`
	q += ` id(p) as platformID, p.os as platformOS, p.arch as platformArch, p.variant as platformVariant`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	rs, qerr := n.Query(q)
	if qerr != nil {
		// Surface the error so the caller knows the hydration failed.
		// The Index node itself is still returned; downstream serialisation
		// will produce a valid but empty-manifests response, matching the
		// pre-fix behaviour for the "truly empty Index" case.
		err = utils.ErrorHandler(logrus.WithField("func", "IndexGetByRequest"), span, qerr, "descriptor hydration query failed")
		return
	}
	ml.Manifests = make([]types.Manifest, 0, len(rs))
	for _, r := range rs {
		m, _, mErr := n.ManifestFromResult(ctx, r)
		if mErr != nil {
			logrus.Errorf("IndexGetByRequest: ManifestFromResult failed for a descriptor: %v", mErr)
			continue
		}
		// Platform is OPTIONAL MATCH so PlatformFromResult may fail if the
		// manifest has no platform (e.g. ORAS artifacts). In that case we
		// leave m.Platform as zero-value and still include the descriptor
		// in the result — the OCI output will show an entry without a
		// platform block, which is the correct behaviour for non-image
		// artifacts.
		p, pErr := PlatformFromResult(r)
		if pErr == nil {
			m.Platform = p
		}
		ml.Manifests = append(ml.Manifests, m)
	}
	return
}

func (n Neo) ManifestListExistsForTagProfile(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "ManifestListExistsForTagProfile",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"profile": up.Profile,
	})
	defer span.End()
	_ = ctx
	q := `MATCH (prf:Profile)-[:REF]->(ml:Index)<-[:REF]-(c:Repo)-[:HAS_TAG]->(t:Tag)-[:REF]->(ml)`
	q += fmt.Sprintf(` WHERE prf.label = "%s" AND c.repo = "%s" AND t.name = "%s"`, up.Profile, up.Repo, up.Tag)
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo, ml.mediaType AS mediaType`
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return "", false, err
	}
	if !found {
		utils.Debug(l, span, "Index not found")
		return "", false, nil
	}
	mediaType, e = res.GetStr("mediaType")
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get mediaType from record")
		return "", false, err
	}
	colID, e := res.GetInt64("collectionID")
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "colID could not be found")
		return "", false, err
	}
	l.Debugf("found collection with id %d and manifestList with type '%s'", colID, mediaType)
	return
}

func (n Neo) ManifestListExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{"package": pkgNameML,
		"func":    "ManifestListExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
		"profile": up.Profile,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (prf:Profile)-[:REF]->(ml:Index)<-[:REF]-(c:Repo)-[:HAS_TAG]->(t:Tag)-[:REF]->(ml)`
	if up.Digest.String() != "" {
		q += fmt.Sprintf(` WHERE prf.label = "%s" AND c.repo = "%s" AND ml.digest = "%s"`, up.Profile, up.Repo, up.Digest.String())
	} else {
		q += fmt.Sprintf(` WHERE prf.label = "%s" AND c.repo = "%s" AND t.name = "%s"`, up.Profile, up.Repo, up.Tag)
	}
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo, ml.mediaType AS mediaType`
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return "", false, err
	}
	if found {
		md, err := res.GetStr("mediaType")
		if err != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get mediaType from record")
			return "", false, err
		}
		colID, err := res.GetInt64("collectionID")
		if err != nil {
			err = utils.ErrorHandler(l, span, e, "colID could not be found")
			return "", false, err
		}
		l.Debugf("found collection with id %d and manifestList with type '%s' (wrapper-era REF path)", colID, md)
		return md, true, nil
	}

	// Slot-based fallback (Path Z): walk via Slot-[:POINTS_TO]->Index.
	if mdSlot, ok := n.manifestListViaSlot(l, span, up); ok {
		return mdSlot, true, nil
	}

	utils.Debug(l, span, "Index not found")
	return "", false, nil
}

// manifestListViaSlot is the Path-Z fallback for ManifestListExists: it walks
// Slot-[:POINTS_TO]->Index to guarantee read-after-write consistency for
// tag-based (non-digest) queries. Only runs when repo/tag/profile are all set.
func (n Neo) manifestListViaSlot(l *logrus.Entry, span trace.Span, up upstream.UpstreamTriple) (string, bool) {
	if up.Digest.String() != "" || up.Profile == "" || up.Repo == "" || up.Tag == "" {
		return "", false
	}
	qSlot := fmt.Sprintf(`MATCH (s:Slot {repo: "%s", tag: "%s", profile: "%s"})-[:POINTS_TO]->(ml:Index) RETURN ml.mediaType AS mediaType LIMIT 1`,
		up.Repo, up.Tag, up.Profile)
	utils.Debug(l, span, "ManifestListExists: Slot-based fallback query: %s", qSlot)
	resSlot, eSlot := n.Execute(qSlot)
	if eSlot != nil {
		utils.Debug(l, span, "ManifestListExists: Slot-based fallback query error (non-fatal): %v", eSlot)
		return "", false
	}
	mdSlot, errSlot := resSlot.GetStr("mediaType")
	if errSlot != nil || mdSlot == "" {
		return "", false
	}
	utils.Debug(l, span, "found Index via Slot POINTS_TO with type '%s'", mdSlot)
	return mdSlot, true
}

// Index origin labels used by SetIndexOrigin. These are the two sources of
// truth for where an :Index node came from in the slot-centred model.
const (
	// IndexOriginClientPushed indicates the Index was pushed by a client
	// via the PUT /v2/<name>/manifests/<reference> endpoint where the
	// body is an OCI image index. The :Index node's bytes are stored
	// verbatim in minio, and the REFERENCES edges are built from the
	// descriptors in the pushed manifest list. Set by StoreIndexUpload.
	IndexOriginClientPushed = "client-pushed"
	// IndexOriginComputed indicates the Index was built server-side by
	// the compute path from the :Slot's HAS_PUSHED manifests. This
	// happens on every successful manifest push via
	// ComputeManifestlistForCTTP / wireSlotPointsToOnCompute. Set by
	// the compute path.
	IndexOriginComputed = "computed"
)

// SetIndexOrigin sets the `origin` property on an :Index node using
// first-writer-wins semantics: if the node already has an origin set,
// the call is a no-op. This supports Path Z design decision D8's
// "client-pushed vs computed" attribution — the first path to touch
// the Index wins and subsequent pushes that produce the same digest
// preserve the original origin.
//
// Cypher:
//
//	MATCH (i:Index) WHERE id(i) = $id
//	WITH i WHERE i.origin IS NULL
//	SET i.origin = $origin
//
// The `WITH ... WHERE` filter only matches Index nodes that have no
// origin property yet, so subsequent calls with different origins
// are silently dropped at the graph layer.
func (n Neo) SetIndexOrigin(ctx context.Context, indexID int64, origin string) (err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "SetIndexOrigin",
		"indexID": indexID,
		"origin":  origin,
	})
	defer span.End()
	if origin == "" {
		return utils.NewError(l, span, "SetIndexOrigin: empty origin")
	}
	q := fmt.Sprintf(
		`MATCH (i:Index) WHERE id(i) = %d WITH i WHERE i.origin IS NULL SET i.origin = "%s"`,
		indexID, origin,
	)
	if _, e := n.Execute(q); e != nil {
		return utils.ErrorHandler(l, span, e, "SetIndexOrigin: executing SET query")
	}
	utils.Debug(l, span, "SetIndexOrigin: first-writer-wins attempt for index %d -> origin %s", indexID, origin)
	return nil
}

// LinkIndexReferenceWithOrder creates a (:Index)-[:REFERENCES {order}]->(:Manifest)
// edge, using MERGE to be idempotent on repeated calls. The `order`
// property captures the descriptor's position in the client-pushed
// OCI manifest list so the pull path can reconstruct the original
// ordering without walking every descriptor.
//
// Path Z task 8.2. The `platform` property is NOT set here because it
// duplicates the :HAS_PLATFORM edge on the content-leaf Manifest;
// keeping it on the leaf avoids double-writes and keeps the edge
// properties minimal. If a future pull path needs the platform in the
// REFERENCES edge itself, it can be added as a secondary property.
func (n Neo) LinkIndexReferenceWithOrder(ctx context.Context, indexID, mdID int64, order int) (err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameML,
		"func":    "LinkIndexReferenceWithOrder",
		"indexID": indexID,
		"mdID":    mdID,
		"order":   order,
	})
	defer span.End()
	if indexID < 0 || mdID < 0 {
		return utils.NewError(l, span, "LinkIndexReferenceWithOrder: invalid IDs indexID:%d mdID:%d", indexID, mdID)
	}
	q := fmt.Sprintf(
		`MATCH (i:Index), (md:Manifest) WHERE id(i) = %d AND id(md) = %d MERGE (i)-[r:REFERENCES]->(md) ON CREATE SET r.order = %d ON MATCH SET r.order = %d`,
		indexID, mdID, order, order,
	)
	if _, e := n.Execute(q); e != nil {
		return utils.ErrorHandler(l, span, e, "LinkIndexReferenceWithOrder: executing MERGE")
	}
	utils.Debug(l, span, "LinkIndexReferenceWithOrder: index %d -[:REFERENCES {order: %d}]-> md %d", indexID, order, mdID)
	return nil
}
