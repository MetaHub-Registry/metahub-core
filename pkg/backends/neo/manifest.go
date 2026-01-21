package neo

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/minio/minio-go/pkg/set"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameMani   = "neo/manifest.go"
	AnnoOSVersion = "org.qnib.system.os.version"
	AnnoOSId      = "org.qnib.system.os.id"
)

func (n Neo) Manifest_LinkToAnnotation(ctx context.Context, maniID, annotationID int64) (err error) {
	q := `MATCH (m:Manifest), (a:Annotation) WHERE id(m) = %d AND id(a) = %d MERGE (m)-[:HAS_ANNOTATION]->(a)`
	query := fmt.Sprintf(q, maniID, annotationID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) ManifestUnlinkAnnotation(ctx context.Context, maniID, annotationID int64) (err error) {
	q := `MATCH (m:Manifest)-[r]->(a:Annotation) WHERE id(m) = %d AND id(a) = %d DELETE r`
	query := fmt.Sprintf(q, maniID, annotationID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) Manifest_GetInsert_WithDesc(ctx context.Context, up types.Upstream, maniIn manifestlist.ManifestDescriptor) (maniID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMani,
		"func":    "Manifest_GetInsert_WithDesc",
		"repo":    up.Repo,
		"tag":     up.Tag,
	})
	ctx, span := tracer.Start(ctx, "Manifest_GetInsert_WithDesc")
	defer span.End()
	// If Manifest between collection and upstream does not exists
	label := fmt.Sprintf("%s/%s:%s-%s", up.Reg, up.Repo, up.Tag, maniIn.Platform.Architecture)
	if maniIn.Platform.Variant != "" {
		label += fmt.Sprintf("-%s", maniIn.Platform.Variant)
	}
	mani, found, err := n.ManifestGetByLabel(ctx, label)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "while executing Manifest_GetByLabel")
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

func (n Neo) ManifestGetInsertWithRequest(ctx context.Context, m types.Manifest, req types.Request) (maniID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMani,
		"func":    "ManifestGetInsertWithRequest",
		"profile": req.Profile,
		"repo":    req.Repo,
		"tag":     req.Tag,
	})
	/*** TODO:
	1. Check if Manifest exists as a connection between
	   a. Upstream
	   b. Collection
	   (c. Annotation?)
	*/
	up := m.Upstream
	_, err = n.Upstream_GetInsert(ctx, &up)
	if err != nil {
		l.Error(err)
		return
	}
	maniID, err = n.ManifestGetInsert(ctx, up)
	if err != nil {
		logrus.Error(err)
		return
	}
	for _, a := range m.Annotations.Data {
		aID, e := n.Annotation_GetInsert(ctx, a)
		if e != nil {
			err = errors.Wrap(e, fmt.Sprintf("failed to get/insert annotation: %s", a.Key))
			return
		}
		e = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
		if e != nil {
			err = errors.Wrap(e, fmt.Sprintf("failed to link manifest to annotation: %s", a.Key))
			return
		}
		// Check if Annotation exists
	}
	/**** Do we need this still ****/
	/// OK! We got the ManifestStandin, Collection and Upstream
	// Check if Manifest exists
	//maniID, found, err = n.ManifestData_Get(msID, m.Digest)
	return
}

func (n Neo) ManifestGetInsert(ctx context.Context, up types.Upstream, as ...types.Annotation) (maniID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestGetInsert",
		"repo":    up.Repo,
		"tag":     up.Tag,
	})
	defer span.End()
	/*** TODO:
	1. Check if Manifest exists as a connection between
	   a. Upstream
	   b. Collection
	   (c. Annotation?)
	*/
	_, err = n.Upstream_GetInsert(ctx, &up)
	if err != nil {
		l.Error(err)
		return
	}
	// If Manifest between collection and upstream does not exists
	maniID, found, err := n.Manifest_Get(ctx, up.ID)
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		utils.Debug(l, span, "Manifest found: %d", maniID)
	} else {
		// Create ManifestStandin
		maniID, err = n.ManifestCreate(ctx, up.ID)
		if err != nil {
			logrus.Error(err)
			return
		}
		l.Debugf("Manifest created: %d", maniID)
	}
	for _, a := range as {
		aID, e := n.Annotation_GetInsert(ctx, a)
		if e != nil {
			err = errors.Wrap(e, fmt.Sprintf("failed to get/insert annotation: %s", a.Key))
			return
		}
		e = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
		if e != nil {
			err = errors.Wrap(e, fmt.Sprintf("failed to link manifest to annotation: %s", a.Key))
			return
		}
		// Check if Annotation exists
	}
	/**** Do we need this still ****/
	/// OK! We got the ManifestStandin, Collection and Upstream
	// Check if Manifest exists
	//maniID, found, err = n.ManifestData_Get(msID, m.Digest)
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
	m, found, e := n.ManifestGetByLabel(ctx, label)
	if e != nil {
		return msID, utils.ErrorHandler(l, span, e, types.FailManifestGetByLabel)
	}
	if found {
		return m.ID, nil
	}
	q := fmt.Sprintf(`CREATE (m:Manifest { label: "%s" }) RETURN id(m) AS ID`, label)
	if res, e := n.Execute(q); e != nil {
		return msID, utils.ErrorHandler(l, span, e, "while executing query")
	} else {
		if msID, e = res.GetInt64("ID"); e != nil {
			return msID, utils.ErrorHandler(l, span, e, "while getting ID")
		}
	}
	// Link Upstream
	if e = n.ManifestLinkToUpstream(ctx, msID, upID); e != nil {
		return msID, utils.ErrorHandler(l, span, e, types.FailManifestLinkToUpstream)
	}
	return
}

