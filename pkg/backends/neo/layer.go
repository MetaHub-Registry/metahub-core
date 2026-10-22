package neo

import (
	"context"
	"fmt"

	"github.com/docker/distribution"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var ()

const (
	FailQueryExec                 = "query execution failed"
	layerMediaType                = `unknownLayer`
	pkgNameLyr                    = "neo/layer.go"
	layerReturn                   = `id(l) AS layerID, l.digest AS layerDigest, l.mediaType AS layerMediaType, l.size AS layerSize`
	FailGetLayersForManipuLayerID = "failed to get layers for ManipuLayerID"
	FailLayerGetByID              = "failed to get layer by ID"
)

func (n Neo) LayerPlainInsert(ctx context.Context, l types.Layer) (blobID int64, err error) {
	gotBlob, found, err := n.BlobGetByDigest(ctx, l.Digest)
	if err != nil {
		return
	}
	if !found {
		blobID, err = n.CreateLayer(l)
		if err != nil {
			return
		}
		return
	}
	// If layer already exists, return its ID
	blobID = gotBlob.ID
	return
}

func (n Neo) LayerGetByLayerAndManifestID(ctx context.Context, layerID, mdID int64) (lyr types.Layer, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":        pkgNameLyr,
		"func":           "LayerGetByLayerAndManifestID",
		"layerID":        layerID,
		"manifestDataID": mdID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (md:Manifest)-[r]->(n:Blob)`
	q += fmt.Sprintf(` WHERE id(n) = %d AND id(md) = %d`, layerID, mdID)
	q += ` RETURN id(n) AS layerID, n.digest AS layerDigest, n.size AS layerSize, n.mediaType AS layerMediaType`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return lyr, found, utils.ErrorHandler(l, span, err, FailLayerGetByID)
	}
	if !found {
		utils.Debug(l, span, "layer not found")
		return lyr, found, nil
	}
	utils.Debug(l, span, "layer found")
	lyr, err = n.LayerFromResult(ctx, res)
	if err != nil {
		return lyr, found, utils.ErrorHandler(l, span, err, "failed to create layer from result")
	}
	return
}

func (n Neo) LayerGetByID(ctx context.Context, layerID int64) (l types.Layer, found bool, err error) {
	q := `MATCH (n:Blob) WHERE id(n) = %d`
	q += ` RETURN id(n) AS ID, n.digest AS digest, n.size AS size`
	query := fmt.Sprintf(q, layerID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf(errFailedRunQueryFmt, err)
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
	}
	return
}

func (n Neo) LayerGetByDigest(ctx context.Context, dig string) (l types.Layer, found bool, err error) {
	q := `MATCH (n:Blob) WHERE n.digest = "%s"`
	q += ` RETURN id(n) AS ID, n.digest AS digest, n.size AS size`
	query := fmt.Sprintf(q, dig)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf(errFailedRunQueryFmt, err)
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
	}
	return
}

func (n Neo) LayerDeleteByID(ctx context.Context, lyrID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLyr,
		"func":    "LayerDeleteByID",
		"layerID": lyrID,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (n:Blob) WHERE id(n) = %d DETACH DELETE n`, lyrID)
	if _, e := n.Execute(q); e != nil {
		return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	}
	return nil
}

func (n Neo) LayerDeleteByDigest(ctx context.Context, dig string) (err error) {
	q := `MATCH (n:Blob) WHERE n.digest = "%s" DETACH DELETE n`
	query := fmt.Sprintf(q, dig)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf(errFailedRunQueryFmt, err)
		return
	}
	return err
}

/*
	func (n Neo) LayerScrubByID(ctx context.Context, lyrID int64) (err error) {
		l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
			"package": pkgNameLyr,
			"func":    "LayerScrub",
			"layerID": lyrID,
		})
		_ = ctx
		defer span.End()

		q := fmt.Sprintf(`MATCH (l:Blob)<-[r]-(:Manifest)  WHERE id(l) = %d`, lyrID)
		q += ` RETURN count(r) AS relCount`
		utils.Debug(l, span, q)
		res, _, e := n.QueryOne(q)
		if e != nil {
			return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
		}
		relCnt, _ := res.GetInt("relCount")
		if relCnt == 0 {
			e = n.LayerDeleteByID(ctx, lyrID)
			if e != nil {
				return utils.ErrorHandler(l, span, e, types.FailLayerDeleteByDigest)
			}
		} else {
			utils.Debug(l, span, "layer w/ ID %d still has %d relationships", lyrID, relCnt)
		}
		return nil
	}
*/

