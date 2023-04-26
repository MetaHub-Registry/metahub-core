package upstream

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/types"
	"github.com/regclient/regclient/types/manifest"
	"github.com/regclient/regclient/types/platform"
	"github.com/regclient/regclient/types/ref"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/regcli"
	mhTypes "github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	tr = otel.Tracer("upstream")
)

func GetBlobByUpstreamTriple(ctx context.Context, up UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	shortDig := utils.ShortenDigest(up.Digest.String())
	l := logrus.WithFields(logrus.Fields{
		"package":    "neomin",
		"func":       "GetBlobByUpstreamTriple",
		"repository": up.Repo,
		"registry":   up.Reg,
		"digest":     shortDig,
	})
	if up.Digest.String() == "" {
		l.Fatal("No digest provided")
	}
	blobUrl := fmt.Sprintf("%s/%s", up.Reg, up.Repo)
	ctx, span := tr.Start(ctx, "GetBlobByUpstreamTriple")
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	span.SetAttributes(attribute.Key("digest").String(shortDig))
	defer span.End()
	blob = mhTypes.BlobMeta{}
	rc := regcli.NewRegClient(up.Reg)
	ref, err := ref.New(blobUrl)
	if err != nil {
		l.Errorf("Failed creating ref: %v", err)
	}
	ref.Digest = up.Digest.String()
	desc := types.Descriptor{Digest: up.Digest, Size: up.Size}
	blobReader, err := rc.BlobGet(ctx, ref, desc)
	if err != nil {
		err = errors.Wrap(err, "while rc.BlogGet with digest")
		l.Fatalf(err.Error())
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(blobReader)
	b = buf.Bytes()
	blob.Size = int64(len(b))
	blob.Digest = up.Digest
	blob.MediaType = up.MediaType
	return
}

func GetBlobByUpstreamTripleByDigest(ctx context.Context, up UpstreamTriple, dig string) (b []byte, blob mhTypes.BlobMeta, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":    "neomin",
		"func":       "GetBlobByUpstreamTripleByDigest",
		"repository": up.Repo,
		"registry":   up.Reg,
		"digest":     utils.ShortenDigest(dig),
	})
	if dig == "" {
		logrus.Fatal("No digest provided")
	}
	blobUrl := fmt.Sprintf("%s/%s", up.Reg, up.Repo)
	l.Warnf("blobUrl: %s", blobUrl)
	ctx, span := tr.Start(ctx, "GetBlobByUpstreamTripleByDigest")
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	span.SetAttributes(attribute.Key("digest").String(utils.ShortenDigest(dig)))
	defer span.End()
	blob = mhTypes.BlobMeta{}
	rc := regcli.NewRegClient(up.Reg)
	ref, err := ref.New(blobUrl)
	if err != nil {
		l.Errorf("Failed creating ref: %v", err)
	}
	ref.Digest = dig
	d := digest.Digest(dig)
	desc := types.Descriptor{Digest: d, Size: up.Size}
	blobReader, err := rc.BlobGet(ctx, ref, desc)
	if err != nil {
		err = errors.Wrapf(err, "while rc.BlogGet with digest: %s", dig)
		span.RecordError(err)
		l.Error(err.Error())
		return
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(blobReader)
	b = buf.Bytes()

	blob.Size = int64(len(b))
	blob.Digest = d
	return
}

func GetManifestByUpstreamTripleByDigest(ctx context.Context, up UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":    "neomin",
		"func":       "GetManifestByUpstreamTripleByDigest",
		"repository": up.Repo,
		"digest":     up.Digest.String(),
		"registry":   up.Reg,
	})
	blobUrl := fmt.Sprintf("%s/%s", up.Reg, up.Repo)
	l.Warnf("blobUrl: %s", blobUrl)
	ctx, span := tr.Start(ctx, "GetManifestByUpstreamTripleByDigest")
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	defer span.End()
	blob = mhTypes.BlobMeta{}
	rc := regcli.NewRegClient(up.Reg)
	ref, err := ref.New(blobUrl)
	if err != nil {
		l.Errorf("Failed creating ref: %v", err)
	}
	var mani manifest.Manifest
	// If we have a digest we use it
	if up.Digest.String() == "" {
		err = errors.New("No digest provided")
		return
	}
	d := types.Descriptor{
		MediaType: types.MediaTypeDocker2Manifest,
		Digest:    up.Digest,
		Platform:  &platform.Platform{},
	}
	mani, err = rc.ManifestGet(ctx, ref, regclient.WithManifestDesc(d))
	b, err = mani.RawBody()
	if err != nil {
		l.Fatalf(err.Error())
	}
	blob.Digest = mani.GetDescriptor().Digest
	blob.Size = mani.GetDescriptor().Size
	blob.MediaType = mani.GetDescriptor().MediaType
	return
}

