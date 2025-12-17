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

const (
	pkgNameBlob = "minio/blob.go"
)

func (m Minio) GetBlob(ctx context.Context, up upstream.UpstreamTriple) (isCached bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "GetBlob",
		"reg":     up.Reg,
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  up.Digest.String(),
	})
	defer span.End()
	_ = l
	_, err = m.BlobStatObj(ctx, up.Digest.String())
	if err != nil {
		if err.Error() == "The specified key does not exist." {
			utils.Debug(l, span, "blob '%s' not found in bucket.", up.Digest.String())
			return false, nil
		}
		l.Error(err)
		return
	} else {
		isCached = true
	}
	return
}

func (m Minio) BlobStatObj(ctx context.Context, object string) (obj minio.ObjectInfo, err error) {
	bucket := m.GetBlobBucketName()
	cli := m.BucketConfig[blobBucketKey].Client
	return cli.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
}

func (m Minio) PutBlob(ctx context.Context, b []byte, dig string) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "PutBlob",
	})
	tr := otel.Tracer("neomin")
	ctx, span := tr.Start(ctx, "PutBlob")
	span.SetAttributes(
		attribute.Key("digest").String(utils.ShortenDigest(dig)),
	)
	defer span.End()
	err = m.BlobPutObj(ctx, dig, b)
	if err != nil {
		l.Error(err)
		return err
	}
	l.Infof("Put blob %s", utils.ShortenDigest(dig))
	return
}

func (m Minio) BlobPutObj(ctx context.Context, object string, data []byte) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "min.BlobPutObj",
	})
	defer span.End()
	blobBucketKey := GetBlobBucketKey()
	cli := m.BucketConfig[blobBucketKey].Client
	info, e := cli.PutObject(ctx, blobBucketKey, object, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to put object")
	}
	utils.Debug(l, span, "Put blob '%s': %d", utils.ShortenDigest(object), info.Size)
	return
}

func (m Minio) BlobRemoveObj(ctx context.Context, object string) (err error) {
	blobBucketKey := GetBlobBucketKey()
	cli := m.BucketConfig[blobBucketKey].Client
	return cli.RemoveObject(ctx, blobBucketKey, object, minio.RemoveObjectOptions{})
}

func (m Minio) GetBlobBucketName() string {
	return m.BucketConfig[blobBucketKey].Bucket
}

func (m Minio) BlobGetBytes(ctx context.Context, path string) (b []byte, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "BlobGetBytes",
		"path":    path,
	})
	blobBucketKey := GetBlobBucketKey()
	cli := m.BucketConfig[blobBucketKey].Client
	obj, err := cli.GetObject(ctx, blobBucketKey, path, minio.GetObjectOptions{})
	if err != nil {
		l.Errorf("Error getting blob: %s", err)
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		if e.Error() == "The specified key does not exist." {
			l.Error("")
			return nil, e
		}
		err = errors.Wrap(e, "failed to read blob")
		l.Fatal(err)
	}
	return
}

func (m Minio) BlobGetPreSignedUrl(ctx context.Context, dig string) (psUrl string, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "BlobGetPreSignedUrl",
	})
	blobBucketKey := GetBlobBucketKey()
	cli := m.BucketConfig[blobBucketKey].Client
	reqParams := make(url.Values)
	psU, err := cli.PresignedGetObject(
		context.Background(),
		blobBucketKey,
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

func (m Minio) GetBlobBucketConfig() BucketConfig {
	return m.BucketConfig[GetBlobBucketKey()]
}