func (n Neo) ManifestGetIDByManifestData(ctx context.Context, maniDataID int64) (maniID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMani,
		"func":       "ManifestGetIDByManifestData",
		"maniDataID": maniDataID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:ManifestData)<-[]-(m:Manifest) WHERE id(md) = %d RETURN id(m) AS maniID`, maniDataID)
	if res, err := n.Query(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailNeoExecute)
		return maniID, false, err
	} else {
		switch len(res) {
		case 0:
			return maniID, false, nil
		case 1:
			if maniID, err = res[0].GetInt64("maniID"); err != nil {
				err = utils.ErrorHandler(l, span, err, "while getting maniID")
				return maniID, false, err
			} else {
				return maniID, true, nil
			}
		default:
			utils.NewError(l, span, "more than one manifest found")
		}
	}
	return maniID, false, err
}

// ManifestCreate creates just a manifest with a label
// since we do not have information about the platform, that'll be defered
func (n Neo) ManifestCreate(ctx context.Context, upID int64) (msID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestCreate",
		"upID":    upID,
	})
	defer span.End()
	up, _, err := n.UpstreamGetByID(ctx, upID)
	if err != nil {
		err = errors.Wrap(err, "failed to get upstream")
		l.Error(err)
		return
	}
	label := fmt.Sprintf("%s/%s:%s", up.Reg, up.Repo, up.Tag)
	return n.manifestCreateViaUpstream(ctx, label, upID)
}

func (n Neo) Manifest_Get_CreateWithProfile(ctx context.Context, upID int64, prfName string) (mani types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMani,
		"func":    "Manifest_CreateWithProfile",
		"upID":    upID,
	})
	ctx, span := tracer.Start(ctx, "Manifest_CreateWithProfile")
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	up, _, err := n.UpstreamGetByID(ctx, upID)
	if err != nil {
		err = errors.Wrap(err, "failed to get upstream")
		l.Error(err)
		return
	}
	label := fmt.Sprintf("%s/%s:%s@%s", up.Reg, up.Repo, up.Tag, prfName)
	l = logrus.WithFields(logrus.Fields{
		"package":  pkgNameMani,
		"func":     "Manifest_CreateWithProfile",
		"upID":     upID,
		"upstream": label,
	})

	q := fmt.Sprintf(`MATCH (u:Upstream) WHERE id(u) = %d`, upID)
	q += fmt.Sprintf(` MERGE (m:Manifest {label: "%s"})-[:UPSTREAM]->(u)`, label)
	q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(m) AS manifestID,`
	q += ` ` + UpstreamReturn
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	for result.Next() {
		return n.ManiFromRec(ctx, result)
	}
	return
}

func (n Neo) LinkManifestToPlatform(ctx context.Context, maniID, platID int64) (err error) {
	_, span := tracer.Start(ctx, "LinkManifestToPlatform")
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniID").Int64(maniID),
		attribute.Key("platID").Int64(platID),
	)
	q := fmt.Sprintf(`MATCH (m:Manifest),(p:Platform) WHERE id(m) = %d AND id(p) = %d`, maniID, platID)
	q += ` MERGE (m)-[:IS]->(p)`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(q, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:Manifest)-[:IS]->(p:Platform): %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	return
}

