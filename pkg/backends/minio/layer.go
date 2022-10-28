package minio

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func (m Minio) GetLayer(ctx context.Context, up upstream.UpstreamTriple) (isCached bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "GetLayer",
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	_ = l
	tr := otel.Tracer("neomin")
	ctx, span := tr.Start(ctx, "GetLayer")
	span.SetAttributes(
		attribute.Key("reg").String(up.Reg),
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("digest").String(utils.ShortenDigest(up.Digest.String())),
	)
	defer span.End()
	_, err = m.LayersStatObj(ctx, up.Digest.String())
	if err != nil {
		if err.Error() == "The specified key does not exist." {
			return false, nil
		}
		l.Error(err)
		return
	} else {
		isCached = true
	}
	return
}

func (m Minio) LayersStatObj(ctx context.Context, object string) (obj minio.ObjectInfo, err error) {
	bucket := m.GetLayersBucketName()
	cli := m.BucketConfig[layersBucketKey].Client
	return cli.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
}

func (m Minio) PutLayer(ctx context.Context, b []byte, dig string) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "PutLayer",
	})
	tr := otel.Tracer("neomin")
	ctx, span := tr.Start(ctx, "PutLayer")
	span.SetAttributes(
		attribute.Key("digest").String(utils.ShortenDigest(dig)),
	)
	defer span.End()
	err = m.LayerPutObj(ctx, dig, b)
	if err != nil {
		l.Error(err)
		return err
	}
	l.Infof("Put layer %s", utils.ShortenDigest(dig))
	return
}

func (m Minio) LayerPutObj(ctx context.Context, object string, data []byte) (err error) {
	cli := m.BucketConfig[layersBucketKey].Client
	_, err = cli.PutObject(ctx, m.GetLayersBucketName(), object, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	return
}

func (m Minio) LayerRemoveObj(ctx context.Context, object string) (err error) {
	cli := m.BucketConfig[layersBucketKey].Client
	return cli.RemoveObject(ctx, m.GetLayersBucketName(), object, minio.RemoveObjectOptions{})
}

func (m Minio) GetLayersBucketName() string {
	return m.BucketConfig[layersBucketKey].Bucket
}

func (m Minio) LayerGetBytes(ctx context.Context, path string) (b []byte, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "LayerGetBytes",
		"path":    path,
	})
	cli := m.BucketConfig[layersBucketKey].Client
	obj, err := cli.GetObject(ctx, m.GetLayersBucketName(), path, minio.GetObjectOptions{})
	if err != nil {
		l.Errorf("Error getting layer: %s", err)
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		if e.Error() == "The specified key does not exist." {
			l.Error("")
			return nil, e
		}
		err = errors.Wrap(e, "failed to read layer")
		l.Fatal(err)
	}
	return
}

func (m Minio) LayerGetPreSignedUrl(ctx context.Context, dig string) (psUrl string, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "LayerGetPreSignedUrl",
	})
	cli := m.BucketConfig[layersBucketKey].Client
	reqParams := make(url.Values)
	psU, err := cli.PresignedGetObject(
		context.Background(),
		m.GetLayersBucketName(),
		dig,
		time.Second*30*60,
		reqParams)
	if err != nil {
		l.Errorf("Error getting presigned url: %s", err)
		return
	}
	psUrl = psU.String()
	return
}

func (m Minio) GetLayerBucketConfig() BucketConfig {
	return m.BucketConfig[layersBucketKey]
}
