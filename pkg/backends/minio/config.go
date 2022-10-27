package minio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/docker/distribution"
	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	imgTypes "github.com/containers/image/v5/types"
)

func (m Minio) GetConfig(ctx context.Context, up upstream.UpstreamTriple) (res types.Config, isCached bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "GetConfig",
	})
	tr := otel.Tracer("neomin")
	ctx, span := tr.Start(ctx, "GetConfig")
	span.SetAttributes(
		attribute.Key("reg").String(up.Reg),
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("cfg").String(utils.ShortenDigest(up.Digest.String())),
	)
	defer span.End()
	_, err = m.ConfigsStatObj(ctx, up.Digest.String())
	if err != nil {
		if err.Error() == "The specified key does not exist." {
			return res, false, nil
		}
		err = errors.Wrap(err, "failed to stat config")
		l.Error(err)
		return
	} else {
		isCached = true
		res, err = m.ConfigGetObj(ctx, up.Digest.String())
		if err != nil {
			err = errors.Wrap(err, "failed to get config")
			l.Error(err)
			return
		}

	}
	return
}

func (m Minio) ConfigGetBytes(ctx context.Context, path string) (b []byte, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "ConfigGetObj",
	})
	_ = l
	cli := m.BucketConfig[configsBucketKey].Client
	obj, err := cli.GetObject(ctx, m.GetConfigsBucketName(), path, minio.GetObjectOptions{})
	if err != nil {
		logrus.Errorf("Error getting config: %s", err)
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		err = errors.Wrap(e, "failed to read config")
		logrus.Fatal(err)
	}
	return
}

func (m Minio) ConfigGetObj(ctx context.Context, path string) (cfg types.Config, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "ConfigGetObj",
		"path":    path,
	})
	b, err := m.ConfigGetBytes(ctx, path)
	if err != nil {
		l.Error(err)
		return cfg, err
	}

	return ConfigFromCfgBlob(b)
}

func (m Minio) PutConfig(ctx context.Context, b []byte, dig string) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "PutConfig",
	})
	tr := otel.Tracer("neomin")
	ctx, span := tr.Start(ctx, "PutConfig")
	span.SetAttributes(
		attribute.Key("digest").String(utils.ShortenDigest(dig)),
	)
	defer span.End()
	err = m.ConfigPutObj(ctx, dig, b)
	if err != nil {
		l.Error(err)
		return err
	}
	return
}

func (m Minio) ConfigPutObj(ctx context.Context, object string, data []byte) (err error) {
	cli := m.BucketConfig[configsBucketKey].Client
	_, err = cli.PutObject(ctx, m.GetConfigsBucketName(), object, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	return
}

func ConfigFromCfgBlob(blob []byte) (cfg types.Config, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "ConfigGetObj",
	})
	var desc distribution.Descriptor
	err = json.Unmarshal(blob, &desc)
	if err != nil {
		l.Error(err)
	}
	cfg = types.Config{
		Digest:    desc.Digest.String(),
		Size:      desc.Size,
		MediaType: desc.MediaType,
		Data:      blob,
	}
	return
}

func (m Minio) configGetter(bi imgTypes.BlobInfo) (b []byte, err error) {
	return m.ConfigGetBytes(context.Background(), bi.Digest.String())
}

func (m Minio) ConfigRemoveObj(ctx context.Context, object string) (err error) {
	cli := m.BucketConfig[configsBucketKey].Client
	return cli.RemoveObject(ctx, m.GetConfigsBucketName(), object, minio.RemoveObjectOptions{})
}

func (m Minio) ConfigsStatObj(ctx context.Context, object string) (obj minio.ObjectInfo, err error) {
	bucket := m.GetConfigsBucketName()
	cli := m.BucketConfig[configsBucketKey].Client
	return cli.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
}

func (m Minio) GetConfigsBucketName() string {
	return m.BucketConfig[configsBucketKey].Bucket
}
