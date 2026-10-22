package magazine

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameRegcli                         = "magazine/regcli.go"
	fmtRepoRef                            = "%s/%s:%s"
	FailNoDigest                          = "No digest provided"
	FailNewRegClient                      = "Failed to create new regclient"
	FailCreateRef                         = "Failed creating ref"
	BlobURLFrmt                           = "blobUrl: %s"
	FailHeadManifestByUpstreamTripleByTag = "Failed to HEAD manifest by upstream triple and tag"
	FailManifestByUpstreamNotFound        = "Failed to find manifest by upstream triple"
	FailHeadManifestByRegRepoTag          = "Failed to HEAD manifest by reg/repo/tag"
	FailHeadManifestByRegRepoTagNotFound  = "Failed to find manifest by reg/repo/tag"
	cacheRegFlag                          = "cache-registry"
	FailGetManifestFromCache              = "Failed getting manifest from cache registry"
	FoundManifestInCache                  = "Found manifest in cache registry"
	FailBlobNotFoundUpstream              = "did NOT find blob in upstream registry"
)

// registryCfgFor returns the first RegistryConfig whose EffectiveURL or
// EffectiveName matches reg (first match wins, config order).
func (mag Magazine) registryCfgFor(reg string) (RegistryConfig, bool) {
	for _, regCfg := range mag.Config.Registries {
		if regCfg.EffectiveURL() == reg || regCfg.EffectiveName() == reg {
			return regCfg, true
		}
	}
	return RegistryConfig{}, false
}

// heuristicTLSOff reports whether the hardcoded fallback heuristics say to
// disable TLS — used for registries not present in the config (e.g. upstream
// discovery before the config was populated).
func heuristicTLSOff(reg string) bool {
	return strings.HasPrefix(reg, "localhost") || reg == "registry:5050" || strings.HasPrefix(reg, "172.")
}

// tlsAndCredsFor resolves the registry host to use plus its TLS setting and
// basic-auth credentials. Resolution order: TEST_REGISTRY override, then the
// matching RegistryConfig entry, then the TLS-off heuristics.
func (mag Magazine) tlsAndCredsFor(l *logrus.Entry, span trace.Span, reg string) (regOut string, tlsEnabled bool, user, pass string) {
	tlsEnabled = true
	regOut = reg
	if testReg := mag.Config.Args["TEST_REGISTRY"]; testReg != "" {
		utils.Info(l, span, "Testregistry configured: '%s'", testReg)
		return testReg, false, "", ""
	}
	// Config-driven path: match on EffectiveURL (network endpoint) first,
	// then EffectiveName for legacy compat.
	if regCfg, found := mag.registryCfgFor(reg); found {
		tlsEnabled = regCfg.TLS
		user, pass = regCfg.User, regCfg.Password
		if !tlsEnabled {
			utils.Info(l, span, "TLS disabled for '%s' (from registry config)", reg)
		}
		if user != "" {
			utils.Info(l, span, "Credentials configured for '%s'", reg)
		}
		return regOut, tlsEnabled, user, pass
	}
	if heuristicTLSOff(reg) {
		utils.Info(l, span, "Disabling TLS for: '%s' (heuristic)", reg)
		tlsEnabled = false
	}
	return regOut, tlsEnabled, user, pass
}

// newRegClientFor builds a regclient for the resolved host settings. A
// config.Host is needed whenever we need non-default settings (TLS off or
// basic-auth credentials); a plain regclient.New() is sufficient otherwise.
func newRegClientFor(l *logrus.Entry, span trace.Span, reg string, tlsEnabled bool, user, pass string) *regclient.RegClient {
	if !tlsEnabled {
		utils.Info(l, span, "Disable TLS for registry '%s'", reg)
	}
	if !tlsEnabled || user != "" {
		tlsConf := config.TLSEnabled
		if !tlsEnabled {
			tlsConf = config.TLSDisabled
		}
		rcHost := config.Host{
			Name:     reg,
			Hostname: reg,
			TLS:      tlsConf,
			User:     user,
			Pass:     pass,
		}
		return regclient.New(regclient.WithConfigHost(rcHost))
	}
	return regclient.New()
}

