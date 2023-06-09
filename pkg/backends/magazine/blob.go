package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameBlob = "magazine/blob.go"
)

var (
	ErrJSONNOTCONFIG = fmt.Errorf("buf is JSON, but not v1.Image")
)

func (mag Magazine) StoreBlobUpload(ctx context.Context, bu *types.OCIBlobUpload, buf []byte) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameBlob,
		"func":     "BlobExists",
		"blobType": bu.BlobType,
		"digest":   bu.Digest,
	})
	defer span.End()
	//// Are we dealing with a Config or a layer?
	v1Img := v1.Image{}
	err = json.Unmarshal(buf, &v1Img)
	switch {
	case err != nil && strings.HasPrefix(err.Error(), "invalid character"):
		l.Debug(l, span, "Blob is a layer")
		return mag.StoreLayerUpload(ctx, bu, buf)
	case v1Img.Architecture != "":
		l.Debug(l, span, "Blob is a config")
		return mag.StoreConfigUpload(ctx, bu, buf)
	default:
		l.Debug(l, span, "Blob is neither a layer nor a config")
		return ErrJSONNOTCONFIG
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
	mt, found, e := mag.Database.ConfigExists(ctx, up)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to execute ConfigExists")
		return "", false, err
	}
	if found {
		utils.Debug(l, span, "config found with mediatype: %s", mt)
		return mt, found, nil
	}
	// No config, must be a layer than
	found, e = mag.Database.LayerExists(ctx, up)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to execute LayerExists")
		return "", false, err
	}
	utils.Info(l, span, "layer found: %v", found)
	if found {
		var reg string
		if lyrUp, found, e := mag.Database.UpstreamGetByLayerDigest(ctx, up.Digest.String()); e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to UpstreamGetByLayerDigest")
			return "", false, err
		} else if found {
			up.Reg = lyrUp.Reg
			up.MediaType = lyrUp.MediaType
		} else {
			/// We did not find a (Layer)-(ManifestData)-(Upstream) relation,
			// might be that the layer was only updated and not linked to a manifest
			bu, found, err := mag.Database.BlobUploadGetByBlobDigest(ctx, up.Digest.String())
			if err != nil {
				return "", false, utils.ErrorHandler(l, span, err, "failed to BlobUploadGetByBlobDigest")
			}
			if !found {
				return "", false, utils.ErrorHandler(l, span, err, "failed to find BlobUpload for digest")
			}
			up.Reg = mag.Config.AuthURL
			if bu.MediaType != "" {
				up.MediaType = bu.MediaType
			} else {
				up.MediaType = v1.MediaTypeImageLayerGzip
			}
		}
		reg = up.Reg
		mt = up.MediaType
		utils.Debug(l, span, "layer found with mediatype '%s' from registry '%s'", mt, reg)
		// Check if blob exists in Minio
		_, err = mag.Minio.LayersStatObj(ctx, up.Digest.String())
		if err != nil {
			// Layer does not exist in Minio, so we need to pull it through
			utils.Debug(l, span, "layer does not exist in Minio")
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
			utils.Debug(l, span, "set mediatype to %s", mt)
		}
		return mt, found, nil
	}
	return "", false, nil
}
