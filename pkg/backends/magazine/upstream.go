package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (mag Magazine) NeoInsertUpstream(ctx context.Context, up types.Upstream) (upID int64, err error) {
	req := types.Request{
		Host:   up.Reg,
		Repo:   up.Repo,
		Tag:    up.Tag,
		Digest: up.Digest.String(),
	}

	res, _, err := mag.Database.Upstream_GetInsert_ByRequest(ctx, req)
	if err != nil {
		return
	}
	upID = res.ID
	return
}

func (mag Magazine) DiscoverUpstream(ctx context.Context, repo, tag string) (blob types.BlobMeta, registry string, found bool, err error) {
	ctx, span := tracer.Start(ctx, "DiscoverUpstream")
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(repo),
		attribute.Key("tag").String(tag),
	)
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/upstream.go",
		"func":    "DiscoverUpstream",
		"repo":    repo,
		"tag":     tag,
	})
	reg, ok := mag.GetConfig("registries")
	if !ok {
		l.Fatalf("Failed to get 'registries' from config, please check your config")
	}
	l.Infof("Using registries: %s", reg)
	regList := strings.Split(reg, ",")
	for _, reg := range regList {
		regCtx, regSpan := tracer.Start(ctx, fmt.Sprintf("DiscoverUpstream_%s", reg))
		defer regSpan.End()

		b, regFound, e := upstream.HeadManifestByUpstreamTripleByTag(regCtx, reg, repo, tag)
		if e != nil {
			l.Error(e.Error())
			span.RecordError(e)
			regSpan.End()
			continue
		}
		msg := fmt.Sprintf(" After Head [%t]", regFound)
		regSpan.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		if !regFound || b.Digest == "" {
			msg := fmt.Sprintf("No manifest found in %s", reg)
			regSpan.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			regSpan.End()
			continue
		}
		msg = fmt.Sprintf("blob:'%s' / mediaType:'%s'", utils.ShortenDigest(b.Digest.String()), b.MediaType)
		regSpan.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		regSpan.End()
		return b, reg, true, nil
	}
	return
}

