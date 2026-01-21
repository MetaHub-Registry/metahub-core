package neo

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
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
	manifestDataReturn                = `id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize, md.label AS manifestLabel`
	pkgNameMD                         = "neo/manifest_data.go"
	FailAnnoUp                        = "AnnotationUpdate failed"
	FailManiLinkAnno                  = "Manifest_LinkToAnnotation failed"
	FailAnnoCreate                    = "Annotation_Create failed"
	FailManifestDataLinkToLayer       = "ManifestDataLinkToLayer failed"
	FailManifestFromResult            = "failed to get manifest from result"
	FailManifestDataGetByUpstream     = "ManifestDataGetByUpstream failed"
	FailManifestDataNotFound          = "ManifestData not found"
	FailManifestDataGetByDigest       = "ManifestDataGetByDigest failed"
	FailManipuLayerLinkToManifestData = "ManipuLayer_LinkToManifestData failed"
	FailMediaTypeEmpty                = "mediaType is empty"
)

func (n Neo) ManifestDataUpdateDesc(ctx context.Context, maniDataID int64, maniDataV1 v1.Descriptor) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestData_GetAll",
		"maniDataID": maniDataID,
	})
	defer span.End()
	_ = ctx
	label := utils.CreateLabelFromDescriptor(maniDataV1)
	q := fmt.Sprintf(`MATCH (md:ManifestData) WHERE md.digest = "%s" RETURN id(md) AS mdID`, maniDataV1.Digest.String())
	utils.Debug(l, span, q)
	if res, found, err := n.QueryOne(q); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to query for manifest")
	} else if !found {
		q = fmt.Sprintf(`MATCH (md:ManifestData) WHERE id(md) = %d`, maniDataID)
		q += fmt.Sprintf(` SET md.digest = "%s", md.mediaType = "%s", md.size = %d, md.label = "%s"`, maniDataV1.Digest.String(), maniDataV1.MediaType, maniDataV1.Size, label)
		utils.Debug(l, span, q)
		if _, e := n.Execute(q); e != nil {
			return utils.ErrorHandler(l, span, e, "failed to update manifest")
		}
	} else {
		// TODO: Link Manifest to already existing ManifestDat
		oldManiDataID, err := res.GetInt64("mdID")
		if err != nil {
			return utils.ErrorHandler(l, span, err, "failed to get manifestID from result")
		}
		q = fmt.Sprintf(`MATCH (md:ManifestData) WHERE id(md) = %d DETACH DELETE md`, oldManiDataID)
		utils.Debug(l, span, q)
		if _, e := n.Execute(q); e != nil {
			return utils.ErrorHandler(l, span, e, "failed to delete old manifest")
		}
		return nil
	}
	return nil
}

func (n Neo) ManifestDataV1GetInsert(ctx context.Context, maniSiID int64, maniDataV1 v1.Descriptor) (maniDataID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataV1GetInsert",
	})
	defer span.End()
	maniDataID, found, err = n.ManifestDataV1Get(ctx, maniDataV1)
	if err != nil {
		err = errors.Wrap(err, "failed to get manifest by digest")
		return
	}
	if !found {
		mdDig := maniDataV1.Digest.String()
		utils.Info(l, span, "did not find ManifestData - Inserting new ManifestData")
		label := utils.CreateLabelFromDescriptor(maniDataV1)
		q := `CREATE (m:ManifestData {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(m) AS manifestID`
		query := fmt.Sprintf(q, mdDig, maniDataV1.MediaType, maniDataV1.Size, label)
		utils.Debug(l, span, query)
		res, e := n.Execute(query)
		if e != nil {
			err = errors.Wrap(e, "failed to create (m:ManifestData)")
			return
		}
		maniDataID, e = res.GetInt64("manifestID")
		if e != nil {
			err = errors.Wrap(e, "failed to get manifestID from result")
			return
		}
	}
	NodeRel := "CREATED"
	// Link to manifest standin
	q := `MATCH (md:ManifestData), (m:Manifest) WHERE id(md) = %d AND id(m) = %d`
	q += fmt.Sprintf(` MERGE (m)-[:%s]->(md)`, NodeRel)
	query := fmt.Sprintf(q, maniDataID, maniSiID)
	_, err = n.Execute(query)
	if err != nil {
		err = errors.Wrapf(err, "failed to create (m)-[:%s]->(md)", NodeRel)
		return
	}
	// Link ManifestData to Upstream
	if upID, upFound, e := n.UpstreamGetIDByManifestID(ctx, maniDataID); e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get upstreamID by manifestID")
		return
	} else if upFound {
		e := n.ManifestLinkToUpstream(ctx, maniDataID, upID)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to link manifest to upstream")
			return
		}
	}
	return
}

func (n Neo) ManifestDataV1Get(ctx context.Context, maniDataV1 v1.Descriptor) (maniDataID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataV1Get",
	})
	defer span.End()
	q := `MATCH (m:ManifestData) WHERE m.digest = "%s" AND m.mediaType = "%s"`
	q += ` RETURN id(m) AS manifestID`
	query := fmt.Sprintf(q, maniDataV1.Digest.String(), maniDataV1.MediaType)
	utils.Debug(l, span, query)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return
	} else if !found {
		utils.Warn(l, span, "Not able to found manifestData for digest: %s", maniDataV1.Digest.String())
		return
	}
	maniDataID, err = res.GetInt64("manifestID")
	if err != nil {
		err = errors.Wrap(err, "failed to get manifestID from result while getting manifest by digest and mediaType")
		return
	}
	utils.Info(l, span, "Found maniDataID: %d", maniDataID)
	return
}

func (n Neo) ManifestDataGetByProfileMediaType(ctx context.Context, mID, prfID int64, mediaTypes []string) (mdID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestDataGetByProfileMediaType",
		"prfID":      prfID,
		"maniID":     mID,
		"mediaTypes": strings.Join(mediaTypes, ","),
	})
	defer span.End()
	_ = ctx
	q := `MATCH (prf:Profile)<-[]-(md:ManifestData)-[]-(m:Manifest)`
	q += fmt.Sprintf(` WHERE id(m) = %d AND id(prf) = %d AND md.mediaType IN ["%s"]`, mID, prfID, strings.Join(mediaTypes, `","`))
	q += ` RETURN id(md) AS manifestID`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		l.Warn("Not able to find manifestData for manifestID: ", mID)
		return
	}
	mdID, err = res.GetInt64("manifestID")
	l.Debugf("%s -> maniID: %d", q, mdID)
	return
}

func (n Neo) ManifestData_GetParsedNodeRel_ByMediaType(ctx context.Context, msID int64, mediaType string) (nr types.NodeRel, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNameMD,
		"func":      "ManifestData_GetParsedNodeRel_ByMediaType",
		"mani.ID":   msID,
		"mediaType": mediaType,
	})
	defer span.End()
	_ = ctx
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