func (n Neo) GetLayer(layer types.Layer) (layerID int64, success bool, err error) {
	q := `MATCH (n:Blob) WHERE n.digest = "%s" AND n.size = %d RETURN id(n) AS ID`
	query := fmt.Sprintf(q, layer.Digest, layer.Size)
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
			err = fmt.Errorf("GetLayer: failed to get ID")
			return
		}
		layerID = v.(int64)
		success = true
	}
	return
}

func (n Neo) GetLayersForManifest(ctx context.Context, m types.Manifest) (layers []types.Layer, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLyr,
		"func":    "GetLayersForManifest",
		"digest":  m.Digest,
	})
	_ = ctx
	defer span.End()
	layers = []types.Layer{}
	q := fmt.Sprintf(`MATCH (l:Blob)<-[lyRel]-(m:Manifest) WHERE m.digest = "%s" AND l.type = "layer"`, m.Digest)
	q += ` RETURN id(l) AS ID, l.digest AS digest, l.mediaType AS mediaType, l.size AS size`
	utils.Debug(l, span, q)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{})
	if err != nil {
		logrus.Errorf(errFailedRunQueryFmt, err)
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
		layer.MediaType = utils.ExpandMediaType(v.(string))
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
	label := utils.ShortenDigest(layer.Digest)
	q := `CREATE (n:Blob {digest: "%s", size: %d, label: "%s", type: "layer"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, layer.Digest, layer.Size, label)
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

func (n Neo) LayerCreateByDescriptor(l distribution.Descriptor) (layerID int64, err error) {
	label := utils.ShortenDigest(l.Digest.String())
	q := `CREATE (n:Blob {digest: "%s", size: %d, label: "%s", type: "layer"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, l.Digest.String(), l.Size, label)
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

func (n Neo) GetLayersForManifestID(ctx context.Context, maniDataID int64) (layers []types.Layer, err error) {
	q := `MATCH (l:Blob)<-[r]-(m:Manifest) WHERE id(m) = %d AND l.type = "layer"`
	q += ` RETURN id(l) AS layerID, l.digest AS layerDigest, l.mediaType AS layerMediaType, l.size AS layerSize`
	q += ` ORDER BY r.order`
	return n.layersForManifestQuery(ctx, fmt.Sprintf(q, maniDataID))
}

// GetLayersForManifestDigest returns the ordered layers of the manifest with
// the given digest. Public callers address a manifest by digest; the element
// ID stays inside this query.
func (n Neo) GetLayersForManifestDigest(ctx context.Context, maniDigest string) (layers []types.Layer, err error) {
	q := `MATCH (l:Blob)<-[r]-(m:Manifest) WHERE m.digest = "%s" AND l.type = "layer"`
	q += ` RETURN id(l) AS layerID, l.digest AS layerDigest, l.mediaType AS layerMediaType, l.size AS layerSize`
	q += ` ORDER BY r.order`
	return n.layersForManifestQuery(ctx, fmt.Sprintf(q, maniDigest))
}

func (n Neo) layersForManifestQuery(ctx context.Context, query string) (layers []types.Layer, err error) {
	layers = []types.Layer{}
	res, err := n.Query(query)
	if err != nil {
		logrus.Errorf(errFailedRunQueryFmt, err)
		return
	}
	for _, r := range res {
		l, e := n.LayerFromResult(ctx, r)
		if e != nil {
			err = errors.Wrap(e, "failed to get layers for manifest")
			return
		}
		layers = append(layers, l)
	}
	return
}

func (n Neo) LayerFromResult(ctx context.Context, res types.Result) (ly types.Layer, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLyr,
		"func":    "LayerFromResult",
	})
	defer span.End()
	e := res.CheckKeys("layerID", "layerDigest", "layerSize")
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to check keys")
		return
	}
	id, _ := res.GetInt64("layerID")
	size, _ := res.GetInt64("layerSize")
	mt, _ := res.GetStr("layerMediaType")
	if mt == "" {
		mt, err = n.LayerGetMediaTypeByID(ctx, id)
		if err != nil {
			return ly, err
		}
	}
	dig, _ := res.GetStr("layerDigest")
	ly = types.Layer{
		ID:        id,
		Digest:    dig,
		MediaType: utils.ExpandMediaType(mt),
		Size:      size,
	}
	return
}

