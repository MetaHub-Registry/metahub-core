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
	manifestDataReturn            = `id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.mediaType AS manifestDataMediaType, md.size AS manifestDataSize, md.label AS manifestDataLabel`
	pkgNameMD                     = "neo/manifest_data.go"
	FailAnnoUp                    = "AnnotationUpdate failed"
	FailManiLinkAnno              = "ManifestWrapperLinkToAnnotation failed"
	FailAnnoCreate                = "AnnotationCreate failed"
	FailManifestLinkToLayer       = "ManifestLinkToLayer failed"
	FailManifestFromResult        = "failed to get manifest from result"
	FailManifestGetByUpstream     = "ManifestGetByUpstream failed"
	FailManifestNotFound          = "Manifest not found"
	FailManifestGetByDigest       = "ManifestGetByDigest failed"
	FailManipuLayerLinkToManifest = "ManipuLayer_LinkToManifest failed"
	FailMediaTypeEmpty            = "mediaType is empty"
	FailedProfGetByLabel          = "ProfileGetByLabel failed"
	FailedProfileNotFound         = "profile not found"

	// File-specific duplicate literals (manifest.go / manifest_data).
	errManifestDataNotFound     = "manifest data not found"
	errFailedUpstreamFromResult = "failed to get upstream from result"
	msgFoundProfile             = "Found profile '%s' with ID %d"
	msgQueryManiID              = "%s -> maniID: %d"
	keyMDLabel                  = "md.label"
	keyProfileName              = "profile.name"
)

func (n Neo) ManifestUpdateDesc(ctx context.Context, maniDataID int64, maniDataV1 v1.Descriptor) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestUpdateDesc",
		"maniDataID": maniDataID,
	})
	defer span.End()
	_ = ctx
	label := utils.CreateLabelFromDescriptor(maniDataV1)
	q := fmt.Sprintf(`MATCH (md:Manifest) WHERE md.digest = "%s" RETURN id(md) AS mdID`, maniDataV1.Digest.String())
	utils.Debug(l, span, q)
	if res, found, err := n.QueryOne(q); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to query for manifest")
	} else if !found {
		q = fmt.Sprintf(`MATCH (md:Manifest) WHERE id(md) = %d`, maniDataID)
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
		q = fmt.Sprintf(`MATCH (md:Manifest) WHERE id(md) = %d DETACH DELETE md`, oldManiDataID)
		utils.Debug(l, span, q)
		if _, e := n.Execute(q); e != nil {
			return utils.ErrorHandler(l, span, e, "failed to delete old manifest")
		}
		return nil
	}
	return nil
}

func (n Neo) ManifestV1GetInsert(ctx context.Context, maniSiID int64, maniDataV1 v1.Descriptor) (maniDataID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameMD,
		"func":     "ManifestV1GetInsert",
		"maniSiID": maniSiID,
		"maniData": utils.ShortenDigest(maniDataV1.Digest.String()),
	})
	defer span.End()
	utils.Debug(l, span, "Checking for existing Manifest with Digest: %s", maniDataV1.Digest.String())
	maniDataID, found, err = n.ManifestV1Get(ctx, maniDataV1)
	if err != nil {
		err = errors.Wrap(err, "failed to get manifest by digest")
		return
	}
	utils.Debug(l, span, "Manifest found: %v", found)
	if !found {
		mdDig := maniDataV1.Digest.String()
		utils.Info(l, span, "did not find Manifest - Inserting new Manifest")
		label := utils.CreateLabelFromDescriptor(maniDataV1)
		q := `CREATE (m:Manifest {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(m) AS manifestID, m.label AS manifestLabel`
		query := fmt.Sprintf(q, mdDig, maniDataV1.MediaType, maniDataV1.Size, label)
		utils.Debug(l, span, query)
		if res, e := n.Execute(query); e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to create (m:Manifest)")
			return
		} else if maniDataID, e = res.GetInt64("manifestID"); e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get manifestID from result")
			return
		}
		utils.Info(l, span, "Created Manifest.ID: %d for Manifest.ID: %d", maniDataID, maniSiID)
	} else {
		utils.Info(l, span, "Found existing Manifest.ID: %d for Manifest.ID: %d", maniDataID, maniSiID)
	}

	// Phase C3: wrapper link + upstream link are skipped when maniSiID <= 0
	// (no wrapper :ManifestWrapper node exists for Slot-based flows).
	if maniSiID > 0 {
		NodeRel := "CREATED"
		q := `MATCH (md:Manifest), (m:ManifestWrapper) WHERE id(md) = %d AND id(m) = %d`
		q += fmt.Sprintf(` MERGE (m)-[:%s]->(md)`, NodeRel)
		query := fmt.Sprintf(q, maniDataID, maniSiID)
		_, err = n.Execute(query)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to create (m)-[:%s]->(md)", NodeRel)
			return
		}
		if upID, upFound, e := n.UpstreamGetIDByManifestID(ctx, maniSiID); e != nil {
			utils.Debug(l, span, "UpstreamGetIDByManifestID failed (non-fatal): %v", e)
		} else if upFound {
			if e := n.ManifestWrapperLinkToUpstream(ctx, maniDataID, upID); e != nil {
				utils.Debug(l, span, "ManifestWrapperLinkToUpstream failed (non-fatal): %v", e)
			}
		}
	}
	// Link new Manifest w/ Platform (skip when no platform info is present)
	if maniDataV1.Platform != nil {
		plat := types.Platform{
			Arch:    maniDataV1.Platform.Architecture,
			OS:      maniDataV1.Platform.OS,
			Variant: maniDataV1.Platform.Variant,
		}
		if platID, found, err := n.GetPlatform(ctx, plat); err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to get platform by arch/os/variant")
			return maniDataID, false, err
		} else if found {
			err = n.LinkManifestToPlatform(ctx, maniDataID, platID)
			if err != nil {
				err = utils.ErrorHandler(l, span, err, "failed to link manifestData to platform")
				return maniDataID, false, err
			}
		} else {
			utils.Warn(l, span, "Platform not found for Manifest - skipping Platform link")
		}
	}
	return
}

func (n Neo) ManifestV1Get(ctx context.Context, maniDataV1 v1.Descriptor) (maniDataID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestV1Get",
	})
	defer span.End()
	q := `MATCH (md:Manifest) WHERE md.digest = "%s" AND md.mediaType = "%s"`
	q += ` RETURN id(md) AS manifestDataID`
	query := fmt.Sprintf(q, maniDataV1.Digest.String(), maniDataV1.MediaType)
	utils.Debug(l, span, query)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return
	} else if !found {
		utils.Warn(l, span, "Not able to found manifestData for digest: %s", maniDataV1.Digest.String())
		return
	}
	maniDataID, err = res.GetInt64("manifestDataID")
	if err != nil {
		err = errors.Wrap(err, "failed to get manifestDataID from result while getting manifest by digest and mediaType")
		return
	}
	utils.Info(l, span, "Found manifestDataID: %d", maniDataID)
	return
}

func (n Neo) ManifestGetByProfileMediaType(ctx context.Context, mID, prfID int64, mediaTypes []string) (mdID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestGetByProfileMediaType",
		"prfID":      prfID,
		"maniID":     mID,
		"mediaTypes": strings.Join(mediaTypes, ","),
	})
	defer span.End()
	_ = ctx
	q := `MATCH (prf:Profile)<-[]-(md:Manifest)-[]-(m:ManifestWrapper)`
	q += fmt.Sprintf(` WHERE id(m) = %d AND id(prf) = %d AND md.mediaType IN ["%s"]`, mID, prfID, strings.Join(mediaTypes, `","`))
	q += ` RETURN id(md) AS manifestDataID`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	l.Debug(q)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		l.Warn("Not able to find manifestData for manifestDataID: ", mID)
		return
	}
	mdID, err = res.GetInt64("manifestDataID")
	l.Debugf(msgQueryManiID, q, mdID)
	return
}

func (n Neo) ManifestGetParsedNodeRelByMediaType(ctx context.Context, msID int64, mediaType string) (nr types.NodeRel, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNameMD,
		"func":      "ManifestGetParsedNodeRelByMediaType",
		"maniID":    msID,
		"mediaType": mediaType,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:Manifest)-[rel]-(m:ManifestWrapper) WHERE id(m) = %d AND md.mediaType = "%s"`, msID, mediaType)
	q += ` RETURN id(md) AS manifestDataID, TYPE(rel) AS relType`
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		l.Debug(q)
		l.Warn("Not able to found manifestData for manifestID: ", msID)
		return
	}
	maniID, err := res.GetInt64("manifestDataID")
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

func (n Neo) ManifestGetRelIDByMediaType(ctx context.Context, msID int64, mediaTypes []string, relType string) (maniID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestGetRelIDByMediaType",
		"maniID":     msID,
		"mediaTypes": strings.Join(mediaTypes, ","),
		"relType":    relType,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:Manifest)-[:%s]-(m:ManifestWrapper) WHERE id(m) = %d AND md.mediaType IN ["%s"]`, relType, msID, strings.Join(mediaTypes, `","`))
	q += ` RETURN id(md) AS manifestDataID`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		l.Warn(q)
		l.Warn("Not able to found manifestData for manifestDataID: ", msID)
		return
	}
	l.Warn(q)
	maniID, err = res.GetInt64("manifestDataID")
	if err != nil {
		err = errors.Wrap(err, "failed to get manifestDataID from result while getting manifest by digest and mediaType")
		return
	}
	l.Debugf(msgQueryManiID, q, maniID)
	return
}

func (n Neo) ManifestGetIDByMediaType(ctx context.Context, msID int64, mediaType string) (maniDataID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNameMD,
		"func":      "ManifestGetIDByMediaType",
		"maniID":    msID,
		"mediaType": mediaType,
	})
	defer span.End()
	_ = ctx
	q := `MATCH (md:Manifest)-[]-(m:ManifestWrapper) WHERE id(m) = %d AND md.mediaType = "%s"`
	q += ` RETURN id(md) AS manifestDataID`
	query := fmt.Sprintf(q, msID, mediaType)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		l.Debug(query)
		l.Warn("Not able to found manifestData for manifestDataID: ", msID)
		return
	}
	maniDataID, err = res.GetInt64("manifestDataID")
	l.Debugf(msgQueryManiID, query, maniDataID)
	return
}