func (n Neo) ManifestDataGetRelIDByMediaType(ctx context.Context, msID int64, mediaTypes []string, relType string) (maniID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestDataGetRelIDByMediaType",
		"mani.ID":    msID,
		"mediaTypes": strings.Join(mediaTypes, ","),
		"relType":    relType,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[:%s]-(m:Manifest) WHERE id(m) = %d AND md.mediaType IN ["%s"]`, relType, msID, strings.Join(mediaTypes, `","`))
	q += ` RETURN id(md) AS manifestID`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		l.Warn(q)
		l.Warn("Not able to found manifestData for manifestID: ", msID)
		return
	}
	l.Warn(q)
	maniID, err = res.GetInt64("manifestID")
	if err != nil {
		err = errors.Wrap(err, "failed to get manifestID from result while getting manifest by digest and mediaType")
		return
	}
	l.Debugf("%s -> maniID: %d", q, maniID)
	return
}

func (n Neo) ManifestDataGetIDByMediaType(ctx context.Context, msID int64, mediaType string) (maniDataID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNameMD,
		"func":      "ManifestDataGetIDByMediaType",
		"mani.ID":   msID,
		"mediaType": mediaType,
	})
	defer span.End()
	_ = ctx
	q := `MATCH (md:ManifestData)-[]-(m:Manifest) WHERE id(m) = %d AND md.mediaType = "%s"`
	q += ` RETURN id(md) AS manifestID`
	query := fmt.Sprintf(q, msID, mediaType)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		l.Debug(query)
		l.Warn("Not able to found manifestData for manifestID: ", msID)
		return
	}
	maniDataID, err = res.GetInt64("manifestID")
	l.Debugf("%s -> maniID: %d", query, maniDataID)
	return
}

func (n Neo) ManifestData_GetAll(ctx context.Context) (manis []types.ManifestData, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{

		"package": pkgNameMD,
		"func":    "ManifestData_GetAll",
	})
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[r]-(m:Manifest)-[]-(u:Upstream)`
	q += ` MATCH (m)-[]-(md:ManifestData)-[rel]-(cfg:Blob) WHERE TYPE(rel) STARTS WITH "docker_container_image" OR TYPE(rel) STARTS WITH "oci_image_config"`
	q += ` RETURN type(r) AS manifestdataRelType,`
	q += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(m) AS manifestID,`
	q += ` id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.mediaType AS manifestDataMediaType, md.size AS manifestDataSize,`
	q += ` ` + UpstreamReturn + `,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, TYPE(rel) AS configMediaType, cfg.size AS configSize`
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
		c, e := n.ConfigFromRecord(ctx, result)
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
		m.Layers, err = n.GetLayersForManifestDataID(ctx, m.ID)
		if err != nil {
			return
		}
		// Add manifest to list
		manis = append(manis, m)
	}
	return
}

func (n Neo) ManifestData_GetAllByManifestID(ctx context.Context, msID int64) (manis []types.ManifestData, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestData_GetAllByManifestID",
		"maniID":  msID,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (ms:Manifest)-[r]->(m:ManifestData) WHERE id(ms) = %d`, msID)
	q += ` MATCH (m)-[cfgRel]-(cfg:Blob) WHERE cfg.type = "config"`
	q += ` RETURN DISTINCT id(m) AS manifestID, m.digest AS manifestDigest, m.mediaType AS manifestMediaType, m.size AS manifestSize,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, TYPE(cfgRel) AS configMediaType, cfg.size AS configSize,`
	q += ` type(r) AS manifestdataRelType`
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	mdDigs := set.NewStringSet()
	manis = make([]types.ManifestData, 0)
	for _, r := range res {
		m, _, e := n.ManifestDataFromResult(ctx, r)
		if e != nil {
			err = e
			return
		}
		// FIXME: manifestdataRelType might be connected twice: PARSED, PULLED_BY
		if mdDigs.Contains(m.Digest) {
			continue
		}
		c, e := n.ConfigFromResult(ctx, r)
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
		m.Layers, err = n.GetLayersForManifestDataID(ctx, m.ID)
		if err != nil {
			return
		}
		manis = append(manis, m)
		mdDigs.Add(m.Digest)
	}
	return
}

func (n Neo) ManifestData_GetAllByManifestID_WithProfile(ctx context.Context, msID, prfID int64) (manis []types.ManifestData, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestData_GetAllByManifestID_WithProfile",
		"maniID":  msID,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (ms:Manifest)-[r]->(m:ManifestData) WHERE id(ms) = %d`, msID)
	q += ` MATCH (m)-[cfgRel]-(cfg:Blob) WHERE cfg.type = "config"`
	q += ` RETURN id(m) AS manifestID, m.digest AS manifestDigest, m.mediaType AS manifestMediaType, m.size AS manifestSize,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, TYPE(cfgRel) AS configMediaType, cfg.size AS configSize,`
	q += ` type(r) AS manifestdataRelType`
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	prf, found, e := n.ProfileGetByID(ctx, prfID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "ProfileGetByID")
		return
	} else if !found {
		err = utils.NewError(l, span, "profile not found: %d", prfID)
		return
	}
	manis = make([]types.ManifestData, 0)
	for _, r := range res {
		m, _, e := n.ManifestDataFromResult(ctx, r, prf)
		if e != nil {
			err = e
			return
		}
		c, e := n.ConfigFromResult(ctx, r)
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
		m.Layers, err = n.GetLayersForManifestDataID(ctx, m.ID)
		if err != nil {
			return
		}
		manis = append(manis, m)
	}
	return
}

func (n Neo) ManifestData_LinkToAnnotation(ctx context.Context, maniID, annotationID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgNameMD,
		"func":         "ManifestData_LinkToAnnotation",
		"maniID":       maniID,
		"annotationID": annotationID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (m:ManifestData), (a:Annotation) WHERE id(m) = %d AND id(a) = %d`, maniID, annotationID)
	q += ` MERGE (m)-[:HAS_ANNOTATION]->(a)`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	_, err = n.Execute(q)
	if err != nil {
		return
	}
	// TODO: Check if annotation is linked to a rule which has all filters rules satisfied

	return
}

func (n Neo) ManifestData_LinkToOCIv1Config(ctx context.Context, maniID int64, cfg types.Config) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestData_LinkToOCIv1Config",
		"maniID":  maniID,
		"cfgID":   cfg.ID,
	})
	defer span.End()
	_ = ctx
	if cfg.MediaType == "" {
		cfg.MediaType = v1.MediaTypeImageConfig
	}
	mt := utils.ShortenMediaType(cfg.MediaType)
	q := fmt.Sprintf(`MATCH (m:ManifestData), (c:Blob) WHERE id(m) = %d AND id(c) = %d MERGE (m)-[:%s]->(c)`, maniID, cfg.ID, mt)
	utils.Debug(l, span, q)
	_, err = n.Execute(q)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	return
}