func (mag Magazine) NewRegClient(ctx context.Context, reg string) (rc *regclient.RegClient, regOut string, err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameRegcli,
		"func":     "NewRegClient",
		"registry": reg,
	})
	defer span.End()

	reg, tlsEnabled, user, pass := mag.tlsAndCredsFor(l, span, reg)
	utils.Info(l, span, "Create new regclient")
	return newRegClientFor(l, span, reg, tlsEnabled, user, pass), reg, nil
}

func (mag Magazine) getBlob(ctx context.Context, reg, repo, dig, mt string, size int64) (b []byte, blob mhTypes.BlobMeta, found bool, err error) {
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
	found = true
	return
}

// getBlobReader is the streaming counterpart of getBlob: it returns the
// regclient body reader instead of buffering the whole blob, so callers can
// stream straight into storage (constant memory).
func (mag Magazine) getBlobReader(ctx context.Context, reg, repo, dig, mt string, size int64) (r io.ReadCloser, blob mhTypes.BlobMeta, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameRegcli,
		"func":       "getBlobReader",
		"repository": repo,
		"registry":   reg,
		"digest":     dig,
	})
	defer span.End()
	if dig == "" {
		err = utils.NewError(l, span, FailNoDigest)
		return
	}
	rc, regURL, err := mag.NewRegClient(ctx, reg)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, FailNewRegClient)
		return
	}
	blobUrl := fmt.Sprintf("%s/%s", regURL, repo)
	ref, e := ref.New(blobUrl)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailCreateRef)
		return
	}
	ref.Digest = dig
	ddigest := digest.Digest(dig)
	desc := types.Descriptor{Digest: ddigest, Size: size}
	reader, e := rc.BlobGet(ctx, ref, desc)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while rc.BlobGet with digest")
		return
	}
	blob.Digest = ddigest
	blob.MediaType = mt
	blob.Size = size
	return reader, blob, true, nil
}

// GetBlobReaderByUpstreamTripleByDigest streams a blob from the cache registry
// (if configured) or the upstream registry. The caller MUST Close the returned
// reader. Mirrors getBlobByRegRepoMediaTypeDigestSize's precedence.
func (mag Magazine) GetBlobReaderByUpstreamTripleByDigest(ctx context.Context, up upstream.UpstreamTriple, dig string) (r io.ReadCloser, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameRegcli,
		"func":       "GetBlobReaderByUpstreamTripleByDigest",
		"repository": up.Repo,
		"registry":   up.Reg,
		"digest":     utils.ShortenDigest(dig),
	})
	defer span.End()
	if cacheReg, ok := mag.Config.Args[cacheRegFlag]; ok {
		r, blob, found, err := mag.getBlobReader(ctx, cacheReg, up.Repo, dig, up.MediaType, up.Size)
		if err != nil {
			return nil, blob, utils.ErrorHandler(l, span, err, FailGetManifestFromCache)
		}
		if found {
			return r, blob, nil
		}
	}
	r, blob, found, err := mag.getBlobReader(ctx, up.Reg, up.Repo, dig, up.MediaType, up.Size)
	if err != nil {
		return nil, blob, utils.ErrorHandler(l, span, err, FailGetManifestFromCache)
	}
	if !found {
		err = utils.NewError(l, span, FailBlobNotFoundUpstream)
		return nil, blob, err
	}
	return r, blob, nil
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
	// Do we have a cache registry?
	if cacheReg, ok := mag.Config.Args[cacheRegFlag]; ok {
		b, blob, found, err := mag.getBlob(ctx, cacheReg, up.Repo, up.Digest.String(), up.MediaType, up.Size)
		if err != nil {
			return b, blob, utils.ErrorHandler(l, span, err, FailGetManifestFromCache)
		}
		if found {
			utils.Debug(l, span, FoundManifestInCache)
			return b, blob, nil
		}
		utils.Debug(l, span, "did NOT find blob in cache registry")
	}
	b, blob, found, err := mag.getBlob(ctx, up.Reg, up.Repo, up.Digest.String(), up.MediaType, up.Size)
	if err != nil {
		return b, blob, utils.ErrorHandler(l, span, err, FailGetManifestFromCache)
	}
	if !found {
		err = utils.NewError(l, span, FailBlobNotFoundUpstream)
		return b, blob, err
	}
	return
}

