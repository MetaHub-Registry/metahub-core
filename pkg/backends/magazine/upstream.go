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

const (
	prfGenericLabel = `generic`
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

func (mag Magazine) upstreamBlob_InsertManifestDesc(ctx context.Context, req types.Request, maniIn manifestlist.ManifestDescriptor) (err error) {
	ctx, span := tracer.Start(ctx, "upstreamBlob_InsertManifestDesc")
	defer span.End()
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/upstream.go",
		"func":    "upstreamBlob_InsertManifestDesc",
	})
	dig := maniIn.Digest
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
	ups.ID = upID
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
	msg = fmt.Sprintf("Found profile '%s'", req.Profile)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
	// Insert Manifest
	maniID, e := mag.Database.Manifest_GetInsert_WithDesc(ctx, req, ups, maniIn)
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
	maniData, e := mag.Minio.NewManifestDataFromBytes(ctx, b)
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

func (mag Magazine) upstreamBlob_InsertGeneric(ctx context.Context, req types.Request, col types.Collection, ups types.Upstream, maniData types.ManifestData) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "magazine/upstream.go",
		"func":    "upstreamBlob_InsertGeneric",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
		"digest":  ups.Digest.String(),
	})
	defer span.End()
	// FIXME: Is that correct? I reckon we need to build from ManifestData.Labels?
	as := maniData.Annotations.ToSlice()
	// 1. Insert Manifest
	maniID, e := mag.Database.Manifest_GetInsert(ctx, ups, as...)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to insert manifest")
	}
	utils.Debug(l, span, "Insert Manifest with ID '%d'", maniID)
	l.Warnf("Platform ID: %d", maniData.Platform.ID)
	// 2. Link Manifest to Platform
	e = mag.NeoLinkManifestToPlatform(ctx, maniID, maniData.Platform.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to link manifest to platform")
	}
	// 3. Link Collection to Manifest
	e = mag.Database.LinkCollectionToManifest(ctx, col.ID, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to link collection to manifest")
	}
	// 4. Link CollectionTag to Manifest
	e = mag.Database.LinkCollectionTagToManifest(ctx, col.ColTag.ID, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to link collection tag to manifest")
	}
	// 5. Insert Manifest Data
	maniDataID, e := mag.NeoInsertManifestData(ctx, maniID, maniData, maniData.Type)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to insert manifest data")
	}
	l.Warnf("ManifestData ID: %d", maniDataID)
	// 6 Link Manifest Data to generic profile
	//// 6.1 Get 'generic' profile

	prf, prfFound, e := mag.Database.Profile_Get_ByLabel(ctx, prfGenericLabel)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to get collection")
	}
	if !prfFound {
		return utils.NewError(l, span, "profile '%s' not found", prfGenericLabel)
	}
	utils.Debug(l, span, "Found profile '%s'", prfGenericLabel)
	//// 6.2 Link
	e = mag.Database.LinkManifestDataToProfile(ctx, maniDataID, prf.ID, "PULLED_BY")
	if e != nil {
		err = errors.Wrap(e, "failed to link profile and manifestdata")
		l.Error(err)
		return
	}
	// Create generic OCI Manifest (no manipulations)
	mani, maniFound, e := mag.Database.Manifest_GetByID(ctx, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to get manifest by ID")
	}
	if !maniFound {
		return utils.NewError(l, span, "manifest with ID '%d' not found", maniID)
	}
	ociManiID, e := mag.OCI_CreateBaseManifest(ctx, mani, maniData, 0)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to create OCIv1 manifest")
	}

	e = mag.ComputeManifestlists_ForManiProfile(ctx, ociManiID, prf.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to compute ManifestList manifest by ID")
	}
	return
}

func (mag Magazine) upstreamBlob_InsertProfile(ctx context.Context, req types.Request, col types.Collection, ups types.Upstream, maniData types.ManifestData) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "magazine/upstream.go",
		"func":    "upstreamBlob_InsertProfile",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
		"digest":  ups.Digest.String(),
	})
	defer span.End()
	prf, prfFound, e := mag.Database.Profile_Get_ByLabel(ctx, req.Profile)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to create collection")
	}
	if !prfFound {
		return utils.NewError(l, span, "profile '%s' not found", req.Profile)
	}
	// 1. Insert Manifest
	maniID, e := mag.Database.Manifest_GetInsert(ctx, ups)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to insert manifest")
	}
	utils.Debug(l, span, "Insert Manifest with ID '%d'", maniID)
	l.Warnf("Platform ID: %d", maniData.Platform.ID)
	// 2. Link Manifest to Platform
	e = mag.NeoLinkManifestToPlatform(ctx, maniID, maniData.Platform.ID)
	if e != nil {
		err = errors.Wrap(e, "Failed to link manifest to platform")
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}
	// 3. Link Collection to Manifest
	e = mag.Database.LinkCollectionToManifest(ctx, col.ID, maniID)
	if e != nil {
		err = errors.Wrap(e, "failed to link collection to manifest")
		span.RecordError(err)
		l.Error(err)
		return
	}
	// 4. Link CollectionTag to Manifest
	e = mag.Database.LinkCollectionTagToManifest(ctx, col.ColTag.ID, maniID)
	if e != nil {
		err = errors.Wrap(e, "failed to link collection tag to manifest")
		span.RecordError(err)
		l.Error(err)
		return
	}
	// 5. Insert Manifest Data
	maniDataID, e := mag.NeoInsertManifestData(ctx, maniID, maniData, "PULLED_BY")
	if e != nil {
		err = errors.Wrap(e, "Failed to insert manifest data")
		l.Error(err.Error())
		span.RecordError(err)
		return err
	}
	l.Warnf("ManifestData ID: %d", maniDataID)

	e = mag.Database.LinkManifestDataToProfile(ctx, maniDataID, prf.ID, "PULLED_BY")
	if e != nil {
		err = errors.Wrap(e, "failed to link profile and manifestdata")
		l.Error(err)
		return
	}
	// TODO: Unwind Profile
	/*
		e = mag.Database.Profile_UnwindRules(ctx, prf.ID, maniDataID)
		if e != nil {
			err = errors.Wrap(e, "failed to unwind profile")
			l.Error(err)
			span.RecordError(err)
			return
		}
	*/
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