func (n Neo) ManifestDataLinkToOCIv1Layer(ctx context.Context, maniID int64, ld types.LayerDesc) (err error) {
	shortMT := utils.ShortenMediaType(ld.Descriptor.MediaType)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNameMD,
		"func":      "ManifestDataLinkToOCIv1Layer",
		"maniID":    maniID,
		"layerID":   ld.LayerID,
		"mediaType": shortMT,
	})
	defer span.End()
	_ = ctx
	if ld.Descriptor.MediaType == "" {
		return utils.NewError(l, span, FailMediaTypeEmpty)
	}
	// Check if blob is already linked
	existingRelTypes := []string{}
	qCheck := fmt.Sprintf(`MATCH (m:ManifestData)-[rel]->(l:Blob) WHERE id(m) = %d AND id(l) = %d AND l.type = "layer"`, maniID, ld.LayerID)
	qCheck += ` RETURN TYPE(rel) AS relType`
	utils.Debug(l, span, qCheck)
	res, err := n.Query(qCheck)
	if err != nil {
		return
	}
	for _, r := range res {
		relType, e := r.GetStr("relType")
		if e != nil {
			return e
		}
		existingRelTypes = append(existingRelTypes, relType)
	}
	if reflect.DeepEqual(existingRelTypes, []string{shortMT}) {
		utils.Info(l, span, "Layer already linked with correct mediaType: %s", shortMT)
		return
	} else if len(existingRelTypes) > 0 {
		// Remove old relations
		for _, rt := range existingRelTypes {
			utils.Info(l, span, "Removing old relation type: %s", rt)
			qDel := fmt.Sprintf(`MATCH (m:ManifestData)-[rel:%s]->(l:Blob) WHERE id(m) = %d AND id(l) = %d AND l.type = "layer" DELETE rel`, rt, maniID, ld.LayerID)
			utils.Debug(l, span, qDel)
			_, err = n.Execute(qDel)
			if err != nil {
				return
			}
		}
	}
	// Create new relation
	q := fmt.Sprintf(`MATCH (m:ManifestData), (l:Blob) WHERE id(m) = %d AND id(l) = %d AND l.type = "layer" MERGE (m)-[:%s]->(l)`, maniID, ld.LayerID, shortMT)
	utils.Debug(l, span, q)
	_, err = n.Execute(q)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	return
}

func (n Neo) ManifestData_AddAnnotations(ctx context.Context, maniID int64, as *types.Annotations) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestData_AddAnnotations",
		"maniID":  maniID,
	})
	defer span.End()
	for k, a := range as.Data {
		logrus.Warnf("adding annotation %s=%s", k, a.Value)
		aID, e := n.Annotation_GetInsert(ctx, a)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to add annotation")
		}
		a.ID = aID
		e = n.ManifestData_LinkToAnnotation(ctx, maniID, aID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to link annotation")
		}
	}
	return
}

func (n Neo) ManifestDataForManipuLayerID(ctx context.Context, mplID int64) (maniData types.ManifestData, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataForManipuLayerID",
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:ManifestData)<-[]-(mpl:ManipuLayer) WHERE id(mpl) = %d RETURN %s`, mplID, manifestDataReturn)
	utils.Debug(l, span, q)
	if res, found, err := n.QueryOne(q); err != nil {
		return maniData, utils.ErrorHandler(l, span, err, "failed to query manifest data")
	} else if !found {
		return maniData, utils.NewError(l, span, "manifest data not found connected to manipuLayer")
	} else {
		if md, ok, err := n.ManifestDataFromResult(ctx, res); err != nil {
			return maniData, utils.ErrorHandler(l, span, err, "failed to create ManifestData from result")
		} else if !ok {
			return maniData, utils.NewError(l, span, "manifestData from result returned !ok: No profile connection?")
		} else {
			return md, nil
		}
	}
}

func (n Neo) ManifestDataGetTypesByRequest(ctx context.Context, req types.Request) (res types.ManifestData, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataGetTypesByRequest",
		"host":    req.Host,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  req.Digest,
		"profile": req.Profile,
	})
	defer span.End()
	if req.Digest != "" {
		utils.Debug(l, span, "getting manifest data by digest")
		return n.ManifestDataGetByDigest(ctx, req.Digest)
	} else if req.Tag != "" {
		up := types.Upstream{
			Reg:     req.Host,
			Repo:    req.Repo,
			Tag:     req.Tag,
			Profile: req.Profile,
		}
		return n.ManifestDataGetByUpstream(ctx, up)
	}
	return
}

func (n Neo) ManifestData_GetID_ByDigest(ctx context.Context, dig string) (maniID int64, found bool, err error) {
	m, found, err := n.ManifestDataGetByDigest(ctx, dig)
	if err != nil || !found {
		return
	}
	maniID = m.ID
	return
}

func (n Neo) ManifestData_GetByRule(ctx context.Context, rule types.Rule) (mds []types.ManifestData, err error) {
	ctx, span := tracer.Start(ctx, "ManifestData_GetByRule")
	defer span.End()
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
	ctx, span := tracer.Start(ctx, "ManifestData_GetByPlatformRule")
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (r:Rule)-[%s]->(p:Platform)<-[:IS]-(m:Manifest)-[:CREATED]-(md:ManifestData) WHERE id(r) = %d`, rel, rule.ID)
	q += ` MATCH (u:Upstream)-[]-(m)`
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize, md.label AS manifestLabel,`
	q += ` id(p) AS platformID, p.os AS platformOS, p.arch AS platformArch, p.variant AS platformVariant,`
	q += ` ` + UpstreamReturn

	//fmt.Printf("query: \n%s\n", q)
	res, err := n.Query(q)
	for _, r := range res {
		m, _, e := n.ManifestDataFromResult(ctx, r)
		if e != nil {
			err = errors.Wrap(e, FailManifestFromResult)
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

func (n Neo) ManifestDataCheckProfile(ctx context.Context, md types.ManifestData, prf types.Profile) (ok bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgNameMD,
		"func":         "ManifestDataCheckProfile",
		"md.label":     md.Label,
		"profile.name": prf.Name,
	})
	_ = ctx
	defer span.End()
	if ok, err := n.ManifestDataCheckProfileCreationRelationship(ctx, md, prf); err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to check profile creation")
	} else {
		if !ok {
			return false, nil
		}
	}
	if ok, err := n.ManifestDataCheckProfileAnnotations(ctx, md, prf); err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to check profile annotations")
	} else {
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// ManifestDataCheckProfileCreationRelationship checks if the manifest data is created by the profile
func (n Neo) ManifestDataCheckProfileCreationRelationship(ctx context.Context, md types.ManifestData, prf types.Profile) (ok bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgNameMD,
		"func":         "ManifestDataCheckProfileCreationRelationship",
		"md.label":     md.Label,
		"profile.name": prf.Name,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (md:ManifestData)-[r:CREATED_BY]->(p:Profile)`
	q += fmt.Sprintf(` WHERE id(md) = %d AND id(p) = %d RETURN count(r) AS relCount`, md.ID, prf.ID)
	utils.Debug(l, span, q)
	res, _, err := n.QueryOne(q)
	if err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to QueryOne")
	}
	cnt, err := res.GetInt("relCount")
	if err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to get count annotations")
	}
	utils.Debug(l, span, "relCount: %d", cnt)
	ok = cnt == 1
	return
}

