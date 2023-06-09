package magazine

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameLayer = "magazine/layer.go"
)

func (mag Magazine) PullLayerFromAllRegs(ctx context.Context, up upstream.UpstreamTriple) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "mag.PullLayerFromAllRegs",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	defer span.End()
	regs, ok := mag.GetConfig("registries")
	if !ok {
		err = utils.NewError(l, span, "Fail to get registries from config")
		return
	}
	for _, reg := range strings.Split(regs, ",") {
		up.Reg = reg
		utils.Debug(l, span, "Using registry: %s", reg)
		e := mag.PullLayer(ctx, up)
		if e != nil {
			utils.Warn(l, span, "Error pulling layer: %v", e.Error())
		} else {
			utils.Debug(l, span, "PullLayer successful - break")
			break
		}
	}
	return

}

func (mag Magazine) PullLayer(ctx context.Context, up upstream.UpstreamTriple) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "mag.PullLayer",
		"reg":     up.Reg,
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	defer span.End()
	b, blobs, err := mag.GetBlobByUpstreamTripleByDigest(ctx, up, up.Digest.String())
	if err != nil {
		err = errors.Wrapf(err, "failed to get blob for %s", up.GetLabel())
		l.Error(err)
		return err
	}
	_ = blobs
	err = mag.Minio.PutLayer(ctx, b, up.Digest.String())
	if err != nil {
		logrus.Error(err)
		return err
	}
	return
}

func (mag Magazine) parseCacheLayers() (cacheLayers int, err error) {
	cacheLayers = -1
	if cLyr, ok := mag.GetConfig("cache-layers"); ok {
		switch cLyr {
		case "all":
			cacheLayers = -1
		case "none":
			cacheLayers = 0
		default:
			if cacheLayers, err = strconv.Atoi(cLyr); err != nil {
				return cacheLayers, fmt.Errorf("failed to parse cache-layers: %s", cLyr)
			}
		}
	}
	return
}

func (mag Magazine) CacheLayers(ctx context.Context, mani types.ManifestData) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "mag.CacheLayers",
	})
	defer span.End()
	up := upstream.UpstreamTriple{
		Reg:  mani.Upstream.Reg,
		Repo: mani.Upstream.Repo,
		Tag:  mani.Upstream.Tag,
	}
	cacheLayer, err := mag.parseCacheLayers()
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to parse cache-layers")
	}
	utils.Warn(l, span, "Cache layers: %d", cacheLayer)
	for idx, layer := range mani.Layers {
		if cacheLayer == 0 || (cacheLayer > 0 && idx >= cacheLayer) {
			utils.Info(l, span, " >> Skip caching layer[%-2d] %s for %-90s", idx, utils.ShortenDigest(layer.Digest), up.GetLabel())
			continue
		}
		up.Digest = digest.Digest(layer.Digest)
		isCached, e := mag.Minio.GetLayer(ctx, up)
		if e != nil {
			err = e
			logrus.Error(err)
			return
		}
		utils.Info(l, span, "Caching layer[%-2d][%-20s] %s for %-90s -> %t", idx, utils.ShortenMediaType(layer.MediaType), utils.ShortenDigest(layer.Digest), up.GetLabel(), isCached)
		if !isCached {
			err = mag.PullLayer(ctx, up)
			if err != nil {
				logrus.Error(err)
				return err
			}
		}
	}
	return
}

func (mag Magazine) Layer_PlainInsert(ctx context.Context, layer types.Layer) (layerID int64, err error) {
	return mag.Database.Layer_PlainInsert(ctx, layer)
}

func (mag Magazine) NeoInsertLayer(ctx context.Context, layer types.Layer) (layerID int64, err error) {
	return mag.Layer_PlainInsert(ctx, layer)
}

func (mag Magazine) StoreLayerUpload(ctx context.Context, bu *types.OCIBlobUpload, buf []byte) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameLayer,
		"func":     "mag.BaseLayerEval",
		"blobType": bu.BlobType,
		"digest":   bu.Digest,
	})
	defer span.End()
	err = mag.Minio.PutLayer(ctx, buf, bu.Digest)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to put layer into minio")
	}
	// Insert Layer into SQLite
	lyr := types.Layer{
		Digest:    bu.Digest,
		MediaType: "unknown",
		Size:      int64(len(buf)),
	}
	//err = mag.SQlite.Layer.Upsert(&l)
	_, e := mag.Database.Layer_PlainInsert(ctx, lyr)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to insert layer into graph")
	}
	// Link Layer to BlobUpload
	if e := mag.Database.BlobUpload_Update_ByUUID(ctx, bu); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to link layer to blob upload")
	}
	return nil
}

func (mag Magazine) Layer_GetPresignedUrlByHTTPRequest(r *http.Request) (psUrl string, err error) {
	ctx := context.Background()
	req := types.NewRequest(ctx, r)
	return mag.Minio.LayerGetPreSignedUrl(ctx, req.Digest)
}

