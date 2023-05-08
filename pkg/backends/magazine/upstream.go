package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/opencontainers/go-digest"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	prfGenericLabel                 = `generic`
	pkgNameUp                       = "magazine/upstream.go"
	failInsMani                     = "failed to insert manifest"
	failInsManiData                 = "failed to insert manifestData"
	failGetMani                     = "Failed to get manifest"
	FailLinkManifestToPlatform      = "Failed to link manifest to platform"
	FailLinkCollectionToPlatform    = "failed to link collection to manifest"
	FailLinkCollectionTagToPlatform = "failed to link collection tag to manifest"
	FailLinkProfileToManifestData   = "failed to link profile and manifestdata"
	FailUnwind                      = "failed to unwind profile"
	FailCreateOCIv1Manifest         = "failed to create OCIv1 manifest"
)

func (mag Magazine) NeoInsertUpstream(ctx context.Context, up types.Upstream) (upID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "NeoInsertUpstream",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  up.Digest.String(),
	})
	defer span.End()
	req := types.Request{
		Host:   up.Reg,
		Repo:   up.Repo,
		Tag:    up.Tag,
		Digest: up.Digest.String(),
	}

	res, _, e := mag.Database.Upstream_GetInsert_ByRequest(ctx, req)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to Upstream_GetInsert_ByRequest")
		return

	}
	utils.Debug(l, span, "insert sucessful with ID '%d'", res.ID)
	upID = res.ID
	return
}

func (mag Magazine) DiscoverUpstream(ctx context.Context, repo, tag string) (blob types.BlobMeta, registry string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "DiscoverUpstream",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()
	reg, ok := mag.GetConfig("registries")
	if !ok {
		err = utils.NewError(l, span, "Failed to get 'registries' from config, please check your config")
		return
	}
	utils.Info(l, span, "Using registries: %s", reg)
	regList := strings.Split(reg, ",")
	for _, reg := range regList {
		regCtx, regSpan := tracer.Start(ctx, fmt.Sprintf("DiscoverUpstream_%s", reg))
		defer regSpan.End()
		b, regFound, e := upstream.HeadManifestByUpstreamTripleByTag(regCtx, reg, repo, tag)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to HeadManifestByUpstreamTripleByTag")
			continue
		}
		utils.Debug(l, span, " After Head [%t]", regFound)
		if !regFound || b.Digest == "" {
			utils.Debug(l, span, "No manifest found in %s", reg)
			regSpan.End()
			continue
		}
		utils.Debug(l, span, "blob:'%s' / mediaType:'%s'", utils.ShortenDigest(b.Digest.String()), b.MediaType)
		regSpan.End()
		return b, reg, true, nil
	}
	return
}