func (n Neo) ManifestDataCheckProfileAnnotations(ctx context.Context, md types.ManifestData, prf types.Profile) (ok bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgNameMD,
		"func":         "ManifestDataCheckProfileAnnotations",
		"md.label":     md.Label,
		"profile.name": prf.Name,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
	q += fmt.Sprintf(` WHERE id(md) = %d AND id(p) = %d RETURN count(a) AS countAnnotations`, md.ID, prf.ID)
	utils.Debug(l, span, q)
	res, _, err := n.QueryOne(q)
	if err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to QueryOne")
	}
	cnt, err := res.GetInt("countAnnotations")
	if err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to get count annotations")
	}
	utils.Debug(l, span, "countAnnotations: %d", cnt)
	ok = cnt == 0
	return
}

func (n Neo) ManifestData_GetByAnnotationRule(ctx context.Context, rule types.Rule) (mds []types.ManifestData, err error) {
	ctx, span := tracer.Start(ctx, "ManifestData_GetByAnnotationRule")
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (u:Upstream)-[]-(m:Manifest)-[:CREATED]-(md:ManifestData)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule) WHERE id(r) = %d`, rel, rule.ID)
	q += ` MATCH (m)-[:IS]->(p:Platform)`
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize, md.label AS manifestLabel,`
	q += ` id(p) AS platformID, p.os AS platformOS, p.arch AS platformArch, p.variant AS platformVariant,`
	q += ` ` + UpstreamReturn
	//fmt.Printf("query: \n%s\n", q)
	res, err := n.Query(q)
	for _, r := range res {
		m, _, e := n.ManifestDataFromResult(ctx, r)
		if e != nil {
			err = errors.Wrap(e, FailManifestFromResult)
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

func (n Neo) ManifestDataLinkToConfig(ctx context.Context, maniID int64, cfg types.Config) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/,anifest_data.go",
		"func":    "ManifestDataLinkToConfig",
		"maniID":  maniID,
		"cfg.ID":  cfg.ID,
	})
	_, span := tracer.Start(ctx, "ManifestDataLinkToConfig")
	defer span.End()
	if cfg.MediaType == "" {
		err = fmt.Errorf("%s", FailMediaTypeEmpty)
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	mt := utils.ShortenMediaType(cfg.MediaType)
	q := fmt.Sprintf(`MATCH (m:ManifestData),(c:Blob) WHERE id(m) = %d AND id(c) = %d`, maniID, cfg.ID)
	q += fmt.Sprintf(` MERGE (m)-[:%s]->(c)`, mt)
	q += fmt.Sprintf(` SET c.mediaType = "%s", c.type = "config"`, cfg.MediaType)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(q, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:ManifestData)-[:%s]->(c:Blob): %v", mt, err)
		return
	}
	return
}

func (n Neo) ManifestDataLinkToLayer(ctx context.Context, maniID int64, layer types.Layer, idx int) (err error) {
	shortMT := utils.ShortenMediaType(layer.MediaType)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/,anifest_data.go",
		"func":    "ManifestDataLinkToLayer",
		"maniID":  maniID,
		"layerID": layer.ID,
		"idx":     idx,
		"shortMT": shortMT,
	})
	defer span.End()
	if layer.MediaType == "" {
		return utils.NewError(l, span, FailMediaTypeEmpty)
	}
	_ = ctx
	q := fmt.Sprintf(`MATCH (m:ManifestData),(l:Blob) WHERE id(m) = %d AND id(l) = %d`, maniID, layer.ID)
	q += fmt.Sprintf(` MERGE (m)-[:LayerLink {order: %d, mediaType: "%s"}]->(l)`, idx, layer.MediaType)
	q += fmt.Sprintf(` SET l.mediaType = "%s", l.type = "layer"`, layer.MediaType)
	utils.Debug(l, span, q)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(q, map[string]interface{}{})
	if err != nil {
		err = errors.Wrapf(err, "failed to create link")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())

		return
	}
	return
}

func (n Neo) ManifestDataRemove(ctx context.Context, req types.Request) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataRemove",
	})
	defer span.End()
	if mani, found, err := n.ManifestDataGetTypesByRequest(ctx, req); err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to get manifest by id")
	} else if !found {
		return found, err
	} else if err = n.ManifestDataDeleteByDigest(ctx, mani.Digest); err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to delete manifest")
	}
	return true, nil
}

func (n Neo) ManifestDataInsert(ctx context.Context, maniData *types.ManifestData) (mdId int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataInsert",
		"repo":    maniData.Upstream.Repo,
		"tag":     maniData.Upstream.Tag,
		"digest":  maniData.Digest,
	})
	defer span.End()
	if fMD, found, err := n.ManifestDataGetByDigest(ctx, maniData.Digest); err != nil {
		err = utils.ErrorHandler(l, span, err, "ManifestDataGetByDigest failed")
		return 0, err
	} else if found {
		utils.Debug(l, span, "manifestData already exists: %s", fMD.Label)
		// TODO: check if manifestData and FMD are exactly the same
		maniData = &fMD
		return fMD.ID, nil
	}
	mdID, err := n.createManifestData(ctx, *maniData)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "createManifestData failed")
		return 0, err
	}
	maniData.ID = mdID
	return mdID, nil
}

func (n Neo) createManifestData(ctx context.Context, mani types.ManifestData) (maniID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "createManifestData",
		"repo":    mani.Upstream.Repo,
		"tag":     mani.Upstream.Tag,
		"digest":  mani.Digest,
	})
	defer span.End()
	label := utils.CreateLabelFromTypeAndDigest(mani.MediaType, mani.Digest)
	q := fmt.Sprintf(`CREATE (m:ManifestData {digest: "%s", mediaType: "%s", size: %d, label: "%s"`, mani.Digest, mani.MediaType, mani.Size, label)
	if mani.ArtifactType != "" {
		q += fmt.Sprintf(`, artifactType: "%s"`, mani.ArtifactType)
	}
	q += `})`
	q += ` RETURN id(m) AS manifestID`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Execute(q)
	if err != nil {
		logrus.Errorf("failed to create manifest: %v", err)
		return
	}
	maniID, err = res.GetInt64("manifestID")
	if err != nil {
		logrus.Errorf("failed to create manifestID from result")
		fmt.Printf("Query: \n%s\n", q)
		return
	}
	// Link Config
	if mani.Config.Digest != "" {
		cfg, found, err := n.ConfigGetByDigest(ctx, mani.Config.Digest)
		if err != nil {
			err = errors.Wrap(err, "failed to get config id")
			return maniID, err
		}
		if !found {
			err = fmt.Errorf("config w/ digest '%s' not found", utils.ShortenDigest(mani.Config.Digest))
			return maniID, err
		}
		cfg.MediaType = mani.Config.MediaType
		err = n.ManifestDataLinkToConfig(context.Background(), maniID, cfg)
		if err != nil {
			err = errors.Wrap(err, "failed to link manifest to config")
			return maniID, err
		}
	}
	// Link Layers
	for idx, layer := range mani.Layers {
		lyr, found, err := n.LayerGetByDigest(ctx, layer.Digest)
		layer.ID = lyr.ID
		if err != nil {
			err = errors.Wrapf(err, "failed to get layer%d id", idx)
			return maniID, err
		}
		if !found {
			err = errors.New("layer not found")
			return maniID, err
		}
		err = n.ManifestDataLinkToLayer(ctx, maniID, layer, idx)
		if err != nil {
			err = errors.Wrap(err, "failed to link manifest to layer")
			return maniID, err
		}
	}
	// Link Upstream (if present)
	if !reflect.ValueOf(mani.Upstream).IsZero() {
		if mani.Upstream.Digest.String() != "" {
			if upID, found, e := n.UpstreamGet(ctx, mani.Upstream); e != nil {
				return maniID, utils.ErrorHandler(l, span, e, "failed to get upstream manifest")
			} else if !found {
				return maniID, utils.NewError(l, span, "upstream manifest not found")
			} else {
				mani.Upstream.ID = upID
			}
		}
		if err = n.ManifestDataLinkToUpstream(ctx, maniID, mani.Upstream.ID); err != nil {
			return maniID, utils.ErrorHandler(l, span, err, "failed to link manifest to upstream")
		}
	}

	return
}

