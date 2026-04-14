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
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameProfile         = "neo/profiles.go"
	errQueryOne            = "failed to get QueryOne"
	errProfileFromResult   = "failed to get profile from result"
	errGetCountAnnotations = "failed to get countAnnotations"
)

var (
	returnStatementProfile = `id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule, prf.label AS profileLabel`
)

func (n Neo) ProfileGetInsert(ctx context.Context, p *types.Profile) (err error) {
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
		e := n.ProfileLinkRule(ctx, prfID, r.ID)
		if e != nil {
			err = errors.Wrap(e, "failed to link profile to rule")
			return err
		}
	}
	// Connect to ManipuLayer
	for _, mp := range p.Manipulations {
		if mpID, found, err := n.ManipulationGetIDByLabel(ctx, mp.Label); err != nil || !found {
			return fmt.Errorf("failed to get Manipulation for label '%s': %v", mp.Label, err)
		} else {
			err = n.ProfileLinkToManipulation(ctx, p.ID, mpID)
			if err != nil {
				return err
			}
		}
	}
	return
}

func (n Neo) ProfileGetByLabel(ctx context.Context, label string) (p types.Profile, found bool, err error) {
	_, span := tracer.Start(ctx, "ProfileGetByLabel")
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

// ProfileGetByLabelWithStats returns a single profile by label together with
// the count of distinct Slots (≈ collections) and distinct Manifests served
// under it, computed from the Slot→Index→Manifest graph path.
func (n Neo) ProfileGetByLabelWithStats(ctx context.Context, label string) (ps types.ProfileStats, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileGetByLabelWithStats",
		"label":   label,
	})
	defer span.End()

	// First get the profile itself.
	prf, prfFound, e := n.ProfileGetByLabel(ctx, label)
	if e != nil {
		err = e
		utils.ErrorHandler(l, span, err, "ProfileGetByLabel failed")
		return
	}
	if !prfFound {
		return
	}
	found = true
	ps.Profile = prf

	// Fetch linked rules for this profile.
	rq := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule) WHERE id(prf) = %d RETURN %s`, prf.ID, ruleReturn)
	rres, e := n.Query(rq)
	if e != nil {
		utils.ErrorHandler(l, span, e, "while fetching rules for profile")
		// Non-fatal: leave Rules nil.
	} else {
		ps.Rules = make([]types.ProfileRule, 0, len(rres))
		for _, rr := range rres {
			rule, re := RuleFromResult(rr)
			if re != nil {
				logrus.Warnf("ProfileGetByLabelWithStats: skipping malformed rule: %v", re)
				continue
			}
			ps.Rules = append(ps.Rules, types.ProfileRule{
				ID:    rule.ID,
				Type:  rule.Type,
				Op:    rule.Op,
				Key:   rule.Key,
				Value: rule.Value,
			})
		}
	}

	// Count distinct Slots and Manifests via the Slot→Index→Manifest path.
	statsQ := fmt.Sprintf(
		`MATCH (p:Profile) WHERE id(p) = %d `+
			`OPTIONAL MATCH (p)<-[:OF_PROFILE]-(s:Slot)-[:POINTS_TO]->(i:Index)-[:REFERENCES]->(m:Manifest) `+
			`RETURN count(DISTINCT s) AS slotCount, count(DISTINCT m) AS manifestCount`,
		prf.ID,
	)
	utils.Debug(l, span, statsQ)
	statsRes, statsFound, qErr := n.QueryOne(statsQ)
	if qErr != nil {
		utils.ErrorHandler(l, span, qErr, "while fetching profile stats")
		// Non-fatal: leave counts at zero.
		return
	}
	if statsFound {
		if sc, ok := statsRes.Int64s["slotCount"]; ok {
			ps.CollectionCount = sc
		}
		if mc, ok := statsRes.Int64s["manifestCount"]; ok {
			ps.ManifestCount = mc
		}
	}
	return
}

func (n Neo) ProfileGetByNameTag(ctx context.Context, name, tag string) (p types.Profile, found bool, err error) {
	q := fmt.Sprintf(`MATCH (prf:Profile) WHERE prf.name = "%s" AND prf.tag = "%s"`, name, tag)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	p, err = n.ProfileFromResult(res)
	return
}

func (n Neo) ProfileGetByID(ctx context.Context, prfID int64) (p types.Profile, found bool, err error) {
	q := fmt.Sprintf(`MATCH (prf:Profile) WHERE id(prf) = %d`, prfID)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	p, err = n.ProfileFromResult(res)
	return
}

func (n Neo) ProfilesGetByRegexLabel(ctx context.Context, labelRegex string) (ps []types.Profile, err error) {
	_, span := tracer.Start(ctx, "ProfilesGetByRegexLabel")
	defer span.End()
	q := fmt.Sprintf(`MATCH (prf:Profile) WHERE prf.label =~ "%s"`, labelRegex)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	logrus.Debugf("q: %s", q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
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

// ProfileGetAll returns all Profile nodes from the graph, each with its linked Rules populated.
// It fetches profiles first, then resolves their rules using the existing rule query infrastructure.
func (n Neo) ProfileGetAll(ctx context.Context) (ps []types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileGetAll",
	})
	defer span.End()

	q := fmt.Sprintf(`MATCH (prf:Profile) RETURN %s`, returnStatementProfile)
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		utils.ErrorHandler(l, span, err, "while executing ProfileGetAll query")
		return
	}
	ps = make([]types.Profile, 0)
	for _, r := range res {
		p, e := n.ProfileFromResult(r)
		if e != nil {
			logrus.Error(e)
			continue
		}
		// Fetch linked rules for this profile
		rq := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule) WHERE id(prf) = %d RETURN %s`, p.ID, ruleReturn)
		rres, e := n.Query(rq)
		if e != nil {
			utils.ErrorHandler(l, span, e, "while fetching rules for profile")
			continue
		}
		p.Rules = make([]types.ProfileRule, 0)
		for _, rr := range rres {
			rule, e := RuleFromResult(rr)
			if e != nil {
				logrus.Warnf("ProfileGetAll: skipping malformed rule: %v", e)
				continue
			}
			p.Rules = append(p.Rules, types.ProfileRule{
				ID:    rule.ID,
				Type:  rule.Type,
				Op:    rule.Op,
				Key:   rule.Key,
				Value: rule.Value,
			})
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
		logrus.Errorf(errFailedRunQueryFmt, err)
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

func (n Neo) ProfileLinkRule(ctx context.Context, profileID, ruleID int64) (err error) {
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
		f.Rules, err = n.RulesGetByFilterID(ctx, id)
		if err != nil {
			logrus.Error(err)
			return
		}
	*/
	return
}

func (n Neo) ProfileFromManifestWrapperID(ctx context.Context, maniID int64) (prfs []types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileFromManifestID",
		"maniID":  maniID,
	})
	prfLabels := set.NewStringSet()
	_ = l
	defer span.End()
	prfs, e := n.ProfileFromManifestIDViaAnnotations(ctx, maniID)
	if e != nil {
		err = errors.Wrap(e, "failed to get profiles via annotations for Manifest")
		return
	}
	for _, prf := range prfs {
		prfLabels.Add(prf.Label)
	}
	prfs2, e := n.ProfileFromManifestIDViaPlatform(ctx, maniID)
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

	prfs3, e := n.ProfileFromManifestIDViaOriginalManifest(ctx, maniID)
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