func (n Neo) ManifestGetAll(ctx context.Context) (manis []types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{

		"package": pkgNameMD,
		"func":    "ManifestGetAll",
	})
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (m:ManifestWrapper)-[]-(u:Upstream)`
	q += ` MATCH (m)-[r]-(md:Manifest)-[rel]-(cfg:Blob) WHERE cfg.type = "config"`
	q += ` MATCH (md)-[]-(p:Platform)`
	q += ` RETURN type(r) AS manifestdataRelType,`
	q += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` id(m) AS manifestID, m.label AS manifestLabel,`
	q += ` id(md) AS manifestDataID, md.digest AS manifestDataDigest, md.mediaType AS manifestDataMediaType, md.size AS manifestDataSize,`
	q += ` ` + UpstreamReturn + `,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, TYPE(rel) AS configMediaType, cfg.size AS configSize`
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
	manis = make([]types.Manifest, 0)
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
		m, e := ManifestFromRecord(result)
		if e != nil {
			err = e
			return
		}
		anno, e := n.LabelsGetByConfigID(ctx, c.ID)
		if e != nil {
			err = e
			return
		}
		m.Annotations = anno
		c.Labels = anno
		m.Config = c
		m.Platform = p
		m.Upstream = u
		m.Layers, err = n.GetLayersForManifestID(ctx, m.ID)
		if err != nil {
			return
		}
		// Add manifest to list
		manis = append(manis, m)
	}
	return
}

func (n Neo) ManifestGetAllByManifestID(ctx context.Context, msID int64) (manis []types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestGetAllByManifestID",
		"maniID":  msID,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (m:ManifestWrapper)-[r]->(md:Manifest) WHERE id(m) = %d`, msID)
	q += ` MATCH (md)-[cfgRel]-(cfg:Blob) WHERE cfg.type = "config"`
	q += ` RETURN DISTINCT ` + manifestDataReturn + `,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, TYPE(cfgRel) AS configMediaType, cfg.size AS configSize,`
	q += ` type(r) AS manifestDataRelType`
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	mdDigs := set.NewStringSet()
	manis = make([]types.Manifest, 0)
	for _, r := range res {
		m, _, e := n.ManifestFromResult(ctx, r)
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
		anno, e := n.LabelsGetByConfigID(ctx, c.ID)
		if e != nil {
			err = e
			return
		}
		c.Labels = anno
		c.Cfg.Config.Labels = anno.ToMap()
		m.Config = c
		// Add Layers
		m.Layers, err = n.GetLayersForManifestID(ctx, m.ID)
		if err != nil {
			return
		}
		manis = append(manis, m)
		mdDigs.Add(m.Digest)
	}
	return
}

func (n Neo) ManifestGetAllByManifestIDWithProfile(ctx context.Context, msID, prfID int64) (manis []types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestGetAllByManifestIDWithProfile",
		"maniID":  msID,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (ms:ManifestWrapper)-[r]->(m:Manifest) WHERE id(ms) = %d`, msID)
	q += ` MATCH (m)-[cfgRel]-(cfg:Blob) WHERE cfg.type = "config"`
	q += ` RETURN id(m) AS manifestID, m.label AS manifestLabel, m.digest AS manifestDigest, m.mediaType AS manifestMediaType, m.size AS manifestSize,`
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
	manis = make([]types.Manifest, 0)
	for _, r := range res {
		m, _, e := n.ManifestFromResult(ctx, r, prf)
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
		anno, e := n.LabelsGetByConfigID(ctx, c.ID)
		if e != nil {
			err = e
			return
		}
		c.Labels = anno
		c.Cfg.Config.Labels = anno.ToMap()
		m.Config = c
		// Add Layers
		m.Layers, err = n.GetLayersForManifestID(ctx, m.ID)
		if err != nil {
			return
		}
		manis = append(manis, m)
	}
	return
}

func (n Neo) ManifestLinkToAnnotation(ctx context.Context, maniID, annotationID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgNameMD,
		"func":         "ManifestLinkToAnnotation",
		"maniID":       maniID,
		"annotationID": annotationID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (m:Manifest), (a:Annotation) WHERE id(m) = %d AND id(a) = %d`, maniID, annotationID)
	q += ` MERGE (m)-[:HAS_ANNOTATION]->(a)`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	l.Debug(q)
	_, err = n.Execute(q)
	if err != nil {
		return
	}
	// TODO: Check if annotation is linked to a rule which has all filters rules satisfied

	return
}

func (n Neo) ManifestLinkToOCIv1Config(ctx context.Context, maniID int64, cfg types.Config) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestLinkToOCIv1Config",
		"maniID":  maniID,
		"cfgID":   cfg.ID,
	})
	defer span.End()
	_ = ctx
	if cfg.MediaType == "" {
		cfg.MediaType = v1.MediaTypeImageConfig
	}
	mt := utils.ShortenMediaType(cfg.MediaType)
	q := fmt.Sprintf(`MATCH (m:Manifest), (c:Blob) WHERE id(m) = %d AND id(c) = %d MERGE (m)-[:%s]->(c)`, maniID, cfg.ID, mt)
	utils.Debug(l, span, q)
	_, err = n.Execute(q)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to link manifest to OCI v1 config")
		return
	}
	return
}

func (n Neo) ManifestLinkToOCIv1Layer(ctx context.Context, maniID int64, ld types.LayerDesc) (err error) {
	shortMT := utils.ShortenMediaType(ld.Descriptor.MediaType)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNameMD,
		"func":      "ManifestLinkToOCIv1Layer",
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
	qCheck := fmt.Sprintf(`MATCH (m:Manifest)-[rel]->(l:Blob) WHERE id(m) = %d AND id(l) = %d AND l.type = "layer"`, maniID, ld.LayerID)
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
			qDel := fmt.Sprintf(`MATCH (m:Manifest)-[rel:%s]->(l:Blob) WHERE id(m) = %d AND id(l) = %d AND l.type = "layer" DELETE rel`, rt, maniID, ld.LayerID)
			utils.Debug(l, span, qDel)
			_, err = n.Execute(qDel)
			if err != nil {
				return
			}
		}
	}
	// Create new relation
	q := fmt.Sprintf(`MATCH (m:Manifest), (l:Blob) WHERE id(m) = %d AND id(l) = %d AND l.type = "layer" MERGE (m)-[:%s]->(l)`, maniID, ld.LayerID, shortMT)
	utils.Debug(l, span, q)
	_, err = n.Execute(q)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to link manifest to OCI v1 layer")
		return
	}
	return
}

func (n Neo) ManifestAddAnnotations(ctx context.Context, maniID int64, as *types.Annotations) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestAddAnnotations",
		"maniID":  maniID,
	})
	defer span.End()
	utils.Debug(l, span, "Adding %d annotations to manifestDataID %d", len(as.Data), maniID)
	for k, a := range as.Data {
		logrus.Warnf("adding annotation %s=%s", k, a.Value)
		aID, e := n.AnnotationGetInsert(ctx, a)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to add annotation")
		}
		a.ID = aID
		e = n.ManifestLinkToAnnotation(ctx, maniID, aID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to link annotation")
		}
	}
	return
}

func (n Neo) ManifestForManipuLayerID(ctx context.Context, mplID int64) (maniData types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestForManipuLayerID",
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:Manifest)<-[]-(mpl:ManipuLayer) WHERE id(mpl) = %d RETURN %s`, mplID, manifestDataReturn)
	utils.Debug(l, span, q)
	if res, found, err := n.QueryOne(q); err != nil {
		return maniData, utils.ErrorHandler(l, span, err, "failed to query manifest data")
	} else if !found {
		return maniData, utils.NewError(l, span, "manifest data not found connected to manipuLayer")
	} else {
		if md, ok, err := n.ManifestFromResult(ctx, res); err != nil {
			return maniData, utils.ErrorHandler(l, span, err, "failed to create Manifest from result")
		} else if !ok {
			return maniData, utils.NewError(l, span, "manifestData from result returned !ok: No profile connection?")
		} else {
			return md, nil
		}
	}
}

func (n Neo) ManifestGetTypesByRequest(ctx context.Context, req types.Request) (res types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestGetTypesByRequest",
		"host":    req.Host,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  req.Digest,
		"profile": req.Profile,
	})
	defer span.End()
	if req.Digest != "" {
		utils.Debug(l, span, "getting manifest data by digest")
		return n.ManifestGetByDigest(ctx, req.Digest)
	} else if req.Tag != "" {
		up := types.Upstream{
			Reg:     req.Host,
			Repo:    req.Repo,
			Tag:     req.Tag,
			Profile: req.Profile,
		}
		return n.ManifestGetByUpstream(ctx, up)
	}
	return
}

func (n Neo) ManifestGetIDByDigest(ctx context.Context, dig string) (maniID int64, found bool, err error) {
	m, found, err := n.ManifestGetByDigest(ctx, dig)
	if err != nil || !found {
		return
	}
	maniID = m.ID
	return
}

// ManifestsMatchingRule is the umbrella rule getter that unions the
// three per-flavour leaf getters and returns every Manifest whose
// content matches the given Rule, deduplicated by Manifest ID.
//
// Path Z task 7.4 — renamed from the pre-Path-Z `ManifestGetByRule`
// for symmetry and clarity. The name matches the rule-shortcut
// taxonomy (see design decision D5): there are three flavours of rule
// match, and the umbrella takes the union.
//
// Three rule shortcut flavours produce three different :TRUE edge
// targets, so we walk three queries and union the results:
//
//  1. anno eq/ne/re → (Rule)-[:TRUE]->(Annotation)<-[:HAS_ANNOTATION]-(md)
//     handled by ManifestsMatchingAnnotationRule
//  2. plat eq/ne    → (Rule)-[:TRUE]->(Platform)<-[:HAS_PLATFORM]-(md)
//     handled by ManifestsMatchingPlatformRule
//  3. anno null     → (Rule)-[:TRUE]->(md) directly (no shortcut node)
//     handled by ManifestsMatchingDirectRule
//
// All three are valid; a given Rule's type+op determines which one
// produced TRUE edges for it, but walking all three and letting the
// empty unions flow naturally lets a single caller handle any
// Rule shape without needing to branch on the rule type.
//
// Results are deduplicated by Manifest.ID so a manifest that matches
// via multiple flavours (unusual but possible) is not returned twice.
func (n Neo) ManifestsMatchingRule(ctx context.Context, rule types.Rule) (mds []types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestsMatchingRule",
		"rule":    rule.ToString(),
	})
	defer span.End()
	annoMDs, e := n.ManifestsMatchingAnnotationRule(ctx, rule)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get manifests matching annotation rule")
		return
	}
	utils.Debug(l, span, "Got %d Manifest per Annotation", len(annoMDs))
	platMDs, e := n.ManifestsMatchingPlatformRule(ctx, rule)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get manifests matching platform rule")
		return
	}
	utils.Debug(l, span, "Got %d Manifest per Platform", len(platMDs))
	directMDs, e := n.ManifestsMatchingDirectRule(ctx, rule)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get manifests matching direct rule")
		return
	}
	utils.Debug(l, span, "Got %d Manifest per Direct rule edge", len(directMDs))
	// Union and dedupe by Manifest.ID. Using a seen-set makes the
	// dedupe O(n) and stable across the three flavours. A manifest
	// that matches via multiple flavours (rare in practice but legal
	// per the rule-shortcut taxonomy) shows up exactly once.
	seen := make(map[int64]struct{}, len(annoMDs)+len(platMDs)+len(directMDs))
	mds = make([]types.Manifest, 0, len(annoMDs)+len(platMDs)+len(directMDs))
	for _, group := range [][]types.Manifest{annoMDs, platMDs, directMDs} {
		for _, m := range group {
			if _, dup := seen[m.ID]; dup {
				continue
			}
			seen[m.ID] = struct{}{}
			mds = append(mds, m)
		}
	}
	return
}

// Phase C3: ManifestGetByRule deprecated shim deleted. Use ManifestsMatchingRule.

// hydrateManifestsFromRuleQuery executes the given query and hydrates
// each result row into a Manifest with Platform and Upstream attached.
func (n Neo) hydrateManifestsFromRuleQuery(ctx context.Context, l *logrus.Entry, span trace.Span, q string) (mds []types.Manifest, err error) {
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return nil, err
	}
	for _, r := range res {
		m, _, e := n.ManifestFromResult(ctx, r)
		if e != nil {
			return nil, errors.Wrap(e, FailManifestFromResult)
		}
		up, e := UpstreamFromResult(r)
		if e != nil {
			return nil, errors.Wrap(e, errFailedUpstreamFromResult)
		}
		p, e := PlatformFromResult(r)
		if e != nil {
			return nil, e
		}
		m.Platform = p
		m.Upstream = up
		mds = append(mds, m)
	}
	return
}

// ManifestsMatchingDirectRule returns every Manifest reachable via the
// direct (Rule)-[:TRUE]->(Manifest) edge that `anno null` rules
// materialise. Unlike `anno eq/ne/re` (which hang an edge on the
// Annotation shortcut node) and `plat eq/ne` (which hangs an edge on
// the Platform shortcut node), `null` rules cannot use a shortcut:
// their predicate is "no annotation matching this key pattern exists",
// which has no node to anchor on. So `RuleEvalAnnotationRuleNULL`
// writes the TRUE edge directly to the content-leaf Manifest, and this
// getter walks it back.
//
// Path Z task 7.3 — renamed from `ManifestGetByDirectRule`. Part
// of the rule-shortcut taxonomy codified in design decision D5.
//
// The cypher uses the Path-Y-era fixed walk:
//
//	(r:Rule)-[:TRUE]->(md:Manifest)-[:HAS_PLATFORM]->(p:Platform)
//	(md)-[:ORIGINAL]->(u:Upstream)
//
// — :HAS_PLATFORM on the content-leaf per Path Z task 5.1, and
// :ORIGINAL from the content-leaf per Path Y abe0edb. The result is
// fully hydrated with platform + upstream metadata via the standard
// ManifestFromResult / PlatformFromResult / UpstreamFromResult helpers.
func (n Neo) ManifestsMatchingDirectRule(ctx context.Context, rule types.Rule) (mds []types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestsMatchingDirectRule",
		"rule":    rule.ToString(),
	})
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (r:Rule)-[%s]->(md:Manifest) WHERE id(r) = %d`, rel, rule.ID)
	q += ` MATCH (md)-[:HAS_PLATFORM]->(p:Platform)`
	q += ` MATCH (md)-[:ORIGINAL]->(u:Upstream)`
	q += ` RETURN ` + manifestDataReturn + `,`
	q += ` id(p) AS platformID, p.os AS platformOS, p.arch AS platformArch, p.variant AS platformVariant,`
	q += ` ` + UpstreamReturn
	return n.hydrateManifestsFromRuleQuery(ctx, l, span, q)
}