func (n Neo) ManifestData_GetExactly(ctx context.Context, mani types.ManifestData) (maniID int64, success bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/manifest_data.go",
		"func":    "ManifestData_GetExactly",
	})
	ctx, span := tracer.Start(ctx, "ManifestData_GetExactly")
	_ = ctx
	defer span.End()
	q := `MATCH (m:ManifestData) WHERE m.digest = "%s" AND m.mediaType = "%s" AND m.size = %d RETURN id(m) AS maniID`
	query := fmt.Sprintf(q, mani.Digest, mani.MediaType, mani.Size)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", query),
	))
	l.Debug(query)
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

func (n Neo) ManifestDataDirectGetByDigest(ctx context.Context, dig string) (m types.ManifestData, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataDirectGetByDigest",
		"digest":  dig,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (md:ManifestData) WHERE md.digest = "%s"`, dig)
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize`
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to query for manifest data")
		return
	}
	if !found {
		utils.Debug(l, span, "manifest data not found")
		return
	}
	m, _, e = n.ManifestDataFromResult(ctx, res)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailManifestFromResult)
		return
	}
	if up, relType, found, err := n.UpstreamGetByManifestDataID(ctx, m.ID); err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get upstream by manifest data id")
		return m, found, err

	} else if found {
		m.Upstream = up
		m.Type = relType
	}
	return
}

func (n Neo) ManifestDataGetByDigest(ctx context.Context, dig string) (m types.ManifestData, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataGetByDigest",
		"digest":  dig,
	})
	defer span.End()
	m, found, err = n.ManifestDataDirectGetByDigest(ctx, dig)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get manifest data directly")
		return
	}
	if found {
		utils.Debug(l, span, "found direct (ManifestData)->(Upstream): %s", utils.ShortenDigest(m.Digest))
		return
	}
	m, found, err = n.ManifestDataIndirectGetByDigest(ctx, dig)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get manifest data indirectly")
		return
	}
	if found {
		utils.Debug(l, span, "found indirect (ManifestData)-[]-(Manifest)-[]->(Upstream): %s", utils.ShortenDigest(m.Digest))
		return
	}
	utils.Warn(l, span, "manifest data not found")
	return
}

func (n Neo) ManifestDataIndirectGetByDigest(ctx context.Context, dig string) (m types.ManifestData, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataIndirectGetByDigest",
		"digest":  dig,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[r]-(m:Manifest)-[]->(u:Upstream) WHERE md.digest = "%s"`, dig)
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize,`
	q += ` ` + UpstreamReturn + `,`
	q += ` type(r) AS manifestdataRelType`
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to query for manifest data")
		return
	}
	if !found {
		utils.Debug(l, span, "manifest data not found")
		return
	}
	m, _, e = n.ManifestDataFromResult(ctx, res)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailManifestFromResult)
		return
	}
	up, e := UpstreamFromResult(res)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get upstream from result")
		return
	}
	m.Upstream = up
	return
}

func (n Neo) ManifestDataGetByID(ctx context.Context, mdID int64) (m types.ManifestData, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    "neo/manifest_data.go",
		"func":       "ManifestDataGetByID",
		"maniDataID": mdID,
	})
	defer span.End()
	q := `MATCH (md:ManifestData)`
	q += fmt.Sprintf(` WHERE id(md) = %d`, mdID)
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize`
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to run query")
		return
	}
	if !found {
		return
	}
	m, _, err = n.ManifestDataFromResult(ctx, res)
	if err != nil {
		err = utils.ErrorHandler(l, span, e, "error  ManifestDataFromResult")
		return
	}
	return
}

// ManifestDataGetByUpstream returns a manifest data by upstream.
// It returns only one, which means it will error out if more are found.
// -> With a profile in up.Profile it will check for each candidate if it is connected to the profile.
func (n Neo) ManifestDataGetByUpstream(ctx context.Context, up types.Upstream) (m types.ManifestData, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataGetByUpstream",
		"reg":     up.Reg,
		"repo":    up.Repo,
		"tag":     up.Tag,
		"profile": up.Profile,
	})
	defer span.End()
	_ = ctx
	q := `MATCH (md:ManifestData)-[r]->(u:Upstream)`
	q += fmt.Sprintf(` WHERE u.repo = "%s" AND u.tag = "%s"`, up.Repo, up.Tag)
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize,`
	q += ` ` + UpstreamReturn + `,`
	q += ` type(r) AS manifestdataRelType`
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to run query")
		return
	}
	if !found {
		q := `MATCH (md:ManifestData)<-[r]-(m:Manifest)-[]->(u:Upstream)`
		q += fmt.Sprintf(` WHERE u.repo = "%s" AND u.tag = "%s"`, up.Repo, up.Tag)
		q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize,`
		q += ` ` + UpstreamReturn + `,`
		q += ` type(r) AS manifestdataRelType`
		utils.Debug(l, span, q)
		res, found, e = n.QueryOne(q)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "Failed to run query")
			return
		} else if !found {
			utils.Debug(l, span, "manifest data not found")
			return m, false, nil
		}
	}
	if up.Profile != "" {
		prf, prfFound, e := n.Profile_Get_ByLabel(ctx, up.Profile)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "Profile_Get_ByLabel failed")
			return m, false, err
		}
		if !prfFound {
			utils.NewError(l, span, "profile not found")
			return m, false, err
		}
		utils.Debug(l, span, "Found profile '%s' with ID %d", up.Profile, prf.ID)
		var ok bool
		m, ok, e = n.ManifestDataFromResult(ctx, res, prf)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, FailManifestFromResult)
			return
		}
		if !ok {
			utils.NewError(l, span, "manifest data is not connected to  profile '%s'", up.Profile)
			return m, false, nil
		}
		m.Label = fmt.Sprintf("%s/%s:%s@%s-%s", m.Upstream.Reg, m.Upstream.Repo, m.Upstream.Tag, prf.Label, m.Platform.Arch)
	} else {
		m, _, e = n.ManifestDataFromResult(ctx, res)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, FailManifestFromResult)
			return
		}
		m.Label = fmt.Sprintf("%s/%s:%s@%s", m.Upstream.Reg, m.Upstream.Repo, m.Upstream.Tag, m.Platform.Arch)
	}
	if up, e = UpstreamFromResult(res); e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get upstream from result")
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

func (n Neo) ManifestDataLinkToManipulayer(ctx context.Context, maniDataID, mplID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "neo.ManifestDataLinkToManipulayer",
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:ManifestData),(mpl:ManipuLayer) WHERE id(md) = %d AND id(mpl) = %d`, maniDataID, mplID)
	q += ` MERGE (md)<-[:USES]-(mpl)`
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailQueryExec)
		return err
	}
	return nil
}