func (mag Magazine) upstreamBlob_Insert(ctx context.Context, req types.Request, dig digest.Digest) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "magazine/upstream.go",
		"func":    "upstreamBlob_Insert",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
		"digest":  dig.String(),
	})
	defer span.End()
	// Get Collection
	col, _, e := mag.Database.Collection_GetCreate_ByRepoTag(ctx, req.Repo, req.Tag)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to get/create collection")
	}
	up := types.Upstream{
		Reg:     req.Host,
		Repo:    req.Repo,
		Tag:     req.Tag,
		Digest:  dig,
		Profile: req.Profile,
	}
	// 1. Insert into Graph
	upID, e := mag.NeoInsertUpstream(ctx, up)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while inserting upstream")
	}
	l.Debugf("Upstream ID: %d", upID)
	// 2 . Get the Blob
	maniBytes, maniMeta, e := upstream.GetManifest_ByUpstreamByDigest(ctx, up)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while getting manifest")
	}
	utils.Debug(l, span, "Found manifest upstream: %s", utils.ShortenDigest(maniMeta.Digest.String()))
	// Put Manifest in Minio
	e = mag.Minio.PutManifest_ByUpstream(ctx, maniBytes, up)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to get put manifest in minio")
	}
	rawMani, e := minio.RawManifestFromBlob(maniBytes)
	if e != nil {
		utils.ErrorHandler(l, span, e, "failed to RawManifestFromBlob")
	}
	// Fetch Upstream Config
	configUpt := upstream.UpstreamTriple{
		Reg:     req.Host,
		Repo:    req.Repo,
		Tag:     req.Tag,
		Digest:  dig,
		Profile: req.Profile,
	}
	switch maniMeta.MediaType {
	case "application/vnd.docker.distribution.manifest.v1+prettyjws":
		err = fmt.Errorf("manifest type is not supported: %s", maniMeta.MediaType)
		return err
	case "application/vnd.docker.distribution.manifest.v2+json":
		configUpt.Digest = rawMani.ConfigInfo().Digest
	default:
		l.Fatalf("Unknown media type: %s", maniMeta.MediaType)
	}
	cfgByte, cfgBlob, e := upstream.GetBlobByUpstreamTriple(ctx, configUpt)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to get config via GetBlobByUpstreamTriple")
	}
	utils.Debug(l, span, "Found config upstream: %s", utils.ShortenDigest(cfgBlob.Digest.String()))
	e = mag.Minio.PutConfig(ctx, cfgByte, cfgBlob.Digest.String())
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while putting config in Minio")
	}
	maniData, e := mag.Minio.NewManifestDataFromBytes(ctx, maniBytes)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while Minio.NewManifestData_FromBytes")

	}
	maniData.Type = "PULLED_BY"
	// Insert Raw Config
	cfgID, e := mag.Database.Config_Insert(ctx, maniData.Config)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to insert config")
	}
	maniData.Config.ID = cfgID
	utils.Debug(l, span, "Config ID: %d", cfgID)
	utils.Debug(l, span, "Platform: %s/%s", maniData.Platform.OS, maniData.Platform.Arch)
	e = mag.Database.Platform_GetInsert(ctx, &maniData.Platform)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to insert platform")
	}
	// Insert Layer
	for idx, layer := range maniData.Layers {
		_, e := mag.Database.Layer_PlainInsert(ctx, layer)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to insert layer #%d w/ digest '%s'", idx, utils.ShortenDigest(layer.Digest))
		}
		utils.Debug(l, span, "inserted layer #%d w/ digest '%s'", idx, utils.ShortenDigest(layer.Digest))
		// TODO: Do we need to pull them here?
	}
	/// Generic
	err = mag.upstreamBlob_InsertGeneric(ctx, req, col, up, maniData)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "while inserting generic")
	}
	// Profile
	if req.Profile == "generic" {
		utils.Debug(l, span, "Skipping profile 'generic'")
		return
	}
	up.Profile = req.Profile
	err = mag.upstreamBlob_InsertProfile(ctx, req, col, up, maniData)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "while inserting for profile")
	}
	return
}

func (mag Magazine) UpstreamBlob_InsertManifest(ctx context.Context, req types.Request, blob types.BlobMeta) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "magazine/upstream.go",
		"func":    "UpstreamBlob_InsertManifest",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
	})
	defer span.End()
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
				err := mag.upstreamBlob_InsertManifestDesc(ctx, req, m)
				if err != nil {
					l.Error(err.Error())
					span.RecordError(err)
					return err
				}
			case "arm64":
				err := mag.upstreamBlob_InsertManifestDesc(ctx, req, m)
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
