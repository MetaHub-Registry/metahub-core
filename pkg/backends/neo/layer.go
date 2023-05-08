package neo

import (
	"context"
	"fmt"

	"github.com/docker/distribution"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var ()

const (
	FailQueryExec  = "query execution failed"
	layerMediaType = `unknownLayer`
	pkgNameLyr     = "neo/layer.go"
)

func (n Neo) Layer_PlainInsert(ctx context.Context, l types.Layer) (layerID int64, err error) {
	gotL, success, err := n.LayerGetByDigest(l.Digest)
	if err != nil {
		return
	}
	if !success {
		layerID, err = n.CreateLayer(l)
		if err != nil {
			return
		}
		return
	}
	// If layer already exists, return its ID
	layerID = gotL.ID
	return
}

func (n Neo) Layer_Get_ByID(ctx context.Context, layerID int64) (l types.Layer, found bool, err error) {
	q := `MATCH (n:Layer) WHERE id(n) = %d`
	q += ` RETURN id(n) AS ID, n.digest AS digest, n.mediaType AS mediaType, n.size AS size`
	query := fmt.Sprintf(q, layerID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		found = true
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get ID")
			return
		}
		l.ID = v.(int64)
		v, ok = result.Record().Get("size")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get size")
			return
		}
		l.Size = v.(int64)
		v, ok = result.Record().Get("digest")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get digest")
			return
		}
		l.Digest = v.(string)
		v, ok = result.Record().Get("mediaType")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get mediaType")
			return
		}
		l.MediaType = v.(string)
	}
	return
}
func (n Neo) LayerGetByDigest(dig string) (l types.Layer, found bool, err error) {
	q := `MATCH (n:Layer) WHERE n.digest = "%s"`
	q += ` RETURN id(n) AS ID, n.digest AS digest, n.mediaType AS mediaType, n.size AS size`
	query := fmt.Sprintf(q, dig)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		found = true
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get ID")
			return
		}
		l.ID = v.(int64)
		v, ok = result.Record().Get("size")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get size")
			return
		}
		l.Size = v.(int64)
		v, ok = result.Record().Get("digest")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get digest")
			return
		}
		l.Digest = v.(string)
		v, ok = result.Record().Get("mediaType")
		if !ok {
			err = fmt.Errorf("LayerGetByDigest: failed to get mediaType")
			return
		}
		l.MediaType = v.(string)
	}
	return
}

func (n Neo) LayerDeleteByID(ctx context.Context, lyrID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMD,
		"func":    "LayerDeleteByID",
		"layerID": lyrID,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (n:Layer) WHERE id(n) = %d DETACH DELETE n`, lyrID)
	if _, e := n.Execute(q); e != nil {
		return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	}
	return nil

}
func (n Neo) LayerDeleteByDigest(dig string) (err error) {
	q := `MATCH (n:Layer) WHERE n.digest = "%s" DETACH DELETE n`
	query := fmt.Sprintf(q, dig)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}

	return err
}

func (n Neo) GetLayer(layer types.Layer) (layerID int64, success bool, err error) {
	q := `MATCH (n:Layer) WHERE n.digest = "%s" AND n.mediaType = "%s" AND n.size = %d RETURN id(n) AS ID`
	query := fmt.Sprintf(q, layer.Digest, layer.MediaType, layer.Size)
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
			err = fmt.Errorf("GetLayer: failed to get ID")
			return
		}
		layerID = v.(int64)
		success = true
	}
	return
}

func (n Neo) GetLayersForManifest(ctx context.Context, m types.ManifestData) (layers []types.Layer, err error) {
	shortDig := utils.ShortenDigest(m.Digest)
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameLyr,
		"func":    "GetLayersForManifest",
		"digest":  shortDig,
	})
	ctx, span := tracer.Start(ctx, "GetLayersForManifest")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("digest").String(shortDig),
	)
	layers = []types.Layer{}
	q := fmt.Sprintf(`MATCH (l:Layer)<-[lyRel]-(m:ManifestData) WHERE m.digest = "%s"`, m.Digest)
	q += ` RETURN id(l) AS ID, l.digest AS digest, TYPE(lyRel) AS mediaType, l.size AS size`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	for result.Next() {
		layer := types.Layer{}
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("GetLayersForManifest: failed to get ID")
			return
		}
		layer.NeoID = v.(int64)
		v, ok = result.Record().Get("digest")
		if !ok {
			err = fmt.Errorf("GetLayersForManifest: failed to get digest")
			return
		}
		layer.Digest = v.(string)
		v, ok = result.Record().Get("mediaType")
		if !ok {
			err = fmt.Errorf("GetLayersForManifest: failed to get mediaType")
			return
		}
		layer.MediaType = v.(string)
		v, ok = result.Record().Get("size")
		if !ok {
			err = fmt.Errorf("GetLayersForManifest: failed to get size")
			return
		}
		layer.Size = v.(int64)
		layers = append(layers, layer)
	}
	return
}

func (n Neo) CreateLayer(layer types.Layer) (layerID int64, err error) {
	label := utils.CreateLabelFromTypeAndDigest(layer.MediaType, layer.Digest)
	q := `CREATE (n:Layer {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, layer.Digest, layer.MediaType, layer.Size, label)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create layer: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("CreateLayer: failed to get ID")
			return
		}
		layerID = v.(int64)
	}
	return
}