func (n Neo) ManifestDataIsLinkedWithProfile(ctx context.Context, maniDataID int64, prf types.Profile) (linked bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "neo.ManifestDataIsLinkedWIthProfile",
	})
	defer span.End()
	_ = ctx
	// Case 1: Manifest is linked with profile through platform, annotation or the orginal manifestData (the parsed or pulled_by one)
	if maniID, found, err := n.ManifestGetIDByManifestData(ctx, maniDataID); err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get manifest id by manifest data")
		return false, err
	} else if found {
		_ = maniID
		prfs, err := n.ProfileFromManifestID(ctx, maniID)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to get profiles from manifest id")
			return false, err
		}
		for _, p := range prfs {
			if p.ID == prf.ID {
				utils.Debug(l, span, "found profile '%s' <> manifest '%d' <> manifestdata '%d' connection", prf.Label, maniID, maniDataID)
				return true, nil
			}
		}
		return linked, err
	}
	// TODO: Is the ManifestData linked with the profile?
	q := `MATCH (prf:Profile)-[]->(ml:ManifestList)-[:INCLUDES]->(md:ManifestData)`
	q += fmt.Sprintf(` WHERE id(prf)=%d AND id(md)=%d`, prf.ID, maniDataID)
	q += ` RETURN id(ml) AS manifestListID`
	utils.Debug(l, span, q)
	if _, linkedManifestList, e := n.QueryOne(q); e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to query for manifest list id")
		return
	} else {
		linked = linkedManifestList
	}
	// Second option is that the Manifest is an original and this has a connection
	q = `MATCH (prf:Profile)<-[r]-(md:ManifestData)`
	q += fmt.Sprintf(` WHERE id(prf)=%d AND id(md)=%d`, prf.ID, maniDataID)
	q += ` RETURN TYPE(r) AS rel`
	utils.Debug(l, span, q)
	if _, linkedPulled, e := n.QueryOne(q); e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to query for manifest list id")
		return
	} else {
		linked = linked || linkedPulled
	}
	return linked, nil
}

func (n Neo) ManifestDataFromResult(ctx context.Context, res types.Result, prfs ...types.Profile) (m types.ManifestData, ok bool, err error) {
	prf := types.Profile{ID: -1}
	prfProvided := len(prfs) > 0
	if prfProvided {
		prf = prfs[0]
	}
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/manifest_data.go",
		"func":    "neo.ManifestDataFromResult",
		"prfs":    prf.ID,
	})
	defer span.End()

	err = res.CheckKeys("manifestID", "manifestDigest", "manifestMediaType", "manifestSize")
	if err != nil {
		utils.ErrorHandler(l, span, err, "failed to check keys")
		return
	}
	maniID, _ := res.GetInt64("manifestID")
	manifestDigest, _ := res.GetStr("manifestDigest")
	manifestMediaType, _ := res.GetStr("manifestMediaType")
	manifestSize, _ := res.GetInt64("manifestSize")
	manifestOrigin, _ := res.GetStr("manifestOrigin")
	manifestLabel, _ := res.GetStr("manifestLabel")
	manifestdataRelType, _ := res.GetStr("manifestdataRelType")
	span.SetAttributes(
		attribute.Key("maniID").Int64(maniID),
		attribute.Key("manifestSize").Int64(manifestSize),
		attribute.Key("manifestDigest").String(manifestDigest),
		attribute.Key("manifestMediaType").String(manifestMediaType),
		attribute.Key("manifestOrigin").String(manifestOrigin),
		attribute.Key("manifestLabel").String(manifestLabel),
		attribute.Key("manifestdataRelType").String(manifestdataRelType),
	)
	if prfProvided {
		/// TODO: Check if it is connected
		linked, e := n.ManifestDataIsLinkedWithProfile(ctx, maniID, prf)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to check if manifest data is linked with profile")
			return
		}
		if !linked {
			err = utils.NewError(l, span, "manifest data is not linked with profile '%s'", prf.Label)
			return
		}
	}

	// Get Annotations for ManifestData
	annos, err := n.GetAnnotationsByManifestDataID(ctx, maniID)
	if err != nil {
		err = errors.Wrap(err, "failed to get annotations for manifest data")
		return
	}
	// Config
	cfg, cfgFound, err := n.ConfigGetByManiDataID(ctx, maniID)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get config by manifest data id")
		return
	}
	if !cfgFound {
		utils.Debug(l, span, "no config found for manifestData (might already have been deleted)")
	}
	lyrs, e := n.GetLayersForManifestDataID(ctx, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get layers for manifest data")
		return
	}
	up, e := UpstreamFromResult(res)
	if e != nil {
		utils.Debug(l, span, "failed to get upstream from result, but it's fine")
	}
	m = types.ManifestData{
		ID:          maniID,
		Digest:      manifestDigest,
		MediaType:   manifestMediaType,
		Size:        manifestSize,
		Label:       manifestLabel,
		Origin:      manifestOrigin,
		Type:        manifestdataRelType,
		Config:      cfg,
		Annotations: annos,
		Layers:      lyrs,
		Upstream:    up,
	}
	return m, true, nil
}

func (n Neo) ManifestData_GetByCollectionTag(ctx context.Context, colID, colTagID int64) (manis []types.ManifestData, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameMD,
		"func":     "ManifestData_GetByCollectionTag",
		"colID":    colID,
		"colTagID": colTagID,
	})
	defer span.End()
	utils.Info(l, span, "getting manifest data by collection and tag")
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Platform)-[r]-(m:Manifest)-[]-(u:Upstream)`
	q += fmt.Sprintf(` MATCH (c:Collection)-[:UPSTREAM]->(m)-[:CREATED]-(md:ManifestData)-[cfgRel]->(cfg:Blob) WHERE id(c) = %d AND cfg.type = "config"`, colID)
	q += fmt.Sprintf(` MATCH (t:Tag)-[:UPSTREAM]->(m) WHERE id(t) = %d`, colTagID)
	q += ` RETURN type(r) AS manifestdataRelType,`
	q += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.label AS manifestDataLabel, md.mediaType AS manifestDataMediaType, md.size AS manifestDataSize,`
	q += UpstreamReturn + ` ,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, TYPE(cfgRel) AS configMediaType, cfg.size AS configSize`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	//l.Debug(q)
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
		c, e := n.ConfigFromRecord(ctx, result)
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
		prfs, e := n.Profile_FromManifestDataID(ctx, m.ID)
		if e != nil {
			err = errors.Wrap(e, "failed to get profiles for manifest data")
			return
		}
		utils.Debug(l, span, "found %d profiles for manifest data", len(prfs))
		switch len(prfs) {
		case 0:
			m.Profile = "unknown"
		default:
			m.Profile = prfs[0].Label
		}
		if len(prfs) == 0 {
			err = fmt.Errorf("no profiles found for manifest data: %d", m.ID)
			continue
		}
		for _, prf := range prfs {
			ok, e := n.ManifestDataCheckProfile(ctx, m, prf)
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
				manis = append(manis, m)
			}
		}
	}
	return
}

