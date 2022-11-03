package minio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/containers/image/v5/manifest"
	"github.com/minio/minio-go/v7"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var aliasPrefix = "aliases"

func (m Minio) GetManifest(ctx context.Context, up upstream.UpstreamTriple) (res types.Manifest, isCached bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "GetManifest",
	})
	tr := otel.Tracer("neomin")
	ctx, span := tr.Start(ctx, "GetManifest")
	span.SetAttributes(
		attribute.Key("reg").String(up.Reg),
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
	)
	defer span.End()
	aliasPath := filepath.Join(aliasPrefix, up.GetObjPath())
	l.Warnf("aliasPath: %s", aliasPath)
	_, err = m.ManifestsStatObj(ctx, aliasPath)
	if err != nil {
		if err.Error() == "The specified key does not exist." {
			l.Warn("NOT CACHED -> return")
			return res, false, nil
		}
		l.Error(err)
		return
	} else {
		isCached = true
		res, err = m.GetManifestViaPath(ctx, aliasPath)
		if err != nil {
			l.Error(err)
			return
		}

	}
	return
}

func (m Minio) PutManifest(ctx context.Context, b []byte, up upstream.UpstreamTriple) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "PutManifest",
	})
	tr := otel.Tracer("neomin")
	ctx, span := tr.Start(ctx, "PutManifestObj")
	span.SetAttributes(
		attribute.Key("reg").String(up.Reg),
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
	)
	defer span.End()
	aliasPath := filepath.Join(aliasPrefix, up.GetObjPath())
	l.Warnf("aliasPath: %s", aliasPath)
	err = m.ManifestsPutObj(ctx, aliasPath, up.ToBytes())
	if err != nil {
		l.Error(err)
		return err
	}
	err = m.ManifestsPutObj(ctx, up.Digest.String(), b)
	if err != nil {
		l.Error(err)
		return err
	}
	return
}

func RawManifestFromBlob(b []byte) (mani manifest.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "backend/minio/manifest.go",
		"func":       "RawManifestFromBlob",
	})
	mt := manifest.GuessMIMEType(b)
	mani, err = manifest.FromBlob(b, mt)
	if err != nil {
		err = errors.WithMessagef(err, "manifest.FromBlob %s", string(b))
		l.Error(err)
	}
	return
}
func (m Minio) NewManifestFromBytes(b []byte) (mani types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "backend/minio/manifest.go",
		"func":       "NewManifestFromBytes",
	})
	mt := manifest.GuessMIMEType(b)
	cntMani, err := manifest.FromBlob(b, mt)
	if err != nil {
		err = errors.WithMessagef(err, "manifest.FromBlob %s", string(b))
		l.Error(err)
	}
	bInfo := cntMani.ConfigInfo()
	// TODO: Is deriving digest from byte safe?
	maniDig := digest.FromBytes(b)
	mani = types.Manifest{
		MediaType: mt,
		Digest:    maniDig.String(),
		Size:      bInfo.Size,
		Data:      b,
	}
	imgInfo, err := cntMani.Inspect(m.configGetter)
	if err != nil {
		err = errors.Wrapf(err, "failed to inspect manifest")
		l.Error(err)
		return
	}
	mani.Config = types.Config{
		Digest:    bInfo.Digest.String(),
		Size:      bInfo.Size,
		MediaType: bInfo.MediaType,
	}
	mani.Platform = types.Platform{
		Arch:    imgInfo.Architecture,
		OS:      imgInfo.Os,
		Variant: imgInfo.Variant,
	}
	for _, layer := range cntMani.LayerInfos() {
		mani.Layers = append(mani.Layers, types.Layer{
			MediaType: layer.MediaType,
			Digest:    layer.Digest.String(),
			Size:      layer.Size,
		})
	}
	return
}

func (m Minio) RemoveManifestAliases() (err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "backend/minio/manifest.go",
		"func":       "RemoveManifestAliases",
	})
	l.Info("Start")
	tr := otel.Tracer("neomin")
	ctx, span := tr.Start(context.Background(), "RemoveManifestAliases")
	defer span.End()
	cli := m.BucketConfig[manifestsBucketKey].Client
	objectCh := cli.ListObjects(ctx, m.GetManifestsBucketName(), minio.ListObjectsOptions{
		Prefix:    aliasPrefix,
		Recursive: true,
	})
	for object := range objectCh {
		if object.Err != nil {
			fmt.Println(object.Err)
			return
		}
		err = m.ManifestRemoveObj(ctx, object.Key)
		if err != nil {
			l.Error(err)
			continue
		}
	}
	return
}

func (m Minio) ManifestGetObj(ctx context.Context, path string) (mani types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "backend/minio/manifest.go",
		"func":       "ManifestGetObj",
		"path":       path,
	})
	cli := m.BucketConfig[manifestsBucketKey].Client
	obj, err := cli.GetObject(ctx, m.GetManifestsBucketName(), path, minio.GetObjectOptions{})
	if err != nil {
		logrus.Errorf("Error getting manifest: %s", err)
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		logrus.Error(e)
		return
	}
	mani, err = m.NewManifestFromBytes(b)
	if err != nil {
		err = errors.Wrapf(err, "m.NewManifestFromBytes")
		l.Error(err)
	}
	return
}

func (m Minio) GetManifestViaPath(ctx context.Context, path string) (mani types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "backend/minio/manifest.go",
		"func":       "GetManifestViaPath",
		"path":       path,
	})
	cli := m.BucketConfig[manifestsBucketKey].Client
	_, err = cli.StatObject(ctx, m.GetManifestsBucketName(), path, minio.StatObjectOptions{})
	if err != nil {
		err = errors.Wrapf(err, "StatObject %s", path)
		l.Error(err)
		return
	}
	obj, err := cli.GetObject(ctx, m.GetManifestsBucketName(), path, minio.GetObjectOptions{})
	if err != nil {
		l.Errorf("Error getting manifest via path: %s", err)
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		l.Error(e)
	}
	var up upstream.UpstreamTriple
	err = json.Unmarshal(b, &up)
	if err != nil {
		l.Error(err)
		return
	}
	return m.ManifestGetObj(ctx, up.Digest.String())
}

func (m Minio) ManifestsStatObj(ctx context.Context, object string) (obj minio.ObjectInfo, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "backend/minio/manifest.go",
		"func":       "ManifestsStatObj",
		"object":     object,
	})
	_ = l
	bucket := m.GetManifestsBucketName()
	cli := m.BucketConfig[manifestsBucketKey].Client
	obj, err = cli.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
	return
}

func (m Minio) ManifestsPutObj(ctx context.Context, object string, data []byte) (err error) {
	cli := m.BucketConfig[manifestsBucketKey].Client
	_, err = cli.PutObject(ctx, m.GetManifestsBucketName(), object, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	return
}

func (m Minio) ManifestRemoveObj(ctx context.Context, object string) (err error) {
	cli := m.BucketConfig[manifestsBucketKey].Client
	return cli.RemoveObject(ctx, m.GetManifestsBucketName(), object, minio.RemoveObjectOptions{})
}

func (m Minio) GetManifestsBucketName() string {
	return m.BucketConfig[manifestsBucketKey].Bucket
}