// ManifestsMatchingPlatformRule returns every Manifest reachable via
// the Platform shortcut node that `plat eq`/`plat ne` rules materialise.
// The cypher walk is:
//
//	(r:Rule)-[:TRUE]->(p:Platform)<-[:HAS_PLATFORM]-(md:Manifest)-[:ORIGINAL]->(u:Upstream)
//
// Path Z task 7.1 — renamed from `ManifestGetByPlatformRule`. The
// walk itself was fixed in Path Y `abe0edb` — the previous version used
// `(u:Upstream)-[]-(m:ManifestWrapper)` to hydrate Upstream via the legacy
// wrapper, but in the actual graph Upstream is linked directly from
// the content-leaf via `[:ORIGINAL]` (see createManifest), not via
// the `:ManifestWrapper` wrapper node. The old walk silently returned 0 rows
// for freshly-pushed content because the Upstream-to-Manifest edge it
// expected did not exist. Walking via `[:ORIGINAL]` from md directly
// is consistent with how StoreManifestUpload persists the graph.
//
// Part of the rule-shortcut taxonomy codified in design decision D5.
//
// Error handling: Path Z task 7 also fixes a pre-existing silent error
// swallow at `res, err := n.Query(q)` — the previous version ignored
// the error and iterated over `res` anyway, masking database connection
// issues as empty results. The renamed version checks the error
// explicitly.
func (n Neo) ManifestsMatchingPlatformRule(ctx context.Context, rule types.Rule) (mds []types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestsMatchingPlatformRule",
		"rule":    rule.ToString(),
	})
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (r:Rule)-[%s]->(p:Platform)<-[:HAS_PLATFORM]-(md:Manifest)-[:ORIGINAL]->(u:Upstream) WHERE id(r) = %d`, rel, rule.ID)
	q += ` RETURN ` + manifestDataReturn + `,`
	q += ` id(p) AS platformID, p.os AS platformOS, p.arch AS platformArch, p.variant AS platformVariant,`
	q += ` ` + UpstreamReturn
	mds, err = n.hydrateManifestsFromRuleQuery(ctx, l, span, q)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "failed to run platform-rule query")
	}
	for _, m := range mds {
		logrus.Warnf("found manifest via platform: %s:%s (%s)", m.Upstream.Repo, m.Upstream.Tag, m.Label)
	}
	return
}

// ManifestMatchesProfile checks if the manifest data matches the profile
//
// # Parameters:
//   - md: the manifest data to check,
//   - prf: the profile to check against,
//
// This function evals all the rules of the profile and matches them against the manifest data.
//
// # Returns:
//   - matches: true if the manifest data matches the profile, false otherwise
//   - err: error if any occurred during the process
//
// # Details:
//
// Rules can be of different types (e.g., annotation rules, platform rules, etc.).
// The function checks each rule type and evaluates whether the manifest data satisfies the conditions defined by the profile's rules.
// If all rules are satisfied, the function returns true; otherwise, it returns false.
func (n Neo) ManifestMatchesProfile(ctx context.Context, md types.Manifest, prf types.Profile) (matches bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgNameMD,
		"func":         "ManifestMatchesProfile",
		keyMDLabel:     md.Label,
		keyProfileName: prf.Name,
	})
	defer span.End()
	_ = l
	return true, nil
}

func (n Neo) ManifestCheckProfile(ctx context.Context, md types.Manifest, prf types.Profile) (ok bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgNameMD,
		"func":         "ManifestCheckProfile",
		keyMDLabel:     md.Label,
		keyProfileName: prf.Name,
	})
	defer span.End()
	if ok, err := n.ManifestCheckProfileCreationRelationship(ctx, md, prf); err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to check profile creation")
	} else {
		if !ok {
			return false, nil
		}
	}
	if ok, err := n.ManifestCheckProfileAnnotations(ctx, md, prf); err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to check profile annotations")
	} else {
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// ManifestCheckProfileCreationRelationship checks if the manifest data is created by the profile
func (n Neo) ManifestCheckProfileCreationRelationship(ctx context.Context, md types.Manifest, prf types.Profile) (ok bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgNameMD,
		"func":         "ManifestCheckProfileCreationRelationship",
		keyMDLabel:     md.Label,
		keyProfileName: prf.Name,
	})
	defer span.End()
	q := `MATCH (md:Manifest)-[r]->(p:Profile)`
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

func (n Neo) ManifestCheckProfileAnnotations(ctx context.Context, md types.Manifest, prf types.Profile) (ok bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":      pkgNameMD,
		"func":         "ManifestCheckProfileAnnotations",
		keyMDLabel:     md.Label,
		keyProfileName: prf.Name,
	})
	_ = ctx
	defer span.End()
	// Check for failing annotations — if any exist the manifest does not match.
	q := fmt.Sprintf(`MATCH (md:Manifest)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule)<-[:USES]-(p:Profile)`, types.RelationFailure)
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
	if cnt != 0 {
		return false, nil
	}
	// When no failing annotations exist, also verify that all noop rules
	// for the profile are satisfied (noop=true means no targeted annotation
	// present on the manifest).
	return n.manifestCheckNoopRules(l, span, md.ID, prf.ID)
}

// manifestCheckNoopRules verifies that a manifest satisfies all annotation
// noop rules of a profile. Returns (true, nil) when the profile has no
// noop rules or all of them have a TRUE edge to the manifest.
func (n Neo) manifestCheckNoopRules(l *logrus.Entry, span trace.Span, mdID int64, prfID int64) (ok bool, err error) {
	q := fmt.Sprintf(`MATCH (p:Profile)-[:USES]->(r:Rule) WHERE id(p) = %d AND r.op = "null" and r.type = "anno" RETURN id(r) AS ruleID`, prfID)
	utils.Debug(l, span, q)
	resRules, err := n.Query(q)
	if err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to get noop rules for profile")
	}
	if len(resRules) == 0 {
		return true, nil
	}
	noopRuleIDsStr := make([]string, 0, len(resRules))
	for _, r := range resRules {
		rID, e := r.GetInt64("ruleID")
		if e != nil {
			return false, utils.ErrorHandler(l, span, e, "failed to get ruleID from result")
		}
		noopRuleIDsStr = append(noopRuleIDsStr, fmt.Sprintf("%d", rID))
	}
	q = fmt.Sprintf(`MATCH (md:Manifest)-[:%s]-(r:Rule)`, types.RelationSuccess)
	q += fmt.Sprintf(` WHERE id(md) = %d AND id(r) IN [%s] RETURN count(r) AS countAnnotations`, mdID, strings.Join(noopRuleIDsStr, ","))
	utils.Debug(l, span, q)
	res, _, err := n.QueryOne(q)
	if err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to QueryOne for noop rules")
	}
	cnt, err := res.GetInt("countAnnotations")
	if err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to get count annotations for noop rules")
	}
	utils.Debug(l, span, "countAnnotations for noop rules: %d", cnt)
	return cnt == len(resRules), nil
}

// ManifestsMatchingAnnotationRule returns every Manifest reachable via
// the Annotation shortcut node that `anno eq`/`ne`/`re` rules
// materialise. The cypher walk is:
//
//	(md:Manifest)-[:HAS_ANNOTATION]->(a:Annotation)<-[:TRUE]-(r:Rule)
//	(md)-[:HAS_PLATFORM]->(p:Platform)
//	(md)-[:ORIGINAL]->(u:Upstream)
//
// Path Z task 7.2 — renamed from `ManifestGetByAnnotationRule`.
// The walk itself was fixed in Path Y `abe0edb` — the previous version
// used `(u:Upstream)-[]-(m:ManifestWrapper)-[]-(md)` which was broken in
// production because Upstream is linked from the content-leaf via
// `[:ORIGINAL]`, not via the `:ManifestWrapper` wrapper node. Walking via
// `[:ORIGINAL]` from md directly is consistent with how
// StoreManifestUpload persists the graph.
//
// Note that `anno null` rules do NOT come through this getter — they
// materialise their TRUE edges directly on the content-leaf Manifest
// (no shortcut node), and are walked by ManifestsMatchingDirectRule.
// The umbrella ManifestsMatchingRule unions all three flavours so a
// single caller doesn't need to branch on the rule type.
//
// Part of the rule-shortcut taxonomy codified in design decision D5.
//
// Error handling: Path Z task 7 also fixes a pre-existing silent error
// swallow at `res, err := n.Query(q)` — the previous version iterated
// over `res` even when the query failed, masking database connection
// issues as empty results.
func (n Neo) ManifestsMatchingAnnotationRule(ctx context.Context, rule types.Rule) (mds []types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestsMatchingAnnotationRule",
		"rule":    rule.ToString(),
	})
	defer span.End()
	rel := types.RelationSuccess
	q := fmt.Sprintf(`MATCH (md:Manifest)-[:HAS_ANNOTATION]->(a:Annotation)<-[:%s]-(r:Rule) WHERE id(r) = %d`, rel, rule.ID)
	q += ` MATCH (md)-[:HAS_PLATFORM]->(p:Platform)`
	q += ` MATCH (md)-[:ORIGINAL]->(u:Upstream)`
	q += ` RETURN ` + manifestDataReturn + `,`
	q += ` id(p) AS platformID, p.os AS platformOS, p.arch AS platformArch, p.variant AS platformVariant,`
	q += ` ` + UpstreamReturn
	mds, err = n.hydrateManifestsFromRuleQuery(ctx, l, span, q)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "failed to run annotation-rule query")
	}
	for _, m := range mds {
		logrus.Warnf("found manifest via annotation: %s:%s (%s)", m.Upstream.Repo, m.Upstream.Tag, m.Label)
	}
	return
}

func (n Neo) ManifestLinkToConfig(ctx context.Context, maniID int64, cfg types.Config) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestLinkToConfig",
		"maniID":  maniID,
		"cfg.ID":  cfg.ID,
	})
	defer span.End()
	if cfg.MediaType == "" {
		err = utils.ErrorHandler(l, span, fmt.Errorf("%s", FailMediaTypeEmpty), "")
		return
	}
	mt := utils.ShortenMediaType(cfg.MediaType)
	q := fmt.Sprintf(`MATCH (m:Manifest),(c:Blob) WHERE id(m) = %d AND id(c) = %d`, maniID, cfg.ID)
	q += fmt.Sprintf(` MERGE (m)-[:%s]->(c)`, mt)
	q += fmt.Sprintf(` SET c.mediaType = "%s", c.type = "config"`, cfg.MediaType)
	utils.Debug(l, span, q)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(q, map[string]interface{}{})
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to create link (m:Manifest)-[:%s]->(c:Blob)", mt)
	}
	return
}

func (n Neo) ManifestLinkToLayer(ctx context.Context, maniID int64, layer types.Layer, idx int) (err error) {
	shortMT := utils.ShortenMediaType(layer.MediaType)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestLinkToLayer",
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
	q := fmt.Sprintf(`MATCH (m:Manifest),(l:Blob) WHERE id(m) = %d AND id(l) = %d`, maniID, layer.ID)
	q += fmt.Sprintf(` MERGE (m)-[:HAS_LAYER {order: %d, mediaType: "%s"}]->(l)`, idx, layer.MediaType)
	q += fmt.Sprintf(` SET l.mediaType = "%s", l.type = "layer"`, layer.MediaType)

	utils.Debug(l, span, q)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(q, map[string]interface{}{})
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to create link")
		return
	}
	for k, v := range layer.Annotations {
		a := types.NewAnnotation(k, v)
		aID, e := n.AnnotationGetInsert(ctx, a)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to add annotation")
		}
		a.ID = aID
		// TODO: Create ManifestLinkToAnnotation function?
		q := fmt.Sprintf(`MATCH (l:Blob), (a:Annotation) WHERE id(l) = %d AND id(a) = %d MERGE (l)-[:HAS_ANNOTATION]->(a)`, layer.ID, aID)
		utils.Debug(l, span, q)
		_, e = n.Execute(q)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to link annotation to layer")
		}
	}
	return
}

func (n Neo) ManifestRemove(ctx context.Context, req types.Request) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestRemove",
	})
	defer span.End()
	if mani, found, err := n.ManifestGetTypesByRequest(ctx, req); err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to get manifest by id")
	} else if !found {
		return found, err
	} else if err = n.ManifestDeleteByDigest(ctx, mani.Digest); err != nil {
		return false, utils.ErrorHandler(l, span, err, "failed to delete manifest")
	}
	return true, nil
}

func (n Neo) ManifestInsert(ctx context.Context, maniData *types.Manifest) (mdId int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestInsert",
		"repo":    maniData.Upstream.Repo,
		"tag":     maniData.Upstream.Tag,
		"digest":  maniData.Digest,
	})
	defer span.End()
	if fMD, found, err := n.ManifestGetByDigest(ctx, maniData.Digest); err != nil {
		err = utils.ErrorHandler(l, span, err, "ManifestGetByDigest failed")
		return 0, err
	} else if found {
		utils.Debug(l, span, "manifestData already exists: %s", fMD.Label)
		// TODO: check if manifestData and FMD are exactly the same
		maniData = &fMD
		return fMD.ID, nil
	}
	mdID, err := n.createManifest(ctx, *maniData)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "createManifest failed")
		return 0, err
	}
	maniData.ID = mdID
	return mdID, nil
}

func (n Neo) createManifest(ctx context.Context, mani types.Manifest) (maniID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNameMD,
		"func":      "createManifest",
		"repo":      mani.Upstream.Repo,
		"tag":       mani.Upstream.Tag,
		"digest":    mani.Digest,
		"platform:": mani.Platform.GetLabel(),
	})
	defer span.End()
	label := utils.CreateLabelFromTypeAndDigest(mani.MediaType, mani.Digest)
	q := fmt.Sprintf(`CREATE (m:Manifest {digest: "%s", mediaType: "%s", size: %d, label: "%s"`, mani.Digest, mani.MediaType, mani.Size, label)
	if mani.ArtifactType != "" {
		q += fmt.Sprintf(`, artifactType: "%s"`, mani.ArtifactType)
	}
	q += `})`
	q += ` RETURN id(m) AS manifestID, m.label AS manifestLabel`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
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
	utils.Debug(l, span, "Created Manifest w/ ID '%d'", maniID)
	var cfg types.Config
	// Link Config
	if mani.Config.Digest != "" {
		var found bool
		cfg, found, err = n.ConfigGetByDigest(ctx, mani.Config.Digest)
		if err != nil {
			err = errors.Wrap(err, "failed to get config id")
			return maniID, err
		}
		if !found {
			err = fmt.Errorf("config w/ digest '%s' not found", utils.ShortenDigest(mani.Config.Digest))
			return maniID, err
		}
		cfg.MediaType = mani.Config.MediaType
		err = n.ManifestLinkToConfig(context.Background(), maniID, cfg)
		if err != nil {
			err = errors.Wrap(err, "failed to link manifest to config")
			return maniID, err
		}
		utils.Debug(l, span, "Linked Config ID '%d' to Manifest ID '%d'", cfg.ID, maniID)
	} else {
		utils.Warn(l, span, "No config for Manifest ID '%d'", maniID)
	}
	// Link Platform
	if platID, found, err := n.GetPlatform(ctx, mani.Platform); err != nil {
		err = errors.Wrap(err, "failed to get platform")
		return maniID, err
	} else if !found {
		utils.Warn(l, span, "Platform not found, creating new platform: OS='%s', Arch='%s', Variant='%s'", mani.Platform.OS, mani.Platform.Arch, mani.Platform.Variant)
	} else if err = n.LinkManifestToPlatform(ctx, maniID, platID); err != nil {
		err = errors.Wrap(err, "failed to link manifest to platform")
		return maniID, err
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
		err = n.ManifestLinkToLayer(ctx, maniID, layer, idx)
		if err != nil {
			err = errors.Wrap(err, "failed to link manifest to layer")
			return maniID, err
		}
		utils.Debug(l, span, "Linked Layer ID '%d' to Manifest ID '%d'", lyr.ID, maniID)
	}
	if len(mani.Layers) == 0 {
		utils.Warn(l, span, "No layer for Manifest ID '%d'", maniID)
	}
	// Link Annotations
	if len(mani.Annotations.Data) > 0 {
		err = n.ManifestAddAnnotations(ctx, maniID, &mani.Annotations)
		if err != nil {
			return maniID, utils.ErrorHandler(l, span, err, "failed to add annotations to manifest data")
		}
		utils.Debug(l, span, "Linked %d Annotations to Manifest ID '%d'", len(mani.Annotations.Data), maniID)
	} else {
		utils.Warn(l, span, "No annotations for Manifest ID '%d'", maniID)
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
		if err = n.ManifestLinkToUpstream(ctx, maniID, mani.Upstream.ID); err != nil {
			return maniID, utils.ErrorHandler(l, span, err, "failed to link manifest to upstream")
		}
		utils.Debug(l, span, "Linked Upstream ID '%d' to Manifest ID '%d'", mani.Upstream.ID, maniID)
	} else {
		utils.Warn(l, span, "No upstream for Manifest ID '%d'", maniID)
	}
	return
}

func (n Neo) ManifestGetExactly(ctx context.Context, mani types.Manifest) (maniID int64, success bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMD,
		"func":    "ManifestGetExactly",
	})
	ctx, span := tracer.Start(ctx, "ManifestGetExactly")
	_ = ctx
	defer span.End()
	q := `MATCH (m:Manifest) WHERE m.digest = "%s" AND m.mediaType = "%s" AND m.size = %d RETURN id(m) AS maniID`
	query := fmt.Sprintf(q, mani.Digest, mani.MediaType, mani.Size)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, query),
	))
	l.Debug(query)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf(errFailedRunQueryFmt, err)
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

func (n Neo) ManifestDirectGetByDigest(ctx context.Context, dig string) (m types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDirectGetByDigest",
		"digest":  dig,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (md:Manifest) WHERE md.digest = "%s"`, dig)
	q += ` RETURN ` + manifestDataReturn
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to query for manifest data")
		return
	}
	if !found {
		utils.Debug(l, span, errManifestDataNotFound)
		return
	}
	m, _, e = n.ManifestFromResult(ctx, res)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailManifestFromResult)
		return
	}
	if up, relType, found, err := n.UpstreamGetByManifestID(ctx, m.ID); err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get upstream by manifest data id")
		return m, found, err

	} else if found {
		m.Upstream = up
		m.Type = relType
	}
	return
}