func (n Neo) LayerGetMediaTypeByID(ctx context.Context, lyID int64) (mt string, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLyr,
		"func":    "LayerGetMediaTypeByID",
		"layerID": lyID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (l:Blob)<-[layerRel]-(md:Manifest) WHERE id(l) = %d AND l.type = "layer"`, lyID)
	q += ` RETURN DISTINCT l.mediaType AS layerMediaType`
	utils.Debug(l, span, q)
	res, e := n.Query(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, errFailedRunQuery)
		return
	}
	switch len(res) {
	case 0:
		utils.Debug(l, span, "mediaType for layer not found, but it is a layer (set to %s)", layerMediaType)
		mt = layerMediaType
	case 1:
		r := res[0]
		mt, e = r.GetStr("layerMediaType")
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get layerMediaType")
			return
		}
		utils.Debug(l, span, "mediaType for layer found: %s", mt)
	default:
		utils.Debug(l, span, "multile results are found: %d", len(res))
		mts := set.CreateStringSet()
		for _, r := range res {
			lmt, e := r.GetStr("layerMediaType")
			if e != nil {
				return "", utils.ErrorHandler(l, span, e, "failed to get layerMediaType")
			}
			tempLMT := utils.ExpandMediaType(lmt)
			utils.Debug(l, span, "  > mediaType for layer found: %s", tempLMT)
			mts.Add(tempLMT)
		}
		if mts.Contains(v1.MediaTypeImageLayerGzip) {
			mt = v1.MediaTypeImageLayerGzip
			return
		}
		return
	}
	mt = utils.ExpandMediaType(mt)
	return
}

func (n Neo) LayerDescriporGet(ctx context.Context, desc v1.Descriptor) (layerID int64, found bool, err error) {
	q := `MATCH (n:Blob) WHERE n.digest = "%s" AND n.type = "layer" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, desc.Digest.String())
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

func (n Neo) LayerDescriptorGetInsert(ctx context.Context, desc v1.Descriptor) (layerID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLyr,
		"func":    "LayerDescriptorGetInsert",
	})
	defer span.End()
	_ = ctx
	layerID, found, err := n.LayerDescriporGet(ctx, desc)
	if err != nil {
		return 0, err
	}
	if found {
		return layerID, nil
	}
	// Create Layer
	label := utils.ShortenDigest(desc.Digest.String())
	q := `CREATE (n:Blob {digest: "%s", size: %d, label: "%s", type: "layer"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, desc.Digest.String(), desc.Size, label)
	utils.Debug(l, span, q)
	res, err := n.Execute(query)
	if err != nil {
		return 0, err
	}
	layerID, err = res.GetInt64("ID")
	if err != nil {
		return 0, utils.ErrorHandler(l, span, err, "failed to get id from result")
	}
	return
}

func (n Neo) LayersDeleteUnused(ctx context.Context) (err error) {
	_, span := tracer.Start(ctx, "LayersDeleteUnused")
	defer span.End()
	q := `MATCH (l:Blob) WHERE l.type = "layer" AND NOT (l)<-[]-(:Manifest) DETACH DELETE l RETURN count(l) AS deletedLayers`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
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
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogMessage, msg),
	))
	return
}

func (n Neo) LayerExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error) {
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
	q := fmt.Sprintf(`MATCH (l:Blob) WHERE l.digest = "%s" AND l.type = "layer"`, up.Digest.String())
	q += ` RETURN id(l) AS layerID, l.digest AS layerDigest, l.size AS layerSize`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return false, err
	}
	if !found {
		msg := "layer not found"
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, attrSevDebug),
			attribute.String(attrLogMessage, msg),
		))
		l.Debug(msg)
		return false, nil
	}
	_, err = n.LayerFromResult(ctx, res)
	if err != nil {
		msg := "failed to create Layer from result"
		err = errors.Wrap(err, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, attrSevError),
			attribute.String(attrLogMessage, msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return false, err
	}
	return true, nil
}

func (n Neo) LayerGetUpstream(ctx context.Context, lID int64) (up types.Upstream, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLyr,
		"func":    "neo.LayerGetUpstream",
		"layerID": lID,
	})
	defer span.End()
	_ = ctx
	// Phase 4 (deprecate-manifest-wrapper): walk Upstream directly from the
	// content-leaf :Manifest via :ORIGINAL instead of through the legacy
	// :ManifestWrapper wrapper node.
	q := `MATCH (u:Upstream)<-[:ORIGINAL]-(md:Manifest)-[]->(l:Blob)`
	q += fmt.Sprintf(` WHERE id(l) = %d AND l.type = "layer"`, lID)
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

func (n Neo) GetLayersForManipuLayerID(ctx context.Context, mplID int64) (layers []types.Layer, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLyr,
		"func":    "neo.GetLayersForManipuLayerID",
		"layerID": mplID,
	})
	defer span.End()
	layers = []types.Layer{}
	q := fmt.Sprintf(`MATCH (l:Blob)<-[r]-(mpl:ManipuLayer) WHERE id(mpl) = %d AND l.type = "layer"`, mplID)
	q += fmt.Sprintf(` RETURN %s`, layerReturn)
	q += ` ORDER BY r.order`
	utils.Debug(l, span, q)
	res, e := n.Query(q)
	if e != nil {
		return layers, utils.ErrorHandler(l, span, e, FailQueryExec)
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