func (mag Magazine) getBlobByRegRepoMediaTypeDigestSize(ctx context.Context, reg, repo, mt, dig string, size int64) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameRegcli,
		"func":       "getBlobByRegRepoMediaTypeDigestSize",
		"repository": repo,
		"registry":   reg,
		"digest":     utils.ShortenDigest(dig),
		"mediaType":  mt,
		"size":       size,
	})
	defer span.End()
	if cacheReg, ok := mag.Config.Args[cacheRegFlag]; ok {
		b, blob, found, err := mag.getBlob(ctx, cacheReg, repo, dig, mt, size)
		if err != nil {
			return b, blob, utils.ErrorHandler(l, span, err, FailGetManifestFromCache)
		}
		if found {
			utils.Debug(l, span, FoundManifestInCache)
			return b, blob, nil
		}
		utils.Debug(l, span, "did NOT find blob in cache registry")
	}
	b, blob, found, err := mag.getBlob(ctx, reg, repo, dig, mt, size)
	if err != nil {
		return b, blob, utils.ErrorHandler(l, span, err, FailGetManifestFromCache)
	}
	if !found {
		err = utils.NewError(l, span, FailBlobNotFoundUpstream)
		return b, blob, err
	}
	return
}

func (mag Magazine) GetBlobByUpstreamTriple(ctx context.Context, up upstream.UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameRegcli,
		"func":       "GetBlobByUpstreamTriple",
		"repository": up.Repo,
		"registry":   up.Reg,
		"digest":     up.Digest.String(),
	})
	defer span.End()
	return mag.getBlobByRegRepoMediaTypeDigestSize(ctx, up.Reg, up.Repo, up.MediaType, up.Digest.String(), up.Size)
}

func (mag Magazine) GetBlobByUpstreamTripleByDigest(ctx context.Context, up upstream.UpstreamTriple, dig string) (b []byte, blob mhTypes.BlobMeta, err error) {
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "GetBlobByUpstreamTripleByDigest",
		"repository": up.Repo,
		"registry":   up.Reg,
		"digest":     utils.ShortenDigest(dig),
	})
	defer span.End()
	return mag.getBlobByRegRepoMediaTypeDigestSize(ctx, up.Reg, up.Repo, up.MediaType, dig, up.Size)
}

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

func (mag Magazine) getManifestBytesByDigestForReg(ctx context.Context, reg, repo, dig string) (b []byte, blob mhTypes.BlobMeta, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "getManifestBytesByDigestForReg",
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
	return b, blob, true, nil
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
	// Do we have a cache registry?
	if cacheReg, ok := mag.Config.Args[cacheRegFlag]; ok {
		b, blob, found, e := mag.getManifestBytesByDigestForReg(ctx, cacheReg, repo, dig)
		if e != nil {
			return b, blob, utils.ErrorHandler(l, span, err, FailGetManifestFromCache)
		}
		if found {
			utils.Debug(l, span, FoundManifestInCache)
			return b, blob, nil
		}
	}
	b, blob, found, e := mag.getManifestBytesByDigestForReg(ctx, reg, repo, dig)
	if e != nil {
		return b, blob, utils.ErrorHandler(l, span, err, FailGetManifestFromCache)
	}
	if !found {
		return b, blob, utils.NewError(l, span, "did NOT find manifest in upstream registry")
	}
	return b, blob, nil
}

