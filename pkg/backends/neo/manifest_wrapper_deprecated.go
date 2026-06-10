package neo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	goSet "github.com/scylladb/go-set/i64set"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameMani   = "neo/manifest.go"
	AnnoOSVersion = "org.qnib.system.os.version"
	AnnoOSId      = "org.qnib.system.os.id"
)

func (n Neo) ManifestWrapperLinkToAnnotation(ctx context.Context, maniID, annotationID int64) (err error) {
	q := `MATCH (m:ManifestWrapper), (a:Annotation) WHERE id(m) = %d AND id(a) = %d MERGE (m)-[:HAS_ANNOTATION]->(a)`
	query := fmt.Sprintf(q, maniID, annotationID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) ManifestWrapperUnlinkAnnotation(ctx context.Context, maniID, annotationID int64) (err error) {
	q := `MATCH (m:ManifestWrapper)-[r]->(a:Annotation) WHERE id(m) = %d AND id(a) = %d DELETE r`
	query := fmt.Sprintf(q, maniID, annotationID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) ManifestWrapperGetInsertWithDesc(ctx context.Context, up types.Upstream, maniIn manifestlist.ManifestDescriptor) (maniID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMani,
		"func":    "ManifestWrapperGetInsertWithDesc",
		"repo":    up.Repo,
		"tag":     up.Tag,
	})
	ctx, span := tracer.Start(ctx, "ManifestWrapperGetInsertWithDesc")
	defer span.End()
	// If Manifest between collection and upstream does not exists
	label := fmt.Sprintf("%s/%s:%s-%s", up.Reg, up.Repo, up.Tag, maniIn.Platform.Architecture)
	if maniIn.Platform.Variant != "" {
		label += fmt.Sprintf("-%s", maniIn.Platform.Variant)
	}
	mani, found, err := n.ManifestWrapperGetByLabel(ctx, label)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "while executing ManifestWrapperGetByLabel")
		return
	}
	if found {
		l.Debugf("Manifest found: %d", mani.ID)
	} else {
		// Create ManifestStandin
		maniID, err = n.manifestCreateViaUpstream(ctx, label, up.ID)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, types.FailmanifestCreateViaUpstream)
			return
		}
		l.Debugf("Manifest created: %d", maniID)
	}
	return
}
func (n Neo) ManifestWrapperGetInsert(ctx context.Context, up types.Upstream, as ...types.Annotation) (maniID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestWrapperGetInsert",
		"repo":    up.Repo,
		"tag":     up.Tag,
	})
	defer span.End()
	/*** TODO:
	1. Check if Manifest exists as a connection between
	   a. Upstream
	   b. Repo
	   (c. Annotation?)
	*/
	utils.Debug(l, span, "UpstreamGetInsert kicked off")
	_, err = n.UpstreamGetInsert(ctx, &up)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get/insert upstream")
		return
	}
	time.Sleep(500 * time.Millisecond) // let the db catch up
	utils.Debug(l, span, "GetInserted Upstream with ID %d", up.ID)
	// If Manifest between collection and upstream does not exists
	mani, found, err := n.ManifestWrapperGetByLabel(ctx, up.GetLabel())
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get manifest")
		return
	}
	if !found {
		maniID, err = n.manifestCreateViaUpstream(ctx, up.GetLabel(), up.ID)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to create manifest via upstream")
			return
		}
	} else {
		maniID = mani.ID
	}
	utils.Debug(l, span, "Manifest has %d annotations", len(as))
	for _, a := range as {
		aID, e := n.AnnotationGetInsert(ctx, a)
		if e != nil {
			err = utils.ErrorHandler(l, span, err, "failed to get/insert annotation: %s", a.Key)
			return
		}
		e = n.ManifestWrapperLinkToAnnotation(ctx, maniID, aID)
		if e != nil {
			err = utils.ErrorHandler(l, span, err, "failed to link manifest to annotation: %s", a.Key)
			return
		}
		// Check if Annotation exists
	}
	/**** Do we need this still ****/
	/// OK! We got the ManifestStandin, Repo and Upstream
	// Check if Manifest exists
	//maniID, found, err = n.ManifestGet(msID, m.Digest)
	return
}

