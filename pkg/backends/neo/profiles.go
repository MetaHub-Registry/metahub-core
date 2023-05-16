package neo

import (
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameProfile = "neo/profiles.go"
)

var (
	returnStatementProfile = `id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule, prf.label AS profileLabel`
)

func (n Neo) Profile_GetInsert(ctx context.Context, p *types.Profile) (err error) {
	prfID, success, err := n.GetProfile(*p)
	if err != nil {
		return
	}
	if !success {
		prfID, err = n.CreateProfile(*p)
		if err != nil {
			return
		}
	}
	p.ID = prfID

	if p.Rule == "" {
		return
	}
	// Connect Profile to Rules
	// FIXME: Do we need to do this each time, or only when a new rule is added?
	lbls, err := utils.ExprGetLabels(p.Rule)
	if err != nil {
		return
	}
	for _, lbl := range lbls.ToSlice() {
		r, found, err := n.RuleGetByLabel(ctx, lbl)
		if err != nil {
			return err
		}
		if !found {
			logrus.Warnf("rule with label '%s' not found", lbl)
			continue
		}
		e := n.Profile_LinkRule(ctx, prfID, r.ID)
		if e != nil {
			err = errors.Wrap(e, "failed to link profile to rule")
			return err
		}
	}
	return
}

func (n Neo) Profile_Get_ByLabel(ctx context.Context, label string) (p types.Profile, found bool, err error) {
	_, span := tracer.Start(ctx, "Profile_Get_ByLabel")
	defer span.End()
	q := fmt.Sprintf(`MATCH (prf:Profile) WHERE prf.label = "%s"`, label)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	span.AddEvent("query", trace.WithAttributes(
		attribute.String("query", q),
	))
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	p, err = n.ProfileFromResult(res)
	return
}

func (n Neo) Profile_Get_ByNameTag(ctx context.Context, name, tag string) (p types.Profile, found bool, err error) {
	q := fmt.Sprintf(`MATCH (prf:Profile) WHERE prf.name = "%s" AND prf.tag = "%s"`, name, tag)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	p, err = n.ProfileFromResult(res)
	return
}

func (n Neo) Profile_Get_ByID(ctx context.Context, prfID int64) (p types.Profile, found bool, err error) {
	q := fmt.Sprintf(`MATCH (prf:Profile) WHERE id(prf) = %d`, prfID)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	p, err = n.ProfileFromResult(res)
	return
}

