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
	pkgNameBlob          = "minio/blob.go"
	keyDoesNotExistError = "The specified key does not exist."
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
		if err.Error() == keyDoesNotExistError {
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

func (m Minio) EnsureBlobExists(c context.Context, objPath string, dat []byte) (err error) {
	l, ctx, span := utils.InitLogging(c, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "EnsureBlobExists",
	})
	defer span.End()
	// Make sure blob is already in Minio
	if _, e := m.BlobStatObj(ctx, objPath); e != nil {
		if e.Error() == keyDoesNotExistError {
			if err = m.PutBlob(ctx, dat, objPath); err != nil {
				return utils.ErrorHandler(l, span, err, "failed to PutBlob")
			}
		} else {
			return utils.ErrorHandler(l, span, e, "failed to BlobStatObj")
		}
	}
	return nil
}

func (m Minio) BlobStatObj(ctx context.Context, object string) (obj minio.ObjectInfo, err error) {
	bucket := m.GetBlobBucketName()
	cli := m.BucketConfig[blobBucketKey].Client
	return cli.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
}

// BlobGetReader returns a streaming reader for a blob identified by its digest
// (the object key in the blob bucket). Unlike BlobGetBytes it does NOT buffer the
// whole blob in memory, so callers can stream arbitrarily large blobs (e.g. model
// files) at constant memory (cf. the model-repackage streaming path). The caller
// MUST Close the returned reader.
func (m Minio) BlobGetReader(ctx context.Context, dig string) (io.ReadCloser, error) {
	bucket := m.GetBlobBucketName()
	cli := m.BucketConfig[blobBucketKey].Client
	obj, err := cli.GetObject(ctx, bucket, dig, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (m Minio) PutBlob(ctx context.Context, b []byte, dig string) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgName,
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

// BlobPutReader streams a blob from r (exactly size bytes) into the blobs bucket
// under object, without buffering the whole payload — used to compose large
// multi-chunk uploads at constant memory.
func (m Minio) BlobPutReader(ctx context.Context, r io.Reader, size int64, object string) (err error) {
	bucket := m.GetBlobBucketName()
	cli := m.BucketConfig[GetBlobBucketKey()].Client
	_, err = cli.PutObject(ctx, bucket, object, r, size, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	return err
}

func (m Minio) BlobPutObj(ctx context.Context, object string, data []byte) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "min.BlobPutObj",
		"object":  object,
	})
	defer span.End()
	bucket := m.GetBlobBucketName()
	cli := m.BucketConfig[GetBlobBucketKey()].Client
	info, e := cli.PutObject(ctx, bucket, object, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to put object")
	}
	utils.Debug(l, span, "Put blob '%s': %d", utils.ShortenDigest(object), info.Size)
	return
}

func (m Minio) BlobRemoveObj(ctx context.Context, object string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "BlobRemoveObj",
		"object":  object,
	})
	defer span.End()
	utils.Debug(l, span, "Removing blob '%s'", utils.ShortenDigest(object))
	bucket := m.GetBlobBucketName()
	cli := m.BucketConfig[GetBlobBucketKey()].Client
	obj, err := m.BlobStatObj(ctx, object)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to stat object before removal")
	}
	utils.Debug(l, span, "Blob '%s' found: %d bytes", utils.ShortenDigest(object), obj.Size)
	err = cli.RemoveObject(ctx, bucket, object, minio.RemoveObjectOptions{})
	utils.Debug(l, span, "Removed blob '%s': %v", utils.ShortenDigest(object), err)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to remove object")
	}
	return err
}

func (m Minio) GetBlobBucketName() string {
	return m.BucketConfig[blobBucketKey].Bucket
}

func (m Minio) BlobGetBytes(ctx context.Context, path string) (b []byte, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgName,
		"func":    "BlobGetBytes",
		"path":    path,
	})
	defer span.End()
	_ = l
	bucket := m.GetBlobBucketName()
	span.SetAttributes(attribute.String("bucket", bucket))
	cli := m.BucketConfig[GetBlobBucketKey()].Client
	obj, err := cli.GetObject(ctx, bucket, path, minio.GetObjectOptions{})
	if err != nil {
		l.Errorf("Error getting blob: %s", err)
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		if e.Error() == keyDoesNotExistError {
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
		"package": pkgName,
		"func":    "BlobGetPreSignedUrl",
	})
	bucket := m.GetBlobBucketName()
	cli := m.BucketConfig[GetBlobBucketKey()].Client
	reqParams := make(url.Values)
	psU, err := cli.PresignedGetObject(
		context.Background(),
		bucket,
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
