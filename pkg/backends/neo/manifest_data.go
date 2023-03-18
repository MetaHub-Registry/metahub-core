package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (n Neo) ManifestData_V1_GetInsert(ctx context.Context, maniSiID int64, mani v1.Descriptor) (maniID int64, found bool, err error) {
	maniID, found, err = n.ManifestData_V1_Get(ctx, mani)
	if err != nil {
		err = errors.Wrap(err, "failed to get manifest by digest")
		return
	}
	if !found {
		label := utils.CreateLabelFromDescriptor(mani)
		q := `CREATE (m:ManifestData {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(m) AS manifestID`
		query := fmt.Sprintf(q, mani.Digest.String(), mani.MediaType, mani.Size, label)
		res, e := n.Execute(query)
		if e != nil {
			err = errors.Wrap(e, "failed to create (m:ManifestData)")
			return
		}
		maniID, e = res.GetInt64("manifestID")
		if e != nil {
			err = errors.Wrap(e, "failed to get manifestID from result")
			return
		}
	}
	NodeRel := "CREATED"
	// Link to manifest standin
	q := `MATCH (md:ManifestData), (m:Manifest) WHERE id(md) = %d AND id(m) = %d`
	q += fmt.Sprintf(` MERGE (m)-[:%s]->(md)`, NodeRel)
	query := fmt.Sprintf(q, maniID, maniSiID)
	_, err = n.Execute(query)
	if err != nil {
		err = errors.Wrapf(err, "failed to create (m)-[:%s]->(md)", NodeRel)
		return
	}
	return
}

func (n Neo) ManifestData_V1_Get(ctx context.Context, mani v1.Descriptor) (maniID int64, found bool, err error) {
	q := `MATCH (m:ManifestData) WHERE m.digest = "%s" AND m.mediaType = "%s"`
	q += ` RETURN id(m) AS manifestID`
	query := fmt.Sprintf(q, mani.Digest.String(), mani.MediaType)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return
	}
	maniID, err = res.GetInt64("manifestID")
	if err != nil {
		err = errors.Wrap(err, "failed to get manifestID from result while getting manifest by digest and mediaType")
		return
	}
	return
}

func (n Neo) ManifestData_Get_ByProfileMediaType(ctx context.Context, mID, prfID int64, mediaType string) (mdID int64, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":   "magazine/manifest_data.go",
		"func":      "ManifestData_Get_ByProfileMediaType",
		"prfID":     prfID,
		"maniID":    mID,
		"mediaType": mediaType,
	})
	_, span := tracer.Start(ctx, "ManifestData_Get_ByProfileMediaType")
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniID").Int64(mID),
		attribute.Key("prfID").Int64(prfID),
		attribute.Key("mani.media_type").String(mediaType),
	)
	q := `MATCH (prf:Profile)<-[:UPLOADED_BY]-(md:ManifestData)-[]-(m:Manifest)`
	q += fmt.Sprintf(` WHERE id(m) = %d AND id(prf) = %d AND md.mediaType = "%s"`, mID, prfID, mediaType)
	q += ` RETURN id(md) AS manifestID`
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		l.Debug(q)
		l.Warn("Not able to find manifestData for manifestID: ", mID)
		return
	}
	mdID, err = res.GetInt64("manifestID")
	l.Debugf("%s -> maniID: %d", q, mdID)
	return
}

