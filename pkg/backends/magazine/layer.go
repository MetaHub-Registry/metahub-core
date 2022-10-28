package magazine

import (
	"context"

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

func (mag Magazine) NeoInsertLayer(ctx context.Context, layer types.Layer) (layerID int64, err error) {
	layerID, success, err := mag.Neo.GetLayer(layer)
	if err != nil {
		return
	}
	if !success {
		layerID, err = mag.Neo.CreateLayer(layer)
		if err != nil {
			return
		}
	}
	return
}