func (n Neo) Layer_Create_ByDescriptor(l distribution.Descriptor) (layerID int64, err error) {
	label := utils.CreateLabelFromTypeAndDigest(l.MediaType, l.Digest.String())
	q := `CREATE (n:Layer {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, l.Digest.String(), l.MediaType, l.Size, label)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create layer: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("Layer_Create_ByDescriptor: failed to get ID")
			return
		}
		layerID = v.(int64)
	}
	return
}

func (n Neo) GetLayersForManifestID(ctx context.Context, maniID int64) (layers []types.Layer, err error) {
	layers = []types.Layer{}
	q := `MATCH (l:Layer)<-[r]-(m:ManifestData) WHERE id(m) = %d`
	q += ` RETURN id(l) AS layerID, l.digest AS layerDigest, TYPE(r) AS layerMediaType, l.size AS layerSize`
	q += ` ORDER BY r.order`
	query := fmt.Sprintf(q, maniID)
	res, err := n.Query(query)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	for _, r := range res {
		l, e := n.LayerFromResult(ctx, r)
		if e != nil {
			err = errors.Wrap(e, "failed to get layers for manifest id")
			return
		}
		layers = append(layers, l)
	}
	return
}

func (n Neo) LayerFromResult(ctx context.Context, res types.Result) (ly types.Layer, err error) {
	ctx, span := tracer.Start(ctx, "LayerFromResult")
	defer span.End()
	err = res.CheckKeys("layerID", "layerDigest", "layerSize")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("layerID")
	size, _ := res.GetInt64("layerSize")
	mt, _ := res.GetStr("layerMediaType")
	if mt == "" {
		mt, err = n.Layer_GetMediaType_ByID(ctx, id)
		if err != nil {
			return ly, err
		}
	}
	dig, _ := res.GetStr("layerDigest")
	ly = types.Layer{
		ID:        id,
		Digest:    dig,
		MediaType: mt,
		Size:      size,
	}
	return
}

func (n Neo) Layer_GetMediaType_ByID(ctx context.Context, lyID int64) (mt string, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameLyr,
		"func":    "Layer_GetMediaType_ByID",
		"layerID": lyID,
	})
	ctx, span := tracer.Start(ctx, "Layer_GetMediaType_ByID")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("layerID").Int64(lyID),
	)
	q := fmt.Sprintf(`MATCH (l:Layer)<-[layerRel]-(md:ManifestData) WHERE id(l) = %d`, lyID)
	q += ` RETURN DISTINCT TYPE(layerRel) AS layerMediaType`
	res, found, err := n.QueryOne(q)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if !found {
		msg := fmt.Sprintf("mediaType for layer not found, but it is a config (set to %s)", layerMediaType)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		mt = layerMediaType
		return
	}
	mt, err = res.GetStr("layerMediaType")
	if err != nil {
		err = fmt.Errorf("failed to get layerMediaType")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	mt = utils.ExpandMediaType(mt)
	return
}

func (n Neo) Layer_GetByTypeAndDigest(ctx context.Context, mt, dig string) (l types.Layer, found bool, err error) {
	ctx, span := tracer.Start(ctx, "Layer_GetByTypeAndDigest")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("dig").String(utils.ShortenDigest(dig)),
		attribute.Key("mediaType").String(mt),
	)
	q := `MATCH (n:Layer) WHERE n.digest = "%s"`
	q += ` RETURN id(n) AS layerID, n.digest AS layerDigest, n.mediaType AS layerMediaType, n.size AS layerSize`
	query := fmt.Sprintf(q, dig)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", query),
	))
	res, err := n.Query(query)
	if err != nil {
		return
	}
	if len(res) == 0 {
		// found is false by default
		return
	}
	for _, r := range res {
		l, err = n.LayerFromResult(ctx, r)
		if err != nil {
			err = errors.Wrap(err, "failed to get layer by type and digest")
		}
		switch {
		case l.MediaType == "unknown":
			tracing.SpanDebug(span, "Found layer with 'unkown' mediaType. Update to '%s'", mt)
			l.MediaType = mt
			label := utils.CreateLabelFromTypeAndDigest(mt, l.Digest)
			q := fmt.Sprintf(`MATCH (n:Layer) WHERE id(n) = %d SET n.mediaType = "%s", n.label = "%s"`, l.ID, mt, label)
			_, err = n.Execute(q)
			if err != nil {
				err = errors.Wrap(err, "failed while updating 'unkown' mediatype")
				span.RecordError(err)
				return
			}
			found = true
			return
		case l.MediaType == mt:
			found = true
			return
		default:
			logrus.Warnf("found layer with same digest but different media type: %s", l.Digest)
			label := utils.CreateLabelFromTypeAndDigest(mt, l.Digest)
			qDef := fmt.Sprintf(`CREATE (n:Layer {digest: "%s", mediaType: "%s", size: %d, label: "%s"})`, l.Digest, mt, l.Size, label)
			qDef += ` RETURN id(n) AS layerID, n.digest AS layerDigest, n.mediaType AS layerMediaType, n.size AS layerSize`
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.query", qDef),
			))
			resDef, e := n.Execute(qDef)
			if e != nil {
				err = errors.Wrap(e, "failed while creating new layer with different mediatype")
				span.RecordError(err)
				return
			}
			l, err = n.LayerFromResult(ctx, resDef)
			if err != nil {
				err = errors.Wrap(err, "failed to get layer by type and digest")
				span.RecordError(err)
				return
			}
			found = true
			return
		}
	}

	return
}

func (n Neo) Layer_DescriporGet(ctx context.Context, desc v1.Descriptor) (layerID int64, found bool, err error) {
	q := `MATCH (n:Layer) WHERE n.digest = "%s" AND n.mediaType = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, desc.Digest.String(), desc.MediaType)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return
	}
	if !found {
		return
	}
	layerID, err = res.GetInt64("ID")
	if err != nil {
		logrus.Errorf("failed to get layer id: %v", err)
	}
	return
}

