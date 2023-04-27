package neo

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/distribution/manifest/manifestlist"
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

func (n Neo) Manifest_LinkToAnnotation(ctx context.Context, maniID, annotationID int64) (err error) {
	q := `MATCH (m:Manifest), (a:Annotation) WHERE id(m) = %d AND id(a) = %d MERGE (m)-[:HAS_ANNOTATION]->(a)`
	query := fmt.Sprintf(q, maniID, annotationID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) Manifest_GetInsert_WithDesc(ctx context.Context, req types.Request, up types.Upstream, maniIn manifestlist.ManifestDescriptor) (maniID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_GetInsertWithProfile",
		"profile": req.Profile,
		"repo":    req.Repo,
		"tag":     req.Tag,
	})
	ctx, span := tracer.Start(ctx, "Manifest_GetInsertWithProfile")
	defer span.End()
	// If Manifest between collection and upstream does not exists
	label := fmt.Sprintf("%s/%s:%s-%s", up.Reg, up.Repo, up.Tag, maniIn.Platform.Architecture)
	if maniIn.Platform.Variant != "" {
		label += fmt.Sprintf("-%s", maniIn.Platform.Variant)
	}
	mani, found, err := n.Manifest_GetByLabel(ctx, label)
	if err != nil {
		utils.ErrorHandler(l, span, err, "while executing Manifest_GetByLabel")
		return
	}
	if found {
		l.Debugf("Manifest found: %d", mani.ID)
	} else {
		// Create ManifestStandin
		maniID, err = n.manifest_Create_WithLabel(ctx, label, up.ID)
		if err != nil {
			utils.ErrorHandler(l, span, err, "while executing manifest_Create_WithLabel")
			return
		}
		l.Debugf("Manifest created: %d", maniID)
	}
	return
}

func (n Neo) Manifest_GetInsertWithProfile(ctx context.Context, upID int64, prfName string) (maniID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_GetInsertWithProfile",
		"profile": prfName,
		"upID":    upID,
	})
	ctx, span := tracer.Start(ctx, "Manifest_GetInsertWithProfile")
	defer span.End()
	// If Manifest between collection and upstream does not exists
	maniID, found, err := n.Manifest_Get(ctx, upID, prfName)
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		l.Debugf("Manifest found: %d", maniID)
	} else {
		// Create ManifestStandin
		maniID, err = n.Manifest_CreateWithProfile(ctx, upID, prfName)
		if err != nil {
			logrus.Error(err)
			return
		}
		l.Debugf("Manifest created: %d", maniID)
	}
	return
}

func (n Neo) Manifest_GetInsertWithRequest(ctx context.Context, m types.Manifest, req types.Request) (maniID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_GetInsertWithRequest",
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
	maniID, err = n.Manifest_GetInsertWithProfile(ctx, up.ID, req.Profile)
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

func (n Neo) Manifest_GetInsert(ctx context.Context, m types.Manifest) (maniID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_GetInsert",
		"repo":    m.Upstream.Repo,
		"tag":     m.Upstream.Tag,
	})
	ctx, span := tracer.Start(ctx, "Manifest_GetInsert")
	defer span.End()
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
	// If Manifest between collection and upstream does not exists
	maniID, found, err := n.Manifest_Get(ctx, up.ID)
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		l.Debugf("Manifest found: %d", maniID)
	} else {
		// Create ManifestStandin
		maniID, err = n.Manifest_Create(ctx, up.ID)
		if err != nil {
			logrus.Error(err)
			return
		}
		l.Debugf("Manifest created: %d", maniID)
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

func (n Neo) manifest_Create_WithLabel(ctx context.Context, label string, upID int64) (msID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "manifest_Create_WithLabel",
		"upID":    upID,
		"label":   label,
	})
	ctx, span := tracer.Start(ctx, "manifest_Create_WithLabel")
	defer span.End()
	q := fmt.Sprintf(`MATCH (u:Upstream) WHERE id(u) = %d`, upID)
	q += fmt.Sprintf(` MERGE (m:Manifest {label: "%s"})-[:UPSTREAM]->(u)`, label)
	q += ` RETURN id(m) AS ID`
	l.Debug(q)
	res, err := n.Execute(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	msID, err = res.GetInt64("ID")
	return
}
func (n Neo) Manifest_Create(ctx context.Context, upID int64) (msID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_Create",
		"upID":    upID,
	})
	ctx, span := tracer.Start(ctx, "Manifest_Create")
	defer span.End()
	up, err := n.Upstream_Get_ByID(ctx, upID)
	if err != nil {
		err = errors.Wrap(err, "failed to get upstream")
		l.Error(err)
		return
	}
	label := fmt.Sprintf("%s/%s:%s", up.Reg, up.Repo, up.Tag)
	l = logrus.WithFields(logrus.Fields{
		"package":  "neo/manifest.go",
		"func":     "Manifest_Create",
		"upID":     upID,
		"digest":   utils.ShortenDigest(up.Digest.String()),
		"upstream": label,
	})
	return n.manifest_Create_WithLabel(ctx, label, upID)
}