func (n Neo) ProfileFromManifestIDViaOriginalManifest(ctx context.Context, maniID int64) (prfs []types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileFromManifestIDViaOriginalManifest",
		"maniID":  maniID,
	})
	defer span.End()
	_ = ctx
	// TODO: Get Original Manifest (parsed, uploaded or pulled)
	mds, e := n.ManifestGetAllByManifestID(ctx, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get Manifest for Manifest")
		return
	}
	selectMds := []types.Manifest{}
	for _, md := range mds {
		if md.Type == "CREATED" {
			utils.Debug(l, span, "Manifest is of type '%s' - skipping", md.Type)
		} else {
			utils.Debug(l, span, "Manifest is of type '%s' - good for us", md.Type)
			selectMds = append(selectMds, md)
		}
	}
	prfs = []types.Profile{}
	// TODO: Get Profiles from Original Manifest
	for _, md := range selectMds {
		if ps, err := n.ProfileFromManifestIDViaOrig(ctx, md.ID); err != nil {
			return prfs, err
		} else {
			utils.Debug(l, span, "Found %d profiles via Manifest %d", len(ps), md.ID)
			prfs = append(prfs, ps...)
		}
	}
	return
}

func (n Neo) ProfileFromManifestIDViaOrig(ctx context.Context, maniDataID int64) (prfs []types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameProfile,
		"func":       "ProfileFromManifestIDViaOrig",
		"maniDataID": maniDataID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(:Rule)-[:TRUE]->(p:Platform)<-[r]-(md:Manifest) WHERE id(md) = %d`, maniDataID)
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
			err = utils.ErrorHandler(l, span, e, errProfileFromResult)
			continue
		}
		prfs = append(prfs, p)
	}
	return prfs, nil

}

func (n Neo) ProfileFromManifestWrapperIDViaAnnotations(ctx context.Context, mID int64) (prfs []types.Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "ProfileFromManifestIDViaAnnotations",
		"maniID":  mID,
	})
	_, span := tracer.Start(ctx, "ProfileFromManifestIDViaAnnotations")
	defer span.End()
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule)-[:%s]-(a:Annotation)-[]-(m:ManifestWrapper) WHERE id(m) = %d`, types.RelationSuccess, mID)
	q += fmt.Sprintf(` RETURN DISTINCT %s`, returnStatementProfile)
	res, err := n.Query(q)
	l.Debug(q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	if err != nil {
		return
	}
	prfs = make([]types.Profile, 0)
	for _, r := range res {
		p, e := n.ProfileFromResult(r)
		if e != nil {
			err = errors.Wrap(e, errProfileFromResult)
			l.Error(err)
			continue
		}
		q := fmt.Sprintf(`MATCH (m:ManifestWrapper)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
		q += fmt.Sprintf(` WHERE id(m) = %d AND id(p) = %d RETURN count(a) AS countAnnotations`, mID, p.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, attrSevDebug),
			attribute.String(attrLogQuery, q),
		))
		res, _, e := n.QueryOne(q)
		if e != nil {
			err = errors.Wrap(e, errQueryOne)
			l.Error(err)
			continue
		}
		cnt, e := res.GetInt("countAnnotations")
		if e != nil {
			err = errors.Wrap(e, errGetCountAnnotations)
			l.Error(err)

			return
		}
		if cnt == 0 {
			prfs = append(prfs, p)
		}
	}
	return
}

func (n Neo) ProfileFromManifestWrapperIDViaPlatform(ctx context.Context, maniID int64) (prfs []types.Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "ProfileFromManifestIDViaPlatform",
		"maniID":  maniID,
	})
	_, span := tracer.Start(ctx, "ProfileFromManifestIDViaPlatform")
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule)-[:%s]->(p:Platform)<-[:HAS_PLATFORM]-(m:ManifestWrapper) WHERE id(m) = %d`, rel, maniID)
	q += fmt.Sprintf(` RETURN DISTINCT %s`, returnStatementProfile)
	res, err := n.Query(q)
	l.Debug(q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
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
		q := fmt.Sprintf(`MATCH (m:ManifestWrapper)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
		q += fmt.Sprintf(` WHERE id(m) = %d AND id(p) = %d RETURN count(a) AS countAnnotations`, maniID, p.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, attrSevDebug),
			attribute.String(attrLogQuery, q),
		))
		res, _, e := n.QueryOne(q)
		if e != nil {
			err = errors.Wrap(e, errQueryOne)
			l.Error(err)
			continue
		}
		cnt, e := res.GetInt("countAnnotations")
		if e != nil {
			err = errors.Wrap(e, errGetCountAnnotations)
			l.Error(err)
			continue
		}
		if cnt == 0 {
			prfs = append(prfs, p)
		}
	}
	return
}

func (n Neo) ProfileFromManifestID(ctx context.Context, mdID int64) (prfs []types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileFromManifestID",
		"maniID":  mdID,
	})
	defer span.End()
	prfs, e := n.ProfileFromManifestIDViaAnnotations(ctx, mdID)
	if e != nil {
		utils.ErrorHandler(l, span, e, "failed to get profiles via annotations for Manifest")
		return
	}
	if len(prfs) == 0 {
		prfs2, e := n.ProfileFromManifestIDViaPlatform(ctx, mdID)
		if e != nil {
			utils.ErrorHandler(l, span, e, "failed to get profiles via platform for Manifest")
			return
		}
		utils.Debug(l, span, "found %d profiles via platform for Manifest", len(prfs2))
		prfs = append(prfs, prfs2...)
	} else {
		utils.Debug(l, span, "found %d profiles via annotations for Manifest", len(prfs))
	}
	return
}

func (n Neo) ProfileFromManifestIDViaAnnotations(ctx context.Context, mdID int64) (prfs []types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameProfile,
		"func":    "ProfileFromManifestIDViaAnnotations",
		"maniID":  mdID,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule)-[:%s]-(a:Annotation)-[]-(md:Manifest) WHERE id(md) = %d`, types.RelationSuccess, mdID)
	q += fmt.Sprintf(` RETURN DISTINCT %s`, returnStatementProfile)
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return
	}
	utils.Debug(l, span, "Number of profiles found: %d", len(res))
	prfs = make([]types.Profile, 0)
	for _, r := range res {
		p, e := n.ProfileFromResult(r)
		if e != nil {
			err = errors.Wrap(e, errProfileFromResult)
			l.Error(err)
			continue
		}
		q := fmt.Sprintf(`MATCH (md:Manifest)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
		q += fmt.Sprintf(` WHERE id(md) = %d AND id(p) = %d RETURN count(a) AS countAnnotations`, mdID, p.ID)
		utils.Debug(l, span, q)
		res, _, e := n.QueryOne(q)
		if e != nil {
			utils.ErrorHandler(l, span, e, errQueryOne)
			continue
		}
		cnt, e := res.GetInt("countAnnotations")
		if e != nil {
			err = errors.Wrap(e, errGetCountAnnotations)
			l.Error(err)

			return
		}
		if cnt == 0 {
			prfs = append(prfs, p)
		}
	}
	return
}

func (n Neo) ProfileFromManifestIDViaPlatform(ctx context.Context, mdID int64) (prfs []types.Profile, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "ProfileFromManifestIDViaPlatform",
		"maniID":  mdID,
	})
	_, span := tracer.Start(ctx, "ProfileFromManifestIDViaPlatform")
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule)-[:%s]->(p:Platform)<-[:HAS_PLATFORM]-(md:Manifest) WHERE id(md) = %d`, rel, mdID)
	q += fmt.Sprintf(` RETURN DISTINCT %s`, returnStatementProfile)
	res, err := n.Query(q)
	l.Debug(q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
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
		q := fmt.Sprintf(`MATCH (md:Manifest)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
		q += fmt.Sprintf(` WHERE id(md) = %d AND id(p) = %d RETURN count(a) AS countAnnotations`, mdID, p.ID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, attrSevDebug),
			attribute.String(attrLogQuery, q),
		))
		res, _, e := n.QueryOne(q)
		if e != nil {
			err = errors.Wrap(e, errQueryOne)
			l.Error(err)
			continue
		}
		cnt, e := res.GetInt("countAnnotations")
		if e != nil {
			err = errors.Wrap(e, errGetCountAnnotations)
			l.Error(err)
			continue
		}
		if cnt == 0 {
			prfs = append(prfs, p)
		}
	}
	return
}

func (n Neo) EvalUploadProfile(ctx context.Context, mani types.ManifestWrapper, origMani types.Manifest) (err error) {
	ctx, span := tracer.Start(ctx, "EvalUploadProfile")
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (bu:BlobUpload)-[:UPLOADED]->(cfg:Blob) WHERE id(cfg) = %d`, origMani.Config.ID)
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
	logrus.Debugf("blobUploadProfile[%-4d]: %s", blobUploadID, blobUploadProfile)

	return
}

func (n Neo) ProfileAuthorizedPushHandler(ctx context.Context, req types.Request) (prf types.Profile, err error) {
	ctx, span := tracer.Start(ctx, "ProfileAuthorizedPushHandler")
	defer span.End()

	// Check if profile exists — first by label, then by tag as fallback.
	// The profile field from docker login may contain only the tag portion
	// (e.g. "zen2" instead of "cpu#zen2").
	prf, found, e := n.ProfileGetByLabel(ctx, req.Profile)
	if e != nil {
		err = errors.Wrapf(e, "failed to get profile '%s'", req.Profile)
		span.RecordError(err)
		return
	}
	if !found {
		// Try matching by tag
		prfs, prfErr := n.ProfileGetAll(ctx)
		if prfErr == nil {
			for _, candidate := range prfs {
				if candidate.Tag == req.Profile {
					prf = candidate
					found = true
					break
				}
			}
		}
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

func (n Neo) ProfileManifestType(ctx context.Context, maniID int64) (relType string, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "ProfileManifestType",
		"maniID":  maniID,
	})
	_, span := tracer.Start(ctx, "ProfileManifestType")
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniID").Int64(maniID),
	)
	q := `MATCH (prf:Profile)<-[r]-(md:Manifest)<-[]-(m:ManifestWrapper)`
	q += fmt.Sprintf(` WHERE id(m) = %d`, maniID)
	q += ` RETURN TYPE(r) AS relType`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	l.Debug(q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, attrSevError),
			attribute.String("log.error", err.Error()),
		))
		span.RecordError(err)
		return
	}
	if !found {
		err = fmt.Errorf("no manifest found for id %d", maniID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, attrSevError),
			attribute.String("log.error", err.Error()),
		))
		return
	}
	relType, _ = res.GetStr("relType")
	return
}