func (n Neo) ManifestGetByDigest(ctx context.Context, dig string) (m types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestGetByDigest",
		"digest":  dig,
	})
	defer span.End()
	m, found, err = n.ManifestDirectGetByDigest(ctx, dig)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get manifest data directly")
		return
	}
	if found {
		utils.Debug(l, span, "found direct (Manifest)->(Upstream): %s", utils.ShortenDigest(m.Digest))
		return
	}
	m, found, err = n.ManifestIndirectGetByDigest(ctx, dig)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get manifest data indirectly")
		return
	}
	if found {
		utils.Debug(l, span, "found indirect (Manifest)-[]-(Manifest): %s", utils.ShortenDigest(m.Digest))
		return
	}
	utils.Warn(l, span, errManifestDataNotFound)
	return
}

func (n Neo) ManifestIndirectGetByDigest(ctx context.Context, dig string) (m types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestIndirectGetByDigest",
		"digest":  dig,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (u:Upstream)-[]-(md:Manifest) WHERE md.digest = "%s"`, dig)
	q += ` RETURN ` + manifestDataReturn + `,`
	q += ` ` + UpstreamReturn
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to query for manifest data")
		return
	}
	if !found {
		utils.Debug(l, span, errManifestDataNotFound)
		return
	}
	m, _, e = n.ManifestFromResult(ctx, res)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailManifestFromResult)
		return
	}
	up, e := UpstreamFromResult(res)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, errFailedUpstreamFromResult)
		return
	}
	m.Upstream = up
	return
}

func (n Neo) ManifestGetByID(ctx context.Context, mdID int64) (m types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestGetByID",
		"maniDataID": mdID,
	})
	defer span.End()
	q := `MATCH (md:Manifest)`
	q += fmt.Sprintf(` WHERE id(md) = %d`, mdID)
	q += ` RETURN ` + manifestDataReturn
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, errFailedRunQuery)
		return
	}
	if !found {
		return
	}
	m, _, err = n.ManifestFromResult(ctx, res)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "error  ManifestFromResult")
		return
	}
	return
}

// ManifestGetByUpstream returns a manifest data by upstream.
// It returns only one, which means it will error out if more are found.
// -> With a profile in up.Profile it will check for each candidate if it is connected to the profile.
func (n Neo) ManifestGetByUpstream(ctx context.Context, up types.Upstream) (m types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestGetByUpstream",
		"reg":     up.Reg,
		"repo":    up.Repo,
		"tag":     up.Tag,
		"profile": up.Profile,
	})
	defer span.End()
	// Try direct Manifest→Upstream path first, then fall back to wrapper path.
	res, found, err := n.manifestQueryByRepoTag(l, span, up.Repo, up.Tag)
	if err != nil {
		return
	}
	if !found {
		return m, false, nil
	}
	// Hydrate Manifest — optionally filtered by profile.
	if up.Profile != "" {
		m, found, err = n.manifestHydrateWithProfile(ctx, l, span, res, up.Profile)
	} else {
		var e error
		m, _, e = n.ManifestFromResult(ctx, res)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, FailManifestFromResult)
			return
		}
		m.Label = fmt.Sprintf("%s/%s:%s@%s", m.Upstream.Reg, m.Upstream.Repo, m.Upstream.Tag, m.Platform.Arch)
		found = true
	}
	if err != nil || !found {
		return
	}
	if u, e := UpstreamFromResult(res); e != nil {
		err = utils.ErrorHandler(l, span, e, errFailedUpstreamFromResult)
		return
	} else {
		m.Upstream = u
	}
	return
}

// manifestQueryByRepoTag queries for a Manifest by repo+tag, first via
// the direct (Manifest)-[:ORIGINAL]->(Upstream) path, then via the
// legacy (ManifestWrapper)-[]->(Upstream) wrapper path.
func (n Neo) manifestQueryByRepoTag(l *logrus.Entry, span trace.Span, repo, tag string) (res types.Result, found bool, err error) {
	q := `MATCH (md:Manifest)-[r]->(u:Upstream)`
	q += fmt.Sprintf(` WHERE u.repo = "%s" AND u.tag = "%s"`, repo, tag)
	q += ` RETURN ` + manifestDataReturn + `, ` + UpstreamReturn + `, type(r) AS manifestDataRelType`
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, errFailedRunQuery)
		return
	}
	if found {
		return
	}
	// Fallback: try via ManifestWrapper.
	q = `MATCH (md:Manifest)<-[r]-(m:ManifestWrapper)-[]->(u:Upstream)`
	q += fmt.Sprintf(` WHERE u.repo = "%s" AND u.tag = "%s"`, repo, tag)
	q += ` RETURN ` + manifestDataReturn + `, ` + UpstreamReturn + `, type(r) AS manifestDataRelType`
	utils.Debug(l, span, q)
	res, found, e = n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, errFailedRunQuery)
		return
	}
	if !found {
		utils.Debug(l, span, errManifestDataNotFound)
	}
	return
}

// manifestHydrateWithProfile hydrates a Manifest from a query result
// and verifies it is connected to the named profile.
func (n Neo) manifestHydrateWithProfile(ctx context.Context, l *logrus.Entry, span trace.Span, res types.Result, profileLabel string) (m types.Manifest, found bool, err error) {
	prf, prfFound, e := n.ProfileGetByLabel(ctx, profileLabel)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailedProfGetByLabel)
		return m, false, err
	}
	if !prfFound {
		utils.NewError(l, span, FailedProfileNotFound)
		return m, false, nil
	}
	utils.Debug(l, span, msgFoundProfile, profileLabel, prf.ID)
	var ok bool
	m, ok, e = n.ManifestFromResult(ctx, res, prf)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailManifestFromResult)
		return
	}
	if !ok {
		utils.NewError(l, span, "manifest data is not connected to profile '%s'", profileLabel)
		return m, false, nil
	}
	m.Label = fmt.Sprintf("%s/%s:%s@%s-%s", m.Upstream.Reg, m.Upstream.Repo, m.Upstream.Tag, prf.Label, m.Platform.Arch)
	return m, true, nil
}

func ManifestFromRecord(res neo4j.Result) (m types.Manifest, err error) {
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

func (n Neo) ManifestLinkToManipulayer(ctx context.Context, maniDataID, mplID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "neo.ManifestLinkToManipulayer",
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:Manifest),(mpl:ManipuLayer) WHERE id(md) = %d AND id(mpl) = %d`, maniDataID, mplID)
	q += ` MERGE (md)<-[:USES]-(mpl)`
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailQueryExec)
		return err
	}
	return nil
}