func (n Neo) ManifestData_GetParsedNodeRel_ByMediaType(ctx context.Context, msID int64, mediaType string) (nr types.NodeRel, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":   "magazine/manifest_data.go",
		"func":      "ManifestData_GetParsedNodeRel_ByMediaType",
		"mani.ID":   msID,
		"mediaType": mediaType,
	})
	_, span := tracer.Start(ctx, "ManifestData_GetParsedNodeRel_ByMediaType")
	defer span.End()
	span.SetAttributes(
		attribute.Key("mani.id").Int64(msID),
		attribute.Key("mani.media_type").String(mediaType),
	)
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[rel]-(m:Manifest) WHERE id(m) = %d AND md.mediaType = "%s"`, msID, mediaType)
	q += ` RETURN id(md) AS manifestID, TYPE(rel) AS relType`
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		l.Debug(q)
		l.Warn("Not able to found manifestData for manifestID: ", msID)
		return
	}
	maniID, err := res.GetInt64("manifestID")
	if err != nil {
		err = errors.Wrap(err, "failed to get manifestID from result while getting manifest by digest and mediaType")
		return
	}
	relType, err := res.GetStr("relType")
	if err != nil {
		err = errors.Wrap(err, "failed to get relType from result while getting manifest by digest and mediaType")
		return
	}
	nr = types.NewNodeRel(maniID, relType)
	l.Debugf("%s -> NodeRel: %d [%s]", q, nr.NodeID, nr.NodeRel)
	return
}

func (n Neo) ManifestData_GetParsedID_ByMediaType(ctx context.Context, msID int64, mediaType string, relType string) (maniID int64, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":   "magazine/manifest_data.go",
		"func":      "ManifestData_GetParsedID_ByMediaType",
		"mani.ID":   msID,
		"mediaType": mediaType,
	})
	_, span := tracer.Start(ctx, "ManifestData_GetParsedID_ByMediaType")
	defer span.End()
	span.SetAttributes(
		attribute.Key("mani.id").Int64(msID),
		attribute.Key("mani.media_type").String(mediaType),
	)
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[:%s]-(m:Manifest) WHERE id(m) = %d AND md.mediaType = "%s"`, relType, msID, mediaType)
	q += ` RETURN id(md) AS manifestID`
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		l.Debug(q)
		l.Warn("Not able to found manifestData for manifestID: ", msID)
		return
	}
	maniID, err = res.GetInt64("manifestID")
	if err != nil {
		err = errors.Wrap(err, "failed to get manifestID from result while getting manifest by digest and mediaType")
		return
	}
	l.Debugf("%s -> maniID: %d", q, maniID)
	return
}

func (n Neo) ManifestData_GetID_ByMediaType(ctx context.Context, msID int64, mediaType string) (maniID int64, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":   "magazine/manifest_data.go",
		"func":      "ManifestData_GetID_ByMediaType",
		"mani.ID":   msID,
		"mediaType": mediaType,
	})
	_, span := tracer.Start(ctx, "ManifestData_GetID_ByMediaType")
	defer span.End()
	span.SetAttributes(
		attribute.Key("mani.id").Int64(msID),
		attribute.Key("mani.media_type").String(mediaType),
	)
	q := `MATCH (md:ManifestData)-[]-(m:Manifest) WHERE id(m) = %d AND md.mediaType = "%s"`
	q += ` RETURN id(md) AS manifestID`
	query := fmt.Sprintf(q, msID, mediaType)

	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		l.Debug(query)
		l.Warn("Not able to found manifestData for manifestID: ", msID)
		return
	}
	maniID, err = res.GetInt64("manifestID")
	l.Debugf("%s -> maniID: %d", query, maniID)
	return
}

func (n Neo) ManifestData_GetAll() (manis []types.ManifestData, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[r]-(m:Manifest)-[]-(u:Upstream)`
	q += ` MATCH (m)-[]-(md:ManifestData)-[:USES]-(cfg:Config)`
	q += ` RETURN type(r) AS manifestdataRelType,`
	q += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(m) AS manifestID,`
	q += ` id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.mediaType AS manifestDataMediaType, md.size AS manifestDataSize,`
	q += ` ` + UpstreamReturn + `,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, cfg.mediaType AS configMediaType, cfg.size AS configSize`
	//fmt.Printf("query: \n%s\n", q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	ctx := context.Background()
	manis = make([]types.ManifestData, 0)
	for result.Next() {
		p, e := PlatformFromRecord(result)
		if e != nil {
			err = e
			return
		}
		c, e := ConfigFromRecord(result)
		if e != nil {
			err = e
			return
		}
		u, e := UpstreamFromRecord(result)
		if e != nil {
			err = e
			return
		}
		m, e := ManifestDataFromRecord(result)
		if e != nil {
			err = e
			return
		}
		anno, e := n.Labels_GetByConfigID(ctx, c.ID)
		if e != nil {
			err = e
			return
		}
		m.Annotations = anno
		c.Labels = anno
		m.Config = c
		m.Platform = p
		m.Upstream = u
		m.Layers, err = n.GetLayersForManifestID(m.ID)
		if err != nil {
			return
		}
		// Add manifest to list
		manis = append(manis, m)
	}
	return
}