func (n Neo) ProfileGetUploadedByViaManifestID(ctx context.Context, maniID int64) (p types.Profile, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "ProfileGetUploadedByViaManifestID",
		"maniID":  maniID,
	})
	_, span := tracer.Start(ctx, "Profile_GetUploadedBy_BiaManifestID")
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniID").Int64(maniID),
	)
	q := `MATCH (prf:Profile)<-[:UPLOADED_BY]-(md:Manifest)`
	q += fmt.Sprintf(` WHERE id(md) = %d`, maniID)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	l.Debug(q)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	p, err = n.ProfileFromResult(res)
	return
}

func (n Neo) ProfileGetPulledByViaManifestID(ctx context.Context, maniID int64) (p types.Profile, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfile,
		"func":    "ProfileGetPulledByViaManifestID",
		"maniID":  maniID,
	})
	_, span := tracer.Start(ctx, "ProfileGetPulledByViaManifestID")
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniID").Int64(maniID),
	)
	q := `MATCH (prf:Profile)<-[:PULLED_BY]-(md:Manifest)`
	q += fmt.Sprintf(` WHERE id(md) = %d`, maniID)
	q += fmt.Sprintf(` RETURN %s`, returnStatementProfile)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
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

func (n Neo) ProfileUnwindRules(ctx context.Context, prfID, maniDataID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameProfile,
		"func":       "ProfileUnwindRules",
		"maniDataID": maniDataID,
		"prfID":      prfID,
	})
	defer span.End()
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
			aID, err := n.AnnotationGetInsert(ctx, a)
			if err != nil {
				err = errors.Wrapf(err, "failed to get annotation %s:%s", a.Key, a.Value)
				return err
			}
			e := n.ManifestLinkToAnnotation(ctx, maniDataID, aID)
			if e != nil {
				err = errors.Wrapf(e, "failed to link manifestData '%d' with  annotation %d", maniDataID, aID)
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

func (n Neo) ProfileFromYAML(ctx context.Context, p config.Profile) (prf types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":     pkgNameProfile,
		"func":        "ProfileFromYAML",
		"profileName": p.Name,
		"profileTag":  p.Tag,
		"profileRule": p.Rule,
	})
	defer span.End()
	mps := []types.Manipulation{}
	for _, ele := range p.Manipulations {
		mp, e := n.ManipulationFromYAML(ctx, ele)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get manipulation from yaml")
			return prf, err
		}
		mps = append(mps, mp)
	}
	prf = types.Profile{
		Name:          p.Name,
		Tag:           p.Tag,
		Rule:          p.Rule,
		Manipulations: mps,
	}
	return
}

