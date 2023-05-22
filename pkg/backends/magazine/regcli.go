package magazine

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	mhTypes "github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"

	"github.com/opencontainers/go-digest"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/config"
	"github.com/regclient/regclient/types"
	"github.com/regclient/regclient/types/manifest"
	"github.com/regclient/regclient/types/platform"
	"github.com/regclient/regclient/types/ref"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameRegcli                         = "magazine/regcli.go"
	FailNoDigest                          = "No digest provided"
	FailNewRegClient                      = "Failed to create new regclient"
	FailCreateRef                         = "Failed creating ref"
	BlobURLFrmt                           = "blobUrl: %s"
	FailHeadManifestByUpstreamTripleByTag = "Failed to HEAD manifest by upstream triple and tag"
	FailManifestByUpstreamNotFound        = "Failed to find manifest by upstream triple"
	FailHeadManifestByRegRepoTag          = "Failed to HEAD manifest by reg/repo/tag"
	FailHeadManifestByRegRepoTagNotFound  = "Failed to find manifest by reg/repo/tag"
)

func (mag Magazine) NewRegClient(ctx context.Context, reg string) (rc *regclient.RegClient, regOut string, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameRegcli,
		"func":     "NewRegClient",
		"registry": reg,
	})
	defer span.End()
	_ = ctx
	tlsEnabled := true
	testReg, testRegOK := mag.Config.Args["TEST_REGISTRY"]
	switch {
	case testRegOK && testReg != "":
		utils.Info(l, span, "Testregistry configured: '%s'", testReg)
		reg = testReg
		tlsEnabled = false
	case strings.HasPrefix(reg, "localhost") || reg == "registry:5050" || strings.HasPrefix(reg, "172."):
		utils.Info(l, span, "Disabling TLS for: '%s'", reg)
		tlsEnabled = false
	}
	utils.Info(l, span, "Create new regclient")
	rc = regclient.New()
	// Deal with TLS
	if !tlsEnabled {
		rcHost := config.Host{
			Name:     reg,
			Hostname: reg,
			TLS:      config.TLSDisabled,
		}
		rc = regclient.New(
			regclient.WithConfigHost(rcHost),
		)
	}
	return rc, reg, nil

}

func (mag Magazine) getBlob(ctx context.Context, reg, repo, dig, mt string, size int64) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameRegcli,
		"func":       "getBlob",
		"repository": repo,
		"registry":   reg,
		"digest":     dig,
	})
	defer span.End()
	if dig == "" {
		err = utils.NewError(l, span, FailNoDigest)
		return
	}

	blob = mhTypes.BlobMeta{}
	rc, reg, err := mag.NewRegClient(ctx, reg)
	blobUrl := fmt.Sprintf("%s/%s", reg, repo)
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	ref, e := ref.New(blobUrl)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailCreateRef)
		return
	}
	ref.Digest = dig
	ddigest := digest.Digest(dig)
	desc := types.Descriptor{Digest: ddigest, Size: size}
	blobReader, e := rc.BlobGet(ctx, ref, desc)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while rc.BlogGet with digest")
		return
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(blobReader)
	b = buf.Bytes()
	blob.Size = int64(len(b))
	blob.Digest = ddigest
	blob.MediaType = mt
	return
}

func (mag Magazine) GetBlobByUpstream(ctx context.Context, up mhTypes.Upstream) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameRegcli,
		"func":       "GetBlobByUpstreamTriple",
		"repository": up.Repo,
		"registry":   up.Reg,
		"digest":     up.Digest.String(),
	})
	defer span.End()
	_ = l
	return mag.getBlob(ctx, up.Reg, up.Repo, up.Digest.String(), up.MediaType, up.Size)
}

func (mag Magazine) GetBlobByUpstreamTriple(ctx context.Context, up upstream.UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameRegcli,
		"func":       "GetBlobByUpstreamTriple",
		"repository": up.Repo,
		"registry":   up.Reg,
		"digest":     up.Digest.String(),
	})
	defer span.End()
	_ = l
	return mag.getBlob(ctx, up.Reg, up.Repo, up.Digest.String(), up.MediaType, up.Size)
}

func (mag Magazine) GetBlobByUpstreamTripleByDigest(ctx context.Context, up upstream.UpstreamTriple, dig string) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "GetBlobByUpstreamTripleByDigest",
		"repository": up.Repo,
		"registry":   up.Reg,
		"digest":     utils.ShortenDigest(dig),
	})
	defer span.End()
	_ = l
	return mag.getBlob(ctx, up.Reg, up.Repo, dig, up.MediaType, up.Size)
}

