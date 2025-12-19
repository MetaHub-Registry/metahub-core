package neo

import (
	"context"
	"fmt"

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

var (
	configMediaType = `unknownConfig`
	pkgNameConfig   = "neo/config.go"
)

func (n Neo) Config_GetID_ByDigest(ctx context.Context, dig string) (cfgID int64, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameConfig,
		"func":    "Config_GetID_ByDigest",
	})
	ctx, span := tracer.Start(ctx, "Config_GetID_ByDigest")
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (c:Blob) WHERE c.digest = "%s" RETURN id(c) as ID`, dig)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	cfgID, err = res.GetInt64("ID")
	return
}

func (n Neo) Config_Get_ByManiDataID(ctx context.Context, mdID int64) (cfg types.Config, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameConfig,
		"func":       "Config_Get_ByManiDataID",
		"maniDataID": mdID,
	})
	defer span.End()
	//q += ` MATCH (m)-[]-(md:ManifestData)-[rel]-(cfg:Blob) WHERE TYPE(rel) STARTS WITH "docker_container_image" OR TYPE(rel) STARTS WITH "oci_image_config"`
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[rel]->(n:Blob) WHERE id(md) = %d AND (TYPE(rel) STARTS WITH "docker_container_image" OR TYPE(rel) STARTS WITH "oci_image_config")`, mdID)
	q += ` RETURN id(n) AS configID, n.digest AS configDigest, n.size AS configSize`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		utils.ErrorHandler(l, span, err, "Failed to run query")
		return
	}
	if !found {
		utils.Debug(l, span, "Config not found for manifestDataID %d", mdID)
		return
	}
	cfg, err = n.ConfigFromResult(ctx, res)
	if err != nil {
		utils.ErrorHandler(l, span, err, "failed to create config from DB result")
		return
	}
	return
}

func (n Neo) Config_Get_ByID(ctx context.Context, cfgID int64) (cfg types.Config, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameConfig,
		"func":     "Config_Get_ByID",
		"configID": cfgID,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (n:Blob) WHERE id(n) = %d`, cfgID)
	q += ` RETURN id(n) AS configID, n.digest AS configDigest, n.size AS configSize`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "Failed to run query")
		return
	}
	if !found {
		utils.Debug(l, span, "Config not found for ID %d", cfgID)
		return
	}
	cfg, err = n.ConfigFromResult(ctx, res)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to create config from DB result")
		return
	}
	return
}

func (n Neo) Config_GetByDigest(ctx context.Context, dig string) (cfg types.Config, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "Config_GetByDigest",
	})
	_ = ctx
	defer span.End()
	if dig == "" {
		err = utils.NewError(l, span, "digest is empty")
		return
	}
	shortDig := utils.ShortenDigest(dig)
	span.SetAttributes(
		attribute.Key("dig").String(utils.ShortenDigest(dig)),
	)
	q := fmt.Sprintf(`MATCH (n:Blob) WHERE n.digest = "%s"`, dig)
	q += ` RETURN id(n) AS configID, n.digest AS configDigest, n.size AS configSize`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "Failed to run query")
		return
	}
	if !found {
		utils.Debug(l, span, "Config '%s' not found", shortDig)
		return
	}
	cfg, err = n.ConfigFromResult(ctx, res)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to create config from DB result")
		return
	}
	return
}

func (n Neo) Config_GetByID(ctx context.Context, cfgID int64) (cfg types.Config, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameConfig,
		"func":    "Config_GetByID",
	})
	ctx, span := tracer.Start(ctx, "Config_GetByID")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("cfgID").Int64(cfgID),
	)
	q := fmt.Sprintf(`MATCH (n:Blob) WHERE id(n) = %d`, cfgID)
	q += ` RETURN id(n) AS configID, n.digest AS configDigest, n.size AS configSize`
	l.Debug(q)
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
		msg := fmt.Sprintf("Config '%d' not found", cfgID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		return
	}
	cfg, err = n.ConfigFromResult(ctx, res)
	if err != nil {
		err = errors.Wrap(err, "failed to create config from DB result")
		l.Error(err)
		span.RecordError(err)
		return
	}
	return
}

func (n Neo) ConfigDeleteByDigest(ctx context.Context, dig string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "ConfigDeleteByDigest",
		"digest":  dig,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (c:Blob)<-[]-(md:ManifestData) WHERE c.digest = "%s" RETURN count(md) AS count`, dig)
	utils.Debug(l, span, q)
	if res, _, err := n.QueryOne(q); err != nil {
		return utils.ErrorHandler(l, span, err, types.FailNeoExecute)
	} else {
		count, _ := res.GetInt("count")
		if count > 0 {
			utils.Debug(l, span, "config '%s' still connected to %d manifestData -> Skipping", dig, count)
			return nil
		}
		utils.Debug(l, span, "config '%s' not connected to manifestData", dig)
	}
	// Check for labels
	if as, e := n.LabelsGetByConfigDigest(ctx, dig); e != nil {
		return utils.ErrorHandler(l, span, e, types.FailLabelsGetByConfigDigest)
	} else {
		for _, a := range as.ToSlice() {
			q := fmt.Sprintf(`MATCH (c:Blob)-[]->(l:Label) WHERE (NOT c.digest = "%s") AND id(l) = %d`, dig, a.ID)
			q += ` RETURN count(c) AS countConfig`
			utils.Debug(l, span, q)
			if res, _, e := n.QueryOne(q); e != nil {
				return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
			} else {
				countConfig, _ := res.GetInt("countConfig")
				if countConfig == 0 {
					q := fmt.Sprintf(`MATCH (l:Label) WHERE id(l) = %d DETACH DELETE l`, a.ID)
					if _, e := n.Execute(q); e != nil {
						return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
					}
					utils.Debug(l, span, "removed label with ID '%d'", a.ID)
				}
			}
		}
	}
	q = fmt.Sprintf(`MATCH (n:Blob) WHERE n.digest = "%s" DETACH DELETE n`, dig)
	_, e := n.Execute(q)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	}

	return nil
}