func (n Neo) Manifest_CreateWithProfile(ctx context.Context, upID int64, prfName string) (msID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_CreateWithProfile",
		"upID":    upID,
	})
	ctx, span := tracer.Start(ctx, "Manifest_CreateWithProfile")
	defer span.End()
	/*
		prf, prfFound, e := n.Profile_Get_ByLabel(ctx, prfName)
		if e != nil {
			err = errors.Wrap(e, "failed to get profile")
			l.Error(err)
			return
		}
		if !prfFound {
			err = errors.Wrap(e, "profile not found")
			l.Error(err)
			return
		}
	*/
	up, err := n.Upstream_Get_ByID(ctx, upID)
	if err != nil {
		err = errors.Wrap(err, "failed to get upstream")
		l.Error(err)
		return
	}
	label := fmt.Sprintf("%s/%s:%s@%s", up.Reg, up.Repo, up.Tag, prfName)
	l = logrus.WithFields(logrus.Fields{
		"package":  "neo/manifest.go",
		"func":     "Manifest_Create",
		"upID":     upID,
		"upstream": label,
	})

	q := fmt.Sprintf(`MATCH (u:Upstream) WHERE id(u) = %d`, upID)
	q += fmt.Sprintf(` MERGE (m:Manifest {label: "%s"})-[:UPSTREAM]->(u)`, label)
	q += ` RETURN id(m) AS ID`
	l.Debug(q)
	res, err := n.Execute(q)
	if err != nil {
		err = errors.Wrapf(err, "failed to execute query:\n%s", q)
		l.Error(err)
		return
	}
	msID, err = res.GetInt64("ID")
	return
}

func (n Neo) Manifest_Get_CreateWithProfile(ctx context.Context, upID int64, prfName string) (mani types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_CreateWithProfile",
		"upID":    upID,
	})
	ctx, span := tracer.Start(ctx, "Manifest_CreateWithProfile")
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	up, err := n.Upstream_Get_ByID(ctx, upID)
	if err != nil {
		err = errors.Wrap(err, "failed to get upstream")
		l.Error(err)
		return
	}
	label := fmt.Sprintf("%s/%s:%s@%s", up.Reg, up.Repo, up.Tag, prfName)
	l = logrus.WithFields(logrus.Fields{
		"package":  "neo/manifest.go",
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
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_Get",
		"suffix":  suffix,
	})
	ctx, span := tracer.Start(ctx, "Manifest_Get")
	defer span.End()
	q := `MATCH (m:Manifest)-[]->(u:Upstream)`
	q += fmt.Sprintf(` WHERE id(u) = %d`, upID)
	if len(suffix) == 1 {
		q += fmt.Sprintf(` AND m.label ENDS WITH "%s"`, suffix[0])
	}
	q += ` RETURN id(m) AS ID`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
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
	q := `MATCH (m:Manifest), (md:ManifestData) WHERE id(m) = %d AND id(md) = %d`
	switch {
	case len(props) > 0:
		q += fmt.Sprintf(` MERGE (m)-[:%s]->(md)`, strings.ToUpper(props[0]))
	case len(props) == 0:
		q += ` MERGE (m)-[:PARSED]->(md)`
	default:
		return fmt.Errorf("expected one property for LinkManifestToManifestData, got %d", len(props))
	}
	query := fmt.Sprintf(q, maniID, maniDataID)
	_, err = n.Execute(query)
	return
}

func (n Neo) Manifest_GetByUpstreamDigest(ctx context.Context, upDigest string) (mani types.Manifest, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
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

func (n Neo) Manifest_GetByID(ctx context.Context, maniID int64) (mani types.Manifest, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_GetByID",
	})
	ctx, span := tracer.Start(ctx, "Manifest_GetByID")
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
	return mani, false, fmt.Errorf("no manifest found")
}

func (n Neo) Manifest_GetByID_WithProfile(ctx context.Context, maniID, prfID int64) (mani types.Manifest, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_GetByID_WithProfile",
	})
	ctx, span := tracer.Start(ctx, "Manifest_GetByID_WithProfile")
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

func (n Neo) Manifest_GetByLabel(ctx context.Context, maniName string) (mani types.Manifest, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "Manifest_GetByLabel",
	})
	_, span := tracer.Start(ctx, "Manifest_GetByLabel")
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[]-(m:Manifest)-[]-(u:Upstream)`
	q += fmt.Sprintf(` WHERE m.label = "%s"`, maniName)
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

func (n Neo) Manifest_GetAll(ctx context.Context) (manis []types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
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
		m, err := n.ManiFromRec(ctx, result)
		if err != nil {
			l.Error(err)
			continue
		}
		// Add manifest to list
		manis = append(manis, m)
	}
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

func (n Neo) Manifest_LinkToUpstream(ctx context.Context, maniID, upID int64) (err error) {
	_, span := tracer.Start(ctx, "Manifest_LinkToUpstream")
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniID").Int64(maniID),
		attribute.Key("upID").Int64(upID),
	)
	q := `MATCH (m:Manifest),(u:Upstream) WHERE id(m) = %d AND id(u) = %d`
	q += ` MERGE (m)-[:UPSTREAM]->(u)`
	query := fmt.Sprintf(q, maniID, upID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", query),
	))
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:Manifest)-[:UPSTREAM]->(u:Upstream): %v", err)
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

func (n Neo) ManifestExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest.go",
		"func":    "ManifestExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
		"profile": up.Profile,
	})
	ctx, span := tracer.Start(ctx, "ManifestExists")
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("digest").String(utils.ShortenDigest(up.Digest.String())),
	)
	mediaType, found, err = n.ManifestListExists(ctx, up)
	if err != nil {
		msg := "failed to execute ManifestListExists"
		err = errors.Wrap(err, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return "", false, err
	}
	if found {
		return mediaType, found, nil
	}
	mediaType, found, err = n.ManifestDataExists(ctx, up)
	if err != nil {
		msg := "failed to execute ManifestDataExists"
		err = errors.Wrap(err, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return "", false, err
	}
	if found {
		return mediaType, found, nil
	}
	return
}