func (n Neo) Layer_DescriporGetInsert(ctx context.Context, desc v1.Descriptor) (layerID int64, err error) {
	// check if layer exists
	layerID, found, err := n.Layer_DescriporGet(ctx, desc)
	if err != nil {
		return 0, err
	}
	if found {
		return layerID, nil
	}
	// Create Layer
	label := utils.CreateLabelFromTypeAndDigest(desc.MediaType, desc.Digest.String())
	q := `CREATE (n:Layer {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, desc.Digest.String(), desc.MediaType, desc.Size, label)
	res, err := n.Execute(query)
	if err != nil {
		return 0, err
	}
	layerID, err = res.GetInt64("ID")
	if err != nil {
		fmt.Printf("Query: \n%s\n", query)
		logrus.Fatal(err)
		logrus.Errorf("failed to get layer id: %v", err)
	}
	return
}

func (n Neo) Layers_DeleteUnused(ctx context.Context) (err error) {
	_, span := tracer.Start(ctx, "Layers_DeleteUnused")
	defer span.End()
	q := `MATCH (l:Layer) WHERE NOT (l)<-[]-(:ManifestData) DETACH DELETE l RETURN count(l) AS deletedLayers`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Execute(q)
	if err != nil {
		err = errors.Wrap(err, "failed to delete unused layers")
		return
	}
	deletedLayers, err := res.GetInt("deletedLayers")
	if err != nil {
		err = errors.Wrap(err, "failed to get count of deleted layers")
		return
	}
	msg := fmt.Sprintf("Deleted %d layers", deletedLayers)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	return
}

func (n Neo) Layer_GetUpstreamRegistry(ctx context.Context, up upstream.UpstreamTriple) (registry string, err error) {
	shortMT := utils.ShortenMediaType(up.MediaType)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLyr,
		"func":    "Layer_GetUpstreamRegistry",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("digest").String(utils.ShortenDigest(up.Digest.String())),
	)
	q := `MATCH (l:Layer)<-[r]-(md:ManifestData)<-[]-(m:Manifest)-[:UPSTREAM]-(u:Upstream) `
	q += fmt.Sprintf(` WHERE l.digest = "%s" AND TYPE(r) = "%s"`, up.Digest.String(), shortMT)
	q += " RETURN u.reg AS registry"
	utils.Debug(l, span, q)

	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return "", err
	}
	if !found {
		err = errors.New("upstream registry not found")
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", err.Error()),
		))
		span.RecordError(err)
		l.Error(err)
		return "", err
	}
	reg, err := res.GetStr("registry")
	if err != nil {
		msg := "failed to get registry from result"
		err = errors.Wrap(err, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(err)
		return "", err
	}
	return reg, nil
}

func (n Neo) LayerExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameLyr,
		"func":    "LayerExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	ctx, span := tracer.Start(ctx, "neo.LayerExists")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("digest").String(utils.ShortenDigest(up.Digest.String())),
	)
	q := fmt.Sprintf(`MATCH (l:Layer) WHERE l.digest = "%s"`, up.Digest.String())
	q += ` RETURN id(l) AS layerID, l.digest AS layerDigest, l.size AS layerSize`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return "", false, err
	}
	if !found {
		msg := "layer not found"
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		return "", false, nil
	}
	ly, err := n.LayerFromResult(ctx, res)
	if err != nil {
		msg := "failed to create Layer from result"
		err = errors.Wrap(err, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return "", false, err
	}

	l.Debugf("found layer with type '%s'", ly.MediaType)
	return ly.MediaType, true, nil
}

func (n Neo) LayerGetUpstream(ctx context.Context, lID int64) (up types.Upstream, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLyr,
		"func":    "neo.LayerGetUpstream",
		"layerID": lID,
	})
	defer span.End()
	_ = ctx
	q := `MATCH (u:Upstream)<-[]-(m:Manifest)-[]->(md:ManifestData)-[]->(l:Layer)`
	q += fmt.Sprintf(` WHERE id(l) = %d`, lID)
	q += ` RETURN DISTINCT id(u) AS upstreamID, u.reg AS upstreamReg, u.repo AS upstreamRepo, u.tag AS upstreamTag, u.digest AS upstreamDigest`
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return
	}
	if !found {
		err = utils.NewError(l, span, "upstream not found")
		return
	}
	up, e = UpstreamFromResult(res)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to create Upstream from result")
		return
	}
	return
}