func (mag Magazine) GetManifestByUpstreamByDigest(ctx context.Context, up mhTypes.Upstream) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "GetManifestByUpstreamByDigest",
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
	blobUrl := fmt.Sprintf("%s/%s", up.Reg, up.Repo)
	utils.Debug(l, span, BlobURLFrmt, blobUrl)
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	defer span.End()
	blob = mhTypes.BlobMeta{}
	origUpReg := up.Reg
	// Do we have a cache registry?
	if cacheReg, ok := mag.Config.Args[cacheRegFlag]; ok {
		up.Reg = cacheReg
		b, blob, found, e := mag.getManifestByUpstreamTripleByDigest(ctx, up)
		if e != nil {
			return b, blob, utils.ErrorHandler(l, span, e, FailGetManifestFromCache)
		}
		if found {
			utils.Debug(l, span, FoundManifestInCache)
			return b, blob, err
		}
	}
	up.Reg = origUpReg
	b, blob, found, e := mag.getManifestByUpstreamTripleByDigest(ctx, up)
	if e != nil {
		return b, blob, utils.ErrorHandler(l, span, e, FailGetManifestFromCache)
	}
	if !found {
		return b, blob, utils.NewError(l, span, "Could not find manifest in upstream registry")
	}
	return b, blob, err
}

func (mag Magazine) getManifestByUpstreamTripleByDigest(ctx context.Context, up upstream.UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, found bool, err error) {
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "getManifestByUpstreamTripleByDigest",
		"repository": up.Repo,
		"digest":     up.Digest.String(),
		"registry":   up.Reg,
	})
	defer span.End()
	return mag.getManifestByRegRepoRef(ctx, up.Reg, up.Repo, up.Digest.String())
}

func (mag Magazine) GetManifestByUpstreamTripleByTag(ctx context.Context, up upstream.UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "GetManifestByUpstreamTripleByTag",
		"repository": up.Repo,
		"tag":        up.Tag,
		"registry":   up.Reg,
	})
	blobUrl := fmt.Sprintf(fmtRepoRef, up.Reg, up.Repo, up.Tag)
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	defer span.End()
	msg := fmt.Sprintf("BlobURL: '%s'", blobUrl)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
	blob = mhTypes.BlobMeta{}
	origUpReg := up.Reg
	// Do we have a cache registry?
	if cacheReg, ok := mag.Config.Args[cacheRegFlag]; ok {
		up.Reg = cacheReg
		b, blob, found, e := mag.getManifestByUpstreamTripleByTag(ctx, up)
		if e != nil {
			return b, blob, utils.ErrorHandler(l, span, e, FailGetManifestFromCache)
		}
		if found {
			utils.Debug(l, span, FoundManifestInCache)
			return b, blob, nil
		}
	}
	up.Reg = origUpReg
	b, blob, found, e := mag.getManifestByUpstreamTripleByTag(ctx, up)
	if e != nil {
		return b, blob, utils.ErrorHandler(l, span, e, FailGetManifestFromCache)
	}
	if !found {
		return b, blob, utils.NewError(l, span, "Could not find manifest in upstream registry")
	}
	return
}