func (n Neo) Manifest_AddAnnotation(ctx context.Context, maniID int64, a *types.Annotation) (err error) {
	aID, err := n.Annotation_GetInsert(ctx, *a)
	if err != nil {
		return
	}
	a.ID = aID
	err = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
	return
}

func (n Neo) Manifest_AddAnnotations(ctx context.Context, maniID int64, as *types.Annotations) (err error) {
	for _, a := range as.Data {
		aID, e := n.Annotation_GetInsert(ctx, a)
		if e != nil {
			err = errors.Wrap(e, "failed to add annotation")
			return
		}
		a.ID = aID
		err = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
	}
	return
}

func (n Neo) Manifest_Get(ctx context.Context, upID int64, suffix ...string) (msID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "Manifest_Get",
		"suffix":  strings.Join(suffix, ","),
	})
	_ = ctx
	defer span.End()
	q := `MATCH (m:Manifest)-[]->(u:Upstream)`
	q += fmt.Sprintf(` WHERE id(u) = %d`, upID)
	if len(suffix) == 1 {
		q += fmt.Sprintf(` AND m.label ENDS WITH "%s"`, suffix[0])
	}
	q += ` RETURN id(m) AS ID`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !found {
		// Not found, return
		return
	}
	msID, err = res.GetInt64("ID")
	// error will be nil if found
	return

}

func (n Neo) LinkManifestToManifestData(ctx context.Context, maniID, maniDataID int64, props ...string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMani,
		"func":       "LinkManifestToManifestData",
		"maniID":     maniID,
		"maniDataID": maniDataID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (m:Manifest), (md:ManifestData) WHERE id(m) = %d AND id(md) = %d`
	rel := "PARSED"
	if len(props) > 1 {
		return utils.NewError(l, span, "relationship type has %d entries, should be at max 1", len(props))
	}
	if len(props) > 0 && props[0] != "" {
		rel = strings.ToUpper(props[0])
	}
	q += fmt.Sprintf(` MERGE (m)-[:%s]->(md)`, rel)
	utils.Debug(l, span, q)
	query := fmt.Sprintf(q, maniID, maniDataID)
	if _, e := n.Execute(query); e != nil {
		return utils.ErrorHandler(l, span, e, FailNeoExecute)
	}
	return
}

func (n Neo) Manifest_GetByUpstreamDigest(ctx context.Context, upDigest string) (mani types.Manifest, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMani,
		"func":    "Manifest_GetByUpstreamDigest",
	})
	_, span := tracer.Start(ctx, "Manifest_GetByUpstreamDigest")
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[]-(m:Manifest)-[]-(u:Upstream)`
	q += fmt.Sprintf(` WHERE u.digest = "%s"`, upDigest)
	q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(m) AS manifestID,`
	q += ` ` + UpstreamReturn
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	for result.Next() {
		mani, err = n.ManiFromRec(ctx, result)
		return mani, true, err
	}
	return mani, false, fmt.Errorf("no manifest found")
}

func (n Neo) ManifestGetByRepoTag(ctx context.Context, repo, tag string) (mani types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/manifest_data.go",
		"func":    "ManifestGetByRepoTag",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (col:Collection)-[:UPSTREAM]->(m:Manifest)<-[:UPSTREAM]-(t:Tag)`
	q += `MATCH (p:Platform)-[]-(m)-[]-(u:Upstream)`
	q += fmt.Sprintf(` WHERE col.repo = "%s" AND t.name = "%s"`, repo, tag)
	q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(m) AS manifestID,`
	q += ` ` + UpstreamReturn
	utils.Debug(l, span, q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	for result.Next() {
		mani, err = n.ManiFromRec(ctx, result)
		return mani, true, err
	}
	return mani, false, fmt.Errorf("no manifest found")
}

func (n Neo) ManifestCleanUpByID(ctx context.Context, maniID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestCleanUpByID",
		"maniID":  maniID,
	})
	_ = ctx
	defer span.End()
	// Check how mani connection the Manifest still has
	q := fmt.Sprintf(`MATCH (m:Manifest)-[]->(md:ManifestData) WHERE id(m) = %d`, maniID)
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
	utils.Debug(l, span, "Manifest has '%d' relationships to ManifestData", countManiData)
	if countManiData == 0 {
		q := fmt.Sprintf(`MATCH (m:Manifest) WHERE id(m) = %d DETACH DELETE m`, maniID)
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

func (n Neo) ManifestGetIDByManifestDataID(ctx context.Context, maniDataID int64) (maniID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMani,
		"func":       "ManifestGetIDByManifestDataID",
		"maniDataID": maniDataID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (m:Manifest)-[]->(md:ManifestData)`
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

func (n Neo) ManifestGetByID(ctx context.Context, maniID int64) (mani types.Manifest, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMani,
		"func":    "ManifestGetByID",
	})
	ctx, span := tracer.Start(ctx, "ManifestGetByID")
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[]-(m:Manifest)-[]-(u:Upstream)`
	q += fmt.Sprintf(` WHERE id(m) = %d`, maniID)
	q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(m) AS manifestID,`
	q += ` ` + UpstreamReturn
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	for result.Next() {
		mani, err = n.ManiFromRec(ctx, result)
		return mani, true, err
	}
	return mani, false, nil
}