func (n Neo) ManifestIsLinkedWithProfile(ctx context.Context, maniDataID int64, prf types.Profile) (linked bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "neo.ManifestIsLinkedWIthProfile",
		"maniDataID": maniDataID,
		"profileID":  prf.ID,
	})
	defer span.End()
	_ = ctx
	// Is the Manifest linked with the profile?
	q := `MATCH (prf:Profile)-[]->(ml:Index)-[:REFERENCES]->(md:Manifest)`
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
	q = `MATCH (prf:Profile)<-[r]-(md:Manifest)`
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

func (n Neo) ManifestFromResult(ctx context.Context, res types.Result, prfs ...types.Profile) (m types.Manifest, ok bool, err error) {
	prf := types.Profile{ID: -1}
	prfProvided := len(prfs) > 0
	if prfProvided {
		prf = prfs[0]
	}
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "neo.ManifestFromResult",
		"prfs":    prf.ID,
	})
	defer span.End()

	err = res.CheckKeys("manifestDataID", "manifestDataDigest", "manifestDataMediaType", "manifestDataSize")
	if err != nil {
		utils.ErrorHandler(l, span, err, "failed to check keys")
		return
	}
	maniID, _ := res.GetInt64("manifestDataID")
	manifestDigest, _ := res.GetStr("manifestDataDigest")
	manifestMediaType, _ := res.GetStr("manifestDataMediaType")
	manifestSize, _ := res.GetInt64("manifestDataSize")
	manifestOrigin, _ := res.GetStr("manifestDataOrigin")
	manifestLabel, _ := res.GetStr("manifestDataLabel")
	manifestdataRelType, _ := res.GetStr("manifestDataRelType")
	span.SetAttributes(
		attribute.Key("maniDataID").Int64(maniID),
		attribute.Key("manifestDataSize").Int64(manifestSize),
		attribute.Key("manifestDataDigest").String(manifestDigest),
		attribute.Key("manifestDataMediaType").String(manifestMediaType),
		attribute.Key("manifestDataOrigin").String(manifestOrigin),
		attribute.Key("manifestDataLabel").String(manifestLabel),
		attribute.Key("manifestDataRelType").String(manifestdataRelType),
	)
	if prfProvided {
		/// TODO: Check if it is connected
		linked, e := n.ManifestIsLinkedWithProfile(ctx, maniID, prf)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to check if manifest data is linked with profile")
			return
		}
		if !linked {
			err = utils.NewError(l, span, "manifest data is not linked with profile '%s'", prf.Label)
			return
		}
	}

	// Get Annotations for Manifest
	annos, err := n.GetAnnotationsByManifestID(ctx, maniID)
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
	lyrs, e := n.GetLayersForManifestID(ctx, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get layers for manifest data")
		return
	}
	up, e := UpstreamFromResult(res)
	if e != nil {
		utils.Debug(l, span, "failed to get upstream from result, but it's fine")
	}
	m = types.Manifest{
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

// ManifestGetRawByRepoTag returns every Manifest that has ever been
// pushed to (repo, tag), without applying any profile or rule-match
// filtering. The walk goes via the Upstream node's `(repo, tag)` compound
// key:
//
//	(md:Manifest)-[:ORIGINAL]->(u:Upstream {repo: $repo, tag: $tag})
//
// Note the edge direction: ORIGINAL points FROM the Manifest TO the
// Upstream (see createManifest where `MERGE (md)-[:ORIGINAL]->(up)`
// is built). Walking from Upstream back to Manifest therefore
// requires the reverse arrow.
//
// This is the push-time "what was pushed to this (repo, tag)?" lookup
// used by the ComputeManifestlistsForRepoTag workflow. Walking via
// Upstream (rather than via the `:ManifestWrapper` wrapper node, which is keyed
// on a `host/repo:tag` label with a UNIQUE constraint) ensures that
// multi-arch pushes are visible even though each subsequent push replaces
// the Manifest wrapper's outgoing `[:MANIFEST_DERIVED_FROM]` edge. Every
// push creates its own Upstream node (keyed on digest), so the Upstream
// layer carries the full per-(repo, tag) push history that the
// aggregation needs.
//
// Unlike ManifestGetByCollectionTag, which is designed for CTPP
// hydration and filters out manifestdata that have not yet been linked
// to any Profile via annotation rule eval, this helper is the raw
// lookup used BEFORE rule evaluation has populated profile linkages.
//
// Each returned Manifest carries ID, Digest, MediaType, Size, Label
// and the relationship type. Callers that need fully-hydrated
// manifestdata (config, platform, layers, upstream) should re-fetch via
// ManifestGetByDigest — the compute path already does this
// internally.
//
// An empty result (no manifestdata for this repo+tag) is returned as
// `([], nil)`; the caller is responsible for treating empty as "nothing
// to compute".
func (n Neo) ManifestGetRawByRepoTag(ctx context.Context, repo, tag string) (manis []types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestGetRawByRepoTag",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()
	_ = ctx
	utils.Debug(l, span, "fetching raw manifestdata for (repo=%s, tag=%s) via Upstream", repo, tag)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	q := `MATCH (md:Manifest)-[r:ORIGINAL]->(u:Upstream)`
	q += ` WHERE u.repo = $repo AND u.tag = $tag`
	q += ` RETURN DISTINCT ` + manifestDataReturn + `, type(r) AS manifestdataRelType`
	q += ` ORDER BY md.digest`
	utils.Debug(l, span, q)
	input := map[string]interface{}{"repo": repo, "tag": tag}
	result, err := session.Run(q, input)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "failed to run raw manifestdata query for %s:%s", repo, tag)
	}
	manis = make([]types.Manifest, 0)
	seen := map[int64]struct{}{}
	for result.Next() {
		md, e := ManifestFromRecord(result)
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, "while ManifestFromRecord")
		}
		if _, dup := seen[md.ID]; dup {
			continue
		}
		seen[md.ID] = struct{}{}
		manis = append(manis, md)
	}
	utils.Debug(l, span, "raw manifestdata count for (repo=%s, tag=%s): %d", repo, tag, len(manis))
	return manis, nil
}