func (n Neo) ManifestData_GetByManifestList(ctx context.Context, mlID int64) (manifests []types.ManifestData, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/manifest_data.go",
		"func":    "ManifestData_GetByManifestList",
		"mlID":    mlID,
	})
	_ = l
	ctx, span := tracer.Start(ctx, "ManifestData_GetByManifestList")
	defer span.End()
	// TODO: We'll get different types of ManifestData per Manifest - do we need to filter out the default (OCI manifest)
	//       - create a map afterwards based on collection name and type
	q := `MATCH (ml:ManifestList)-[:INCLUDES]-(m:Manifest)-[:IS]->(p:Platform) WHERE id(ml) = %d`
	q += ` MATCH (cfg:Blob)<-[:USES]-(md:ManifestData)-[r]-(m)-[:UPSTREAM]->(u:Upstream) WHERE cfg.type = "config"`
	q += ` RETURN type(r) AS manifestdataRelType,`
	q += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize,`
	q += ` ` + UpstreamReturn + `,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, cfg.mediaType AS configMediaType, cfg.size AS configSize`
	input := map[string]interface{}{}
	query := fmt.Sprintf(q, mlID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", query),
	))
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, input)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	manifests = make([]types.ManifestData, 0)
	for result.Next() {
		p, e := PlatformFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get platform from record")
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
		c, e := n.ConfigFromRecord(ctx, result)
		if e != nil {
			err = errors.Wrap(e, "failed to get config from record")
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
		u, e := UpstreamFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get upstream from record")
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
		m, e := ManifestDataFromRecord(result)
		if e != nil {
			err = errors.Wrap(e, "failed to get manifest from record")
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
		m.Config = c
		m.Platform = p
		m.Upstream = u
		manifests = append(manifests, m)
	}
	return
}

func (n Neo) ManifestDataGetInsert(ctx context.Context, mani *types.ManifestData) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataGetInsert",
		"repo":    mani.Upstream.Repo,
		"tag":     mani.Upstream.Tag,
		"digest":  mani.Digest,
		"profile": mani.Profile,
	})
	defer span.End()
	_ = l
	m, found, err := n.ManifestDataGetByDigest(ctx, mani.Digest)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to get manifest data by digest")
	}
	if found {
		utils.Debug(l, span, "Found ManifestData %s/%s#%s", mani.Upstream.Repo, mani.Upstream.Tag, mani.Digest)
		mani.ID = m.ID
		return
	}
	maniID, err := n.createManifestData(ctx, *mani)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to create manifest data")
	}
	mani.ID = maniID
	return
}

func (n Neo) ManifestDataLinkToProfile(ctx context.Context, maniDataID, profileID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestDataLinkToProfile",
		"maniDataID": maniDataID,
		"profileID":  profileID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:ManifestData),(p:Profile) WHERE id(md) = %d AND id(p) = %d`, maniDataID, profileID)
	q += ` MERGE (md)-[:CREATED_BY]->(p)`
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailQueryExec)
		return err
	}
	return nil
}

func (n Neo) ManifestDataLinkToUpstream(ctx context.Context, maniDataID, upID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestDataLinkToUpstream",
		"maniDataID": maniDataID,
		"upID":       upID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:ManifestData),(up:Upstream) WHERE id(md) = %d AND id(up) = %d`, maniDataID, upID)
	q += ` MERGE (md)-[:ORIGINAL]->(up)`
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailQueryExec)
		return err
	}
	return
}

func (n Neo) LinkManifestDataToProfile(ctx context.Context, mdID, prfID int64, rel string) error {
	_, span := tracer.Start(ctx, "LinkManifestDataToProfile")
	defer span.End()
	q := fmt.Sprintf(`MATCH (m:ManifestData),(prf:Profile) WHERE id(m) = %d AND id(prf) = %d`, mdID, prfID)
	q += fmt.Sprintf(` MERGE (m)-[:%s]->(prf)`, rel)
	span.AddEvent("query", trace.WithAttributes(
		attribute.String("query", q),
	))
	_, err := n.Execute(q)
	if err != nil {
		return err
	}
	return nil
}

func (n Neo) ManifestDataGetPushedByProfile(ctx context.Context, up types.Upstream, prf types.Profile) (m types.ManifestData, found bool, err error) {
	ctx, span := tracer.Start(ctx, "ManifestDataGetPushedByProfile")
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
	m, _, err = n.ManifestDataFromResult(ctx, res)
	if err != nil {
		err = errors.Wrap(err, FailManifestFromResult)
		return
	}
	m.Upstream = up
	return
}

func (n Neo) ManifestDataDeleteByDigest(ctx context.Context, dig string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataDeleteByDigest",
		"digest":  dig,
	})
	defer span.End()
	md, found, e := n.ManifestDataGetByDigest(ctx, dig)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailManifestDataGetByDigest)
	}
	if !found {
		return utils.NewError(l, span, types.FailManifestDataNotFound)
	}
	err = n.ManifestDataDelete(ctx, md)
	if err != nil {
		return utils.ErrorHandler(l, span, err, types.FailManifestDataDelete)
	}
	return
}

func (n Neo) ManifestDataUnlinkLayer(ctx context.Context, mdID, lyrID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataUnlinkLayer",
		"layerID": lyrID,
		"mdID":    mdID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (md:ManifestData)-[r]->(l:Blob)`
	q += fmt.Sprintf(` WHERE id(md) = %d AND id(l) = %d AND l.type = "layer"`, mdID, lyrID)
	q += ` DELETE r`
	utils.Debug(l, span, q)
	_, e := n.Execute(q)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailManifestDataUnlinkLayer)
	}
	return nil
}

