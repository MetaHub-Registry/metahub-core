package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
		r, found, err := n.Rule_GetByLabel(ctx, lbl)
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

func (n Neo) Profile_Get_ByNameTag(ctx context.Context, name, tag string) (p types.Profile, found bool, err error) {
	q := fmt.Sprintf(`MATCH (prf:Profile) WHERE prf.name = "%s" AND prf.tag = "%s"`, name, tag)
	q += ` RETURN id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule`
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	p, err = n.ProfileFromResult(res)
	return
}

func (n Neo) GetProfiles() (ps []types.Profile, err error) {
	q := `MATCH (prf:Profile)`
	q += ` RETURN id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule`
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
	p = types.Profile{
		ID:            id,
		Name:          name,
		Tag:           tag,
		Rule:          profileRule,
		Manipulations: []string{},
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
func (n Neo) Profile_FromManifestDataID(ctx context.Context, mdID int64) (prfs []types.Profile, err error) {
	ctx, span := tracer.Start(ctx, "Profile_FromManifestDataID")
	defer span.End()
	prfs, e := n.Profile_FromManifestDataID_ViaAnnotations(ctx, mdID)
	if e != nil {
		err = errors.Wrap(e, "failed to get profiles via annotations for ManifestData")
		return
	}
	prfs2, e := n.Profile_FromManifestDataID_ViaPlatform(ctx, mdID)
	if e != nil {
		err = errors.Wrap(e, "failed to get profiles via platform for ManifestData")
		return
	}
	prfs = append(prfs, prfs2...)
	return
}

func (n Neo) Profile_FromManifestDataID_ViaAnnotations(ctx context.Context, mdID int64) (prfs []types.Profile, err error) {
	_, span := tracer.Start(ctx, "Profile_FromManifestDataID_ViaAnnotations")
	defer span.End()
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule)-[]-(a:Annotation)-[]-(md:ManifestData) WHERE id(md) = %d`, mdID)
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule`
	res, err := n.Query(q)
	//fmt.Printf("Query: \n%s\n", q)
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
		prfs = append(prfs, p)
	}
	return
}

func (n Neo) Profile_FromManifestDataID_ViaPlatform(ctx context.Context, mdID int64) (prfs []types.Profile, err error) {
	_, span := tracer.Start(ctx, "Profile_FromManifestDataID_ViaPlatform")
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(r:Rule)-[%s]->(p:Platform)<-[:IS]-(m:Manifest)-[:CREATED]-(md:ManifestData) WHERE id(md) = %d`, rel, mdID)
	q += ` RETURN DISTINCT id(prf) AS profileID, prf.name AS profileName, prf.tag AS profileTag, prf.rule AS profileRule`
	res, err := n.Query(q)
	//fmt.Printf("Query: \n%s\n", q)
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
		prfs = append(prfs, p)
	}
	return
}