func (mag Magazine) upstreamBlob_Insert(ctx context.Context, req types.Request, dig digest.Digest) (err error) {
	ctx, span := tracer.Start(ctx, "upstreamBlob_Insert")
	defer span.End()
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/upstream.go",
		"func":    "upstreamBlob_Insert",
	})
	ups := types.Upstream{
		Reg:     req.Host,
		Repo:    req.Repo,
		Tag:     req.Tag,
		Digest:  dig,
		Profile: req.Profile,
	}
	upID, err := mag.NeoInsertUpstream(ctx, ups)
	if err != nil {
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}
	msg := fmt.Sprintf("Insert Upstream with ID '%d'", upID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Warn(msg)
	prf, prfFound, e := mag.Database.Profile_Get_ByLabel(ctx, req.Profile)
	if e != nil {
		err = errors.Wrap(e, "failed to create collection")
		l.Error(err.Error())
		span.RecordError(err)
		return
	}
	if !prfFound {
		err = errors.Errorf("profile '%s' not found", req.Profile)
		l.Error(err.Error())
		span.RecordError(err)
		return
	}
	// Insert Manifest
	maniID, e := mag.Database.Manifest_GetInsertWithProfile(ctx, upID, req.Profile)
	if e != nil {
		err = errors.Wrap(e, "Failed to insert manifest")
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}
	msg = fmt.Sprintf("Insert Manifest with ID '%d'", maniID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Warn(msg)
	upt := upstream.UpstreamTriple{
		Reg:     req.Host,
		Repo:    req.Repo,
		Tag:     req.Tag,
		Digest:  dig,
		Profile: req.Profile,
	}
	b, maniMeta, e := upstream.GetManifestByUpstreamTripleByDigest(ctx, upt)
	if e != nil {
		err = errors.Wrap(e, "Failed to get manifest")
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}
	msg = fmt.Sprintf("Got Manifest with digest '%s'", utils.ShortenDigest(maniMeta.Digest.String()))
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Warn(msg)
	// Put Manifest in Minio
	e = mag.Minio.PutManifest(ctx, b, upt)
	if e != nil {
		err = errors.Wrap(e, "Failed to get put manifest in minio")
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}
	rawMani, e := minio.RawManifestFromBlob(b)
	if e != nil {
		err = errors.Wrap(err, "failed to RawManifestFromBlob")
		span.RecordError(err)
		l.Error(err)
		return err
	}
	switch maniMeta.MediaType {
	case "application/vnd.docker.distribution.manifest.v1+prettyjws":
		err = fmt.Errorf("manifest type is not supported: %s", maniMeta.MediaType)
		return err
	case "application/vnd.docker.distribution.manifest.v2+json":
		upt.Digest = rawMani.ConfigInfo().Digest
	default:
		l.Fatalf("Unknown media type: %s", maniMeta.MediaType)
	}
	cfgByte, cfgBlob, e := upstream.GetBlobByUpstreamTriple(ctx, upt)
	if e != nil {
		err = errors.Wrapf(e, "failed to GetBlobByUpstreamTriple")
		l.Error(err)
		return err
	}
	l.Warnf("Config Digest: %s", cfgBlob.Digest.String())
	e = mag.Minio.PutConfig(ctx, cfgByte, cfgBlob.Digest.String())
	if e != nil {
		err = errors.Wrapf(err, "while Minio.PutConfig")
		l.Error(err)
		return err
	}
	maniData, e := mag.Minio.NewManifestFromBytes(b)
	if e != nil {
		err = errors.Wrapf(err, "while Minio.NewManifestFromBytes")
		l.Error(err)
		return err
	}
	maniData.Type = "PULLED_BY"
	// Insert Config
	cfgID, e := mag.Database.Config_Insert(ctx, maniData.Config)
	if e != nil {
		err = errors.Wrap(e, "Failed to insert config")
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}
	l.Warnf("Config ID: %d", cfgID)
	l.Warnf("Platform: %s/%s", maniData.Platform.OS, maniData.Platform.Arch)
	e = mag.Database.Platform_GetInsert(ctx, &maniData.Platform)
	if e != nil {
		err = errors.Wrap(e, "Failed to insert platform")
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}
	l.Warnf("Platform ID: %d", maniData.Platform.ID)
	e = mag.NeoLinkManifestToPlatform(ctx, maniID, maniData.Platform.ID)
	if e != nil {
		err = errors.Wrap(e, "Failed to link manifest to platform")
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}
	// Create Collection
	col, _, e := mag.Database.Collection_GetCreate_ByRepoTag(ctx, req.Repo, req.Tag)
	if e != nil {
		err = errors.Wrap(e, "failed to get collection")
		span.RecordError(err)
		l.Error(err)
		return
	}
	e = mag.Database.LinkCollectionToManifest(ctx, col.ID, maniID)
	if e != nil {
		err = errors.Wrap(e, "failed to link collection to manifest")
		span.RecordError(err)
		l.Error(err)
		return
	}
	e = mag.Database.LinkCollectionTagToManifest(ctx, col.ColTag.ID, maniID)
	if e != nil {
		err = errors.Wrap(e, "failed to link collection tag to manifest")
		span.RecordError(err)
		l.Error(err)
		return
	}
	// Insert Layer
	for idx, layer := range maniData.Layers {
		lID, e := mag.Database.Layer_PlainInsert(ctx, layer)
		if e != nil {
			err = errors.Wrap(e, "Failed to insert layer")
			l.Error(err.Error())
			span.RecordError(err)
			return err
		}
		l.Warnf("Layer [%d] ID: %d", idx, lID)
		// TODO: Do we need to pull them here?
	}
	maniDataID, e := mag.NeoInsertManifestData(ctx, maniID, maniData, "PULLED_BY")
	if e != nil {
		err = errors.Wrap(e, "Failed to insert manifest data")
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}
	l.Warnf("ManifestData ID: %d", maniDataID)

	msg = fmt.Sprintf("Insert Config %d", cfgID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Warn(msg)
	e = mag.Database.LinkManifestDataToProfile(ctx, maniDataID, prf.ID, "PULLED_BY")
	if e != nil {
		err = errors.Wrap(e, "failed to link profile and manifestdata")
		l.Error(err)
		return
	}
	// Unwind Profile
	e = mag.Database.Profile_UnwindRules(ctx, prf.ID, maniDataID)
	if e != nil {
		err = errors.Wrap(e, "failed to unwind profile")
		l.Error(err)
		span.RecordError(err)
		return
	}
	// Create OCI Manifest
	mani, _, e := mag.Database.Manifest_GetByID_WithProfile(ctx, maniID, prf.ID)
	if e != nil {
		err = errors.Wrap(e, "Failed to get manifest")
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}

	ociDataID, e := mag.CreateOCIv1_ForProfile(ctx, mani, maniData, prf)
	if e != nil {
		err = errors.Wrap(e, "failed to create OCIv1 manifest")
		l.Error(err)
		span.RecordError(err)
		return
	}
	e = mag.Database.Profile_UnwindRules(ctx, prf.ID, ociDataID)
	if e != nil {
		err = errors.Wrap(e, "failed to unwind profile")
		l.Error(err)
		span.RecordError(err)
		return
	}

	// Compute ManifestList?
	e = mag.ComputeManifestlists(ctx)
	if e != nil {
		err = errors.Wrap(e, "failed to ComputeManifestlists")
		l.Error(err)
		span.RecordError(err)
		return
	}
	return
}

func (mag Magazine) UpstreamBlob_InsertManifest(ctx context.Context, req types.Request, blob types.BlobMeta) (err error) {
	ctx, span := tracer.Start(ctx, "UpstreamBlob_InsertManifest")
	defer span.End()
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/upstream.go",
		"func":    "UpstreamBlob_InsertManifest",
	})
	l.Warn("START")
	switch blob.MediaType {
	case "application/vnd.docker.distribution.manifest.v2+json":
		err = mag.upstreamBlob_Insert(ctx, req, blob.Digest)
		if err != nil {
			l.Error(err.Error())
			span.RecordError(err)
			return err
		}

		return err
	default:
		err = fmt.Errorf("unknown mediaType: %s", blob.MediaType)
		l.Error(err.Error())
	}
	return
}