func (n Neo) Profiles_Get_ByRegexLabel(ctx context.Context, labelRegex string) (ps []types.Profile, err error) {
	_, span := tracer.Start(ctx, "Profiles_Get_ByRegexLabel")
	defer span.End()
	q := fmt.Sprintf(`MATCH (prf:Profile) WHERE prf.label =~ "%s"`, labelRegex)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	logrus.Fatalf("q: %s", q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Query(q)
	ps = make([]types.Profile, 0)
	for _, r := range res {
		p, err := n.ProfileFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		ps = append(ps, p)
	}
	return
}

func (n Neo) GetProfiles(ctx context.Context) (ps []types.Profile, err error) {
	_, span := tracer.Start(ctx, "GetProfiles")
	defer span.End()
	q := `MATCH (prf:Profile)`
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	res, err := n.Query(q)
	ps = make([]types.Profile, 0)
	for _, r := range res {
		p, err := n.ProfileFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		ps = append(ps, p)
	}
	return
}

func (n Neo) GetProfile(prf types.Profile) (prfID int64, success bool, err error) {
	q := `MATCH (n:Profile) WHERE n.name = "%s" AND n.tag = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, prf.Name, prf.Tag)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("GetProfile: failed to get ID")
			return
		}
		prfID = v.(int64)
		success = true
	}
	return
}

func (n Neo) CreateProfile(prf types.Profile) (prfID int64, err error) {
	q := `CREATE (n:Profile {name: "%s", tag: "%s", label: "%s", rule: "%s"}) RETURN id(n) AS ID`
	label := fmt.Sprintf("%s#%s", prf.Name, prf.Tag)
	if prf.Name == "" {
		label = prf.Tag
	}
	query := fmt.Sprintf(q, prf.Name, prf.Tag, label, prf.Rule)
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
			err = fmt.Errorf("CreateProfile: failed to get ID")
			return
		}
		prfID = v.(int64)

	} else {
		err = fmt.Errorf("CreateProfile: no result")
		return
	}
	return
}

func (n Neo) Profile_LinkRule(ctx context.Context, profileID, ruleID int64) (err error) {
	q := `MATCH (p:Profile),(r:Rule) WHERE id(p) = %d AND id(r) = %d`
	q += ` MERGE (p)-[:USES]->(r)`
	query := fmt.Sprintf(q, profileID, ruleID)
	_, err = n.Execute(query)
	return
}

func ProfileFromRecord(res neo4j.Result) (p types.Profile, err error) {
	r := res.Record()
	v, ok := r.Get("profileID")
	if !ok {
		err = fmt.Errorf("no profileID found in record: %v", r)
		return
	}
	p.ID = v.(int64)
	v, ok = r.Get("profileName")
	if !ok {
		err = fmt.Errorf("no profileName found in record: %v", r)
		return
	}
	p.Name = v.(string)
	return
}

func (n Neo) ProfileFromResult(res types.Result) (p types.Profile, err error) {
	//ctx := context.Background()
	err = res.CheckKeys("profileID", "profileName", "profileTag", "profileRule")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("profileID")
	name, _ := res.GetStr("profileName")
	tag, _ := res.GetStr("profileTag")
	profileRule, _ := res.GetStr("profileRule")
	profileLabel, _ := res.GetStr("profileLabel")
	p = types.Profile{
		ID:            id,
		Name:          name,
		Tag:           tag,
		Rule:          profileRule,
		Label:         profileLabel,
		Manipulations: []types.Manipulation{},
	}
	// TODO: Evaluate the rule string and get the rules attached?
	/*
		f.Rules, err = n.Rules_GetByFilterID(ctx, id)
		if err != nil {
			logrus.Error(err)
			return
		}
	*/
	return
}

func (n Neo) Profile_FromManifestID(ctx context.Context, maniID int64) (prfs []types.Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "Profile_FromManifestID",
		"maniID":  maniID,
	})
	prfLabels := set.NewStringSet()
	_ = l
	ctx, span := tracer.Start(ctx, "Profile_FromManifestID")
	defer span.End()
	prfs, e := n.Profile_FromManifestID_ViaAnnotations(ctx, maniID)
	if e != nil {
		err = errors.Wrap(e, "failed to get profiles via annotations for Manifest")
		return
	}
	for _, prf := range prfs {
		prfLabels.Add(prf.Label)
	}
	prfs2, e := n.Profile_FromManifestID_ViaPlatform(ctx, maniID)
	if e != nil {
		err = errors.Wrap(e, "failed to get profiles via platform for Manifest")
		return
	}
	for _, prf := range prfs2 {
		if !prfLabels.Contains(prf.Label) {
			prfLabels.Add(prf.Label)
			prfs = append(prfs, prf)
		}
	}

	prfs3, e := n.ProfileFromManifestIDViaOriginalManifestData(ctx, maniID)
	if e != nil {
		err = errors.Wrap(e, "failed to get profiles via platform for Manifest")
		return
	}
	for _, prf := range prfs3 {
		if !prfLabels.Contains(prf.Label) {
			prfLabels.Add(prf.Label)
			prfs = append(prfs, prf)
		}
	}

	return
}

func (n Neo) ProfileFromManifestIDViaOriginalManifestData(ctx context.Context, maniID int64) (prfs []types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileFromManifestIDViaOriginalManifestData",
		"maniID":  maniID,
	})
	defer span.End()
	_ = ctx
	// TODO: Get Original ManifestData (parsed, uploaded or pulled)
	mds, e := n.ManifestData_GetAllByManifestID(ctx, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get ManifestData for Manifest")
		return
	}
	selectMds := []types.ManifestData{}
	for _, md := range mds {
		if md.Type == "CREATED" {
			utils.Debug(l, span, "ManifestData is of type '%s' - skipping", md.Type)
		} else {
			utils.Debug(l, span, "ManifestData is of type '%s' - good for us", md.Type)
			selectMds = append(selectMds, md)
		}
	}
	prfs = []types.Profile{}
	// TODO: Get Profiles from Original ManifestData
	for _, md := range selectMds {
		if ps, err := n.ProfileFromManifestDataIDViaOrig(ctx, md.ID); err != nil {
			return prfs, err
		} else {
			utils.Debug(l, span, "Found %d profiles via ManifestData %d", len(ps), md.ID)
			prfs = append(prfs, ps...)
		}
	}
	return
}

func (n Neo) ProfileFromManifestDataIDViaOrig(ctx context.Context, maniDataID int64) (prfs []types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameProfile,
		"func":       "ProfileFromManifestDataIDViaOrig",
		"maniDataID": maniDataID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (prf:Profile)<-[r]-(md:ManifestData) WHERE id(md) = %d`, maniDataID)
	q += fmt.Sprintf(` RETURN DISTINCT %s`, returnStatementProfile)
	utils.Debug(l, span, q)
	res, e := n.Query(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailNeoExecute)
		return
	}
	prfs = make([]types.Profile, 0)
	for _, r := range res {
		p, e := n.ProfileFromResult(r)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get profile from result")
			continue
		}
		prfs = append(prfs, p)
	}
	return prfs, nil

}

