package minio

import (
	"context"
	"encoding/json"
	"fmt"

	imgTypes "github.com/containers/image/v5/types"
	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	v1RC "github.com/regclient/regclient/types/oci/v1"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameConfig   = "minio/config.go"
	keyDownNotExist = "The specified key does not exist."
)

func (m Minio) GetConfig(ctx context.Context, dig string) (cfg v1.Image, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "minio.GetConfig",
		"digest":  dig,
	})

	defer span.End()
	b, e := m.BlobGetBytes(ctx, dig)
	switch {
	case e != nil && e.Error() == keyDownNotExist:
		utils.Warn(l, span, "digest not found in storage")
		err = nil
		return
	case e != nil:
		err = utils.ErrorHandler(l, span, e, "failed to ConfigGetBytes")
		return
	}
	// No error - off we go
	found = true
	cfg = v1.Image{}
	e = json.Unmarshal(b, &cfg)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to unmarshal v1.Image: %v", err)
		return
	}
	if cfg.Config.Labels == nil {
		cfg.Config.Labels = make(map[string]string)
	}
	found = true
	return
}

func (m Minio) GetConfigByUpTriple(ctx context.Context, up upstream.UpstreamTriple) (res types.Config, isCached bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameConfig,
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
	_, err = m.BlobStatObj(ctx, up.Digest.String())
	if err != nil {
		if err.Error() == keyDownNotExist {
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

// func (m Minio) ConfigGetBytes(ctx context.Context, path string) (b []byte, err error) {
// 	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
// 		"package": pkgNameConfig,
// 		"func":    "ConfigGetBytes",
// 	})
// 	defer span.End()
// 	cli := m.BucketConfig[configsBucketKey].Client
// 	_, err = cli.StatObject(ctx, m.GetConfigsBucketName(), path, minio.StatObjectOptions{})
// 	if err != nil {
// 		return
// 	}
// 	obj, e := cli.GetObject(ctx, m.GetConfigsBucketName(), path, minio.GetObjectOptions{})
// 	if e != nil {
// 		err = utils.ErrorHandler(l, span, e, "error getting config")
// 		return b, err
// 	}
// 	defer obj.Close()
// 	b, e = io.ReadAll(obj)
// 	if e != nil {
// 		err = utils.ErrorHandler(l, span, e, "failed to read config obj")
// 		return b, err
// 	}
// 	utils.Debug(l, span, "got config obj with len %d", len(b))
// 	return
// }

func (m Minio) ConfigGetObj(ctx context.Context, path string) (cfg types.Config, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "minio.ConfigGetObj",
		"path":    path,
	})
	defer span.End()
	b, err := m.BlobGetBytes(ctx, path)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to ConfigGetBytes")
		return cfg, err
	}
	utils.Debug(l, span, "got config obj with len %d", len(b))

	cfg, err = ConfigFromCfgBlob(ctx, b)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to ConfigFromCfgBlob")
		return cfg, err
	}

	return
}

// func (m Minio) PutConfig(ctx context.Context, b []byte, dig string) (err error) {
// 	l := logrus.WithFields(logrus.Fields{
// 		"package": pkgNameConfig,
// 		"func":    "PutConfig",
// 	})
// 	tr := otel.Tracer("neomin")
// 	ctx, span := tr.Start(ctx, "PutConfig")
// 	span.SetAttributes(
// 		attribute.Key("digest").String(utils.ShortenDigest(dig)),
// 	)
// 	defer span.End()
// 	if digest.FromBytes(b).String() != dig {
// 		err = errors.Errorf("digest mismatch: %s != %s", digest.FromBytes(b).String(), dig)
// 		l.Error(err)
// 		return
// 	}
// 	err = m.ConfigPutObj(ctx, dig, b)
// 	if err != nil {
// 		l.Error(err)
// 		return err
// 	}
// 	return
// }

// func (m Minio) ConfigPutObj(ctx context.Context, object string, data []byte) (err error) {
// 	cli := m.BucketConfig[configsBucketKey].Client
// 	_, err = cli.PutObject(ctx, m.GetConfigsBucketName(), object, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
// 	return
// }

func ConfigFromCfgBlob(ctx context.Context, blob []byte) (cfg types.Config, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "ConfigGetObj",
	})
	defer span.End()
	_ = ctx
	span.AddEvent("blob", trace.WithAttributes(
		attribute.KeyValue{Key: "data", Value: attribute.StringValue(string(blob))},
	))
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
	if cfg.Digest == "" {
		cfg.Digest = digest.FromBytes(blob).String()
	} else {
		if cfg.Digest != digest.FromBytes(blob).String() {
			err = utils.NewError(l, span, "digest mismatch: %s != %s", cfg.Digest, digest.FromBytes(blob).String())
			return
		}
	}
	if cfg.Size == 0 {
		cfg.Size = int64(len(blob))
	} else {
		if cfg.Size != int64(len(blob)) {
			err = utils.NewError(l, span, "size mismatch: %d != %d", cfg.Size, len(blob))
			return
		}
	}

	if cfg.MediaType == "" {
		v1Cfg := v1RC.Image{}
		err = json.Unmarshal(blob, &v1Cfg)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to unmarshal v1 config")
			return
		}
		cfg.Cfg = v1Cfg
		cfg.MediaType = v1.MediaTypeImageConfig
	}
	return
}

func (m Minio) configGetter(bi imgTypes.BlobInfo) (b []byte, err error) {
	logrus.Warnf("configGetter :%s", bi.Digest.String())
	return m.BlobGetBytes(context.Background(), bi.Digest.String())
}

func (m Minio) GetTypesConfig(ctx context.Context, dig string) (cfg types.Config, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameConfig,
		"func":    "GetTypesConfig",
		"digest":  dig,
	})
	defer span.End()
	_ = l
	b, err := m.BlobGetBytes(ctx, dig)
	switch {
	case err != nil && err.Error() == keyDownNotExist:
		// found is false
		logrus.Debugf("GetTypesConfig(%s) not found", dig)
		err = nil
		return
	case err != nil:
		logrus.Warn(err)
		return
	}
	if digest.FromBytes(b).String() != dig {
		err = fmt.Errorf("digest mismatch: %s != %s", dig, digest.FromBytes(b).String())
		return
	}
	// No error - off we go
	found = true
	cfg, err = types.NewConfigFromBytes(ctx, b)
	/*
		cfg.Labels = types.NewAnnotations()
		if err != nil {
			logrus.Error(err)
		}
	*/
	return
}