func (n Neo) manifestCreateViaUpstream(ctx context.Context, label string, upID int64) (msID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "manifestCreateViaUpstream",
		"upID":    upID,
		"label":   label,
	})
	defer span.End()
	m, found, e := n.ManifestWrapperGetByLabel(ctx, label)
	if e != nil {
		return msID, utils.ErrorHandler(l, span, e, types.FailManifestGetByLabel)
	}
	if found {
		return m.ID, nil
	}
	q := fmt.Sprintf(`CREATE (m:ManifestWrapper { label: "%s" }) RETURN id(m) AS ID`, label)
	if res, e := n.Execute(q); e != nil {
		return msID, utils.ErrorHandler(l, span, e, "while executing query")
	} else {
		if msID, e = res.GetInt64("ID"); e != nil {
			return msID, utils.ErrorHandler(l, span, e, "while getting ID")
		}
	}
	// Link Upstream
	if e = n.ManifestWrapperLinkToUpstream(ctx, msID, upID); e != nil {
		return msID, utils.ErrorHandler(l, span, e, types.FailManifestLinkToUpstream)
	}
	return
}

// NOTE: ManifestGetIDByManifest and ManifestCreate were deleted in
// Path Z Phase C3 step 3. Both were only called by the TestManifestCR
// test function (also deleted in the same commit) that exercised raw
// wrapper :ManifestWrapper node creation and traversal. In the slot-centred
// model there is no standalone :ManifestWrapper wrapper node to create or
// traverse — the Slot node carries (repo, tag, profile) identity and
// content-leaf :Manifest is attached via :HAS_PUSHED or :POINTS_TO
// edges depending on whether the manifest was pushed or computed.

// Phase C3: LinkManifestToPlatform moved to manifest_data.go.

// NOTE: ManifestGet was deleted in Path Z Phase C3 step 3. It walked
// the wrapper :ManifestWrapper node looking for the one attached to a given
// :Upstream, an operation that is not meaningful in the slot-centred
// model (the Slot node carries the (repo, tag, profile) identity and
// is keyed by property rather than traversed through a :Manifest
// wrapper). Only called by the three wrapper-era tests deleted in
// the same commit (TestManifestCR and TestManifestWrapperGetInsert).