func (n Neo) ManifestGetByIDWithProfile(ctx context.Context, maniID, prfID int64) (mani types.Manifest, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMani,
		"func":    "ManifestGetByIDWithProfile",
	})
	ctx, span := tracer.Start(ctx, "ManifestGetByIDWithProfile")
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[]-(m:Manifest)-[]-(u:Upstream)`
	q += fmt.Sprintf(` WHERE id(m) = %d`, maniID)
	q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(m) AS manifestID,`
	q += ` ` + UpstreamReturn
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	for result.Next() {
		mani, err = n.ManiFromRec_WithProfile(ctx, result, prfID)
		return mani, true, err
	}
	return mani, false, fmt.Errorf("no manifest found")
}

func (n Neo) ManifestGetByLabel(ctx context.Context, maniLabel string) (mani types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestGetByLabel",
	})
	defer span.End()
	// Manifest MUST be connected to upstream, optional to platform (later)
	q := `MATCH (m:Manifest)-[]-(u:Upstream)`
	q += fmt.Sprintf(` WHERE m.label = "%s"`, maniLabel)
	q += ` RETURN id(m) AS manifestID, m.label AS manifestLabel,`
	q += ` ` + UpstreamReturn
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		return mani, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)

	}
	if !found {
		utils.Debug(l, span, types.FailManifestNotFound)
		return mani, false, nil
	}
	mani, e = n.ManiFromResult(ctx, res)
	if e != nil {
		return mani, false, utils.ErrorHandler(l, span, e, types.FailManiFromResult)
	}
	return
}

func (n Neo) Manifest_GetAll(ctx context.Context) (manis []types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMani,
		"func":    "Manifest_GetAll",
	})
	ctx, span := tracer.Start(ctx, "Manifest_GetAll")
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[]-(m:Manifest)-[]-(u:Upstream)`
	q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(m) AS manifestID,`
	q += ` ` + UpstreamReturn
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	manis = make([]types.Manifest, 0)
	for result.Next() {
		if m, err := n.ManiFromRec(ctx, result); err != nil {
			return manis, utils.ErrorHandler(l, span, err, types.FailManiFromRecord)
		} else {
			manis = append(manis, m)
		}
	}
	return
}

func (n Neo) ManiFromResult(ctx context.Context, res types.Result) (m types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManiFromResult",
	})
	defer span.End()
	err = res.CheckKeys("manifestID", "manifestLabel")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("manifestID")
	label, _ := res.GetStr("manifestLabel")
	up, err := UpstreamFromResult(res)
	if err != nil {
		return
	}
	plat, found, e := n.PlatformGetByManifestID(ctx, id)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, types.FailPlatformGetByManifestID)
		return
	}
	if !found {
		plat = types.Platform{}
	}
	m = types.Manifest{
		ID:          id,
		Label:       label,
		Upstream:    up,
		Platform:    plat,
		Annotations: types.NewAnnotations(),
		Manifests:   []types.ManifestData{},
	}
	anno, e := n.GetAnnotationsByManifestID(ctx, m.ID)
	if e != nil {
		err = e
		return
	}
	m.Annotations = anno
	mds, e := n.ManifestData_GetAllByManifestID(ctx, m.ID)
	if e != nil {
		err = e
		return
	}
	m.Manifests = mds
	return
}

func (n Neo) ManiFromRec(ctx context.Context, res neo4j.Result) (m types.Manifest, err error) {
	ctx, span := tracer.Start(ctx, "ManiFromRec")
	defer span.End()
	p, e := PlatformFromRecord(res)
	if e != nil {
		err = e
		return
	}
	u, e := UpstreamFromRecord(res)
	if e != nil {
		err = e
		return
	}
	m, e = ManifestFromRecord(res)
	if e != nil {
		err = e
		return
	}
	anno, e := n.GetAnnotationsByManifestID(ctx, m.ID)
	if e != nil {
		err = e
		return
	}
	m.Annotations = anno
	m.Platform = p
	m.Upstream = u
	mds, e := n.ManifestData_GetAllByManifestID(ctx, m.ID)
	if e != nil {
		err = e
		return
	}
	m.Manifests = mds
	return
}