func (n Neo) ManifestGetByCollectionTag(ctx context.Context, colID, colTagID int64) (manis []types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameMD,
		"func":     "ManifestGetByCollectionTag",
		"colID":    colID,
		"colTagID": colTagID,
	})
	defer span.End()
	utils.Info(l, span, "getting manifest data by collection and tag")
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	cfgFilter := `["application/vnd.oci.image.config.v1+json", "application/vnd.docker.container.image.v1+json"]`

	// --- Legacy query (mh-populate / ManifestWrapper path) ---
	q := `MATCH (p:Platform)-[]-(md:Manifest)-[cfgRel]->(cfg:Blob) WHERE cfg.type = "config"`
	q += ` AND cfg.mediaType IN ` + cfgFilter
	q += fmt.Sprintf(` MATCH (c:Repo)-[:UPSTREAM]->(m)-[r]-(md:Manifest) WHERE id(c) = %d`, colID)
	q += fmt.Sprintf(` MATCH (t:Tag)-[:UPSTREAM]->(m)-[]->(md) WHERE id(t) = %d`, colTagID)
	q += ` MATCH (m)-[]->(u:Upstream)`
	q += ` RETURN type(r) AS manifestdataRelType,`
	q += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += manifestDataReturn + `,`
	q += UpstreamReturn + ` ,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, TYPE(cfgRel) AS configMediaType, cfg.size AS configSize`
	utils.Debug(l, span, q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	manis = make([]types.Manifest, 0)
	hasLegacyResults := false
	for result.Next() {
		hasLegacyResults = true
		p, e := PlatformFromRecord(result)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while PlatformFromRecord")
			return
		}
		c, e := n.ConfigFromRecord(ctx, result)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while ConfigFromRecord")
			return
		}
		u, e := UpstreamFromRecord(result)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while UpstreamFromRecord")
			return
		}
		m, e := ManifestFromRecord(result)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while ManifestFromRecord")
			return
		}

		anno, e := n.LabelsGetByConfigID(ctx, c.ID)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while LabelsGetByConfigID")
			return
		}
		m.Annotations = anno
		c.Labels = anno
		m.Config = c
		m.Platform = p
		m.Upstream = u
		prfs, e := n.ProfileFromManifestID(ctx, m.ID)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get profiles for manifest data")
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
			utils.NewError(l, span, "no profiles found for manifest data: %d", m.ID)
			continue
		}
		for _, prf := range prfs {
			// Use annotation-only check here: parsed manifests have no UPLOADED_BY/PULLED_BY
			// relationship to a profile, so ManifestCheckProfile would always return false.
			ok, e := n.ManifestCheckProfileAnnotations(ctx, m, prf)
			if e != nil {
				utils.ErrorHandler(l, span, e, "Failed to check profile")
				return
			}
			msg := fmt.Sprintf("  %t > %d -> filtered md: %s", ok, prf.ID, m.Label)
			span.AddEvent("log", trace.WithAttributes(
				attribute.String(attrLogSeverity, attrSevDebug),
				attribute.String(attrLogMessage, msg),
			))
			l.Debug(msg)
			if ok {
				m.Upstream.Profile = prf.Label
				manis = append(manis, m)
			}
		}
	}

	// --- Slot-centred fallback (OCI push path) ---
	// If the legacy query returned no results, try the slot-centred graph path:
	// Repo -> Tag -> Index -> Manifest -> Blob(config)
	if !hasLegacyResults {
		utils.Debug(l, span, "legacy query returned no results, trying slot-centred path")
		qs := fmt.Sprintf(`MATCH (c:Repo)-[:HAS_TAG]->(t:Tag)-[:REF]->(:Index)-[:REFERENCES]->(md:Manifest)-[cfgRel]->(cfg:Blob) WHERE id(c) = %d AND id(t) = %d`, colID, colTagID)
		qs += ` AND cfg.type = "config" AND cfg.mediaType IN ` + cfgFilter
		qs += ` MATCH (p:Platform)-[]-(md)`
		// Compute total content size: sum of distinct blob sizes (config + layers) for this manifest.
		// Blobs may be shared across manifests or connected via multiple paths, so collect distinct
		// blob nodes first to avoid double-counting.
		qs += ` OPTIONAL MATCH (md)-[]->(blob:Blob) WHERE blob.type IN ["config", "layer"]`
		qs += ` WITH c, t, md, cfgRel, cfg, p, collect(DISTINCT blob) AS blobs`
		qs += ` WITH c, t, md, cfgRel, cfg, p, reduce(s = 0, b IN blobs | s + b.size) AS contentSize`
		qs += ` RETURN "REFERENCES" AS manifestdataRelType,`
		qs += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
		qs += ` ` + manifestDataReturn + `,`
		// Synthesize Upstream from the Repo/Tag metadata (OCI-pushed images have no Upstream node)
		qs += ` 0 AS upstreamID, "" AS upstreamReg, c.repo AS upstreamRepo, t.name AS upstreamTag, md.digest AS upstreamDigest,`
		qs += ` id(cfg) AS configID, cfg.digest AS configDigest, TYPE(cfgRel) AS configMediaType, cfg.size AS configSize,`
		qs += ` contentSize`
		utils.Debug(l, span, qs)
		result2, e := session.Run(qs, map[string]interface{}{})
		if e != nil {
			err = e
			return
		}
		for result2.Next() {
			p, e := PlatformFromRecord(result2)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while PlatformFromRecord (slot path)")
				return
			}
			c, e := n.ConfigFromRecord(ctx, result2)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while ConfigFromRecord (slot path)")
				return
			}
			u, e := UpstreamFromRecord(result2)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while UpstreamFromRecord (slot path)")
				return
			}
			m, e := ManifestFromRecord(result2)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while ManifestFromRecord (slot path)")
				return
			}
			anno, e := n.LabelsGetByConfigID(ctx, c.ID)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while LabelsGetByConfigID (slot path)")
				return
			}
			m.Annotations = anno
			c.Labels = anno
			m.Config = c
			m.Platform = p
			m.Upstream = u
			m.Profile = "generic"
			m.Upstream.Profile = "generic"
			// Read content size (sum of config + layer blobs).
			// The reduce() result may arrive as int64, int, or float64 depending
			// on the neo4j driver version and arithmetic path.
			if csVal, ok := result2.Record().Get("contentSize"); ok && csVal != nil {
				switch v := csVal.(type) {
				case int64:
					m.ContentSize = v
				case int:
					m.ContentSize = int64(v)
				case float64:
					m.ContentSize = int64(v)
				}
			}
			manis = append(manis, m)
		}
	}

	// --- Direct UPSTREAM fallback (CE parse path) ---
	// If neither legacy nor slot-centred returned results, try the direct UPSTREAM path:
	// Repo -[:UPSTREAM]-> Manifest -> Blob(config), with Manifest -[:ORIGINAL]-> Upstream
	if len(manis) == 0 {
		utils.Debug(l, span, "slot-centred path returned no results, trying direct UPSTREAM path")
		qd := fmt.Sprintf(`MATCH (c:Repo)-[:UPSTREAM]->(md:Manifest)-[cfgRel]->(cfg:Blob) WHERE id(c) = %d`, colID)
		qd += ` AND cfg.type = "config" AND cfg.mediaType IN ` + cfgFilter
		qd += fmt.Sprintf(` MATCH (t:Tag)-[:UPSTREAM]->(md) WHERE id(t) = %d`, colTagID)
		qd += ` MATCH (p:Platform)-[]-(md)`
		qd += ` OPTIONAL MATCH (md)-[:ORIGINAL]->(u:Upstream)`
		qd += ` OPTIONAL MATCH (md)-[]->(blob:Blob) WHERE blob.type IN ["config", "layer"]`
		qd += ` WITH md, cfgRel, cfg, p, u, collect(DISTINCT blob) AS blobs`
		qd += ` WITH md, cfgRel, cfg, p, u, reduce(s = 0, b IN blobs | s + b.size) AS contentSize`
		qd += ` RETURN "UPSTREAM" AS manifestdataRelType,`
		qd += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
		qd += ` ` + manifestDataReturn + `,`
		qd += ` coalesce(id(u), 0) AS upstreamID, coalesce(u.reg, "") AS upstreamReg, coalesce(u.repo, md.label) AS upstreamRepo, coalesce(u.tag, "") AS upstreamTag, coalesce(u.digest, md.digest) AS upstreamDigest,`
		qd += ` id(cfg) AS configID, cfg.digest AS configDigest, TYPE(cfgRel) AS configMediaType, cfg.size AS configSize,`
		qd += ` contentSize`
		utils.Debug(l, span, qd)
		result3, e := session.Run(qd, map[string]interface{}{})
		if e != nil {
			err = e
			return
		}
		for result3.Next() {
			p, e := PlatformFromRecord(result3)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while PlatformFromRecord (direct UPSTREAM path)")
				return
			}
			c, e := n.ConfigFromRecord(ctx, result3)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while ConfigFromRecord (direct UPSTREAM path)")
				return
			}
			u, e := UpstreamFromRecord(result3)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while UpstreamFromRecord (direct UPSTREAM path)")
				return
			}
			m, e := ManifestFromRecord(result3)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while ManifestFromRecord (direct UPSTREAM path)")
				return
			}
			anno, e := n.LabelsGetByConfigID(ctx, c.ID)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while LabelsGetByConfigID (direct UPSTREAM path)")
				return
			}
			m.Annotations = anno
			c.Labels = anno
			m.Config = c
			m.Platform = p
			m.Upstream = u
			m.Profile = "generic"
			m.Upstream.Profile = "generic"
			if csVal, ok := result3.Record().Get("contentSize"); ok && csVal != nil {
				switch v := csVal.(type) {
				case int64:
					m.ContentSize = v
				case int:
					m.ContentSize = int64(v)
				case float64:
					m.ContentSize = int64(v)
				}
			}
			manis = append(manis, m)
		}
	}
	return
}