func (n Neo) ManifestData_GetAllByManifestID(ctx context.Context, msID int64) (manis []types.ManifestData, err error) {
	q := `MATCH (ms:Manifest)-[r]->(m:ManifestData) WHERE id(ms) = %d`
	q += ` MATCH (m)-[]-(cfg:Config)`
	q += ` RETURN id(m) AS manifestID, m.digest AS manifestDigest, m.mediaType AS manifestMediaType, m.size AS manifestSize,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, cfg.mediaType AS configMediaType, cfg.size AS configSize,`
	q += ` type(r) AS manifestOrigin`
	query := fmt.Sprintf(q, msID)
	res, err := n.Query(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	manis = make([]types.ManifestData, 0)
	for _, r := range res {
		m, e := n.ManifestDataFromResult(ctx, r)
		if e != nil {
			err = e
			return
		}
		c, e := ConfigFromResult(r)
		if e != nil {
			err = e
			return
		}
		// Get Labels for Config
		anno, e := n.Labels_GetByConfigID(ctx, c.ID)
		if e != nil {
			err = e
			return
		}
		// Get Labels for Config
		labels, e := n.Labels_GetByConfigID(ctx, c.ID)
		if e != nil {
			err = e
			return
		}
		err = anno.MergeAnnotations(labels)
		if err != nil {
			return
		}
		c.Labels = anno
		c.Cfg.Config.Labels = anno.ToMap()
		m.Config = c
		// Add Layers
		m.Layers, err = n.GetLayersForManifestID(m.ID)
		if err != nil {
			return
		}
		manis = append(manis, m)
	}
	return
}

func (n Neo) ManifestData_Update(ctx context.Context, maniID int64, mani types.ManifestData) (err error) {
	q := `MATCH (m:ManifestData) WHERE id(m) = %d SET m.digest = "%s", m.mediaType = "%s", m.size = %d`
	query := fmt.Sprintf(q, maniID, mani.Digest, mani.MediaType, mani.Size)
	//fmt.Printf("query: \n%s\n", query)
	_, err = n.Execute(query)
	if err != nil {
		logrus.Errorf("failed to update manifest: %v", err)
		return
	}
	return
}

func (n Neo) ManifestData_LinkToAnnotation(ctx context.Context, maniID, annotationID int64) (err error) {
	q := `MATCH (m:ManifestData), (a:Annotation) WHERE id(m) = %d AND id(a) = %d MERGE (m)-[:HAS_ANNOTATION]->(a)`
	query := fmt.Sprintf(q, maniID, annotationID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	// TODO: Check if annotation is linked to a rule which has all filters rules satisfied

	return
}

func (n Neo) ManifestData_LinkToOCIv1Config(ctx context.Context, maniID, cfgID int64) (err error) {
	q := `MATCH (m:ManifestData), (c:Config) WHERE id(m) = %d AND id(c) = %d MERGE (m)-[:USES]->(c)`
	query := fmt.Sprintf(q, maniID, cfgID)
	//fmt.Printf("query: \n%s\n", query)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) ManifestData_LinkToOCIv1Layer(ctx context.Context, maniID, layerID int64) (err error) {
	q := `MATCH (m:ManifestData), (l:Layer) WHERE id(m) = %d AND id(l) = %d MERGE (m)-[:HAS]->(l)`
	query := fmt.Sprintf(q, maniID, layerID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) ManifestData_AddAnnotations(ctx context.Context, maniID int64, as *types.Annotations) (err error) {
	for k, a := range as.Data {
		logrus.Warnf("adding annotation %s=%s", k, a.Value)
		aID, e := n.Annotation_GetInsert(ctx, a)
		if e != nil {
			err = errors.Wrap(e, "failed to add annotation")
			return
		}
		a.ID = aID
		err = n.ManifestData_LinkToAnnotation(ctx, maniID, aID)
	}
	return
}

func (n Neo) ManifestData_GetTypes_ByRequest(ctx context.Context, req types.Request) (res types.ManifestData, found bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestData_GetTypes_ByRequest")
	defer span.End()
	span.SetAttributes(
		attribute.Key("req.Digest").String(req.Digest),
		attribute.Key("req.Host").String(req.Host),
		attribute.Key("req.Repo").String(req.Repo),
		attribute.Key("req.Tag").String(req.Tag),
		attribute.Key("req.Profile").String(req.Profile),
	)
	if req.Digest != "" {
		return n.ManifestData_Get_ByDigest(ctx, req.Digest)
	} else if req.Tag != "" {
		up := types.Upstream{
			Reg:     req.Host,
			Repo:    req.Repo,
			Tag:     req.Tag,
			Profile: req.Profile,
		}
		return n.ManifestData_Get_ByUpstream(ctx, up)
	}
	return
}

func (n Neo) ManifestData_GetID_ByDigest(ctx context.Context, dig string) (maniID int64, found bool, err error) {
	m, found, err := n.ManifestData_Get_ByDigest(ctx, dig)
	if err != nil || !found {
		return
	}
	maniID = m.ID
	return
}

func (n Neo) ManifestData_GetByRule(ctx context.Context, rule types.Rule) (mds []types.ManifestData, err error) {
	mds, e := n.ManifestData_GetByAnnotationRule(ctx, rule)
	if e != nil {
		err = errors.Wrap(e, "failed to get manifest data by annotation rule")
		return
	}
	pMDs, e := n.ManifestData_GetByPlatformRule(ctx, rule)
	if e != nil {
		err = errors.Wrap(e, "failed to get manifest data by platform rule")
		return
	}
	mds = append(mds, pMDs...)
	return
}

func (n Neo) ManifestData_GetByPlatformRule(ctx context.Context, rule types.Rule) (mds []types.ManifestData, err error) {
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (r:Rule)-[%s]->(p:Platform)<-[:IS]-(m:Manifest)-[:CREATED]-(md:ManifestData) WHERE id(r) = %d`, rel, rule.ID)
	q += ` MATCH (u:Upstream)-[]-(m)`
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize, md.label AS manifestLabel,`
	q += ` id(p) AS platformID, p.os AS platformOS, p.arch AS platformArch, p.variant AS platformVariant,`
	q += ` ` + UpstreamReturn

	//fmt.Printf("query: \n%s\n", q)
	res, err := n.Query(q)
	for _, r := range res {
		m, e := n.ManifestDataFromResult(ctx, r)
		if e != nil {
			err = errors.Wrap(e, "failed to get manifest from result")
			return
		}
		up, e := UpstreamFromResult(r)
		if e != nil {
			err = errors.Wrap(e, "failed to get upstream from result")
			return
		}
		p, e := PlatformFromResult(r)
		if e != nil {
			err = e
			return
		}
		m.Platform = p
		m.Upstream = up
		logrus.Warnf("found manifest via platform: %s:%s (%s)", m.Upstream.Repo, m.Upstream.Tag, m.Label)
		mds = append(mds, m)
	}
	return
}

func (n Neo) ManifestData_CheckProfile(ctx context.Context, md types.ManifestData, prf types.Profile) (ok bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestData_CheckProfile")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("md.label").String(md.Label),
		attribute.Key("profile.name").String(prf.Name),
	)
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
	q += fmt.Sprintf(` WHERE id(md) = %d AND id(p) = %d RETURN count(a) AS countAnnotations`, md.ID, prf.ID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, _, err := n.QueryOne(q)
	if err != nil {
		return
	}
	cnt, err := res.GetInt("countAnnotations")
	if err != nil {
		return
	}
	ok = cnt == 0
	return
}

func (n Neo) ManifestData_GetByAnnotationRule(ctx context.Context, rule types.Rule) (mds []types.ManifestData, err error) {
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (u:Upstream)-[]-(m:Manifest)-[:CREATED]-(md:ManifestData)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule) WHERE id(r) = %d`, rel, rule.ID)
	q += ` MATCH (m)-[:IS]->(p:Platform)`
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize, md.label AS manifestLabel,`
	q += ` id(p) AS platformID, p.os AS platformOS, p.arch AS platformArch, p.variant AS platformVariant,`
	q += ` ` + UpstreamReturn
	//fmt.Printf("query: \n%s\n", q)
	res, err := n.Query(q)
	for _, r := range res {
		m, e := n.ManifestDataFromResult(ctx, r)
		if e != nil {
			err = errors.Wrap(e, "failed to get manifest from result")
			return
		}
		up, e := UpstreamFromResult(r)
		if e != nil {
			err = errors.Wrap(e, "failed to get upstream from result")
			return
		}
		p, e := PlatformFromResult(r)
		if e != nil {
			err = e
			return
		}
		m.Platform = p
		m.Upstream = up
		logrus.Warnf("found manifest via annotation: %s:%s (%s)", m.Upstream.Repo, m.Upstream.Tag, m.Label)
		mds = append(mds, m)
	}
	return
}

func (n Neo) ManifestData_Get_ByID(ctx context.Context, mID int64) (m types.ManifestData, found bool, err error) {
	q := `MATCH (md:ManifestData)-[]-(m:Manifest)-[:UPSTREAM]->(u:Upstream) WHERE id(m) = %d`
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize,`
	q += ` ` + UpstreamReturn
	query := fmt.Sprintf(q, mID)
	//fmt.Printf("Q: %s\n", query)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return
	}
	if !found {
		return
	}
	m, err = n.ManifestDataFromResult(ctx, res)
	if err != nil {
		err = errors.Wrap(err, "failed to get manifest from result")
		return
	}
	up, err := UpstreamFromResult(res)
	if err != nil {
		err = errors.Wrap(err, "failed to get upstream from result")
		return
	}
	m.Upstream = up
	return
}

func (n Neo) ManifestData_LinkToConfig(ctx context.Context, maniID, cfgID int64) (err error) {
	_, span := tracer.Start(ctx, "ManifestData_LinkToConfig")
	defer span.End()
	q := `MATCH (m:ManifestData),(c:Config) WHERE id(m) = %d AND id(c) = %d`
	q += ` MERGE (m)-[:USES]->(c)`
	query := fmt.Sprintf(q, maniID, cfgID)
	//fmt.Printf("query: \n%s\n", query)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:ManifestData)-[:USES]->(c:Config): %v", err)
		return
	}
	return
}

func (n Neo) ManifestData_LinkToLayer(ctx context.Context, maniID, layerID int64, idx int) (err error) {
	_, span := tracer.Start(ctx, "ManifestData_LinkToLayer")
	defer span.End()
	q := `MATCH (m:ManifestData),(l:Layer) WHERE id(m) = %d AND id(l) = %d`
	q += ` MERGE (m)-[:HAS {order: %d}]->(l)`
	query := fmt.Sprintf(q, maniID, layerID, idx)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:ManifestData)-[:HAS {order: %d}]->(l:Layer): %v", idx, err)
		return
	}
	return
}

func (n Neo) ManifestData_Remove(ctx context.Context, req types.Request) (found bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestData_Remove")
	defer span.End()
	mani, found, err := n.ManifestData_GetTypes_ByRequest(ctx, req)
	if err != nil {
		err = errors.Wrap(err, "failed to get manifest by id")
		return
	}
	if !found {
		return
	}
	q := `MATCH (m:ManifestData) WHERE id(m) = %d DETACH DELETE m`
	query := fmt.Sprintf(q, mani.ID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		err = errors.Wrapf(err, "failed to delete (m:ManifestData) with id '%d'", mani.ID)
		return
	}
	return
}

func (n Neo) createManifestData(ctx context.Context, mani types.ManifestData) (maniID int64, err error) {
	ctx, span := tracer.Start(ctx, "createManifestData")
	defer span.End()
	label := utils.CreateLabelFromTypeAndDigest(mani.MediaType, mani.Digest)
	q := `CREATE (m:ManifestData {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(m) AS manifestID`
	query := fmt.Sprintf(q, mani.Digest, mani.MediaType, mani.Size, label)
	res, err := n.Execute(query)
	if err != nil {
		logrus.Errorf("failed to create manifest: %v", err)
		return
	}
	maniID, err = res.GetInt64("manifestID")
	if err != nil {
		logrus.Errorf("failed to create manifestID from result")
		fmt.Printf("Query: \n%s\n", query)
		return
	}
	// Link Config
	if mani.Config.Digest != "" {
		cfgId, found, err := n.Config_GetID_ByDigestAndType(ctx, mani.Config.Digest, mani.Config.MediaType)
		if err != nil {
			err = errors.Wrap(err, "failed to get config id")
			return maniID, err
		}
		if !found {
			err = fmt.Errorf("config w/ digest '%s' not found", utils.ShortenDigest(mani.Config.Digest))
			return maniID, err
		}
		err = n.ManifestData_LinkToConfig(context.Background(), maniID, cfgId)
		if err != nil {
			err = errors.Wrap(err, "failed to link manifest to config")
			return maniID, err
		}
	}
	// Link Layers
	for idx, layer := range mani.Layers {
		layer, found, err := n.Layer_GetByTypeAndDigest(ctx, layer.MediaType, layer.Digest)
		if err != nil {
			err = errors.Wrapf(err, "failed to get layer%d id", idx)
			return maniID, err
		}
		if !found {
			err = errors.New("layer not found")
			return maniID, err
		}
		err = n.ManifestData_LinkToLayer(ctx, maniID, layer.ID, idx)
		if err != nil {
			err = errors.Wrap(err, "failed to link manifest to layer")
			return maniID, err
		}
	}
	return
}

func (n Neo) ManifestData_GetExactly(mani types.ManifestData) (maniID int64, success bool, err error) {
	q := `MATCH (m:ManifestData) WHERE m.digest = "%s" AND m.mediaType = "%s" AND m.size = %d RETURN id(m) AS maniID`
	query := fmt.Sprintf(q, mani.Digest, mani.MediaType, mani.Size)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("maniID")
		if !ok {
			err = fmt.Errorf("failed to get maniID")
			return
		}
		maniID = v.(int64)
		success = true
	}
	return
}

func (n Neo) ManifestData_Get_ByDigest(ctx context.Context, dig string) (m types.ManifestData, found bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestData_Get_ByDigest")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("digest").String(dig),
	)
	q := `MATCH (md:ManifestData)-[r]-(m:Manifest)-[]->(u:Upstream) WHERE md.digest = "%s"`
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize,`
	q += ` ` + UpstreamReturn + `,`
	q += ` type(r) AS relType`
	query := fmt.Sprintf(q, dig)
	//fmt.Printf("Q: \n%s\n", query)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return
	}
	if !found {
		return
	}
	m, err = n.ManifestDataFromResult(ctx, res)
	if err != nil {
		err = errors.Wrap(err, "failed to get manifest from result")
		return
	}
	up, err := UpstreamFromResult(res)
	m.Upstream = up
	if err != nil {
		err = errors.Wrap(err, "failed to get upstream from result")
		return
	}
	return
}

func (n Neo) ManifestData_Get_ByUpstream(ctx context.Context, up types.Upstream) (m types.ManifestData, found bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestData_Get_ByUpstream")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("reg").String(up.Reg),
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("profile").String(up.Profile),
	)
	q := `MATCH (prf:Profile)-[]-(md:ManifestData)-[r]-(m:Manifest)-[:UPSTREAM]->(u:Upstream)`
	q += fmt.Sprintf(` WHERE prf.label = "%s"`, up.Profile)
	q += fmt.Sprintf(` AND u.reg = "%s" AND u.repo = "%s" AND u.tag = "%s"`, up.Reg, up.Repo, up.Tag)
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize,`
	q += ` type(r) AS manifestdataRelType`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, found, err := n.QueryOne(q)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if !found {
		return
	}
	m, err = n.ManifestDataFromResult(ctx, res)
	if err != nil {
		err = errors.Wrap(err, "failed to get manifest from result")
		return
	}
	m.Upstream = up
	return
}

func ManifestDataFromRecord(res neo4j.Result) (m types.ManifestData, err error) {
	r := res.Record()
	v, ok := r.Get("manifestDataID")
	if !ok {
		err = fmt.Errorf("no manifestDataID found in record: %v", r)
		return
	}
	m.ID = v.(int64)
	v, ok = r.Get("manifestDataDigest")
	if !ok {
		err = fmt.Errorf("no manifestDataDigest found in record: %v", r)
		return
	}
	m.Digest = v.(string)
	v, ok = r.Get("manifestDataMediaType")
	if !ok {
		err = fmt.Errorf("no manifestDataMediaType found in record: %v", r)
		return
	}
	m.MediaType = v.(string)
	v, ok = r.Get("manifestdataRelType")
	if !ok {
		err = fmt.Errorf("no manifestdataRelType found in record: %v", r)
		return
	}
	m.Type = v.(string)
	v, ok = r.Get("manifestDataSize")
	if !ok {
		err = fmt.Errorf("no manifestDataSize found in record: %v", r)
		return
	}
	m.Size = v.(int64)
	v, ok = r.Get("manifestDataLabel")
	if ok {
		m.Label = v.(string)
	}
	return
}

func (n Neo) ManifestDataFromResult(ctx context.Context, res types.Result) (m types.ManifestData, err error) {
	err = res.CheckKeys("manifestID", "manifestDigest", "manifestMediaType", "manifestSize")
	if err != nil {
		return
	}
	maniID, _ := res.GetInt64("manifestID")
	manifestDigest, _ := res.GetStr("manifestDigest")
	manifestMediaType, _ := res.GetStr("manifestMediaType")
	manifestSize, _ := res.GetInt64("manifestSize")
	manifestOrigin, _ := res.GetStr("manifestOrigin")
	manifestLabel, _ := res.GetStr("manifestLabel")
	manifestdataRelType, _ := res.GetStr("manifestdataRelType")
	// Get Annotations for ManifestData
	annos, err := n.GetAnnotationsByManifestDataID(ctx, maniID)
	if err != nil {
		err = errors.Wrap(err, "failed to get annotations for manifest data")
		return
	}
	m = types.ManifestData{
		ID:          maniID,
		Digest:      manifestDigest,
		MediaType:   manifestMediaType,
		Size:        manifestSize,
		Label:       manifestLabel,
		Origin:      manifestOrigin,
		Type:        manifestdataRelType,
		Annotations: annos,
	}
	return
}

func (n Neo) ManifestData_GetByCollectionTag(ctx context.Context, colID, colTagID int64) (manis []types.ManifestData, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":  "magazine/manifest_data.go",
		"func":     "ManifestData_GetByCollectionTag",
		"colID":    colID,
		"colTagID": colTagID,
	})
	ctx, span := tracer.Start(ctx, "ManifestData_GetByCollectionTag")
	defer span.End()
	l.Info("START")
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[r]-(m:Manifest)-[]-(u:Upstream)`
	q += fmt.Sprintf(` MATCH (c:Collection)-[:UPSTREAM]->(m)-[:CREATED]-(md:ManifestData)-[:USES]->(cfg:Config) WHERE id(c) = %d`, colID)
	q += fmt.Sprintf(` MATCH (t:Tag)-[:UPSTREAM]->(m) WHERE id(t) = %d`, colTagID)
	//q += ` MATCH (md)-[]-(prf:Profile)`
	q += ` RETURN type(r) AS manifestdataRelType,`
	q += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.label AS manifestDataLabel, md.mediaType AS manifestDataMediaType, md.size AS manifestDataSize,`
	q += UpstreamReturn + ` ,`
	//q += returnStatementProfile + ` ,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, cfg.mediaType AS configMediaType, cfg.size AS configSize`
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
	manis = make([]types.ManifestData, 0)
	for result.Next() {
		p, e := PlatformFromRecord(result)
		if e != nil {
			err = e
			return
		}
		c, e := ConfigFromRecord(result)
		if e != nil {
			err = e
			return
		}
		u, e := UpstreamFromRecord(result)
		if e != nil {
			err = e
			return
		}
		m, e := ManifestDataFromRecord(result)
		if e != nil {
			err = e
			return
		}
		/*
			prf, e := ProfileFromRecord(result)
			if e != nil {
				err = e
				return
			}
		*/
		anno, e := n.Labels_GetByConfigID(ctx, c.ID)
		if e != nil {
			err = e
			return
		}
		m.Annotations = anno
		c.Labels = anno
		m.Config = c
		m.Platform = p
		m.Upstream = u
		//m.Upstream.Profile = prf.Label
		//m.Profile = prf
		prfs, e := n.Profile_FromManifestDataID(ctx, m.ID)
		if e != nil {
			err = errors.Wrap(e, "failed to get profiles for manifest data")
			return
		}
		if len(prfs) == 0 {
			err = fmt.Errorf("no profiles found for manifest data: %d", m.ID)
			continue
		}
		for _, prf := range prfs {
			ok, e := n.ManifestData_CheckProfile(ctx, m, prf)
			if e != nil {
				l.Errorf("Failed to check profile: %v", err)
				return
			}
			msg := fmt.Sprintf("  %t > %d -> filtered md: %s", ok, prf.ID, m.Label)
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			l.Debug(msg)
			if ok {
				m.Upstream.Profile = prf.Label
				m.Profile = prf
				manis = append(manis, m)
			}
		}
	}
	return
}

func (n Neo) ManifestData_GetByManifestList(mlID int64) (manifests []types.ManifestData, err error) {
	// TODO: We'll get different types of ManifestData per Manifest - do we need to filter out the default (OCI manifest)
	//       - create a map afterwards based on collection name and type
	q := `MATCH (ml:ManifestList)-[:INCLUDES]-(m:Manifest)-[:IS]->(p:Platform) WHERE id(ml) = %d`
	q += ` MATCH (cfg:Config)<-[:USES]-(md:ManifestData)-[r]-(m)-[:UPSTREAM]->(u:Upstream)`
	q += ` RETURN type(r) AS manifestdataRelType,`
	q += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize,`
	q += ` ` + UpstreamReturn + `,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, cfg.mediaType AS configMediaType, cfg.size AS configSize`
	input := map[string]interface{}{}
	query := fmt.Sprintf(q, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, input)
	if err != nil {
		return
	}
	manifests = make([]types.ManifestData, 0)
	for result.Next() {
		p, e := PlatformFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get platform from record")
			return
		}
		c, e := ConfigFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get config from record")
			return
		}
		u, e := UpstreamFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get upstream from record")
			return
		}
		m, e := ManifestDataFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get manifest from record")
			return
		}
		m.Config = c
		m.Platform = p
		m.Upstream = u
		manifests = append(manifests, m)
	}
	return
}