func GetManifestByUpstreamTripleByTag(ctx context.Context, up UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":    "neomin",
		"func":       "GetManifestByUpstreamTripleByTag",
		"repository": up.Repo,
		"tag":        up.Tag,
		"registry":   up.Reg,
	})
	blobUrl := fmt.Sprintf("%s/%s:%s", up.Reg, up.Repo, up.Tag)
	l.Warnf("blobUrl: %s", blobUrl)
	ctx, span := tr.Start(ctx, "GetManifestByUpstreamTripleByTag")
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	defer span.End()
	blob = mhTypes.BlobMeta{}
	rc := regcli.NewRegClient(up.Reg)

	ref, err := ref.New(blobUrl)
	if err != nil {
		l.Errorf("Failed creating ref: %v", err)
	}
	var mani manifest.Manifest
	// If we have a digest we use it
	if up.Digest.String() != "" {
		ref.Digest = up.Digest.String()
	} else {
		mani, err = rc.ManifestHead(ctx, ref)
		if err != nil {
			l.Fatalf(err.Error())
		}
		ref.Digest = mani.GetDescriptor().Digest.String()
		blob.Digest = mani.GetDescriptor().Digest
		blob.Size = mani.GetDescriptor().Size
		blob.MediaType = mani.GetDescriptor().MediaType
	}
	d := types.Descriptor{
		MediaType: types.MediaTypeDocker2Manifest,
		Digest:    digest.Digest(ref.Digest),
		Platform:  &platform.Platform{},
	}
	mani, err = rc.ManifestGet(ctx, ref, regclient.WithManifestDesc(d))
	b, err = mani.RawBody()
	if err != nil {
		l.Fatalf(err.Error())
	}
	return
}

func HeadManifestByUpstreamTripleByTag(ctx context.Context, reg, repo, tag string) (blob mhTypes.BlobMeta, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":    "neomin",
		"func":       "HeadManifestByUpstreamTripleByTag",
		"repository": repo,
		"tag":        tag,
		"reg":        reg,
	})
	blobUrl := fmt.Sprintf("%s/%s:%s", reg, repo, tag)
	l.Warnf("blobUrl: %s ", blobUrl)
	ctx, span := tr.Start(ctx, "HeadManifestByUpstreamTripleByTag")
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	span.SetAttributes(
		attribute.Key("reg").String(reg),
		attribute.Key("repo").String(repo),
		attribute.Key("tag").String(tag),
	)
	defer span.End()
	blob = mhTypes.BlobMeta{}
	rc := regcli.NewRegClient(reg)
	upRef, e := ref.New(blobUrl)
	if e != nil {
		if strings.HasPrefix(e.Error(), "invalid reference ") {
			err = errors.Wrap(e, "Failed creating ref, skipping request")
			msg := fmt.Sprintf("Failed creating ref, skipping request: %s", e.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			l.Debug(msg)
			return blob, false, nil
		}
		err = errors.Wrap(e, "Failed creating ref")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	var mani manifest.Manifest
	// If we have a digest we use it
	mani, e = rc.ManifestHead(ctx, upRef)
	if e != nil {
		err = errors.Wrap(e, "Failed getting manifest")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		l.Errorf(err.Error())
		span.RecordError(err)
		return
	}
	upRef.Digest = mani.GetDescriptor().Digest.String()
	blob.Digest = mani.GetDescriptor().Digest
	blob.Size = mani.GetDescriptor().Size
	blob.MediaType = mani.GetDescriptor().MediaType
	found = true
	return
}
