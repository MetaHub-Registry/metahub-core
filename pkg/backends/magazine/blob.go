package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
		err = fmt.Errorf("buf is JSON, but not v1.Image")
		return err
	}
}

func (mag Magazine) BlobDelete(ctx context.Context, req types.Request) (err error) {
	_, found, err := mag.Database.LayerGetByDigest(req.Digest)
	if err != nil {
		return err
	}
	if found {
		err = mag.Database.LayerDeleteByDigest(req.Digest)

		if err != nil {
			logrus.Error(err)
			return
		}
		// Remove Blob
		err = mag.Minio.LayerRemoveObj(ctx, req.Digest)
		return err
	}
	// Not a layer, so it must be a config
	_, found, err = mag.Database.Config_GetByDigest(req.Digest)
	if err != nil {
		return err
	}
	if found {
		err = mag.Database.ConfigDeleteByDigest(req.Digest)
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

func (mag Magazine) BlobExists(ctx context.Context, up upstream.UpstreamTriple) (mediaType string, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/blob.go",
		"func":    "BlobExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	ctx, span := tracer.Start(ctx, "magazine.BlobExists")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("digest").String(utils.ShortenDigest(up.Digest.String())),
	)
	md, found, e := mag.Database.ConfigExists(ctx, up)
	if e != nil {
		msg := "failed to execute ConfigExists"
		err = errors.Wrap(err, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return "", false, err
	}
	if found {
		return md, found, nil
	}
	md, found, e = mag.Database.LayerExists(ctx, up)
	if e != nil {
		msg := "failed to execute LayerExists"
		err = errors.Wrap(err, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return "", false, err
	}
	if found {
		// Check if blob exists in Minio
		_, err = mag.Minio.LayersStatObj(ctx, up.Digest.String())
		if err != nil {
			// Layer does not exist in Minio, so we need to pull it through
			msg := "layer does not exist in Minio"
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			l.Debug(msg)
			// Get upstream registry from Graph
			up.MediaType = md
			reg, err := mag.Database.Layer_GetUpstreamRegistry(ctx, up)
			up.Reg = reg
			e := mag.PullLayer(ctx, up)
			if e != nil {
				msg = "failed to pull layer"
				err = errors.Wrap(err, msg)
				span.AddEvent("log", trace.WithAttributes(
					attribute.String("log.severity", "error"),
					attribute.String("log.message", msg),
				))
				span.RecordError(err)
				l.Error(msg)
				return "", false, err
			}
			_, err = mag.Minio.LayersStatObj(ctx, up.Digest.String())
			if err != nil {
				msg := "after pulling layer the layer is still not in Minio"
				err = errors.Wrap(err, msg)
				span.AddEvent("log", trace.WithAttributes(
					attribute.String("log.severity", "error"),
					attribute.String("log.message", msg),
				))
				span.RecordError(err)
				l.Error(msg)
				return "", false, err
			}
		}
		return md, found, nil
	}
	return "", false, nil
}