func (mag Magazine) upstreamBlobInsertManifestDesc(ctx context.Context, req types.Request, maniIn manifestlist.ManifestDescriptor) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "upstreamBlobInsertManifestDesc",
	})
	defer span.End()
	dig := maniIn.Digest
	ups := types.Upstream{
		Reg:     req.Host,
		Repo:    req.Repo,
		Tag:     req.Tag,
		Digest:  dig,
		Profile: req.Profile,
	}
	upID, maniID, prf, e := mag.Database.CollectionGetCreateUpstreamTriple(ctx, ups, maniIn)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to NeoInsertUpstream")
		return
	}
	ups.ID = upID
	// Insert Manifest

	upt := upstream.UpstreamTriple{
		Reg:     req.Host,
		Repo:    req.Repo,
		Tag:     req.Tag,
		Digest:  dig,
		Profile: req.Profile,
	}
	b, maniMeta, e := upstream.GetManifestByUpstreamTripleByDigest(ctx, upt)
	if e != nil {
		return utils.ErrorHandler(l, span, e, failGetMani)
	}
	utils.Debug(l, span, "Got Manifest with digest '%s'", utils.ShortenDigest(maniMeta.Digest.String()))
	// Put Manifest in Minio
	e = mag.Minio.PutManifest(ctx, b, upt)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to get put manifest in minio")
	}
	rawMani, e := minio.RawManifestFromBlob(b)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to RawManifestFromBlob")
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
		return utils.ErrorHandler(l, span, e, "failed to GetBlobByUpstreamTriple")
	}
	l.Warnf("Config Digest: %s", cfgBlob.Digest.String())
	e = mag.Minio.PutConfig(ctx, cfgByte, cfgBlob.Digest.String())
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while Minio.PutConfig")
	}
	maniData, e := mag.Minio.NewManifestDataFromBytes(ctx, b)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while Minio.NewManifestFromBytes")
	}
	maniData.Type = "PULLED_BY"
	// Insert Config
	cfgID, e := mag.Database.Config_Insert(ctx, maniData.Config)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to insert config")
	}
	utils.Debug(l, span, "Config ID: %d", cfgID)
	utils.Debug(l, span, "Platform: %s/%s", maniData.Platform.OS, maniData.Platform.Arch)
	e = mag.Database.Platform_GetInsert(ctx, &maniData.Platform)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to insert platform")
	}
	e = mag.NeoLinkManifestToPlatform(ctx, maniID, maniData.Platform.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkManifestToPlatform)
	}
	// Create Collection
	col, _, e := mag.Database.Collection_GetCreate_ByRepoTag(ctx, req.Repo, req.Tag)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to get collection")
	}
	e = mag.Database.LinkCollectionToManifest(ctx, col.ID, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkCollectionToPlatform)
	}
	e = mag.Database.LinkCollectionTagToManifest(ctx, col.ColTag.ID, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkCollectionTagToPlatform)
	}
	// Insert Layer
	for idx, layer := range maniData.Layers {
		lID, e := mag.Database.Layer_PlainInsert(ctx, layer)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "Failed to insert layer")
		}
		utils.Debug(l, span, "Layer [%d] ID: %d", idx, lID)
		// TODO: Do we need to pull them here?
	}
	maniDataID, e := mag.NeoInsertManifestData(ctx, maniID, maniData, "PULLED_BY")
	if e != nil {
		return utils.ErrorHandler(l, span, e, failInsManiData)
	}
	l.Warnf("ManifestData ID: %d", maniDataID)

	utils.Debug(l, span, "Insert Config %d", cfgID)
	e = mag.Database.LinkManifestDataToProfile(ctx, maniDataID, prf.ID, "PULLED_BY")
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkProfileToManifestData)
	}
	// Unwind Profile
	e = mag.Database.Profile_UnwindRules(ctx, prf.ID, maniDataID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailUnwind)
	}
	// Create OCI Manifest
	mani, _, e := mag.Database.Manifest_GetByID_WithProfile(ctx, maniID, prf.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, failGetMani)
	}

	ociDataID, e := mag.CreateOCIv1_ForProfile(ctx, mani, maniData, prf)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailCreateOCIv1Manifest)
	}
	e = mag.Database.Profile_UnwindRules(ctx, prf.ID, ociDataID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailUnwind)
	}

	// Compute ManifestList?
	e = mag.ComputeManifestlists(ctx)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to ComputeManifestlists")
	}
	return
}

func (mag Magazine) upstreamBlobInsertGeneric(ctx context.Context, req types.Request, col types.Collection, ups types.Upstream, maniData types.ManifestData) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
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
	maniID, e := mag.Database.ManifestGetInsert(ctx, ups, as...)
	if e != nil {
		return utils.ErrorHandler(l, span, e, failInsMani)
	}
	// 2. Link Manifest to Platform
	e = mag.NeoLinkManifestToPlatform(ctx, maniID, maniData.Platform.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkManifestToPlatform)
	}
	// 3. Link Collection to Manifest
	e = mag.Database.LinkCollectionToManifest(ctx, col.ID, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkCollectionToPlatform)
	}
	// 4. Link CollectionTag to Manifest
	e = mag.Database.LinkCollectionTagToManifest(ctx, col.ColTag.ID, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkCollectionTagToPlatform)
	}
	// 5. Insert Manifest Data
	maniDataID, e := mag.NeoInsertManifestData(ctx, maniID, maniData, maniData.Type)
	if e != nil {
		return utils.ErrorHandler(l, span, e, failInsManiData)
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
		return utils.ErrorHandler(l, span, e, FailLinkProfileToManifestData)
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
		return utils.ErrorHandler(l, span, e, FailCreateOCIv1Manifest)
	}

	e = mag.ComputeManifestlists_ForManiProfile(ctx, ociManiID, prf.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "Failed to compute ManifestList manifest by ID")
	}
	return
}