/*
	func GetManifestByUpstream(ctx context.Context, up mhTypes.Upstream) (mani mhTypes.Manifest, err error) {
		l, ctx, span := utils.InitLogging(ctx, tr, map[string]any{
			"package":    pkgNameUp,
			"func":       "GetManifestByUpstream",
			"repository": up.Repo,
			"registry":   up.Reg,
			"digest":     up.Digest.String(),
		})
		defer span.End()
		if b, blob, err := getManifestBytes(ctx, up.Reg, up.Repo, up.Digest.String()); err != nil {
			return mani, utils.ErrorHandler(l, span, err, FailManifestByUpstreamNotFound)
		} else {
			if rawMani, err := minio.RawManifestFromBlob(b); err != nil {
				return mani, utils.ErrorHandler(l, span, err, "while minio.RawManifestFromBlob")
			}

		}
		return mani, nil
	}
*/

func (mag Magazine) getManifestBytesByTag(ctx context.Context, reg, repo, tag string) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "getManifestBytesByTag",
		"repository": repo,
		"registry":   reg,
		"tag":        tag,
	})
	defer span.End()
	blob, found, err := mag.HeadManifestByRegRepoTag(ctx, reg, repo, tag)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, FailHeadManifestByRegRepoTag)
	} else if !found {
		err = utils.NewError(l, span, FailHeadManifestByRegRepoTagNotFound)
	}
	dig := blob.Digest.String()
	utils.Info(l, span, "Found digest: %s", dig)
	return mag.getManifestBytesByDigest(ctx, reg, repo, dig)
}

func (mag Magazine) getManifestBytesByDigest(ctx context.Context, reg, repo, dig string) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "getManifestBytesByDigest",
		"repository": repo,
		"registry":   reg,
		"digest":     dig,
	})
	defer span.End()
	if dig == "" {
		err = utils.NewError(l, span, FailNoDigest)
		return
	}
	blob = mhTypes.BlobMeta{}
	rc, reg, err := mag.NewRegClient(ctx, reg)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, FailNewRegClient)
		return
	}
	blobUrl := fmt.Sprintf("%s/%s", reg, repo)
	utils.Info(l, span, BlobURLFrmt, blobUrl)
	ref, e := ref.New(blobUrl)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailCreateRef)
		return
	}
	var mani manifest.Manifest
	// If we have a digest we use it
	d := types.Descriptor{
		MediaType: types.MediaTypeDocker2Manifest,
		Digest:    digest.Digest(dig),
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
	return b, blob, nil
}

func (mag Magazine) GetManifest_ByUpstreamByDigest(ctx context.Context, up mhTypes.Upstream) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "GetManifest_ByUpstreamByDigest",
		"reg":     up.Reg,
		"repo":    up.Repo,
		"tag":     up.Tag,
		"profile": up.Profile,
		"digest":  up.Digest.String(),
	})
	defer span.End()
	if up.Digest.String() != "" {
		utils.Debug(l, span, "Digest is set, fetching directly")
		return mag.getManifestBytesByDigest(ctx, up.Reg, up.Repo, up.Digest.String())
	} else {
		return mag.getManifestBytesByTag(ctx, up.Reg, up.Repo, up.Tag)
	}
}

func (mag Magazine) GetManifestByUpstreamTripleByDigest(ctx context.Context, up upstream.UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "GetManifestByUpstreamTripleByDigest",
		"repository": up.Repo,
		"digest":     up.Digest.String(),
		"registry":   up.Reg,
	})
	reg := up.Reg
	blobUrl := fmt.Sprintf("%s/%s", up.Reg, up.Repo)
	utils.Debug(l, span, BlobURLFrmt, blobUrl)
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	defer span.End()
	blob = mhTypes.BlobMeta{}
	rc, reg, err := mag.NewRegClient(ctx, reg)
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

func (mag Magazine) GetManifestByUpstreamTripleByTag(ctx context.Context, up upstream.UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
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
	rc, reg, e := mag.NewRegClient(ctx, up.Reg)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while creating regClient")
		return
	}
	up.Reg = reg
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

func (mag Magazine) HeadManifestByRegRepoTag(ctx context.Context, reg, repo, tag string) (blob mhTypes.BlobMeta, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "HeadManifestByRegRepoTag",
		"repository": repo,
		"tag":        tag,
		"reg":        reg,
	})
	defer span.End()
	blob = mhTypes.BlobMeta{}
	rc, reg, err := mag.NewRegClient(ctx, reg)
	if err != nil {
		return blob, false, utils.ErrorHandler(l, span, err, "Failed creating regClient")
	}
	blobUrl := fmt.Sprintf("%s/%s:%s", reg, repo, tag)
	utils.Debug(l, span, "blobUrl: %s ", blobUrl)
	span.SetAttributes(attribute.Key("url").String(blobUrl))

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
