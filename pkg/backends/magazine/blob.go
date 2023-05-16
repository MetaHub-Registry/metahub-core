package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameBlob = "magazine/blob.go"
)

var (
	ERRJSONNOTCONFIG = fmt.Errorf("buf is JSON, but not v1.Image")
)

func (mag Magazine) StoreBlobUpload(ctx context.Context, bu *types.OCIBlobUpload, buf []byte) (err error) {
	//// Are we dealing with a Config or a layer?
	ctx, span := tracer.Start(ctx, "StoreBlobUpload")
	defer span.End()
	v1Img := v1.Image{}
	err = json.Unmarshal(buf, &v1Img)
	switch {
	case err != nil && strings.HasPrefix(err.Error(), "invalid character"):
		logrus.Infof("Blob is a layer")
		return mag.StoreLayerUpload(ctx, bu, buf)
	case v1Img.Architecture != "":
		logrus.Infof("Blob is a config")
		return mag.StoreConfigUpload(ctx, bu, buf)
	default:
		return ERRJSONNOTCONFIG
	}
}

func (mag Magazine) BlobDelete(ctx context.Context, req types.Request) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "BlobDelete",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
	_, found, err = mag.Database.LayerGetByDigest(ctx, req.Digest)
	if err != nil {
		return false, utils.ErrorHandler(l, span, err, types.FailLayerGetByDigest)
	}
	if found {
		if err = mag.Database.LayerDeleteByDigest(ctx, req.Digest); err != nil {
			return false, utils.ErrorHandler(l, span, err, types.FailLayerDeleteByDigest)
		}
		// Remove Blob
		if err = mag.Minio.LayerRemoveObj(ctx, req.Digest); err != nil {
			return false, utils.ErrorHandler(l, span, err, types.FailLayerRemoveObj)
		}
		return true, nil
	}
	// Not a layer, so it must be a config
	_, found, err = mag.Database.Config_GetByDigest(ctx, req.Digest)
	if err != nil {
		return false, err
	}
	if found {
		if err = mag.Database.ConfigDeleteByDigest(ctx, req.Digest); err != nil {
			return false, utils.ErrorHandler(l, span, err, types.FailConfigDeleteByDigest)
		}
		// Remove Blob
		if err = mag.Minio.ConfigRemoveObj(ctx, req.Digest); err != nil {
			return false, utils.ErrorHandler(l, span, err, types.FailConfigRemoveObj)
		}
		return true, nil
	}
	return false, nil
}

func (mag Magazine) BlobExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "BlobExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	defer span.End()
	md, found, e := mag.Database.ConfigExists(ctx, up)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to execute ConfigExists")
		return "", false, err
	}
	if found {
		utils.Debug(l, span, "config found with mediatype: %s", md)
		return md, found, nil
	}
	md, found, e = mag.Database.LayerExists(ctx, up)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to execute LayerExists")

		return "", false, err
	}
	if found {
		// Check if blob exists in Minio
		_, err = mag.Minio.LayersStatObj(ctx, up.Digest.String())
		if err != nil {
			// Layer does not exist in Minio, so we need to pull it through
			utils.Debug(l, span, "layer does not exist in Minio")
			// Get upstream registry from Graph
			up.MediaType = md
			reg, e := mag.Database.Layer_GetUpstreamRegistry(ctx, up)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to Layer_GetUpstreamRegistry")
				return "", false, err
			}
			up.Reg = reg
			e = mag.PullLayer(ctx, up)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to pull layer")
				return "", false, err
			}
			_, e = mag.Minio.LayersStatObj(ctx, up.Digest.String())
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "after pulling layer the layer is still not in Minio")
				return "", false, err
			}
		}
		return md, found, nil
	}
	return "", false, nil
}