func (n Neo) Profile_FromManifestID_ViaAnnotations(ctx context.Context, mID int64) (prfs []types.Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "Profile_FromManifestID_ViaAnnotations",
		"maniID":  mID,
	})
	_, span := tracer.Start(ctx, "Profile_FromManifestID_ViaAnnotations")
	defer span.End()
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule)-[:%s]-(a:Annotation)-[]-(m:Manifest) WHERE id(m) = %d`, types.RelationSuccess, mID)
	q += fmt.Sprintf(` RETURN DISTINCT %s`, returnStatementProfile)
	res, err := n.Query(q)
	l.Debug(q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	if err != nil {
		return
	}
	prfs = make([]types.Profile, 0)
	for _, r := range res {
		p, e := n.ProfileFromResult(r)
		if e != nil {
			err = errors.Wrap(e, "failed to get profile from result")
			l.Error(err)
			continue
		}
		q := fmt.Sprintf(`MATCH (m:Manifest)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
		q += fmt.Sprintf(` WHERE id(m) = %d AND id(p) = %d RETURN count(a) AS countAnnotations`, mID, p.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.query", q),
		))
		res, _, e := n.QueryOne(q)
		if e != nil {
			err = errors.Wrap(e, "failed to get QueryOne")
			l.Error(err)
			continue
		}
		cnt, e := res.GetInt("countAnnotations")
		if e != nil {
			err = errors.Wrap(e, "failed to get countAnnotations")
			l.Error(err)

			return
		}
		if cnt == 0 {
			prfs = append(prfs, p)
		}
	}
	return
}

func (n Neo) Profile_FromManifestID_ViaPlatform(ctx context.Context, maniID int64) (prfs []types.Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "Profile_FromManifestID_ViaPlatform",
		"maniID":  maniID,
	})
	_, span := tracer.Start(ctx, "Profile_FromManifestID_ViaPlatform")
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule)-[:%s]->(p:Platform)<-[:IS]-(m:Manifest) WHERE id(m) = %d`, rel, maniID)
	q += fmt.Sprintf(` RETURN DISTINCT %s`, returnStatementProfile)
	res, err := n.Query(q)
	l.Debug(q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	if err != nil {
		return
	}
	prfs = make([]types.Profile, 0)
	for _, r := range res {
		p, err := n.ProfileFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		q := fmt.Sprintf(`MATCH (m:Manifest)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
		q += fmt.Sprintf(` WHERE id(m) = %d AND id(p) = %d RETURN count(a) AS countAnnotations`, maniID, p.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.query", q),
		))
		res, _, e := n.QueryOne(q)
		if e != nil {
			err = errors.Wrap(e, "failed to get QueryOne")
			l.Error(err)
			continue
		}
		cnt, e := res.GetInt("countAnnotations")
		if e != nil {
			err = errors.Wrap(e, "failed to get countAnnotations")
			l.Error(err)
			continue
		}
		if cnt == 0 {
			prfs = append(prfs, p)
		}
	}
	return
}

