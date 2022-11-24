package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) StoreBlobUpload(ctx context.Context, bu *types.OCIBlobUpload, buf []byte) (err error) {
	//// Are we dealing with a Config or a layer?
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
		err = fmt.Errorf("buf is JSON, but not v1.Image")
		return err
	}
}

func (mag Magazine) BlobDelete(ctx context.Context, req types.Request) (err error) {
	layerId, found, err := mag.SQlite.Layer.LayerGetByDigest(req.Digest)
	if err != nil {
		return err
	}
	if found {
		err = mag.SQlite.Layer.DeleteLayerByID(layerId.SqlID)
		if err != nil {
			logrus.Error(err)
			return
		}
		// Remove Blob
		err = mag.Minio.LayerRemoveObj(ctx, req.Digest)
		return err
	}
	// Not a layer, so it must be a config
	cfg, found, err := mag.SQlite.Config.ConfigGetByDigest(req.Digest)
	if err != nil {
		return err
	}
	if found {
		err = mag.SQlite.Config.RemoveByID(cfg.SqlID)
		if err != nil {
			logrus.Error(err)
			return
		}
		// Remove Blob
		err = mag.Minio.ConfigRemoveObj(ctx, req.Digest)
		return err
	}
	return
}