func (mag Magazine) UpstreamBlob_InsertIndex(ctx context.Context, req types.Request, blob types.BlobMeta) (err error) {
	ctx, span := tracer.Start(ctx, "UpstreamBlob_InsertIndex")
	defer span.End()
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/upstream.go",
		"func":    "UpstreamBlob_InsertIndex",
	})
	switch blob.MediaType {
	case "application/vnd.docker.distribution.manifest.list.v2+json":
		// TODO: Fetch ManifestList and insert all manifest as upstream
		up := upstream.UpstreamTriple{
			Reg:       req.Host,
			Repo:      req.Repo,
			Tag:       req.Tag,
			Digest:    blob.Digest,
			MediaType: blob.MediaType,
		}
		b, blob, err := upstream.GetManifestByUpstreamTripleByTag(ctx, up)
		if err != nil {
			l.Error(err.Error())
			span.RecordError(err)
			return err
		}
		msg := fmt.Sprintf("Found Blob %v", blob)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Warn(msg)
		var ml manifestlist.ManifestList
		err = json.Unmarshal(b, &ml)
		if err != nil {
			l.Error(err.Error())
			span.RecordError(err)
			return err
		}
		l.Warnf("Found %d manifests", len(ml.Manifests))
		for _, m := range ml.Manifests {
			msg := fmt.Sprintf("Found Manifest with Platform.Arch %s", m.Platform.Architecture)
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			l.Debug(msg)
			switch m.Platform.Architecture {
			case "amd64":
				err := mag.upstreamBlob_Insert(ctx, req, m.Digest)
				if err != nil {
					l.Error(err.Error())
					span.RecordError(err)
					return err
				}
			case "arm64":
				err := mag.upstreamBlob_Insert(ctx, req, m.Digest)
				if err != nil {
					l.Error(err.Error())
					span.RecordError(err)
					return err
				}
			default:
				msg := fmt.Sprintf("Skipping Platform.Arch %s", m.Platform.Architecture)
				span.AddEvent("log", trace.WithAttributes(
					attribute.String("log.severity", "debug"),
					attribute.String("log.message", msg),
				))
				l.Debug(msg)
			}

		}
	}
	return
}