func (mag Magazine) Layer_GetByHTTPRequest(r *http.Request) (layer types.Layer, found bool, err error) {
	ctx := context.Background()
	req := types.NewRequest(ctx, r)
	//logrus.Debugf("Layer_GetByHTTPRequest: %s", req.String())
	// Fetch Layer from SQLite
	layer, found, err = mag.Database.LayerGetByDigest(ctx, req.Digest)
	if err != nil {
		logrus.Errorf("failed to fetch layer from SQLite: %v", err)
		return
	}
	if !found {
		logrus.Errorf("did not find layer with dig: %s", req.Digest)
		return
	}
	// Fetch Blob
	b, err := mag.Minio.LayerGetBytes(ctx, req.Digest)
	switch {
	case err != nil && err.Error() == "The specified key does not exist.":
		// found is false
		return
	case err != nil:
		logrus.Error(err)
		return
	}
	layer.Data = b
	// No error - off we go
	found = true
	return
}

func (mag Magazine) GetLayerByOCIManifest(oMani types.OCIManifest) (layers []types.Layer, err error) {
	ctx := context.Background()
	for _, l := range oMani.Layers {
		layer, found, err := mag.Database.LayerGetByDigest(ctx, l.Digest.String())
		if err != nil {
			logrus.Errorf("failed to fetch layer from SQLite: %v", err)
			return layers, err
		}
		if !found {
			err = fmt.Errorf("did not find layer with dig: %s", l.Digest)
			return layers, err
		}
		// Fetch Blob
		b, err := mag.Minio.LayerGetBytes(ctx, l.Digest.String())
		switch {
		case err != nil && err.Error() == "The specified key does not exist.":
			// found is false
			return layers, err
		case err != nil:
			logrus.Error(err)
			return layers, err
		}
		layer.Data = b
		layers = append(layers, layer)
	}
	return layers, nil
}

func (mag Magazine) Layers_CreateOCIv1LayerDescriptors(ctx context.Context, layerIn []types.Layer) (res []types.LayerDesc, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "Layers_CreateOCIv1LayerDescriptors",
	})
	defer span.End()
	res = []types.LayerDesc{}
	var mt string
	for _, ly := range layerIn {
		utils.Debug(l, span, "Layer (%s) with mediatype: %s", utils.ShortenDigest(ly.Digest), ly.MediaType)
		mtLong := utils.ExpandMediaType(ly.MediaType)
		switch mtLong {
		case "application/vnd.docker.image.rootfs.diff.tar.gzip":
			mt = v1.MediaTypeImageLayerGzip
		case "application/vnd.docker.image.rootfs.diff.tar":
			mt = v1.MediaTypeImageLayer
		case v1.MediaTypeImageLayerGzip:
			mt = v1.MediaTypeImageLayerGzip
		case v1.MediaTypeImageLayer:
			mt = v1.MediaTypeImageLayer

		default:
			return res, utils.NewError(l, span, "unknown media type: %s", mtLong)
		}
		d := v1.Descriptor{
			MediaType: mt,
			Digest:    digest.Digest(ly.Digest),
			Size:      ly.Size,
		}
		// Put Layer in Graph
		lID, err := mag.Database.LayerDescriptorGetInsert(ctx, d)
		if err != nil {
			logrus.Error(err)
			return res, err
		}
		ld := types.LayerDesc{
			LayerID:    lID,
			Descriptor: d,
		}
		res = append(res, ld)
	}
	return
}

func (mag Magazine) Layers_DeleteUnused(ctx context.Context) (err error) {
	_, span := tracer.Start(ctx, "Layers_DeleteUnused")
	defer span.End()
	err = mag.Database.Layers_DeleteUnused(ctx)
	if err != nil {
		err = errors.Wrapf(err, "failed to delete unused layers")
		return
	}
	err = mag.Database.BlobUpload_DeleteUnused(ctx)
	if err != nil {
		err = errors.Wrapf(err, "failed to delete unused blobUploads")
		return
	}
	return
}

func (mag Magazine) BaseLayerEvalOS(ctx context.Context, lyr types.Layer) (os types.OS, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "mag.BaseLayerEval",
		"layerID": lyr.ID,
	})
	defer span.End()
	_, e := mag.Minio.LayersStatObj(ctx, lyr.Digest)
	if e != nil {
		utils.ErrorHandler(l, span, e, "failed to stat layer")
		up, upFound, e := mag.Database.LayerGetUpstream(ctx, lyr.ID)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get LayerGetUpstreamr")
			return
		}
		if !upFound {
			err = utils.NewError(l, span, "failed to find upstream layer")
			return
		}
		upt := upstream.UpstreamTriple{
			Reg:    up.Reg,
			Repo:   up.Repo,
			Tag:    up.Tag,
			Digest: up.Digest,
		}
		e = mag.PullLayer(ctx, upt)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to pull layer")
			return
		}
	}
	// We should have a layer in Minio now
	b, e := mag.Minio.LayerGetBytes(ctx, lyr.Digest)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to fetch layer from storage")
		return

	}
	lyr.Data = b
	mt := utils.ExpandMediaType(lyr.MediaType)
	switch mt {
	case "application/vnd.docker.image.rootfs.diff.tar.gzip":
		utils.Warn(l, span, "layer fetched from storage w/ len %d and mediatype: %s", len(b), mt)
		os, e = lyr.EvalOS(ctx)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to eval layer")
			return
		}
		return os, nil
	default:
		err = utils.NewError(l, span, "unknown media type: %s", mt)
		return
	}
}