func (n Neo) Profile_FromManifestDataID(ctx context.Context, mdID int64) (prfs []types.Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "Profile_FromManifestDataID",
		"maniID":  mdID,
	})
	_ = l
	ctx, span := tracer.Start(ctx, "Profile_FromManifestDataID")
	defer span.End()
	prfs, e := n.Profile_FromManifestDataID_ViaAnnotations(ctx, mdID)
	if e != nil {
		utils.ErrorHandler(l, span, e, "failed to get profiles via annotations for ManifestData")
		return
	}
	if len(prfs) == 0 {
		prfs2, e := n.Profile_FromManifestDataID_ViaPlatform(ctx, mdID)
		if e != nil {
			utils.ErrorHandler(l, span, e, "failed to get profiles via platform for ManifestData")
			return
		}
		utils.Debug(l, span, "found %d profiles via platform for ManifestData", len(prfs2))
		prfs = append(prfs, prfs2...)
	} else {
		utils.Debug(l, span, "found %d profiles via annotations for ManifestData", len(prfs))
	}
	return
}

func (n Neo) Profile_FromManifestDataID_ViaAnnotations(ctx context.Context, mdID int64) (prfs []types.Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "Profile_FromManifestDataID_ViaAnnotations",
		"maniID":  mdID,
	})
	_, span := tracer.Start(ctx, "Profile_FromManifestDataID_ViaAnnotations")
	defer span.End()
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule)-[:%s]-(a:Annotation)-[]-(md:ManifestData) WHERE id(md) = %d`, types.RelationSuccess, mdID)
	q += fmt.Sprintf(` RETURN DISTINCT %s`, returnStatementProfile)
	res, err := n.Query(q)
	l.Debug(q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	if err != nil {
		return
	}
	prfs = make([]types.Profile, 0)
	for _, r := range res {
		p, e := n.ProfileFromResult(r)
		if e != nil {
			err = errors.Wrap(e, "failed to get profile from result")
			l.Error(err)
			continue
		}
		q := fmt.Sprintf(`MATCH (md:ManifestData)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
		q += fmt.Sprintf(` WHERE id(md) = %d AND id(p) = %d RETURN count(a) AS countAnnotations`, mdID, p.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.query", q),
		))
		res, _, e := n.QueryOne(q)
		if e != nil {
			err = errors.Wrap(e, "failed to get QueryOne")
			l.Error(err)
			continue
		}
		cnt, e := res.GetInt("countAnnotations")
		if e != nil {
			err = errors.Wrap(e, "failed to get countAnnotations")
			l.Error(err)

			return
		}
		if cnt == 0 {
			prfs = append(prfs, p)
		}
	}
	return
}

func (n Neo) Profile_FromManifestDataID_ViaPlatform(ctx context.Context, mdID int64) (prfs []types.Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "Profile_FromManifestDataID_ViaPlatform",
		"maniID":  mdID,
	})
	_, span := tracer.Start(ctx, "Profile_FromManifestDataID_ViaPlatform")
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule)-[:%s]->(p:Platform)<-[:IS]-(m:Manifest)-[]-(md:ManifestData) WHERE id(md) = %d`, rel, mdID)
	q += fmt.Sprintf(` RETURN DISTINCT %s`, returnStatementProfile)
	res, err := n.Query(q)
	l.Debug(q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	if err != nil {
		return
	}
	prfs = make([]types.Profile, 0)
	for _, r := range res {
		p, err := n.ProfileFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		q := fmt.Sprintf(`MATCH (md:ManifestData)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
		q += fmt.Sprintf(` WHERE id(md) = %d AND id(p) = %d RETURN count(a) AS countAnnotations`, mdID, p.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.query", q),
		))
		res, _, e := n.QueryOne(q)
		if e != nil {
			err = errors.Wrap(e, "failed to get QueryOne")
			l.Error(err)
			continue
		}
		cnt, e := res.GetInt("countAnnotations")
		if e != nil {
			err = errors.Wrap(e, "failed to get countAnnotations")
			l.Error(err)
			continue
		}
		if cnt == 0 {
			prfs = append(prfs, p)
		}
	}
	return
}