func (n Neo) GetManifestWrapperLinksManifestByID(ctx context.Context, maniID int64) (maniDataIDs *goSet.Set, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "GetManifestWrapperLinksManifestByID",
		"maniID":  maniID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (m:ManifestWrapper)-[]->(md:Manifest)`
	q += fmt.Sprintf(` WHERE id(m) = %d`, maniID)
	q += ` RETURN id(md) AS maniDataID`
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return maniDataIDs, utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	maniDataIDs = goSet.New()
	for _, r := range res {
		maniDataID, e := r.GetInt64("maniDataID")
		if e != nil {
			return maniDataIDs, utils.ErrorHandler(l, span, e, "while getting maniDataID")

		}
		maniDataIDs.Add(maniDataID)
	}
	return maniDataIDs, nil
}

func (n Neo) LinkManifestWrapperToManifest(ctx context.Context, maniID, maniDataID int64, props ...string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMani,
		"func":       "LinkManifestWrapperToManifest",
		"maniID":     maniID,
		"maniDataID": maniDataID,
	})
	_ = ctx
	defer span.End()
	// Is the Manifest already linked to a different Manifest?
	mds, err := n.GetManifestWrapperLinksManifestByID(ctx, maniID)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "while getting manifest linked manifestdata")
	}
	if mds.Has(maniDataID) {
		utils.Debug(l, span, "Manifest ID '%d' is already linked to Manifest ID '%d'", maniID, maniDataID)
		return nil
	} else {
		for _, mid := range mds.List() {
			q := fmt.Sprintf(`MATCH (m:ManifestWrapper)-[r]->(md:Manifest) WHERE id(m) = %d AND id(md) = %d DELETE r`, maniID, mid)
			_, e := n.Execute(q)
			if e != nil {
				return utils.ErrorHandler(l, span, e, "while unlinking existing manifestdata")
			}
			utils.Debug(l, span, "Unlinked existing Manifest ID '%d' from Manifest ID '%d'", mid, maniID)
		}
	}
	q := fmt.Sprintf(`MATCH (m:ManifestWrapper), (md:Manifest) WHERE id(m) = %d AND id(md) = %d`, maniID, maniDataID)
	rel := "PARSED"
	if len(props) > 1 {
		return utils.NewError(l, span, "relationship type has %d entries, should be at max 1", len(props))
	}
	if len(props) > 0 && props[0] != "" {
		rel = strings.ToUpper(props[0])
	}
	q += fmt.Sprintf(` MERGE (m)-[:%s]->(md)`, rel)
	utils.Debug(l, span, q)
	if _, e := n.Execute(q); e != nil {
		return utils.ErrorHandler(l, span, e, FailNeoExecute)
	}
	return
}

func (n Neo) ManifestWrapperCleanUpByID(ctx context.Context, maniID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestWrapperCleanUpByID",
		"maniID":  maniID,
	})
	_ = ctx
	defer span.End()
	// Check how mani connection the Manifest still has
	q := fmt.Sprintf(`MATCH (m:ManifestWrapper)-[]->(md:Manifest) WHERE id(m) = %d`, maniID)
	q += ` RETURN count(md) AS countManiData`
	countManiData := -1
	if res, _, e := n.QueryOne(q); e != nil {
		return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else {
		if countManiData, e = res.GetInt("countManiData"); e != nil {
			return utils.ErrorHandler(l, span, e, types.FailBrGet, "countManiData")
		}
	}
	// Check for CollectionTags
	colTagID, colTagFound, err := n.CollectionTagGetIDByManifestID(ctx, maniID)
	if err != nil {
		return utils.ErrorHandler(l, span, err, types.FailCollectionTagGetIDByManifestID)
	}
	upID, upFound, err := n.UpstreamGetIDByManifestID(ctx, maniID)
	if err != nil {
		return utils.ErrorHandler(l, span, err, types.FailUpstreamGetIDByManifestID)
	}
	utils.Debug(l, span, "Manifest has '%d' relationships to Manifest", countManiData)
	if countManiData == 0 {
		q := fmt.Sprintf(`MATCH (m:ManifestWrapper) WHERE id(m) = %d DETACH DELETE m`, maniID)
		if _, e := n.Execute(q); e != nil {
			return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
		}
	}
	if upFound {
		if err := n.UpstreamCleanupByID(ctx, upID); err != nil {
			return utils.ErrorHandler(l, span, err, types.FailUpstreamCleanupByID)
		}
	} else {
		utils.Debug(l, span, "no upstream found connected to manifest '%d'", maniID)
	}
	if colTagFound {
		if err := n.CollectionTagCleanupByID(ctx, colTagID); err != nil {
			return utils.ErrorHandler(l, span, err, types.FailCollectionTagCleanupByID)
		}
	} else {
		utils.Debug(l, span, "no ColelctionTag found connected to manifest '%d'", maniID)
	}
	return nil
}

func (n Neo) ManifestWrapperGetIDByManifestID(ctx context.Context, maniDataID int64) (maniID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMani,
		"func":       "ManifestWrapperGetIDByManifestID",
		"maniDataID": maniDataID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (m:ManifestWrapper)-[]->(md:Manifest)`
	q += fmt.Sprintf(` WHERE id(md) = %d`, maniDataID)
	q += ` RETURN id(m) AS maniID`
	if res, found, e := n.QueryOne(q); e != nil {
		return maniID, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else {
		if !found {
			return maniID, false, nil
		}
		if maniID, e := res.GetInt64("maniID"); e != nil {
			return maniID, false, utils.ErrorHandler(l, span, e, types.FailBrGet, "maniID")
		} else {
			return maniID, true, nil
		}
	}
}

func (n Neo) ManifestWrapperGetByID(ctx context.Context, maniID int64) (mani types.ManifestWrapper, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestWrapperGetByID",
		"maniID":  maniID,
	})
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (m:ManifestWrapper)`
	q += fmt.Sprintf(` WHERE id(m) = %d`, maniID)
	q += ` RETURN id(m) AS manifestID, m.label AS manifestLabel`
	//q += ` ,` + UpstreamReturn
	utils.Debug(l, span, q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	for result.Next() {
		mani, err = n.ManifestWrapperFromRec(ctx, result)
		return mani, true, err
	}
	return mani, false, nil
}

func (n Neo) ManifestWrapperGetByIDWithProfile(ctx context.Context, maniID, prfID int64) (mani types.ManifestWrapper, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestWrapperGetByIDWithProfile",
		"maniID":  maniID,
		"prfID":   prfID,
	})
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[]-(md:Manifest)-[]-(m:ManifestWrapper)-[]-(u:Upstream)`
	q += fmt.Sprintf(` WHERE id(m) = %d`, maniID)
	q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(m) AS manifestID, m.label AS manifestLabel`
	q += ` ,` + UpstreamReturn
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	l.Debug(q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	for result.Next() {
		mani, err = n.ManifestWrapperFromRecWithProfile(ctx, result, prfID)
		return mani, true, err
	}
	return mani, false, fmt.Errorf("no manifest found")
}

func (n Neo) ManifestWrapperGetByLabel(ctx context.Context, maniLabel string) (mani types.ManifestWrapper, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestWrapperGetByLabel",
	})
	defer span.End()
	// Manifest MUST be connected to upstream, optional to platform (later)
	q := `MATCH (m:ManifestWrapper)`
	q += fmt.Sprintf(` WHERE m.label = "%s"`, maniLabel)
	q += ` RETURN id(m) AS manifestID, m.label AS manifestLabel`
	//q += ` ,` + UpstreamReturn
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		return mani, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)

	}
	if !found {
		utils.Debug(l, span, types.FailManifestNotFound)
		return mani, false, nil
	}
	mani, e = n.ManifestWrapperFromResult(ctx, res)
	if e != nil {
		return mani, false, utils.ErrorHandler(l, span, e, types.FailManiFromResult)
	}
	return
}

func (n Neo) ManifestWrapperGetAll(ctx context.Context) (manis []types.ManifestWrapper, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestWrapperGetAll",
	})
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (m:ManifestWrapper)`
	q += ` RETURN id(m) AS manifestID, m.label AS manifestLabel`
	utils.Debug(l, span, q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	manis = make([]types.ManifestWrapper, 0)

	for result.Next() {
		utils.Debug(l, span, "processing manifest record")
		if m, err := n.ManifestWrapperFromRec(ctx, result); err != nil {
			return manis, utils.ErrorHandler(l, span, err, types.FailManiFromRecord)
		} else {
			manis = append(manis, m)
		}
	}
	return
}