// ProfileInjectDeterministicAnnotations links every deterministic
// ("anno eq") rule of the given profile as an annotation on the given
// Manifest. Non-deterministic rules (regex, ne, null, platform rules,
// etc.) are silently skipped; the returned counts let the caller observe
// how many rules were injected vs skipped and a Debug log line is emitted
// per skipped rule for operator visibility.
//
// It is the push-time cousin of ProfileUnwindRules, used by
// Magazine.ComputeManifestlistsForRepoTag. The helper is idempotent:
// re-running it on the same (profile, manifestdata) pair will not create
// duplicate Annotation nodes or HAS_ANNOTATION edges because both
// AnnotationGetInsert and ManifestLinkToAnnotation rely on MERGE
// semantics in their underlying Cypher.
func (n Neo) ProfileInjectDeterministicAnnotations(ctx context.Context, prfID, maniDataID int64) (injected int, skipped int, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameProfile,
		"func":       "ProfileInjectDeterministicAnnotations",
		"prfID":      prfID,
		"maniDataID": maniDataID,
	})
	defer span.End()

	// Fetch every rule attached to the profile so we can classify them and
	// count the deterministic/non-deterministic split.
	allRules, err := n.RulesGetByProfileID(ctx, prfID)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get rules for profile %d", prfID)
		return
	}

	for _, r := range allRules.Rules {
		if r.Type == types.RuleTypeAnnotation && r.Op == "eq" {
			a := types.NewAnnotation(r.Key, r.Value)
			aID, e := n.AnnotationGetInsert(ctx, a)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to get/insert annotation %s=%s", a.Key, a.Value)
				return
			}
			if e := n.ManifestLinkToAnnotation(ctx, maniDataID, aID); e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to link manifestdata %d to annotation %d", maniDataID, aID)
				return
			}
			injected++
			utils.Debug(l, span, "injected deterministic rule %s (type=%s op=%s %s=%s)", r.Label, r.Type, r.Op, r.Key, r.Value)
			continue
		}
		// Any other rule type/op is non-deterministic at push time — it
		// needs rule eval or regex matching against the manifest's own
		// annotations, which happens during ComputeManifestlistsForRepoTag's
		// RuleEvalAll call. We only count and log it here.
		skipped++
		utils.Debug(l, span, "skipping non-deterministic rule %s (type=%s op=%s)", r.Label, r.Type, r.Op)
	}
	utils.Debug(l, span, "profile %d: injected %d, skipped %d rules on manifestdata %d", prfID, injected, skipped, maniDataID)
	return
}