func (n Neo) EvalUploadProfile(ctx context.Context, mani types.Manifest, origMani types.ManifestData) (err error) {
	ctx, span := tracer.Start(ctx, "EvalUploadProfile")
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (bu:BlobUpload)-[:IS]->(cfg:Config) WHERE id(cfg) = %d`, origMani.Config.ID)
	q += ` RETURN id(bu) AS blobUploadID, bu.upload_profile AS blobUploadProfile`
	res, found, err := n.QueryOne(q)
	if err != nil {
		return
	}
	if !found {
		err = fmt.Errorf("no blob upload found for config %d", origMani.Config.ID)
		return
	}
	blobUploadID, _ := res.GetInt64("blobUploadID")
	blobUploadProfile, _ := res.GetStr("blobUploadProfile")
	logrus.Fatalf("blobUploadProfile[%-4d]: %s", blobUploadID, blobUploadProfile)

	return
}

func (n Neo) ProfileAuthorizedPushHandler(ctx context.Context, req types.Request) (prf types.Profile, err error) {
	ctx, span := tracer.Start(ctx, "ProfileAuthorizedPushHandler")
	defer span.End()
	// check if profile already has a manifest uploaded
	prf, found, e := n.Profile_Get_ByLabel(ctx, req.Profile)

	if e != nil {
		err = errors.Wrapf(e, "failed to get profile '%s'", req.Profile)
		span.RecordError(err)
		return

	}
	if !found {
		err = fmt.Errorf("profile '%s' not found", req.Profile)
		span.RecordError(err)
		return
	}
	// Only allow uploads to a repo prefixed with the profile name
	if !strings.HasPrefix(req.Repo+"/", req.User) && req.UserRole != "admin" {
		err = fmt.Errorf("user '%s' (role: %s) is not allowed to push to repo '%s'", req.User, req.UserRole, req.Repo)
		span.RecordError(err)
		return
	}
	return
}

func (n Neo) Profile_ManifestType(ctx context.Context, maniID int64) (relType string, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "Profile_ManifestType",
		"maniID":  maniID,
	})
	_, span := tracer.Start(ctx, "Profile_ManifestType")
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniID").Int64(maniID),
	)
	q := `MATCH (prf:Profile)<-[r]-(md:ManifestData)<-[]-(m:Manifest)`
	q += fmt.Sprintf(` WHERE id(m) = %d`, maniID)
	q += ` RETURN TYPE(r) AS relType`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.error", err.Error()),
		))
		span.RecordError(err)
		return
	}
	if !found {
		err = fmt.Errorf("no manifest found for id %d", maniID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.error", err.Error()),
		))
		return
	}
	relType, _ = res.GetStr("relType")
	return
}

func (n Neo) Profile_GetUploadedBy_ViaManifestDataID(ctx context.Context, maniID int64) (p types.Profile, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "Profile_GetUploadedBy_ViaManifestDataID",
		"maniID":  maniID,
	})
	_, span := tracer.Start(ctx, "Profile_GetUploadedBy_BiaManifestDataID")
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniID").Int64(maniID),
	)
	q := `MATCH (prf:Profile)<-[:UPLOADED_BY]-(md:ManifestData)`
	q += fmt.Sprintf(` WHERE id(md) = %d`, maniID)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	p, err = n.ProfileFromResult(res)
	return
}

func (n Neo) Profile_GetPulledBy_ViaManifestDataID(ctx context.Context, maniID int64) (p types.Profile, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "Profile_GetPulledBy_ViaManifestDataID",
		"maniID":  maniID,
	})
	_, span := tracer.Start(ctx, "Profile_GetPulledBy_ViaManifestDataID")
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniID").Int64(maniID),
	)
	q := `MATCH (prf:Profile)<-[:PULLED_BY]-(md:ManifestData)`
	q += fmt.Sprintf(` WHERE id(md) = %d`, maniID)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	p, err = n.ProfileFromResult(res)
	return
}

func (n Neo) ProfileGetPlatformRules(ctx context.Context, prfID int64) (rs []types.Rule, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileGetPlatformRules",
		"prfID":   prfID,
	})
	defer span.End()
	_ = ctx
	q := `MATCH (prf:Profile)-[:USES]->(r:Rule)`
	q += fmt.Sprintf(` WHERE id(prf) = %d AND r.type = "%s" AND r.op = "%s"`, prfID, types.RuleTypePlatform, "eq")
	q += fmt.Sprintf(` RETURN %s`, ruleReturn)
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		err = errors.Wrapf(err, "failed to unwind rules for profile %d", prfID)
		return
	}
	rs = make([]types.Rule, 0)
	for _, ele := range res {
		r, _ := RuleFromResult(ele)
		rs = append(rs, r)
	}
	return
}

func (n Neo) ProfileGetAnnoEqRules(ctx context.Context, prfID int64) (rs []types.Rule, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileGetAnnoEqRules",
		"prfID":   prfID,
	})
	defer span.End()
	_ = ctx
	q := `MATCH (prf:Profile)-[:USES]->(r:Rule)`
	q += fmt.Sprintf(` WHERE id(prf) = %d`, prfID)
	q += fmt.Sprintf(` RETURN %s`, ruleReturn)
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		err = errors.Wrapf(err, "failed to unwind rules for profile %d", prfID)
		return
	}
	rs = make([]types.Rule, 0)
	for _, ele := range res {
		r, _ := RuleFromResult(ele)
		if r.Type == types.RuleTypeAnnotation && r.Op == "eq" {
			rs = append(rs, r)
		}
	}
	return
}

func (n Neo) ProfileUnwindRules(ctx context.Context, prfID, maniID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileUnwindRules",
		"maniID":  maniID,
		"prfID":   prfID,
	})
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniID").Int64(maniID),
		attribute.Key("prfID").Int64(prfID),
	)
	rs, e := n.ProfileGetAnnoEqRules(ctx, prfID)
	if e != nil {
		err = errors.Wrapf(e, "failed to get anno rules for profile %d", prfID)
		return
	}
	prls, e := n.ProfileGetPlatformRules(ctx, prfID)
	if e != nil {
		err = errors.Wrapf(e, "failed to get platform rules for profile %d", prfID)
		return err
	}
	rs = append(rs, prls...)

	l.Debugf("found %d rules", len(rs))
	for _, r := range rs {
		l.Debugf(" >> rule: %s", r.ToString())
		switch {
		case r.Type == types.RuleTypeAnnotation && r.Op == "eq":
			a := types.NewAnnotation(r.Key, r.Value)
			aID, err := n.Annotation_GetInsert(ctx, a)
			if err != nil {
				err = errors.Wrapf(err, "failed to get annotation %s:%s", a.Key, a.Value)
				return err
			}
			e := n.ManifestData_LinkToAnnotation(ctx, maniID, aID)
			if e != nil {
				err = errors.Wrapf(e, "failed to link manifestData '%d' with  annotation %d", maniID, aID)
				return err
			}
		}
	}
	return nil
}

func (n Neo) ProfileLinkToManipulation(ctx context.Context, prfID, mpID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileLinkToManipuLayer",
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (p:Profile), (mp:Manipulation) WHERE id(p) = %d AND id(mp) = %d MERGE (p)-[:USES]->(mp)`, prfID, mpID)
	utils.Debug(l, span, q)
	if _, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	return nil
}
