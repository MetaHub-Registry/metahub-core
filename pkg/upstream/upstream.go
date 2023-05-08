package upstream

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/opencontainers/go-digest"
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
	"go.opentelemetry.io/otel/trace"
)

var (
	tr = otel.Tracer("upstream")
)

const (
	pkgNameUp     = "neo/upstream.go"
	FailNoDigest  = "No digest provided"
	FailCreateRef = "Failed creating ref"
	BlobURLFrmt   = "blobUrl: %s"
)

func GetBlobByUpstreamTriple(ctx context.Context, up UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tr, map[string]any{
		"package":    pkgNameUp,
		"func":       "GetBlobByUpstreamTriple",
		"repository": up.Repo,
		"registry":   up.Reg,
		"digest":     up.Digest.String(),
	})
	defer span.End()
	if up.Digest.String() == "" {
		err = utils.NewError(l, span, FailNoDigest)
		return
	}
	blobUrl := fmt.Sprintf("%s/%s", up.Reg, up.Repo)
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	blob = mhTypes.BlobMeta{}
	rc := regcli.NewRegClient(up.Reg)
	ref, e := ref.New(blobUrl)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailCreateRef)
		return
	}
	ref.Digest = up.Digest.String()
	desc := types.Descriptor{Digest: up.Digest, Size: up.Size}
	blobReader, e := rc.BlobGet(ctx, ref, desc)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while rc.BlogGet with digest")
		return
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
	l, ctx, span := utils.InitLogging(ctx, tr, map[string]any{
		"package":    pkgNameUp,
		"func":       "GetBlobByUpstreamTripleByDigest",
		"repository": up.Repo,
		"registry":   up.Reg,
		"digest":     utils.ShortenDigest(dig),
	})
	defer span.End()
	if dig == "" {
		logrus.Fatal(FailNoDigest)
	}
	blobUrl := fmt.Sprintf("%s/%s", up.Reg, up.Repo)
	utils.Warn(l, span, BlobURLFrmt, blobUrl)
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	span.SetAttributes(attribute.Key("digest").String(utils.ShortenDigest(dig)))

	blob = mhTypes.BlobMeta{}
	blob.MediaType = up.MediaType
	rc := regcli.NewRegClient(up.Reg)
	ref, e := ref.New(blobUrl)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailCreateRef)
		return
	}
	ref.Digest = dig
	d := digest.Digest(dig)
	desc := types.Descriptor{
		Digest: d,
		Size:   up.Size,
	}
	blobReader, e := rc.BlobGet(ctx, ref, desc)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while rc.BlogGet with digest: %s", dig)
		return
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(blobReader)
	b = buf.Bytes()
	blob.Size = int64(len(b))
	blob.Digest = d
	utils.Debug(l, span, "Pulled blob with size: %d", blob.Size)
	return b, blob, nil
}

func GetManifest_ByUpstreamByDigest(ctx context.Context, up mhTypes.Upstream) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tr, map[string]any{
		"package": pkgNameUp,
		"func":    "upstreamBlob_Insert",
		"reg":     up.Reg,
		"repo":    up.Repo,
		"tag":     up.Tag,
		"profile": up.Profile,
		"digest":  up.Digest.String(),
	})
	defer span.End()
	blobUrl := fmt.Sprintf("%s/%s", up.Reg, up.Repo)
	utils.Debug(l, span, BlobURLFrmt, blobUrl)
	blob = mhTypes.BlobMeta{}
	rc := regcli.NewRegClient(up.Reg)
	ref, e := ref.New(blobUrl)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailCreateRef)
		return
	}
	var mani manifest.Manifest
	// If we have a digest we use it
	if up.Digest.String() == "" {
		err = utils.NewError(l, span, FailNoDigest)
		return
	}
	d := types.Descriptor{
		MediaType: types.MediaTypeDocker2Manifest,
		Digest:    up.Digest,
		Platform:  &platform.Platform{},
	}
	mani, e = rc.ManifestGet(ctx, ref, regclient.WithManifestDesc(d))
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailCreateRef)
		return
	}
	b, e = mani.RawBody()
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed get RawBody")
		return
	}
	blob.Digest = mani.GetDescriptor().Digest
	blob.Size = mani.GetDescriptor().Size
	blob.MediaType = mani.GetDescriptor().MediaType
	return
}