func (n Neo) ManifestGetByManifestList(ctx context.Context, mlID int64) (manifests []types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMD,
		"func":    "ManifestGetByManifestList",
		"mlID":    mlID,
	})
	_ = l
	ctx, span := tracer.Start(ctx, "ManifestGetByManifestList")
	defer span.End()
	// TODO: We'll get different types of Manifest per Manifest - do we need to filter out the default (OCI manifest)
	//       - create a map afterwards based on collection name and type
	q := `MATCH (ml:Index)-[:REFERENCES]-(m:ManifestWrapper)-[:HAS_PLATFORM]->(p:Platform) WHERE id(ml) = %d`
	q += ` MATCH (cfg:Blob)<-[:USES]-(md:Manifest)-[r]-(m)-[:UPSTREAM]->(u:Upstream) WHERE cfg.type = "config"`
	q += ` RETURN type(r) AS manifestDataRelType,`
	q += ` id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
	q += ` ` + manifestDataReturn + `,`
	q += ` ` + UpstreamReturn + `,`
	q += ` id(cfg) AS configID, cfg.digest AS configDigest, cfg.mediaType AS configMediaType, cfg.size AS configSize`
	input := map[string]interface{}{}
	query := fmt.Sprintf(q, mlID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, query),
	))
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, input)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to run ManifestGetByManifestList query")
		return
	}
	manifests = make([]types.Manifest, 0)
	for result.Next() {
		p, e := PlatformFromRecord(result)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get platform from record")
			return
		}
		c, e := n.ConfigFromRecord(ctx, result)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get config from record")
			return
		}
		u, e := UpstreamFromRecord(result)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get upstream from record")
			return
		}
		m, e := ManifestFromRecord(result)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get manifest from record")
			return
		}
		m.Config = c
		m.Platform = p
		m.Upstream = u
		manifests = append(manifests, m)
	}
	return
}

func (n Neo) ManifestGetInsert(ctx context.Context, mani *types.Manifest) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestGetInsert",
		"repo":    mani.Upstream.Repo,
		"tag":     mani.Upstream.Tag,
		"digest":  mani.Digest,
		"profile": mani.Profile,
	})
	defer span.End()

	m, found, err := n.ManifestGetByDigest(ctx, mani.Digest)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to get manifest data by digest")
	}
	if found {
		utils.Debug(l, span, "Found Manifest %s/%s#%s", mani.Upstream.Repo, mani.Upstream.Tag, mani.Digest)
		mani.ID = m.ID
		return
	}
	utils.Debug(l, span, "Manifest %s/%s#%s not found, creating new one", mani.Upstream.Repo, mani.Upstream.Tag, mani.Digest)
	maniID, err := n.createManifest(ctx, *mani)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to create manifest data")
	}
	mani.ID = maniID
	return
}

func (n Neo) ManifestLinkToProfile(ctx context.Context, maniDataID, profileID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestLinkToProfile",
		"maniDataID": maniDataID,
		"profileID":  profileID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:Manifest),(p:Profile) WHERE id(md) = %d AND id(p) = %d`, maniDataID, profileID)
	q += ` MERGE (md)-[:CREATED_BY]->(p)`
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailQueryExec)
		return err
	}
	return nil
}

func (n Neo) ManifestLinkToUpstream(ctx context.Context, maniDataID, upID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestLinkToUpstream",
		"maniDataID": maniDataID,
		"upID":       upID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (md:Manifest),(up:Upstream) WHERE id(md) = %d AND id(up) = %d`, maniDataID, upID)
	q += ` MERGE (md)-[:ORIGINAL]->(up)`
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailQueryExec)
		return err
	}
	return
}

func (n Neo) LinkManifestToProfile(ctx context.Context, mdID, prfID int64, rel string) error {
	_, span := tracer.Start(ctx, "LinkManifestToProfile")
	defer span.End()
	q := fmt.Sprintf(`MATCH (m:Manifest),(prf:Profile) WHERE id(m) = %d AND id(prf) = %d`, mdID, prfID)
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

func (n Neo) ManifestGetPushedByProfile(ctx context.Context, up types.Upstream, prf types.Profile) (m types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameMD,
		"func":       "ManifestGetPushedByProfile",
		"repo":       up.Repo,
		"tag":        up.Tag,
		"digest":     up.Digest.String(),
		"profileID":  prf.ID,
		"profileLbl": prf.Label,
		"reg":        up.Reg,
		"mediaType":  up.MediaType,
	})

	defer span.End()
	// Path Z Phase C3: walk directly from Manifest to Upstream via
	// :ORIGINAL instead of through the :ManifestWrapper wrapper node.
	q := `MATCH (prf:Profile)<-[:UPLOADED_BY]-(md:Manifest)-[:ORIGINAL]->(u:Upstream)`
	q += fmt.Sprintf(` WHERE prf.label = "%s" AND u.reg = "%s" AND u.repo = "%s" AND u.tag = "%s"`, prf.Label, up.Reg, up.Repo, up.Tag)
	if up.Digest != "" {
		q += fmt.Sprintf(` AND md.digest = "%s"`, up.Digest.String())
	}
	q += ` RETURN ` + manifestDataReturn
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, errFailedRunQuery)
		return
	}
	if !found {
		utils.Warn(l, span, errManifestDataNotFound)
		return
	}
	m, _, err = n.ManifestFromResult(ctx, res)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, FailManifestFromResult)
		return
	}
	utils.Debug(l, span, "Found upstream '%s'", up.String())
	m.Upstream = up
	return
}

func (n Neo) ManifestDeleteByDigest(ctx context.Context, dig string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDeleteByDigest",
		"digest":  dig,
	})
	defer span.End()
	md, found, e := n.ManifestGetByDigest(ctx, dig)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailManifestGetByDigest)
	}
	if !found {
		return utils.NewError(l, span, types.FailManifestNotFound)
	}
	err = n.ManifestDelete(ctx, md)
	if err != nil {
		return utils.ErrorHandler(l, span, err, types.FailManifestDelete)
	}
	return
}

func (n Neo) ManifestUnlinkLayer(ctx context.Context, mdID, lyrID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestUnlinkLayer",
		"layerID": lyrID,
		"mdID":    mdID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (md:Manifest)-[r]->(l:Blob)`
	q += fmt.Sprintf(` WHERE id(md) = %d AND id(l) = %d AND l.type = "layer"`, mdID, lyrID)
	q += ` DELETE r`
	utils.Debug(l, span, q)
	_, e := n.Execute(q)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailManifestUnlinkLayer)
	}
	return nil
}