func (n Neo) ManifestWrapperFromResult(ctx context.Context, res types.Result) (m types.ManifestWrapper, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestWrapperFromResult",
	})
	defer span.End()
	_ = l
	err = res.CheckKeys("manifestID", "manifestLabel")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("manifestID")
	label, _ := res.GetStr("manifestLabel")
	var up types.Upstream
	if e := res.CheckKeys("upstreamID"); e == nil {
		up, e = UpstreamFromResult(res)
		if e != nil {
			return
		}
	}

	m = types.ManifestWrapper{
		ID:          id,
		Label:       label,
		Upstream:    up,
		Annotations: types.NewAnnotations(),
		Manifests:   []types.Manifest{},
	}
	anno, e := n.GetAnnotationsByManifestWrapperID(ctx, m.ID)
	if e != nil {
		err = e
		return
	}
	m.Annotations = anno
	mds, e := n.ManifestGetAllByManifestID(ctx, m.ID)
	if e != nil {
		err = e
		return
	}
	m.Manifests = mds
	return
}

func (n Neo) ManifestWrapperFromRec(ctx context.Context, res neo4j.Result) (m types.ManifestWrapper, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestWrapperFromRec",
	})

	defer span.End()
	var u types.Upstream
	u, e := UpstreamFromRecord(res)
	if e != nil {
		utils.Warn(l, span, "No Upstream for Manifest - which is OK, since Manifest MUST be connected, Manifest not necessarily")
	}
	m, e = ManifestWrapperFromRecord(res)
	if e != nil {
		err = e
		return
	}
	anno, e := n.GetAnnotationsByManifestWrapperID(ctx, m.ID)
	if e != nil {
		err = e
		return
	}
	m.Annotations = anno
	m.Upstream = u
	mds, e := n.ManifestGetAllByManifestID(ctx, m.ID)
	if e != nil {
		err = e
		return
	}
	m.Manifests = mds
	return
}