func GetManifestByUpstreamTripleByDigest(ctx context.Context, up UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tr, map[string]any{
		"package":    pkgNameUp,
		"func":       "GetManifestByUpstreamTripleByDigest",
		"repository": up.Repo,
		"digest":     up.Digest.String(),
		"registry":   up.Reg,
	})
	blobUrl := fmt.Sprintf("%s/%s", up.Reg, up.Repo)
	utils.Debug(l, span, BlobURLFrmt, blobUrl)
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	defer span.End()
	blob = mhTypes.BlobMeta{}
	rc := regcli.NewRegClient(up.Reg)
	ref, e := ref.New(blobUrl)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailCreateRef)
		return
	}
	var mani manifest.Manifest
	// If we have a digest we use it
	if up.Digest.String() == "" {
		err = utils.NewError(l, span, FailNoDigest)
		return
	}
	d := types.Descriptor{
		MediaType: types.MediaTypeDocker2Manifest,
		Digest:    up.Digest,
		Platform:  &platform.Platform{},
	}
	mani, err = rc.ManifestGet(ctx, ref, regclient.WithManifestDesc(d))
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while regcli.ManifestGet")
		return
	}
	b, e = mani.RawBody()
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while getting RawBody")
		return
	}
	blob.Digest = mani.GetDescriptor().Digest
	blob.Size = mani.GetDescriptor().Size
	blob.MediaType = mani.GetDescriptor().MediaType
	return
}

func GetManifestByUpstreamTripleByTag(ctx context.Context, up UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tr, map[string]any{
		"package":    pkgNameUp,
		"func":       "GetManifestByUpstreamTripleByTag",
		"repository": up.Repo,
		"tag":        up.Tag,
		"registry":   up.Reg,
	})
	blobUrl := fmt.Sprintf("%s/%s:%s", up.Reg, up.Repo, up.Tag)
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	defer span.End()
	msg := fmt.Sprintf("BlobURL: '%s'", blobUrl)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
	blob = mhTypes.BlobMeta{}
	rc := regcli.NewRegClient(up.Reg)
	ref, e := ref.New(blobUrl)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while creating ref from blobUrl")
		return
	}
	var mani manifest.Manifest
	// If we have a digest we use it
	if up.Digest.String() != "" {
		ref.Digest = up.Digest.String()
	} else {
		mani, e = rc.ManifestHead(ctx, ref)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while ManifestHead")
			return
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
	mani, e = rc.ManifestGet(ctx, ref, regclient.WithManifestDesc(d))
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while regcli.ManifestGet")
		return
	}
	b, e = mani.RawBody()
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while getting RawBody")
		return
	}
	return
}

func HeadManifestByUpstreamTripleByTag(ctx context.Context, reg, repo, tag string) (blob mhTypes.BlobMeta, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tr, map[string]any{
		"package":    pkgNameUp,
		"func":       "HeadManifestByUpstreamTripleByTag",
		"repository": repo,
		"tag":        tag,
		"reg":        reg,
	})
	defer span.End()
	blobUrl := fmt.Sprintf("%s/%s:%s", reg, repo, tag)
	span.SetAttributes(attribute.Key("url").String(blobUrl))

	utils.Debug(l, span, "blobUrl: %s ", blobUrl)
	blob = mhTypes.BlobMeta{}
	rc := regcli.NewRegClient(reg)
	upRef, e := ref.New(blobUrl)
	if e != nil {
		if strings.HasPrefix(e.Error(), "invalid reference ") {
			// we are not capturing the error and exit
			utils.ErrorHandler(l, span, e, "Failed creating ref, skipping request")
			utils.Debug(l, span, "Failed creating ref, skipping request: %s", e.Error())
			return blob, false, nil
		}
		err = utils.ErrorHandler(l, span, e, FailCreateRef)
		return
	}
	var mani manifest.Manifest
	// If we have a digest we use it
	mani, e = rc.ManifestHead(ctx, upRef)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed getting manifest")
		return
	}
	upRef.Digest = mani.GetDescriptor().Digest.String()
	blob.Digest = mani.GetDescriptor().Digest
	blob.Size = mani.GetDescriptor().Size
	blob.MediaType = mani.GetDescriptor().MediaType
	utils.Debug(l, span, "HEAD sucessfull: %s size:%d ", utils.ShortenDigest(blob.Digest.String()), blob.Size)
	found = true
	return
}