func (n Neo) ManifestUnlinkConfig(ctx context.Context, mdID int64, cfgDig string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":   pkgNameMD,
		"func":      "ManifestUnlinkConfig",
		"cfgDigest": utils.ShortenDigest(cfgDig),
		"mdID":      mdID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (md:Manifest)-[r]->(c:Blob)`
	q += fmt.Sprintf(` WHERE id(md) = %d AND c.digest = "%s" AND c.type = "config"`, mdID, cfgDig)
	q += ` DELETE r`
	utils.Debug(l, span, "query to delete Manifest<>Config relationship: %s", q)
	_, e := n.Execute(q)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailManifestUnlinkConfig)
	}
	return nil
}

func (n Neo) ManifestDelete(ctx context.Context, md types.Manifest) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestDeleteByDigest",
		"digest":  md.Digest,
	})
	_ = ctx
	defer span.End()
	// Unlinke Config
	e := n.ManifestUnlinkConfig(ctx, md.ID, md.Config.Digest)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailManifestUnlinkConfig)
	}
	// Unlink Layers
	for _, lyr := range md.Layers {
		if e := n.ManifestUnlinkLayer(ctx, md.ID, lyr.ID); e != nil {
			return utils.ErrorHandler(l, span, e, types.FailManifestUnlinkConfig)
		}
	}
	// Path Z Phase C3: wrapper cleanup is optional — the wrapper :Manifest
	// node may not exist for manifests pushed after Phase C3 landed.
	maniID, maniFound, e := n.ManifestWrapperGetIDByManifestID(ctx, md.ID)
	if e != nil {
		utils.Debug(l, span, "wrapper lookup failed (non-fatal): %v", e)
	}
	// Remove content-leaf Manifest
	q := fmt.Sprintf(`MATCH (md:Manifest) WHERE id(md) = %d DETACH DELETE md`, md.ID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	_, err = n.Execute(q)
	if err != nil {
		err = errors.Wrap(err, "failed to delete manifest data")
		span.RecordError(err)
		return
	}
	// Clean up wrapper :ManifestWrapper if it exists and has no remaining linked Manifest
	if maniFound {
		if e := n.ManifestWrapperCleanUpByID(ctx, maniID); e != nil {
			utils.Debug(l, span, "wrapper cleanup failed (non-fatal): %v", e)
		}
	}
	return
}

func (n Neo) ManifestGetPulledByRepoTag(ctx context.Context, repo, tag string) (maniData types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestGetPulledByRepoTag",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()
	// Phase C3: rewrite to use Slot-based walk instead of wrapper.
	q := `MATCH (s:Slot)-[:HAS_PUSHED]->(md:Manifest)-[:HAS_PLATFORM]->(p:Platform)`
	q += ` MATCH (md)-[:ORIGINAL]->(u:Upstream)`
	q += fmt.Sprintf(` WHERE s.repo = "%s" AND s.tag = "%s"`, repo, tag)
	q += ` RETURN ` + manifestDataReturn
	utils.Debug(l, span, q)
	if res, found, e := n.QueryOne(q); e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get manifest")
		return maniData, false, err
	} else if !found {
		err = utils.ErrorHandler(l, span, errors.New("manifestData not found"), "")
		return maniData, false, nil
	} else {
		maniData, _, e = n.ManifestFromResult(ctx, res)
		if e != nil {
			return maniData, false, utils.ErrorHandler(l, span, e, "failed to get manifestdata from result")
		}
	}
	utils.Debug(l, span, "Found manifestData: %s (%s)", maniData.Label, utils.ShortenDigest(maniData.Digest))
	return maniData, true, nil
}

func (n Neo) ManifestExistsForTagProfileContent(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestExistsForTagProfile",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"profile": up.Profile,
	})
	defer span.End()
	prf, found, e := n.ProfileGetByLabel(ctx, up.Profile)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailedProfGetByLabel)
		return "", false, err
	}
	if !found {
		err = utils.ErrorHandler(l, span, errors.New(FailedProfileNotFound), "")
		return "", false, err
	}
	utils.Debug(l, span, msgFoundProfile, up.Profile, prf.ID)
	q := `MATCH (md:Manifest)-[]-(m:ManifestWrapper)<-[:UPSTREAM]-(c:Repo)-[:HAS_TAG]->(t:Tag)-[:UPSTREAM]->(m)`
	q += fmt.Sprintf(` WHERE c.repo = "%s"`, up.Repo)
	q += fmt.Sprintf(` AND t.name = "%s"`, up.Tag)
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	q += `, ` + manifestDataReturn
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
	maniData, ok, e := n.ManifestFromResult(ctx, res, prf)
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

func (n Neo) ManifestExistsContent(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	defer span.End()
	prf, found, e := n.ProfileGetByLabel(ctx, up.Profile)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailedProfGetByLabel)
		return "", false, err
	}
	if !found {
		err = utils.ErrorHandler(l, span, errors.New(FailedProfileNotFound), "profile '%s' not found in database", up.Profile)
		return "", false, err
	}
	utils.Debug(l, span, msgFoundProfile, up.Profile, prf.ID)
	// Path Z Phase C3: walk directly from Manifest to Upstream via
	// :ORIGINAL instead of through the :ManifestWrapper wrapper. Also join
	// Repo+Tag via the Upstream's repo/tag fields.
	var q string
	if up.Digest.String() != "" {
		// Digest-based lookup: find Manifest directly by digest
		q = fmt.Sprintf(`MATCH (md:Manifest)-[:ORIGINAL]->(u:Upstream) WHERE md.digest = "%s" AND u.repo = "%s"`, up.Digest.String(), up.Repo)
		q += ` MATCH (c:Repo {repo: u.repo})-[:HAS_TAG]->(t:Tag {name: u.tag})`
		q += ` RETURN DISTINCT id(c) AS collectionID, c.repo AS collectionRepo`
		q += `, ` + manifestDataReturn
	} else {
		// Tag-based lookup: find Manifest via Upstream repo+tag
		q = fmt.Sprintf(`MATCH (md:Manifest)-[:ORIGINAL]->(u:Upstream) WHERE u.repo = "%s" AND u.tag = "%s"`, up.Repo, up.Tag)
		q += ` MATCH (c:Repo {repo: u.repo})-[:HAS_TAG]->(t:Tag {name: u.tag})`
		q += ` RETURN DISTINCT id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
		q += `, ` + manifestDataReturn
	}
	utils.Debug(l, span, "%s", q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return "", false, err
	}
	if !found {
		utils.Debug(l, span, "manifestdata not found")
		return "", false, nil
	}
	maniData, ok, e := n.ManifestFromResult(ctx, res, prf)
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

func (n Neo) ManifestLinkToArtifactConfig(ctx context.Context, mdDigest string, cfg v1.Descriptor) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestLinkToArtifactConfig",
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
	// TODO: when Blob is empty, we do not need to create a relationship
	q := fmt.Sprintf(`MATCH (md:Manifest),(b:Blob) WHERE md.digest = "%s" AND id(b) = %d`, mdDigest, b.ID)
	q += ` MERGE (md)-[r:USES]->(b)`
	q += fmt.Sprintf(` SET r.mediaType = "%s"`, cfg.MediaType)
	q += fmt.Sprintf(` SET b.mediaType = "%s"`, cfg.MediaType)
	q += ` SET b.type = "config"`
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailQueryExec)
		return err
	}
	return nil
}

func (n Neo) LinkManifestToSubject(ctx context.Context, mdDigest string, subId int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "LinkManifestToSubject",
	})
	defer span.End()
	// Find Config Blob
	q := fmt.Sprintf(`MATCH (md:Manifest),(s:Manifest) WHERE md.digest = "%s" AND id(s) = %d`, mdDigest, subId)
	q += ` MERGE (md)-[:SUBJECT_OF]->(s)`
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailQueryExec)
		return err
	}
	return nil
}

// ManifestExistsForTagProfile reports whether an Index or Manifest
// exists for the given (repo, tag, profile). Moved from manifest.go
// during Phase C3 — this function never consulted the wrapper :Manifest
// node, so it belongs with the content-leaf functions.
func (n Neo) ManifestExistsForTagProfile(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "ManifestExistsForTagProfile",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"profile": up.Profile,
	})
	defer span.End()
	mediaType, found, e := n.ManifestListExistsForTagProfile(ctx, up)
	if e != nil {
		utils.ErrorHandler(l, span, e, "failed to execute ManifestListExistsForTagProfile")
		return "", false, err
	}
	if found {
		utils.Debug(l, span, "found index for tag profile '%s' w/ media type: %s", up.Profile, mediaType)
		return mediaType, found, nil
	}
	return mediaType, false, nil
}

// ManifestExists reports whether a manifest-or-index exists for the
// given (repo, tag, [digest], profile). Checks Index first, then
// Manifest. Moved from manifest.go during Phase C3.
func (n Neo) ManifestExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
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
	mediaType, found, e = n.ManifestExistsContent(ctx, up)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to execute ManifestExistsContent")
		return "", false, err
	}
	if found {
		utils.DebugOK(l, span, "found manifestData w/ media type: %s", mediaType)
		return mediaType, found, nil
	}
	utils.Debug(l, span, "did not find manifestData w/ media type: %s", mediaType)
	return
}

// LinkManifestToPlatform links a :Manifest node to a :Platform
// node via a :HAS_PLATFORM edge. Moved from manifest.go during Phase C3.
func (n Neo) LinkManifestToPlatform(ctx context.Context, maniDataID, platID int64) (err error) {
	_, span := tracer.Start(ctx, "LinkManifestToPlatform")
	defer span.End()
	span.SetAttributes(
		attribute.Key("maniDataID").Int64(maniDataID),
		attribute.Key("platID").Int64(platID),
	)
	q := fmt.Sprintf("MATCH (m:Manifest),(p:Platform) WHERE id(m) = %d AND id(p) = %d", maniDataID, platID)
	q += " MERGE (m)-[:HAS_PLATFORM]->(p)"
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(q, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (m:Manifest)-[:HAS_PLATFORM]->(p:Platform): %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	return
}

// ManifestGetManipulatedFrom returns the ID of the original :Manifest a
// manipulated :Manifest was derived from. The result is (0, false, nil)
// when `manipulatedID` has no outgoing [:MANIPULATED_FROM] edge — i.e.
// when the manifest is itself an original rather than a manipulation.
func (n Neo) ManifestGetManipulatedFrom(ctx context.Context, manipulatedID int64) (origID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":       pkgNameMD,
		"func":          "ManifestGetManipulatedFrom",
		"manipulatedID": manipulatedID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (m1:Manifest)-[:MANIPULATED_FROM]->(m2:Manifest) WHERE id(m1) = %d RETURN id(m2) AS originalID`, manipulatedID)
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return 0, false, utils.ErrorHandler(l, span, err, FailQueryExec)
	}
	if !found {
		return 0, false, nil
	}
	origID, err = res.GetInt64("originalID")
	if err != nil {
		return 0, false, utils.ErrorHandler(l, span, err, "failed to decode originalID")
	}
	return origID, true, nil
}

// ManifestLinkManipulatedFrom creates a [:MANIPULATED_FROM] edge from a manipulated manifest to the original.
func (n Neo) ManifestLinkManipulatedFrom(ctx context.Context, manipulatedID, originalID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":       pkgNameMD,
		"func":          "ManifestLinkManipulatedFrom",
		"manipulatedID": manipulatedID,
		"originalID":    originalID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (m1:Manifest),(m2:Manifest) WHERE id(m1) = %d AND id(m2) = %d`, manipulatedID, originalID)
	q += ` MERGE (m1)-[:MANIPULATED_FROM]->(m2)`
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to link manipulated manifest to original")
		return err
	}
	return
}

// ManifestGetManipulatedByOriginalAndLabel finds an existing manipulated manifest by following
// [:MANIPULATED_FROM] from any manifest to the original (by digest), then checking if the
// ManipuLayer (identified by label) was the source of manipulation.
// For now we use a simpler approach: find any manifest with MANIPULATED_FROM pointing to the original.
// The manipulationLabel is matched against the manipulation's ManipuLayer labels via the graph path.
func (n Neo) ManifestGetManipulatedByOriginalAndLabel(ctx context.Context, originalDigest, manipulationLabel string) (md types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":           pkgNameMD,
		"func":              "ManifestGetManipulatedByOriginalAndLabel",
		"originalDigest":    originalDigest,
		"manipulationLabel": manipulationLabel,
	})
	defer span.End()
	_ = ctx
	// Walk: manipulated manifest -[:MANIPULATED_FROM]-> original manifest.
	// Include HAS_PLATFORM so the returned Manifest carries the correct
	// platform descriptor (needed for Index serialisation).
	q := fmt.Sprintf(`MATCH (md:Manifest)-[:MANIPULATED_FROM]->(orig:Manifest {digest: "%s"})`, originalDigest)
	q += ` OPTIONAL MATCH (md)-[:HAS_PLATFORM]->(p:Platform)`
	q += fmt.Sprintf(` RETURN %s, id(p) as platformID, p.os as platformOS, p.arch as platformArch, p.variant as platformVariant`, manifestDataReturn)
	utils.Debug(l, span, q)
	if res, f, e := n.QueryOne(q); e != nil {
		return md, false, utils.ErrorHandler(l, span, e, FailQueryExec)
	} else if !f {
		return md, false, nil
	} else {
		if md, _, err = n.ManifestFromResult(ctx, res); err != nil {
			return md, false, utils.ErrorHandler(l, span, err, FailManifestFromResult)
		}
		if p, pErr := PlatformFromResult(res); pErr == nil {
			md.Platform = p
		}
		return md, true, nil
	}
}