func (n Neo) ManiFromRec_WithProfile(ctx context.Context, res neo4j.Result, prfID int64) (m types.Manifest, err error) {
	ctx, span := tracer.Start(ctx, "ManiFromRec_WithProfile")
	defer span.End()
	p, e := PlatformFromRecord(res)
	if e != nil {
		err = e
		return
	}
	u, e := UpstreamFromRecord(res)
	if e != nil {
		err = e
		return
	}
	m, e = ManifestFromRecord(res)
	if e != nil {
		err = e
		return
	}
	anno, e := n.GetAnnotationsByManifestID(ctx, m.ID)
	if e != nil {
		err = e
		return
	}
	m.Annotations = anno
	m.Platform = p
	m.Upstream = u
	//mds, e := n.ManifestData_GetAllByManifestID_WithProfile(ctx, m.ID, prfID)

	mds, e := n.ManifestData_GetAllByManifestID_WithProfile(ctx, m.ID, prfID)
	if e != nil {
		err = e
		return
	}
	m.Manifests = mds
	return
}

func (n Neo) ManifestLinkToUpstream(ctx context.Context, maniID, upID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestLinkToUpstream",
		"maniID":  maniID,
		"upID":    upID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (m:Manifest),(u:Upstream) WHERE id(m) = %d AND id(u) = %d`, maniID, upID)
	q += ` MERGE (m)-[:UPSTREAM]->(u)`
	utils.Debug(l, span, q)
	_, e := n.Execute(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, types.FailNeoExecute)
		return
	}
	return
}

func ManifestFromRecord(res neo4j.Result) (m types.Manifest, err error) {
	r := res.Record()
	v, ok := r.Get("manifestID")
	if !ok {
		err = fmt.Errorf("no manifestID found in record: %v", r)
		return
	}
	m.ID = v.(int64)
	return
}

func (n Neo) ManifestExistsForTagProfile(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestExistsForTagProfile",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"profile": up.Profile,
	})
	defer span.End()
	mediaType, found, e := n.ManifestListExistsForTagProfile(ctx, up)
	if e != nil {
		utils.ErrorHandler(l, span, e, "failed to execute ManifestListExists")
		return "", false, err
	}
	if found {
		utils.Debug(l, span, "found manifest list for tag profile '%s' w/ media type: %s", up.Profile, mediaType)
		return mediaType, found, nil
	}
	utils.Debug(l, span, "could not find ManifestList for tag profile '%s' w/ media type: %s", up.Profile, mediaType)
	/* TODO: we do not need to check for a ManifestData object as it would have been addressed with digest
	mediaType, found, e = n.ManifestDataExists_ForTagProfile(ctx, up)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to execute ManifestDataExists")
		return "", false, err
	}
	if found {
		utils.Debug(l, span, "found ManifestData for tag profile '%s' w/ media type: %s", up.Profile, mediaType)
		return mediaType, found, nil
	}
	utils.Debug(l, span, "did not find manifestData w/ media type: %s", mediaType)
	*/
	return mediaType, false, nil
}

func (n Neo) ManifestExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
		"profile": up.Profile,
	})
	defer span.End()
	mediaType, found, e := n.ManifestListExists(ctx, up)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to execute ManifestListExists")
		return "", false, err
	}
	if found {
		utils.DebugOK(l, span, "found manifestList w/ media type: %s", mediaType)
		return mediaType, found, nil
	}
	mediaType, found, e = n.ManifestDataExists(ctx, up)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to execute ManifestDataExists")
		return "", false, err
	}
	if found {
		utils.DebugOK(l, span, "found manifestData w/ media type: %s", mediaType)
		return mediaType, found, nil
	}
	utils.Debug(l, span, "did not find manifestData w/ media type: %s", mediaType)
	return
}

func (n Neo) ManifestUpsertOSVersion(ctx context.Context, maniID int64, ver string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "neo.ManifestUpsertOSVersion",
		"mani.ID": maniID,
	})
	defer span.End()
	a := types.NewAnnotation(AnnoOSVersion, ver)
	aID, e := n.Annotation_GetInsert(ctx, a)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailAnnoCreate)
		return err
	}
	e = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailManiLinkAnno)
		return err
	}
	return
}

func (n Neo) ManifestUpsertOSID(ctx context.Context, maniID int64, id string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "neo.ManifestUpsertOSID",
		"mani.ID": maniID,
	})
	defer span.End()
	a := types.NewAnnotation(AnnoOSId, id)
	aID, e := n.Annotation_GetInsert(ctx, a)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailAnnoCreate)
		return err
	}
	e = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailManiLinkAnno)
		return err
	}
	return
}

func (n Neo) ManifestUpsertOS(ctx context.Context, maniID int64, os types.OS) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "neo.ManifestUpsertOS",
		"mani.ID": maniID,
	})
	defer span.End()
	as, e := n.ManifestGetAllSystemAnnos(ctx, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "ManifestGetAllSystemAnnos failed")
		return
	}

	toDo := set.CreateStringSet(AnnoOSId, AnnoOSVersion)
	for _, a := range as.ToSlice() {
		switch a.Key {
		case AnnoOSId:
			if string(os.ID) != a.Value {
				utils.Debug(l, span, "os.ID mismatch: '%s' != '%s'", os.ID, a.Value)
				e = n.ManifestUnlinkAnnotation(ctx, maniID, a.ID)
				if e != nil {
					err = utils.ErrorHandler(l, span, e, types.FailManifestUnlinkAnnotation)
					return
				}
				e := n.ManifestUpsertOSVersion(ctx, maniID, os.GetVersion())
				if e != nil {
					err = utils.ErrorHandler(l, span, e, types.FailManifestUpsertOSVersion)
					return
				}
				toDo.Remove(a.Key)
			}
		case AnnoOSVersion:
			if os.GetVersion() != a.Value {
				utils.Debug(l, span, "os.Version mismatch: '%s' != '%s'", os.GetVersion(), a.Value)
				e = n.ManifestUnlinkAnnotation(ctx, maniID, a.ID)
				if e != nil {
					err = utils.ErrorHandler(l, span, e, types.FailManifestUnlinkAnnotation)
					return
				}
				e := n.ManifestUpsertOSID(ctx, maniID, os.ID)
				if e != nil {
					err = utils.ErrorHandler(l, span, e, types.FailManifestUpsertOSID)
					return
				}
				toDo.Remove(a.Key)
			}
		}
	}
	for _, t := range toDo.ToSlice() {
		switch t {
		case AnnoOSId:
			a := types.NewAnnotation(AnnoOSId, string(os.ID))
			aID, e := n.Annotation_GetInsert(ctx, a)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, FailAnnoCreate)
				return err
			}
			e = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, FailManiLinkAnno)
				return err
			}
		case AnnoOSVersion:
			a := types.NewAnnotation(AnnoOSVersion, os.GetVersion())
			aID, e := n.Annotation_GetInsert(ctx, a)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, FailAnnoCreate)
				return err
			}
			e = n.Manifest_LinkToAnnotation(ctx, maniID, aID)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, FailManiLinkAnno)
				return err
			}
		}
	}
	return
}

func (n Neo) ManifestGetOS(ctx context.Context, maniID int64) (os types.OS, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMani,
		"func":       "neo.ManifestGetOS",
		"maniDataID": maniID,
	})
	defer span.End()
	as, e := n.ManifestGetAllSystemAnnos(ctx, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "ManifestGetAllSystemAnnos failed")
		return
	}
	if len(as.ToSlice()) != 2 {
		utils.NewError(l, span, "expected 2 annotations, got %d", len(as.ToSlice()))
		return
	}
	found = true
	os = types.OS{}
	for _, a := range as.ToSlice() {
		switch a.Key {
		case AnnoOSId:
			os.ID = a.Value
		case AnnoOSVersion:
			os.Major, os.Minor, os.Patch, e = types.ParseVersion(a.Value)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "ParseVersion failed")
				return
			}
		}
	}
	return
}

func (n Neo) ManifestGetAllSystemAnnos(ctx context.Context, maniID int64) (as types.Annotations, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "neo.ManifestGetAllSystemAnnos",
		"mani.ID": maniID,
	})
	defer span.End()
	tempAs, e := n.GetAnnotationsByManifestID(ctx, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to GetAnnotationsByManifestID")
		return
	}
	as = types.NewAnnotations()
	for _, a := range tempAs.ToSlice() {
		if a.Key == AnnoOSId || a.Key == AnnoOSVersion {
			as.AddAnnotation(a)
		}
	}
	return
}