func (mag Magazine) getManifestByRegRepoRef(ctx context.Context, reg, repo, mRef string) (b []byte, blob mhTypes.BlobMeta, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "getManifestByRegRepoRef",
		"repository": repo,
		"ref":        utils.ShortenDigest(mRef),
		"registry":   reg,
	})
	defer span.End()
	defer span.End()
	blob = mhTypes.BlobMeta{}
	rc, reg, e := mag.NewRegClient(ctx, reg)
	if e != nil {
		return b, blob, false, utils.ErrorHandler(l, span, e, "while creating regClient")
	}
	var dig, tag, blobUrl string
	if strings.HasPrefix(mRef, "sha256:") {
		dig = mRef
		blobUrl = fmt.Sprintf("%s/%s", reg, repo)
	} else {
		tag = mRef
		blobUrl = fmt.Sprintf(fmtRepoRef, reg, repo, tag)

	}
	span.SetAttributes(attribute.Key("url").String(blobUrl))
	utils.Info(l, span, "BlobURL: '%s'", blobUrl)
	ref, e := ref.New(blobUrl)
	if e != nil {
		return b, blob, false, utils.ErrorHandler(l, span, e, "while creating ref from blobUrl")
	}
	var mani manifest.Manifest
	// If we have a digest we use it
	if dig != "" {
		ref.Digest = dig
	} else {
		mani, e = rc.ManifestHead(ctx, ref)
		if e != nil {
			return b, blob, false, utils.ErrorHandler(l, span, e, "while ManifestHead")
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
		return b, blob, false, utils.ErrorHandler(l, span, e, "while regcli.ManifestGet")
	}
	b, e = mani.RawBody()
	if e != nil {
		return b, blob, false, utils.ErrorHandler(l, span, e, "while getting RawBody")
	}
	found = true
	return
}

func (mag Magazine) getManifestByUpstreamTripleByTag(ctx context.Context, up upstream.UpstreamTriple) (b []byte, blob mhTypes.BlobMeta, found bool, err error) {
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "getManifestByUpstreamTripleByTag",
		"repository": up.Repo,
		"tag":        up.Tag,
		"registry":   up.Reg,
	})
	defer span.End()
	return mag.getManifestByRegRepoRef(ctx, up.Reg, up.Repo, up.Tag)
}

func (mag Magazine) headManifestByRegRepoTag(ctx context.Context, reg, repo, tag string) (blob mhTypes.BlobMeta, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "HeadManifestByRegRepoTag",
		"repository": repo,
		"tag":        tag,
		"reg":        reg,
	})
	defer span.End()
	rc, reg, err := mag.NewRegClient(ctx, reg)
	if err != nil {
		return blob, false, utils.ErrorHandler(l, span, err, "Failed creating regClient")
	}
	blobUrl := fmt.Sprintf(fmtRepoRef, reg, repo, tag)
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
	// Do we have a cache registry?
	if cacheReg, ok := mag.Config.Args[cacheRegFlag]; ok {
		blob, found, err := mag.headManifestByRegRepoTag(ctx, cacheReg, repo, tag)
		if err != nil {
			return blob, false, utils.ErrorHandler(l, span, err, FailGetManifestFromCache)
		}
		if found {
			utils.Debug(l, span, FoundManifestInCache)
			return blob, true, nil
		}
	}
	utils.Debug(l, span, "No cache registry configured")
	blob, found, err = mag.headManifestByRegRepoTag(ctx, reg, repo, tag)
	if err != nil {
		return blob, false, utils.ErrorHandler(l, span, err, "Failed getting manifest from registry")
	}
	if found {
		return blob, true, nil

	}
	return blob, false, nil
}

// GetBlobRedirectURL builds the absolute upstream URL to which a client's
// blob GET should be redirected. The scheme is derived from the matching
// RegistryConfig.TLS flag: configured registries with TLS=false produce
// http:// URLs, everything else produces https://. Unknown registries
// default to https:// (safe for public upstreams like docker.io).
//
// Match on both EffectiveURL (primary — Upstream.Reg stores the URL) and
// EffectiveName (legacy data where Reg was the slug/hostname).
//
// When digest is non-empty it overrides up.Digest in the URL. Pass ""
// to use up.Digest (mirrors the variadic Upstream.GetBlobUrl contract).
func (mag Magazine) GetBlobRedirectURL(up mhTypes.Upstream, digest string) string {
	scheme := "https"
	for _, reg := range mag.Config.Registries {
		if reg.EffectiveURL() == up.Reg || reg.EffectiveName() == up.Reg {
			if !reg.TLS {
				scheme = "http"
			}
			break
		}
	}
	dig := digest
	if dig == "" {
		dig = up.Digest.String()
	}
	return fmt.Sprintf("%s://%s/v2/%s/blobs/%s", scheme, up.Reg, up.Repo, dig)
}