func (n Neo) ManifestData_GetInsert(ctx context.Context, mani *types.ManifestData) (err error) {
	ctx, span := tracer.Start(ctx, "ManifestData_GetInsert")
	defer span.End()
	m, found, err := n.ManifestData_Get_ByDigest(ctx, mani.Digest)
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		mani.ID = m.ID
		return
	}
	maniID, err := n.createManifestData(ctx, *mani)
	if err != nil {
		logrus.Error(err)
		return
	}
	mani.ID = maniID
	return
}

func (n Neo) ManifestData_Insert(ctx context.Context, m types.ManifestData) (maniID int64, err error) {
	/******* Which ID are we going to return?
	1. ManifestID or ManifestDataID
	=> ManfifestData! The Manifest is just a stand-in that connectes the real ManifestData up to the Collection
	*/
	err = n.ManifestData_GetInsert(ctx, &m)
	if err != nil {
		return
	}
	maniID = m.ID
	return
}

func (n Neo) LinkManifestDataToProfile(ctx context.Context, mdID, prfID int64) error {
	_, span := tracer.Start(ctx, "LinkManifestDataToProfile")
	defer span.End()
	q := fmt.Sprintf(`MATCH (m:ManifestData),(prf:Profile) WHERE id(m) = %d AND id(prf) = %d`, mdID, prfID)
	q += ` MERGE (m)-[:UPLOADED_BY]->(prf)`
	span.AddEvent("query", trace.WithAttributes(
		attribute.String("query", q),
	))
	_, err := n.Execute(q)
	if err != nil {
		return err
	}
	return nil
}