func (n Neo) ManifestDataUnlinkConfig(ctx context.Context, mdID int64, cfgDig string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNameMD,
		"func":      "ManifestDataUnlinkConfig",
		"cfgDigest": utils.ShortenDigest(cfgDig),
		"mdID":      mdID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (md:ManifestData)-[r]->(c:Blob)`
	q += fmt.Sprintf(` WHERE id(md) = %d AND c.digest = "%s" AND c.type = "config"`, mdID, cfgDig)
	q += ` DELETE r`
	utils.Debug(l, span, "query to delete ManifestData<>Config relationship: %s", q)
	_, e := n.Execute(q)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailManifestDataUnlinkConfig)
	}
	return nil
}

func (n Neo) ManifestDataDelete(ctx context.Context, md types.ManifestData) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataDeleteByDigest",
		"digest":  md.Digest,
	})
	_ = ctx
	defer span.End()
	// Unlinke Config
	e := n.ManifestDataUnlinkConfig(ctx, md.ID, md.Config.Digest)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailManifestDataUnlinkConfig)
	}
	// Unlink Layers
	for _, lyr := range md.Layers {
		if e := n.ManifestDataUnlinkLayer(ctx, md.ID, lyr.ID); e != nil {
			return utils.ErrorHandler(l, span, e, types.FailManifestDataUnlinkConfig)
		}
	}
	// Get Manifest ID
	maniID, maniFound, e := n.ManifestGetIDByManifestDataID(ctx, md.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailManifestGetIDByManifestDataID)
	}
	if !maniFound {
		return utils.NewError(l, span, types.FailManifestGetIDByManifestDataID)
	}
	// Remove ManifestData
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
	// Remove Manifest if no ManifestData is linked
	if e := n.ManifestCleanUpByID(ctx, maniID); e != nil {
		return utils.ErrorHandler(l, span, e, types.FailManifestCleanUpByID)
	}
	return
}

func (n Neo) ManifestDataGetPulledByRepoTag(ctx context.Context, repo, tag string) (maniData types.ManifestData, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/manifest_data.go",
		"func":    "ManifestDataGetPulledByRepoTag",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()
	q := `MATCH (col:Collection)-[:UPSTREAM]->(m:Manifest)<-[:UPSTREAM]-(t:Tag)`
	q += ` MATCH (m)-[r]->(md:ManifestData)`
	q += `MATCH (p:Platform)-[]-(m)-[]-(u:Upstream)`
	q += fmt.Sprintf(` WHERE col.repo = "%s" AND t.name = "%s"`, repo, tag)
	q += ` RETURN id(md) AS manifestID, md.digest AS manifestDigest, md.mediaType AS manifestMediaType, md.size AS manifestSize,`
	q += ` type(r) AS manifestdataRelType`
	utils.Debug(l, span, q)
	if res, found, e := n.QueryOne(q); e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get manifest")
		return maniData, false, err
	} else if !found {
		err = utils.ErrorHandler(l, span, errors.New("manifestData not found"), "")
		return maniData, false, nil
	} else {
		maniData, _, e = n.ManifestDataFromResult(ctx, res)
		if e != nil {
			return maniData, false, utils.ErrorHandler(l, span, e, "failed to get manifestdata from result")
		}
	}
	utils.Debug(l, span, "Found manifestData: %s (%s)", maniData.Label, utils.ShortenDigest(maniData.Digest))
	return maniData, true, nil
}

func (n Neo) ManifestDataExists_ForTagProfile(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataExists_ForTagProfile",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"profile": up.Profile,
	})
	defer span.End()
	prf, found, e := n.Profile_Get_ByLabel(ctx, up.Profile)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Profile_Get_ByLabel failed")
		return "", false, err
	}
	if !found {
		err = utils.ErrorHandler(l, span, errors.New("profile not found"), "")
		return "", false, err
	}
	utils.Debug(l, span, "Found profile '%s' with ID %d", up.Profile, prf.ID)
	q := `MATCH (md:ManifestData)-[]-(m:Manifest)<-[:UPSTREAM]-(c:Collection)-[:HAS_TAG]->(t:Tag)-[:UPSTREAM]->(m)`
	q += fmt.Sprintf(` WHERE c.repo = "%s"`, up.Repo)
	q += fmt.Sprintf(` AND t.name = "%s"`, up.Tag)
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	q += `, md.mediaType AS manifestMediaType, id(md) AS manifestID, md.digest AS manifestDigest, md.size AS manifestSize`
	utils.Debug(l, span, "Query: %s", q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return "", false, err
	}
	if !found {
		utils.Debug(l, span, "manifestdata not found")
		return "", false, nil
	}
	maniData, ok, e := n.ManifestDataFromResult(ctx, res, prf)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get manifestData from result")
		return "", false, err
	} else if !ok {
		err = utils.NewError(l, span, "manifestData not found connected to profile")
		return "", false, err
	}
	colID, err := res.GetInt64("collectionID")
	if err != nil {
		err = utils.ErrorHandler(l, span, e, "colID could not be found")
		return "", false, err
	}
	utils.Debug(l, span, "found collection with id %d and manifestData with type '%s'", colID, maniData.MediaType)
	return maniData.MediaType, true, nil
}

func (n Neo) ManifestDataExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	defer span.End()
	prf, found, e := n.Profile_Get_ByLabel(ctx, up.Profile)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Profile_Get_ByLabel failed")
		return "", false, err
	}
	if !found {
		err = utils.ErrorHandler(l, span, errors.New("profile not found"), "")
		return "", false, err
	}
	utils.Debug(l, span, "Found profile '%s' with ID %d", up.Profile, prf.ID)
	q := `MATCH (md:ManifestData)-[]-(m:Manifest)<-[:UPSTREAM]-(c:Collection)-[:HAS_TAG]->(t:Tag)-[:UPSTREAM]->(m)`
	q += fmt.Sprintf(` WHERE c.repo = "%s"`, up.Repo)
	if up.Digest.String() != "" {
		q += fmt.Sprintf(` AND md.digest = "%s"`, up.Digest.String())
	} else {
		q += fmt.Sprintf(` AND t.name = "%s"`, up.Tag)
	}
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	q += `, md.mediaType AS manifestMediaType, id(md) AS manifestID, md.digest AS manifestDigest, md.size AS manifestSize`
	utils.Debug(l, span, "Query: %s", q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return "", false, err
	}
	if !found {
		utils.Debug(l, span, "manifestdata not found")
		return "", false, nil
	}
	maniData, ok, e := n.ManifestDataFromResult(ctx, res, prf)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get manifestData from result")
		return "", false, err
	} else if !ok {
		err = utils.NewError(l, span, "manifestData not found connected to profile")
		return "", false, err
	}
	colID, err := res.GetInt64("collectionID")
	if err != nil {
		err = utils.ErrorHandler(l, span, e, "colID could not be found")
		return "", false, err
	}
	utils.Debug(l, span, "found collection with id %d and manifestData with type '%s'", colID, maniData.MediaType)
	return maniData.MediaType, true, nil
}

func (n Neo) ManifestDataLinkToArtifactConfig(ctx context.Context, mdDigest string, cfg v1.Descriptor) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDataLinkToArtifactConfig",
	})
	defer span.End()
	// Find Config Blob
	b, found, err := n.BlobGetByDigest(ctx, string(cfg.Digest))
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to get blob by digest")
	}
	if !found {
		return utils.NewError(l, span, "config blob not found")
	}
	q := fmt.Sprintf(`MATCH (md:ManifestData),(b:Blob) WHERE md.digest = "%s" AND id(b) = %d`, mdDigest, b.ID)
	q += ` MERGE (md)-[r:USES]->(b)`
	q += fmt.Sprintf(` SET r.mediaType = "%s"`, cfg.MediaType)
	q += fmt.Sprintf(` SET b.mediaType = "%s"`, cfg.MediaType)
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailQueryExec)
		return err
	}
	return nil
}

func (n Neo) LinkManifestDataToSubject(ctx context.Context, mdDigest string, subId int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "LinkManifestDataToSubject",
	})
	defer span.End()
	// Find Config Blob
	q := fmt.Sprintf(`MATCH (md:ManifestData),(s:ManifestData) WHERE md.digest = "%s" AND id(s) = %d`, mdDigest, subId)
	q += ` MERGE (md)-[:SUBJECT_OF]->(s)`
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailQueryExec)
		return err
	}
	return nil
}
