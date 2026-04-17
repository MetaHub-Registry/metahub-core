package minio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/containers/image/v5/manifest"
	"github.com/minio/minio-go/v7"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	tracer = otel.Tracer("minio")
)

const (
	pkgNameMani = "backend/minio/manifest.go"
)

/*
func (m Minio) manifestGetter(bi imgTypes.BlobInfo) (b []byte, err error) {
	return m.ManifestGetBytes(context.Background(), bi.Digest.String())
}

func (m Minio) ManifestGetBytes(ctx context.Context, path string) (b []byte, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestGetBytes",
	})
	defer span.End()
	cli := m.BucketConfig[manifestsBucketKey].Client
	_, err = cli.StatObject(ctx, m.GetManifestsBucketName(), path, minio.StatObjectOptions{})
	if err != nil {
		return
	}
	obj, e := cli.GetObject(ctx, m.GetManifestsBucketName(), path, minio.GetObjectOptions{})
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "error getting manifest")
		return b, err
	}
	defer obj.Close()
	b, e = io.ReadAll(obj)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to read manifest obj")
		return b, err
	}
	utils.Debug(l, span, "got manifest obj with len %d", len(b))
	return
}
*/

func (m Minio) GetCachedManifest(ctx context.Context, dig digest.Digest) (res types.Manifest, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "GetCachedManifest",
		"digest":  utils.ShortenDigest(dig.String()),
	})
	defer span.End()
	if dig.String() == "" {
		err = utils.NewError(l, span, "digest is empty")
		return
	}
	_, err = m.ManifestsStatObj(ctx, dig.String())
	if err != nil {
		if err.Error() == "The specified key does not exist." {
			utils.Debug(l, span, "manifest %s not found", utils.ShortenDigest(dig.String()))
			return res, false, nil
		}
		err = utils.ErrorHandler(l, span, err, "error stating manifest")
		return
	} else {
		found = true
		res, err = m.ManifestGetObj(ctx, dig.String())
		if err != nil {
			l.Error(err)
			return
		}
	}
	return
}

func (m Minio) GetManifest(ctx context.Context, up upstream.UpstreamTriple) (res types.Manifest, isCached bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgName,
		"func":    "GetManifest",
	})
	_ = l
	ctx, span := tracer.Start(ctx, "GetManifest")
	span.SetAttributes(
		attribute.Key("reg").String(up.Reg),
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("digest").String(utils.ShortenDigest(up.Digest.String())),
	)
	defer span.End()
	return m.GetCachedManifest(ctx, up.Digest)
}

func (m Minio) PutManifestByUpstream(ctx context.Context, b []byte, up types.Upstream) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgName,
		"func":    "PutManifestByUpstream",
	})
	ctx, span := tracer.Start(ctx, "PutManifestByUpstream")
	span.SetAttributes(
		attribute.Key("reg").String(up.Reg),
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
	)
	defer span.End()
	err = m.ManifestsPutObj(ctx, up.Digest.String(), b)
	if err != nil {
		l.Error(err)
		return err
	}
	return
}

func (m Minio) PutManifest(ctx context.Context, b []byte, up upstream.UpstreamTriple) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgName,
		"func":    "PutManifest",
	})
	ctx, span := tracer.Start(ctx, "PutManifestObj")
	span.SetAttributes(
		attribute.Key("reg").String(up.Reg),
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
	)
	defer span.End()
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

func (m Minio) NewManifestFromBytes(ctx context.Context, b []byte) (mani types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "NewManifestFromBytes",
	})
	defer span.End()
	_ = ctx
	mt := manifest.GuessMIMEType(b)
	cntMani, e := manifest.FromBlob(b, mt)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "manifest.FromBlob %s", string(b))
		return
	}
	bInfo := cntMani.ConfigInfo()
	utils.Debug(l, span, "BlobInfo digest: %s / mediaType:%s", bInfo.Digest.String(), bInfo.MediaType)
	// TODO: Is deriving digest from byte safe?
	maniDig := digest.FromBytes(b)

	// Manifest.Size is the size of the manifest bytes themselves
	// (what an OCI descriptor for the manifest should carry). It must
	// NOT be confused with Config.Size (which is bInfo.Size — the size
	// of the config blob the manifest references). Computed Indexes
	// emit descriptors whose {digest, size} pair must point at the
	// referenced manifest bytes, so swapping these two produces indexes
	// that fail verification with 'failed to read expected number of
	// bytes: unexpected EOF' in OCI clients like manifest-tool/containerd.
	mani = types.Manifest{
		MediaType: mt,
		Digest:    maniDig.String(),
		Size:      int64(len(b)),
		Data:      b,
	}
	mani.Config = types.Config{
		Digest:    bInfo.Digest.String(),
		Size:      bInfo.Size,
		MediaType: bInfo.MediaType,
	}
	switch bInfo.MediaType {
	case "application/vnd.oci.empty.v1+json":
		utils.Debug(l, span, "empty manifest config detected")
	default:
		imgInfo, e := cntMani.Inspect(m.configGetter)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to inspect config")
			return
		}
		mani.Platform = types.Platform{
			Arch:    imgInfo.Architecture,
			OS:      imgInfo.Os,
			Variant: imgInfo.Variant,
		}
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

func (m Minio) ManifestGetObj(ctx context.Context, path string) (mani types.Manifest, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestGetObj",
		"path":    path,
	})
	defer span.End()
	cli := m.BucketConfig[manifestsBucketKey].Client
	obj, e := cli.GetObject(ctx, m.GetManifestsBucketName(), path, minio.GetObjectOptions{})
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Error GetManifestsBucketName")
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Error ReadAll")
		return
	}
	mani, e = m.NewManifestFromBytes(ctx, b)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "m.NewManifestFromBytes")
		return
	}
	mani.Annotations = types.NewAnnotations()
	return
}

func (m Minio) GetManifestViaPath(ctx context.Context, path string) (mani types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameMani,
		"func":    "GetManifestViaPath",
		"path":    path,
	})
	_ = l
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
		"package": pkgNameMani,
		"func":    "ManifestsStatObj",
		"object":  object,
	})
	_ = l
	bucket := m.GetManifestsBucketName()
	cli := m.BucketConfig[manifestsBucketKey].Client
	obj, err = cli.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
	return
}

func (m Minio) ManifestsPutObj(ctx context.Context, object string, data []byte) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameMani,
		"func":    "ManifestsPutObj",
		"object":  object,
		"len":     len(data),
	})
	defer span.End()
	utils.Debug(l, span, "Putting manifest object %s with len %d", object, len(data))
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

// ManifestsGetRawBytes returns the raw bytes stored in the manifests bucket at
// the given path (typically a digest string). Unlike ManifestGetObj, this does
// not attempt to parse the content — it is suitable for serving persisted
// Indexes or any other raw manifest content directly to clients.
func (m Minio) ManifestsGetRawBytes(ctx context.Context, path string) ([]byte, error) {
	cli := m.BucketConfig[manifestsBucketKey].Client
	obj, err := cli.GetObject(ctx, m.GetManifestsBucketName(), path, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}