func (mag Magazine) upstreamBlobInsertProfile(ctx context.Context, req types.Request, col types.Collection, ups types.Upstream, maniData types.ManifestData) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
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
	maniID, e := mag.Database.ManifestGetInsert(ctx, ups)
	if e != nil {
		return utils.ErrorHandler(l, span, e, failInsMani)
	}
	// 2. Link Manifest to Platform
	e = mag.NeoLinkManifestToPlatform(ctx, maniID, maniData.Platform.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkManifestToPlatform)
	}
	// 3. Link Collection to Manifest
	e = mag.Database.LinkCollectionToManifest(ctx, col.ID, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkCollectionToPlatform)
	}
	// 4. Link CollectionTag to Manifest
	e = mag.Database.LinkCollectionTagToManifest(ctx, col.ColTag.ID, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkCollectionTagToPlatform)
	}
	// 5. Insert Manifest Data
	maniDataID, e := mag.NeoInsertManifestData(ctx, maniID, maniData, "PULLED_BY")
	if e != nil {
		return utils.ErrorHandler(l, span, e, failInsManiData)
	}
	e = mag.Database.LinkManifestDataToProfile(ctx, maniDataID, prf.ID, "PULLED_BY")
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkProfileToManifestData)
	}
	// TODO: Unwind Profile
	/*
		e = mag.Database.Profile_UnwindRules(ctx, prf.ID, maniDataID)
		if e != nil {
			err = errors.Wrap(e, FailUnwind)
			l.Error(err)
			span.RecordError(err)
			return
		}
	*/
	// Create OCI Manifest
	mani, _, e := mag.Database.Manifest_GetByID_WithProfile(ctx, maniID, prf.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, failGetMani)
	}

	ociDataID, e := mag.CreateOCIv1_ForProfile(ctx, mani, maniData, prf)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailCreateOCIv1Manifest)
	}
	e = mag.Database.Profile_UnwindRules(ctx, prf.ID, ociDataID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailUnwind)
	}

	// Compute ManifestList?
	e = mag.ComputeManifestlists(ctx)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to ComputeManifestlists")
	}
	return
}

func (mag Magazine) upstreamInsertGeneric(ctx context.Context, req types.Request, dig digest.Digest) (col types.Collection, ups types.Upstream, maniData types.ManifestData, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "upstreamInsertGeneric",
		"reg":     req.Host,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
		"digest":  dig.String(),
	})
	defer span.End()
	// Get Collection
	col, _, e := mag.Database.Collection_GetCreate_ByRepoTag(ctx, req.Repo, req.Tag)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get/create collection")
		return
	}
	ups = types.Upstream{
		Reg:     req.Host,
		Repo:    req.Repo,
		Tag:     req.Tag,
		Digest:  dig,
		Profile: req.Profile,
	}
	// 1. Insert Upstream/Mani into Graph
	ups.ID, e = mag.NeoInsertUpstream(ctx, ups)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while inserting upstream")
		return
	}
	maniID, e := mag.Database.ManifestGetInsert(ctx, ups)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get/insert manifest")
		return
	}
	e = mag.Database.LinkCollectionToManifest(ctx, col.ID, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to link collection to manifest")
		return
	}
	e = mag.Database.LinkCollectionTagToManifest(ctx, col.ColTag.ID, maniID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to link collectionTag to manifest")
		return
	}
	// 2 . Get the Blob
	maniBytes, maniMeta, e := upstream.GetManifest_ByUpstreamByDigest(ctx, ups)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while getting manifest")
		return
	}
	utils.Debug(l, span, "Found manifest upstream: %s", utils.ShortenDigest(maniMeta.Digest.String()))
	// Put Manifest in Minio
	e = mag.Minio.PutManifest_ByUpstream(ctx, maniBytes, ups)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to get put manifest in minio")
		return
	}
	rawMani, e := minio.RawManifestFromBlob(maniBytes)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to RawManifestFromBlob")
		return
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
		return
	case "application/vnd.docker.distribution.manifest.v2+json":
		configUpt.Digest = rawMani.ConfigInfo().Digest
	default:
		l.Fatalf("Unknown media type: %s", maniMeta.MediaType)
	}
	cfgByte, cfgBlob, e := upstream.GetBlobByUpstreamTriple(ctx, configUpt)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get config via GetBlobByUpstreamTriple")
		return
	}
	utils.Debug(l, span, "Found config upstream: %s", utils.ShortenDigest(cfgBlob.Digest.String()))
	e = mag.Minio.PutConfig(ctx, cfgByte, cfgBlob.Digest.String())
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while putting config in Minio")
		return
	}
	maniData, e = mag.Minio.NewManifestDataFromBytes(ctx, maniBytes)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while Minio.NewManifestData_FromBytes")
		return

	}
	maniData.Type = "PULLED_BY"
	// Insert Raw Config
	cfgID, e := mag.Database.Config_Insert(ctx, maniData.Config)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to insert config")
		return
	}
	maniData.Config.ID = cfgID
	utils.Debug(l, span, "Config ID: %d", cfgID)
	utils.Debug(l, span, "Platform: %s/%s", maniData.Platform.OS, maniData.Platform.Arch)
	e = mag.Database.Platform_GetInsert(ctx, &maniData.Platform)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to insert platform")
		return
	}
	e = mag.NeoLinkManifestToPlatform(ctx, maniID, maniData.Platform.ID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to link manifest to platform")
		return
	}
	for idx, layer := range maniData.Layers {
		lID, e := mag.Database.Layer_PlainInsert(ctx, layer)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to insert layer #%d w/ digest '%s'", idx, utils.ShortenDigest(layer.Digest))
			return
		}
		maniData.Layers[idx].ID = lID
	}
	// We need the ManifestDataID
	maniData.ID, e = mag.Database.ManifestData_Insert(ctx, maniData)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to insert manifest data")
		return
	}
	// Layer annotations already present?
	os, osFound, e := mag.Database.ManifestGetOS(ctx, maniData.ID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to get annotations")
		return
	}
	e = mag.Database.LinkManifestToManifestData(ctx, maniID, maniData.ID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to link manifest to manifest data")
		return
	}
	// Iterate Layer
	for idx, layer := range maniData.Layers {
		if idx == 0 {
			utils.Debug(l, span, "Pulling layer #%d w/ digest '%s'", idx, utils.ShortenDigest(layer.Digest))
			up := upstream.UpstreamTriple{
				MediaType: layer.MediaType,
				Reg:       req.Host,
				Repo:      req.Repo,
				Tag:       req.Tag,
				Digest:    digest.Digest(layer.Digest),
			}
			md, blobFound, e := mag.BlobExists(ctx, up)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to check if blob exists")
				return
			}
			if !blobFound {
				err = utils.ErrorHandler(l, span, e, "blob not found")
				return
			}
			lyrBytes, e := mag.Minio.LayerGetBytes(ctx, layer.Digest)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to get layer #%d w/ digest '%s'", idx, utils.ShortenDigest(layer.Digest))
				return
			}
			switch md {
			case "application/vnd.docker.image.rootfs.diff.tar.gzip":
				utils.Warn(l, span, "Layer #%d (lID: %d) is tar.gzip", idx, layer.ID)
				layer.Data = lyrBytes
				os, e = layer.EvalOS(ctx)
				if e != nil {
					err = utils.ErrorHandler(l, span, e, "failed to eval layer")
					return
				}
				osFound = true
				e := mag.Database.ManifestUpsertOS(ctx, maniID, os)
				if e != nil {
					err = utils.ErrorHandler(l, span, e, "failed to upsert os")
					return
				}
			default:
				err = utils.NewError(l, span, "unknown media-type: %s", md)
			}
			/*os, e := mag.BaseLayerEvalOS(ctx, layer)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to eval layer #%d w/ digest '%s'", idx, utils.ShortenDigest(layer.Digest))
				return
			}
			_ = os
			*/
			// TODO: Add OS annotation to layer

		}
	}
	if !osFound {
		err = utils.NewError(l, span, "OS not found and not parsed")
		return
	}
	utils.Debug(l, span, "OS: %s", os.ID)
	return
}