func (n Neo) ManifestWrapperFromRecWithProfile(ctx context.Context, res neo4j.Result, prfID int64) (m types.ManifestWrapper, err error) {
	ctx, span := tracer.Start(ctx, "ManifestWrapperFromRecWithProfile")
	defer span.End()
	u, e := UpstreamFromRecord(res)
	if e != nil {
		err = e
		return
	}
	m, e = ManifestWrapperFromRecord(res)
	if e != nil {
		err = e
		return
	}
	anno, e := n.GetAnnotationsByManifestWrapperID(ctx, m.ID)
	if e != nil {
		err = e
		return
	}
	m.Annotations = anno
	m.Upstream = u
	//mds, e := n.ManifestGetAllByManifestIDWithProfile(ctx, m.ID, prfID)

	mds, e := n.ManifestGetAllByManifestIDWithProfile(ctx, m.ID, prfID)
	if e != nil {
		err = e
		return
	}
	m.Manifests = mds
	return
}

func (n Neo) GetManifestWrapperLinksUpstreamByID(ctx context.Context, maniID int64) (UpstreamIDs *goSet.Set, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "GetManifestWrapperLinksUpstreamByID",
		"maniID":  maniID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (m:ManifestWrapper)-[]->(up:Upstream)`
	q += fmt.Sprintf(` WHERE id(m) = %d`, maniID)
	q += ` RETURN id(up) AS upID`
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return UpstreamIDs, utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	UpstreamIDs = goSet.New()
	for _, r := range res {
		upID, e := r.GetInt64("upID")
		if e != nil {
			return UpstreamIDs, utils.ErrorHandler(l, span, e, "while getting upID")

		}
		UpstreamIDs.Add(upID)
	}
	return UpstreamIDs, nil
}

func (n Neo) ManifestWrapperLinkToUpstream(ctx context.Context, maniID, upID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestWrapperLinkToUpstream",
		"maniID":  maniID,
		"upID":    upID,
	})
	defer span.End()
	_ = ctx
	// Get Links from Manifest to Upstream
	ups, err := n.GetManifestWrapperLinksUpstreamByID(ctx, maniID)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "while getting manifest linked upstreams")
	}
	if len(ups.List()) > 0 {
		if ups.Has(upID) {
			utils.Debug(l, span, "Manifest ID '%d' is already linked to Upstream ID '%d'", maniID, upID)
			return nil
		} else {
			for _, uid := range ups.List() {
				q := fmt.Sprintf(`MATCH (m:ManifestWrapper)-[r]->(u:Upstream) WHERE id(m) = %d AND id(u) = %d DELETE r`, maniID, uid)
				_, e := n.Execute(q)
				if e != nil {
					return utils.ErrorHandler(l, span, e, "while unlinking existing upstream")
				}
				utils.Debug(l, span, "Unlinked existing Upstream ID '%d' from Manifest ID '%d'", uid, maniID)
			}
		}
	}
	q := fmt.Sprintf(`MATCH (m:ManifestWrapper),(u:Upstream) WHERE id(m) = %d AND id(u) = %d`, maniID, upID)
	q += ` MERGE (m)-[:UPSTREAM]->(u)`
	utils.Debug(l, span, q)
	_, e := n.Execute(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, types.FailNeoExecute)
		return
	}
	return
}

func ManifestWrapperFromRecord(res neo4j.Result) (m types.ManifestWrapper, err error) {
	r := res.Record()
	v, ok := r.Get("manifestID")
	if !ok {
		err = fmt.Errorf("no manifestID found in record: %v", r)
		return
	}
	m.ID = v.(int64)
	v, ok = r.Get("manifestLabel")
	if !ok {
		err = fmt.Errorf("no manifestLabel found in record: %v", r)
		return
	}
	m.Label = v.(string)
	return
}

// ManifestExistsForTagProfile reports whether an Index exists for the given
// (repo, tag, profile) triple. It returns the stored mediaType on hit, the
// empty string on miss. The name intentionally drops the "SI" suffix from
// the wrapper-era ManifestWrapperExistsForTagProfile because this helper is
// Slot-agnostic — it walks :Index directly without touching the legacy
// :ManifestWrapper wrapper node. Part of Path Z Phase C3 cleanup.
// Phase C3: ManifestExistsForTagProfile moved to manifest_data.go.

// ManifestExists reports whether a manifest-or-index exists for the given
// (repo, tag, [digest], profile) request. It walks :Index first (multi-arch
// index case), then falls back to :Manifest (single-arch case) and
// returns the discovered mediaType on hit. The name intentionally drops
// the "SI" suffix from the wrapper-era ManifestWrapperExists because this
// helper is Slot-agnostic — it never consults the legacy :Manifest
// wrapper node. Part of Path Z Phase C3 cleanup.
// Phase C3: ManifestExists moved to manifest_data.go.