func (n Neo) ManifestData_GetPushed_ByProfile(ctx context.Context, up types.Upstream, prf types.Profile) (m types.ManifestData, found bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestData_GetPushed_ByProfile")
	_ = ctx
	defer span.End()
	// FIXME: We need to filter the MDs based on CREATED and manipulations
	//        ->
	q := `MATCH (prf:Profile)<-[:UPLOADED_BY]-(md:ManifestData)-[r]-(m:Manifest)-[:UPSTREAM]->(u:Upstream)`
	q += fmt.Sprintf(` WHERE prf.label = "%s" AND u.reg = "%s" AND u.repo = "%s" AND u.tag = "%s"`, prf.Label, up.Reg, up.Repo, up.Tag)
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize,`
	q += ` type(r) AS manifestdataRelType`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, found, err := n.QueryOne(q)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if !found {
		return
	}
	m, err = n.ManifestDataFromResult(ctx, res)
	if err != nil {
		err = errors.Wrap(err, "failed to get manifest from result")
		return
	}
	m.Upstream = up
	return
}

func (n Neo) ManifestData_Delete(ctx context.Context, md types.ManifestData) (err error) {
	// TODO: Delete the ManifestData and all its relationships
	ctx, span := tracer.Start(ctx, "ManifestData_Delete")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("md.upstream").String(md.Upstream.String()),
		attribute.Key("md.label").String(md.Label),
		attribute.Key("md.id").Int64(md.ID),
	)
	q := fmt.Sprintf(`MATCH (md:ManifestData) WHERE id(md) = %d DETACH DELETE md`, md.ID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	_, err = n.Execute(q)
	if err != nil {
		err = errors.Wrap(err, "failed to delete manifest data")
		span.RecordError(err)
		return
	}
	return
}