func (n Neo) Config_GetInsert_ByOCIManifest(ctx context.Context, ociMani types.OCIManifest, label string) (cfg types.Config, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":   pkgNameConfig,
		"func":      "Config_GetInsert_ByOCIManifest",
		"cfgDigest": ociMani.Config.Digest.String(),
	})
	shortDig := utils.ShortenDigest(ociMani.Config.Digest.String())
	ctx, span := tracer.Start(ctx, "Config_GetInsert_ByOCIManifest")
	defer span.End()
	if ociMani.Config.Digest.String() == "" {
		err = fmt.Errorf("config digest is emtpy")
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		l.Error(err)
		return
	}
	cfg, found, err = n.Config_GetByDigest(ctx, ociMani.Config.Digest.String())
	if err != nil {
		l.Error(err)
		return
	}
	if found {
		msg := fmt.Sprintf("config '%s' found", shortDig)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		return
	}
	cfg = types.Config{
		Digest: ociMani.Config.Digest.String(),
		Size:   ociMani.Config.Size,
	}
	cfgId, err := n.createConfig(ctx, cfg)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		l.Error(err)
		return
	}
	cfg.ID = cfgId
	return
}

func (n Neo) createConfig(ctx context.Context, cfg types.Config) (cfgID int64, err error) {
	_, span := tracer.Start(ctx, "createConfig")
	defer span.End()
	label := utils.ShortenDigest(cfg.Digest)
	q := `CREATE (n:Blob {digest: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, cfg.Digest, cfg.Size, label)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", query),
	))
	res, err := n.Execute(query)
	if err != nil {
		logrus.Errorf("failed to create conifg: %v", err)
		span.RecordError(err)
		return
	}
	cfgID, err = res.GetInt64("ID")
	if err != nil {
		err = fmt.Errorf("createConfig: failed to get ID")
		span.RecordError(err)
		return
	}
	return
}

func (n Neo) Config_Insert(ctx context.Context, cfg types.Config) (cfgID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameConfig,
		"func":    "Config_Insert",
	})
	ctx, span := tracer.Start(ctx, "Config_Insert")
	defer span.End()
	span.SetAttributes(
		attribute.Key("digest").String(utils.ShortenDigest(cfg.Digest)),
	)
	gotCfg, success, err := n.Config_GetByDigest(ctx, cfg.Digest)
	if err != nil {
		return
	}
	if !success {
		msg := "config not found, creating new config"
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		cfgID, err = n.createConfig(ctx, cfg)
		if err != nil {
			return
		}
	} else {
		cfgID = gotCfg.ID
	}
	for k, v := range cfg.Cfg.Config.Labels {
		a := types.NewAnnotation(k, v)
		lID, e := n.Label_GetInsert(ctx, a)
		if e != nil {
			err = errors.Wrap(e, "failed to get or insert label")
			return
		}
		err = n.LinkConfigToLabel(ctx, cfgID, lID)
		if err != nil {
			err = errors.Wrap(err, "failed to link config to label")
			return
		}
	}
	return
}

func (n Neo) ConfigFromResult(ctx context.Context, res types.Result) (cfg types.Config, err error) {
	ctx, span := tracer.Start(ctx, "ConfigFromResult")
	defer span.End()
	err = res.CheckKeys("configID", "configDigest", "configSize")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("configID")
	dig, _ := res.GetStr("configDigest")
	mt, _ := res.GetStr("configMediaType")
	if mt == "" {
		mt, err = n.Config_GetMediaType_ByID(ctx, id)
		if err != nil {
			return cfg, err
		}
	}
	size, _ := res.GetInt64("configSize")
	cfg = types.Config{
		ID:        id,
		Digest:    dig,
		MediaType: mt,
		Size:      size,
	}
	return
}

func (n Neo) Config_GetMediaType_ByID(ctx context.Context, cfgID int64) (mt string, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameConfig,
		"func":    "Config_GetMediaType_ByID",
	})
	ctx, span := tracer.Start(ctx, "Config_GetMediaType_ByID")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("cfgID").Int64(cfgID),
	)
	q := fmt.Sprintf(`MATCH (cfg:Blob)<-[cfgRel]-(md:ManifestData) WHERE id(cfg) = %d`, cfgID)
	q += ` RETURN DISTINCT TYPE(cfgRel) AS configMediaType`
	res, found, err := n.QueryOne(q)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if !found {
		msg := fmt.Sprintf("mediaType for config not found, but it is a config (set to %s)", configMediaType)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		mt = configMediaType
		return
	}
	mt, err = res.GetStr("configMediaType")
	if err != nil {
		err = fmt.Errorf("createConfig: failed to get ID")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	mt = utils.ExpandMediaType(mt)
	return
}

func (n Neo) Config_Update(ctx context.Context, cfgID int64, cfg types.Config) (err error) {
	q := `MATCH (cfg:Blob) WHERE id(cfg) = %d SET cfg.digest = "%s", cfg.size = %d`
	query := fmt.Sprintf(q, cfgID, cfg.Digest, cfg.Size)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) LinkConfigToLabel(ctx context.Context, cfgID, labelID int64) (err error) {
	q := `MATCH (cfg:Blob), (l:Label) WHERE id(cfg) = %d AND id(l) = %d MERGE (cfg)-[:HAS_LABEL]->(l)`
	query := fmt.Sprintf(q, cfgID, labelID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) LinkConfigToAnnotation(ctx context.Context, cfgID, annotationID int64) (err error) {
	q := `MATCH (cfg:Blob), (a:Annotation) WHERE id(cfg) = %d AND id(a) = %d MERGE (cfg)-[:HAS_ANNOTATION]->(a)`
	query := fmt.Sprintf(q, cfgID, annotationID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) Config_AddLabel(ctx context.Context, cfgID int64, a types.Annotation) (err error) {
	// GetAnnotationID
	labelID, err := n.Label_GetInsert(ctx, a)
	if err != nil {
		logrus.Errorf("AddLabel : %v", err)
		return
	}
	q := `MATCH (cfg:Blob) WHERE id(cfg) = %d`
	q += ` MATCH (l:Label) WHERE id(l) = %d`
	q += ` MERGE (cfg)-[:HAS_LABEL]->(l)`
	query := fmt.Sprintf(q, cfgID, labelID)
	_, err = n.Execute(query)
	if err != nil {
		logrus.Errorf("Config_AddLabel: %v", err)
		return
	}
	return
}

func (n Neo) ConfigFromRecord(ctx context.Context, res neo4j.Result) (c types.Config, err error) {
	r := res.Record()
	v, ok := r.Get("configID")
	if !ok {
		err = fmt.Errorf("no configID found in record: %v", r)
		return
	}
	c.ID = v.(int64)
	v, ok = r.Get("configDigest")
	if !ok {
		err = fmt.Errorf("no configDigest found in record: %v", r)
		return
	}
	c.Digest = v.(string)
	v, ok = r.Get("configSize")
	if !ok {
		err = fmt.Errorf("no configSize found in record: %v", r)
		return
	}
	c.Size = v.(int64)
	v, ok = r.Get("configDigest")
	if !ok {
		err = fmt.Errorf("no configDigest found in record: %v", r)
		return
	}
	mt := v.(string)
	if mt == "" {
		mt, err = n.Config_GetMediaType_ByID(ctx, c.ID)
		if err != nil {
			return c, err
		}
	}
	c.MediaType = utils.ExpandMediaType(mt)
	return
}

func (n Neo) Configs_DeleteUnused(ctx context.Context) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameConfig,
		"func":    "Configs_DeleteUnused",
	})
	_, span := tracer.Start(ctx, "Configs_DeleteUnused")
	defer span.End()
	q := `MATCH (c:Blob) WHERE NOT (c)<-[]-(:ManifestData) DETACH DELETE c RETURN count(c) AS deletedConfigs`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Execute(q)
	if err != nil {
		err = errors.Wrap(err, "failed to delete unused configs")
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	deletedConfigs, err := res.GetInt("deletedConfigs")
	if err != nil {
		err = errors.Wrap(err, "failed to get count of deleted configs")
		return
	}
	msg := fmt.Sprintf("Deleted %d configs", deletedConfigs)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	return
}

func (n Neo) ConfigExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameConfig,
		"func":    "ConfigExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	ctx, span := tracer.Start(ctx, "neo.ConfigExists")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("digest").String(utils.ShortenDigest(up.Digest.String())),
	)
	q := `MATCH (c:Blob)`
	q += fmt.Sprintf(` WHERE c.digest = "%s"`, up.Digest.String())
	q += ` RETURN c.digest AS configDigest, c.size AS configSize, id(c) AS configID`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, found, e := n.QueryOne(q)
	if e != nil {
		msg := FailQueryExec
		err = errors.Wrap(e, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return "", false, err
	}
	if !found {
		msg := "config not found"
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		return "", false, nil
	}
	cfg, err := n.ConfigFromResult(ctx, res)
	if err != nil {
		msg := "failed to parse config from result"
		err = errors.Wrap(err, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(msg)
		span.SetStatus(codes.Error, err.Error())
		return "", false, err
	}

	msg := fmt.Sprintf("found config with type '%s'", cfg.MediaType)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
	return cfg.MediaType, true, nil
}

/*
func (n Neo) ConfigScrubByID(ctx context.Context, cfgID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "ConfigScrubByID",
		"layerID": cfgID,
	})
	_ = ctx
	defer span.End()
	key := "configDigest"
	// Delete config if not connected anymore (in Minio?)
	q := fmt.Sprintf(`MATCH (c:Blob)<-[r]-(:ManifestData)  WHERE id(c) = %d`, cfgID)
	q += ` RETURN count(r) AS relCount, c.digest AS configDigest`
	utils.Debug(l, span, "query to count relationships: %s", q)
	res, _, e := n.QueryOne(q)
	if e != nil {
		return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	}
	var cfgDig string
	if cfgDig, err = res.GetStr(key); err != nil {
		return utils.ErrorHandler(l, span, e, types.FailBrGet, key)
	}
	relCnt, _ := res.GetInt("relCount")
	if relCnt == 0 {
		e = n.ConfigDeleteByDigest(ctx, cfgDig)
		if e != nil {
			return utils.ErrorHandler(l, span, e, types.FailConfigDeleteByDigest)
		}
	} else {
		utils.Debug(l, span, "config '%s' still has %d relationships", utils.ShortenDigest(cfgDig), relCnt)
	}
	return nil
}
*/
