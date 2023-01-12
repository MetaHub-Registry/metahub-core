package magazine

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (mag Magazine) CacheLayers(ctx context.Context, mani types.Manifest) (err error) {
	up := upstream.UpstreamTriple{
		Reg:  mani.Upstream.Reg,
		Repo: mani.Upstream.Repo,
		Tag:  mani.Upstream.Tag,
	}
	for idx, layer := range mani.Layers {
		up.Digest = digest.Digest(layer.Digest)
		isCached, e := mag.Minio.GetLayer(ctx, up)
		if e != nil {
			err = e
			logrus.Error(err)
			return
		}
		logrus.Infof("Caching layer[%-2d] %s for %-90s -> %t", idx, utils.ShortenDigest(layer.Digest), up.GetLabel(), isCached)
		if !isCached {
			b, blobs, err := upstream.GetBlobByUpstreamTripleByDigest(ctx, up, layer.Digest)
			if err != nil {
				err = errors.Wrapf(err, "failed to get blob for %s", up.GetLabel())
				logrus.Error(err)
				return err
			}
			_ = blobs
			err = mag.Minio.PutLayer(ctx, b, layer.Digest)
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
	// Store Blob
	err = mag.Minio.PutLayer(ctx, buf, bu.Digest)
	if err != nil {
		logrus.Error(err)
		return
	}
	// Insert Layer into SQLite
	l := types.Layer{
		Digest:    bu.Digest,
		MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
		Size:      int64(len(buf)),
	}
	//err = mag.SQlite.Layer.Upsert(&l)
	_, err = mag.Database.Layer_PlainInsert(ctx, l)
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (mag Magazine) Layer_GetPresignedUrlByHTTPRequest(r *http.Request) (psUrl string, err error) {
	ctx := context.Background()
	req := types.NewRequest(r)
	return mag.Minio.LayerGetPreSignedUrl(ctx, req.Digest)
}

func (mag Magazine) Layer_GetByHTTPRequest(r *http.Request) (layer types.Layer, found bool, err error) {
	ctx := context.Background()
	req := types.NewRequest(r)
	//logrus.Debugf("Layer_GetByHTTPRequest: %s", req.String())
	// Fetch Layer from SQLite
	layer, found, err = mag.Database.LayerGetByDigest(req.Digest)
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
		layer, found, err := mag.Database.LayerGetByDigest(l.Digest.String())
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