func (mag Magazine) upstreamBlobInsert(ctx context.Context, req types.Request, dig digest.Digest) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "upstreamBlobInsert",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
		"digest":  dig.String(),
	})
	defer span.End()
	col, up, maniData, e := mag.upstreamInsertGeneric(ctx, req, dig)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while inserting generic upstream")
	}
	/// Generic
	e = mag.upstreamBlobInsertGeneric(ctx, req, col, up, maniData)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while inserting generic")
	}
	// Profile
	if req.Profile == "generic" {
		utils.Debug(l, span, "Skipping profile 'generic'")
		return
	}
	up.Profile = req.Profile
	e = mag.upstreamBlobInsertProfile(ctx, req, col, up, maniData)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while inserting for profile")
	}
	return
}

func (mag Magazine) UpstreamBlobInsertManifest(ctx context.Context, req types.Request, blob types.BlobMeta) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "UpstreamBlob_InsertManifest",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
	})
	defer span.End()
	switch blob.MediaType {
	case "application/vnd.docker.distribution.manifest.v2+json":
		e := mag.upstreamBlobInsert(ctx, req, blob.Digest)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while inserting upstream")
			return err
		}

		return err
	default:
		err = utils.NewError(l, span, "unknown mediaType: %s", blob.MediaType)
	}
	return
}

func (mag Magazine) UpstreamBlobInsertIndex(ctx context.Context, req types.Request, blob types.BlobMeta) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "UpstreamBlobInsertIndex",
	})
	defer span.End()
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
		utils.Debug(l, span, "Found Blob %v", blob)
		var ml manifestlist.ManifestList
		err = json.Unmarshal(b, &ml)
		if err != nil {
			l.Error(err.Error())
			span.RecordError(err)
			return err
		}
		l.Warnf("Found %d manifests", len(ml.Manifests))
		for _, m := range ml.Manifests {
			utils.Debug(l, span, "Found Manifest with Platform.Arch %s", m.Platform.Architecture)
			switch m.Platform.Architecture {
			case "amd64", "arm64":
				e := mag.upstreamBlobInsertManifestDesc(ctx, req, m)
				if e != nil {
					err = utils.ErrorHandler(l, span, e, "while upstreamBlobInsertManifestDesc")
					return err
				}
			default:
				utils.Debug(l, span, "Skipping Platform.Arch %s", m.Platform.Architecture)
			}

		}
	}
	return
}
