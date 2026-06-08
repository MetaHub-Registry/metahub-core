package magazine

import (
	"context"
	"fmt"
	"strings"

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
	FailLinkManifestToPlatform      = "Failed to link manifestData to platform"
	FailLinkCollectionToPlatform    = "failed to link collection to manifest"
	FailLinkCollectionTagToPlatform = "failed to link collection tag to manifest"
	FailLinkProfileToManifest       = "failed to link profile and manifestdata"
	FailUnwind                      = "failed to unwind profile"
	FailCreateOCIv1Manifest         = "failed to create OCIv1 manifest"
	FailCacheUpstream               = "failed to cache upstream"
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

	res, _, e := mag.Database.UpstreamGetInsertByRequest(ctx, req)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to UpstreamGetInsertByRequest")
		return

	}
	utils.Debug(l, span, "insert sucessful with ID '%d'", res.ID)
	upID = res.ID
	return
}

// DiscoverUpstream tries to find a manifest for repo:tag in upstream registries.
// When targetReg is provided, only that specific registry is tried (it must be
// present in MagConfig.Registries). When targetReg is empty, all configured
// registries are iterated in order (existing behavior).
func (mag Magazine) DiscoverUpstream(ctx context.Context, repo, tag string, targetReg ...string) (blob types.BlobMeta, registry string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "DiscoverUpstream",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()

	// Build the list of registries to try.
	// upstreamRepo is the repo path as the upstream registry knows it — it may
	// differ from repo when the caller uses a prefix-preserving local alias
	// (D5/D6). For example, repo="registry:5050/os/alpine" → upstreamRepo="os/alpine".
	upstreamRepo := repo
	var regList []string
	if len(targetReg) > 0 && targetReg[0] != "" {
		target := targetReg[0]
		// Validate that the target registry is configured. The target may be
		// either a Name (slug) or a URL (endpoint) depending on how the caller
		// resolved it. Match on both.
		var matchedURL string
		for _, reg := range mag.Config.Registries {
			if reg.EffectiveName() == target || reg.EffectiveURL() == target {
				matchedURL = reg.EffectiveURL()
				break
			}
		}
		if matchedURL == "" {
			err = utils.NewError(l, span, "target registry '%s' is not configured", target)
			return
		}
		regList = []string{matchedURL}
		// Strip the registry prefix from the repo path so that the upstream
		// reference is well-formed (e.g. "os/alpine" not "registry:5050/os/alpine").
		if _, stripped, ok := mag.ResolveUpstreamRegistry(repo); ok {
			upstreamRepo = stripped
			utils.Debug(l, span, "stripped registry prefix for upstream query: %s -> %s", repo, upstreamRepo)
		}
		utils.Info(l, span, "Using targeted registry: %s (upstreamRepo: %s)", target, upstreamRepo)
	} else {
		// No target — use all configured registries (fallback to Args for backward compat)
		if len(mag.Config.Registries) > 0 {
			for _, reg := range mag.Config.Registries {
				regList = append(regList, reg.EffectiveURL())
			}
		} else {
			reg, ok := mag.GetConfig("registries")
			if !ok {
				err = utils.NewError(l, span, "Failed to get 'registries' from config, please check your config")
				return
			}
			regList = strings.Split(reg, ",")
		}
		utils.Info(l, span, "Using registries: %v", regList)
	}

	for _, reg := range regList {
		regCtx, regSpan := tracer.Start(ctx, fmt.Sprintf("DiscoverUpstream_%s", reg))
		defer regSpan.End()
		b, regFound, e := mag.HeadManifestByRegRepoTag(regCtx, reg, upstreamRepo, tag)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to HeadManifestByRegRepoTag")
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

/*
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
		b, maniMeta, e := mag.GetManifestByUpstreamTripleByDigest(ctx, upt)
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
		cfgByte, cfgBlob, e := mag.GetBlobByUpstreamTriple(ctx, upt)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to GetBlobByUpstreamTriple")
		}
		l.Warnf("Config Digest: %s", cfgBlob.Digest.String())
		e = mag.Minio.PutConfig(ctx, cfgByte, cfgBlob.Digest.String())
		if e != nil {
			return utils.ErrorHandler(l, span, e, "while Minio.PutConfig")
		}
		maniData, e := mag.Minio.NewManifestFromBytes(ctx, b)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "while Minio.NewManifestFromBytes")
		}
		maniData.Type = "PULLED_BY"
		// Insert Config
		cfgID, e := mag.Database.ConfigInsert(ctx, maniData.Config)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "Failed to insert config")
		}
		utils.Debug(l, span, "Config ID: %d", cfgID)
		utils.Debug(l, span, "Platform: %s/%s", maniData.Platform.OS, maniData.Platform.Arch)
		e = mag.Database.PlatformGetInsert(ctx, &maniData.Platform)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "Failed to insert platform")
		}
		e = mag.NeoLinkManifestToPlatform(ctx, maniID, maniData.Platform.ID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, FailLinkManifestToPlatform)
		}
		// Create Repo
		col, _, e := mag.Database.CollectionGetCreateByRepoTag(ctx, req.Repo, req.Tag)
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
			lID, e := mag.Database.LayerPlainInsert(ctx, layer)
			if e != nil {
				return utils.ErrorHandler(l, span, e, "Failed to insert layer")
			}
			utils.Debug(l, span, "Layer [%d] ID: %d", idx, lID)
			// TODO: Do we need to pull them here?
		}
		maniDataID, e := mag.NeoInsertManifest(ctx, maniID, maniData, "PULLED_BY")
		if e != nil {
			return utils.ErrorHandler(l, span, e, failInsManiData)
		}
		l.Warnf("Manifest ID: %d", maniDataID)

		utils.Debug(l, span, "Insert Config %d", cfgID)
		e = mag.Database.LinkManifestToProfile(ctx, maniDataID, prf.ID, "PULLED_BY")
		if e != nil {
			return utils.ErrorHandler(l, span, e, FailLinkProfileToManifest)
		}
		// Unwind Profile
		e = mag.Database.ProfileUnwindRules(ctx, prf.ID, maniDataID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, FailUnwind)
		}
		// Create OCI Manifest
		mani, _, e := mag.Database.ManifestWrapperGetByIDWithProfile(ctx, maniID, prf.ID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, failGetMani)
		}

		ociDataID, e := mag.CreateOCIv1ForProfile(ctx, mani, maniData, prf)
		if e != nil {
			return utils.ErrorHandler(l, span, e, FailCreateOCIv1Manifest)
		}
		e = mag.Database.ProfileUnwindRules(ctx, prf.ID, ociDataID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, FailUnwind)
		}

		// Compute Index?
		e = mag.ComputeManifestlists(ctx)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to ComputeManifestlists")
		}
		return
	}
*/

func (mag Magazine) upstreamBlobInsertGeneric(ctx context.Context, req types.Request, col types.Repo, ups types.Upstream, maniData types.Manifest) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "upstreamBlob_InsertGeneric",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
		"digest":  ups.Digest.String(),
	})
	defer span.End()
	// Phase C3: wrapper :ManifestWrapper creation + Collection/Tag linking removed.
	// The Manifest was already inserted and linked to Slot in
	// upstreamInsertGeneric. Here we just link to platform and profile,
	// then trigger compute.

	// Link Manifest to Platform
	e := mag.NeoLinkManifestToPlatform(ctx, maniData.ID, maniData.Platform.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkManifestToPlatform)
	}

	// Get 'generic' profile
	prf, prfFound, e := mag.Database.ProfileGetByLabel(ctx, prfGenericLabel)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to get profile")
	}
	if !prfFound {
		return utils.NewError(l, span, "profile '%s' not found", prfGenericLabel)
	}
	utils.Debug(l, span, "Found profile '%s'", prfGenericLabel)

	// Link Manifest to Profile
	e = mag.Database.LinkManifestToProfile(ctx, maniData.ID, prf.ID, "PULLED_BY")
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkProfileToManifest)
	}

	// Phase 4 (deprecate-manifest-wrapper): pass Manifest annotations directly
	// to OCICreateBaseManifest (no wrapper read needed — OS annotations are on
	// maniData.Annotations). The synthetic wrapper is keyed on the content-leaf
	// id so any leaf-keyed reshape query resolves against the real content node.
	syntheticMani := types.ManifestWrapper{
		ID:          maniData.ID,
		Upstream:    ups,
		Manifests:   []types.Manifest{maniData},
		Annotations: maniData.Annotations,
	}
	_, e = mag.OCICreateBaseManifest(ctx, syntheticMani, maniData, 0)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailCreateOCIv1Manifest)
	}

	// Phase C3: trigger compute via the Slot-based path (same as push path)
	mag.wireSlotOnPush(ctx, l, span, req, maniData.ID)

	// Compute the per-profile Index for this (repo, tag) so the image shows
	// up in frontend listings and is served via the Index-based pull path.
	// Pass the requesting profile so manipulation-only profiles (like glow)
	// get their manipulated Index computed at this point.
	var reqPrf *types.Profile
	if req.Profile != "" && req.Profile != prfGenericLabel {
		if prf, prfFound, prfErr := mag.Database.ProfileGetByLabel(ctx, req.Profile); prfErr == nil && prfFound {
			reqPrf = &prf
		}
	}
	if e := mag.ComputeManifestlistsForRepoTag(ctx, req.Repo, req.Tag, reqPrf); e != nil {
		utils.Warn(l, span, "ComputeManifestlistsForRepoTag failed (non-fatal, pullthrough still stored): %v", e)
	}
	return
}

func (mag Magazine) upstreamBlobInsertProfile(ctx context.Context, req types.Request, col types.Repo, ups types.Upstream, maniData types.Manifest) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "upstreamBlobInsertProfile",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
		"digest":  ups.Digest.String(),
	})
	defer span.End()
	// Phase C3: wrapper operations removed. Profile lookup and linking
	// still needed for Slot triple and profile unwind.
	// Try label first, then tag as fallback (docker login sends tag only).
	prf, prfFound, e := mag.Database.ProfileGetByLabel(ctx, req.Profile)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to get profile")
	}
	if !prfFound {
		prfs, prfErr := mag.Database.ProfileGetAll(ctx)
		if prfErr == nil {
			for _, candidate := range prfs {
				if candidate.Tag == req.Profile {
					prf = candidate
					prfFound = true
					utils.Info(l, span, "resolved profile tag '%s' to label '%s'", req.Profile, prf.Label)
					break
				}
			}
		}
	}
	if !prfFound {
		return utils.NewError(l, span, "profile '%s' not found", req.Profile)
	}

	// Link Manifest to Profile
	e = mag.Database.LinkManifestToProfile(ctx, maniData.ID, prf.ID, "PULLED_BY")
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkProfileToManifest)
	}
	// Link Manifest to Platform
	e = mag.NeoLinkManifestToPlatform(ctx, maniData.ID, maniData.Platform.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailLinkManifestToPlatform)
	}

	// Create OCI v1 manifest from the upstream manifest. Phase 4
	// (deprecate-manifest-wrapper): construct a synthetic ManifestWrapper shim
	// keyed on the content-leaf id so the leaf-keyed reshape queries resolve
	// against the real content node. The wrapper type itself is retained until
	// Phase 5; no graph :ManifestWrapper node is read or written.
	syntheticMani := types.ManifestWrapper{
		ID:          maniData.ID,
		Upstream:    ups,
		Manifests:   []types.Manifest{maniData},
		Annotations: maniData.Annotations,
	}
	ociDataID, e := mag.CreateOCIv1ForProfile(ctx, syntheticMani, maniData, prf)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailCreateOCIv1Manifest)
	}

	// Profile unwind rules
	e = mag.Database.ProfileUnwindRules(ctx, prf.ID, ociDataID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailUnwind)
	}

	// Phase C3: trigger compute via the Slot-based path
	mag.wireSlotOnPush(ctx, l, span, req, maniData.ID)
	return
}

func (mag Magazine) upstreamInsertGeneric(ctx context.Context, req types.Request, dig digest.Digest) (col types.Repo, ups types.Upstream, maniData types.Manifest, err error) {
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
	// Phase C3: keep CollectionGetCreateByRepoTag for backward compat
	// (the return value is used by callers). The wrapper :ManifestWrapper node
	// is no longer created — Slot-based attribution replaces it.
	col, _, e := mag.Database.CollectionGetCreateByRepoTag(ctx, req.Repo, req.Tag)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to get/create collection")
		return
	}
	// For prefix-preserving pullthrough (D5/D6), req.Repo is the local alias
	// (e.g. "registry:5050/os/alpine") while the Upstream node must store the
	// repo path as the upstream registry knows it (e.g. "os/alpine"). This
	// ensures GetBlobUrl() and manifest-fetch-by-digest construct valid refs.
	upstreamRepo := req.Repo
	if _, stripped, ok := mag.ResolveUpstreamRegistry(req.Repo); ok {
		upstreamRepo = stripped
		utils.Debug(l, span, "stripped registry prefix for upstream record: %s -> %s", req.Repo, upstreamRepo)
	}
	ups = types.Upstream{
		Reg:     req.Host,
		Repo:    upstreamRepo,
		Tag:     req.Tag,
		Digest:  dig,
		Profile: req.Profile,
	}
	// 1. Insert Upstream into Graph
	ups.ID, e = mag.NeoInsertUpstream(ctx, ups)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while inserting upstream")
		return
	}
	// Phase C3: wrapper :ManifestWrapper node creation removed. OS annotations
	// are now stored on Manifest in-memory and persisted via
	// ManifestAddAnnotations. OCICreateBaseManifest reads them
	// from maniData.Annotations passed by the caller.
	// 2. Get the manifest — check MinIO first, fetch from upstream only if missing.
	var maniBytes []byte
	var maniMeta types.BlobMeta
	cachedMani, cacheErr := mag.Minio.ManifestGetObj(ctx, ups.Digest.String())
	if cacheErr == nil && len(cachedMani.Data) > 0 {
		utils.Info(l, span, "manifest %s already in MinIO, skipping upstream fetch", utils.ShortenDigest(ups.Digest.String()))
		maniBytes = cachedMani.Data
		maniMeta = types.BlobMeta{Digest: digest.Digest(cachedMani.Digest), Size: int64(len(cachedMani.Data)), MediaType: cachedMani.MediaType}
	} else {
		maniBytes, maniMeta, e = mag.GetManifestByUpstreamByDigest(ctx, ups)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while getting manifest")
			return
		}
		utils.Debug(l, span, "Found manifest upstream: %s", utils.ShortenDigest(maniMeta.Digest.String()))
		// Put Manifest in Minio
		e = mag.Minio.PutManifestByUpstream(ctx, maniBytes, ups)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "Failed to get put manifest in minio")
			return
		}
	}
	rawMani, e := minio.RawManifestFromBlob(maniBytes)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to RawManifestFromBlob")
		return
	}
	// Fetch Upstream Config — use upstreamRepo (stripped) to ensure the
	// reference targets the real upstream path, not the prefix-preserved alias.
	configUpt := upstream.UpstreamTriple{
		Reg:     req.Host,
		Repo:    upstreamRepo,
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
	// Check if config blob already exists in MinIO before fetching from upstream
	_, cfgStatErr := mag.Minio.BlobStatObj(ctx, configUpt.Digest.String())
	if cfgStatErr == nil {
		utils.Info(l, span, "config blob %s already in MinIO, skipping upstream fetch", utils.ShortenDigest(configUpt.Digest.String()))
	} else {
		cfgByte, cfgBlob, e := mag.GetBlobByUpstreamTriple(ctx, configUpt)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get config via GetBlobByUpstreamTriple")
			return
		}
		utils.Debug(l, span, "Found config upstream: %s", utils.ShortenDigest(cfgBlob.Digest.String()))
		e = mag.Minio.PutBlob(ctx, cfgByte, cfgBlob.Digest.String())
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while putting config in Minio")
			return
		}
	}
	maniData, e = mag.Minio.NewManifestFromBytes(ctx, maniBytes)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while Minio.NewManifest_FromBytes")
		return

	}
	maniData.Type = "PULLED_BY"
	// Insert Raw Config
	cfgID, e := mag.Database.ConfigInsert(ctx, maniData.Config)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to insert config")
		return
	}
	maniData.Config.ID = cfgID
	utils.Debug(l, span, "Config ID: %d", cfgID)
	utils.Debug(l, span, "Platform: %s/%s", maniData.Platform.OS, maniData.Platform.Arch)
	e = mag.Database.PlatformGetInsert(ctx, &maniData.Platform)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to insert platform")
		return
	}
	e = mag.NeoLinkManifestToPlatform(ctx, maniData.ID, maniData.Platform.ID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to link manifest to platform")
		return
	}
	for idx, layer := range maniData.Layers {
		lID, e := mag.Database.LayerPlainInsert(ctx, layer)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to insert layer #%d w/ digest '%s'", idx, utils.ShortenDigest(layer.Digest))
			return
		}
		maniData.Layers[idx].ID = lID
	}
	// We need the ManifestID
	maniData.ID, e = mag.Database.ManifestInsert(ctx, &maniData)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "Failed to insert manifest data")
		return
	}
	if e := mag.Database.ManifestLinkToUpstream(ctx, maniData.ID, ups.ID); e != nil {
		err = utils.ErrorHandler(l, span, e, "while linking manifestData to upstream")
		return
	}
	// Phase C3: wire Slot-based attribution for upstream-pull path
	mag.wireSlotOnPush(ctx, l, span, req, maniData.ID)
	// The wrapper annotation read/write helpers are removed. OS
	// annotations are discovered from layer evaluation below and stored
	// on the content-leaf via ManifestAddAnnotations.
	var os types.OS
	osFound := false
	// Iterate Layer
	for idx, layer := range maniData.Layers {
		if idx == 0 {
			utils.Warn(l, span, "Pulling layer #%d w/ digest '%s' (%s)", idx, utils.ShortenDigest(layer.Digest), layer.MediaType)
			up := upstream.UpstreamTriple{
				MediaType: layer.MediaType,
				Reg:       req.Host,
				Repo:      upstreamRepo,
				Tag:       req.Tag,
				Digest:    digest.Digest(layer.Digest),
			}
			blobFound, e := mag.BlobExists(ctx, up)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to check if blob exists")
				return
			}
			if !blobFound {
				err = utils.ErrorHandler(l, span, e, "blob not found")
				return
			}
			// Fetch Layer/MD relationship
			if lyr, found, e := mag.Database.LayerGetByLayerAndManifestID(ctx, layer.ID, maniData.ID); e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to get layer by layer and manifest data id")
				return
			} else if !found {
				err = utils.NewError(l, span, "layer not found")
				return
			} else {
				layer.MediaType = lyr.MediaType
			}
			utils.Debug(l, span, "Layer #%d (lID: %d) is %s", idx, layer.ID, layer.MediaType)
			lyrBytes, e := mag.Minio.BlobGetBytes(ctx, layer.Digest)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "failed to get layer #%d w/ digest '%s'", idx, utils.ShortenDigest(layer.Digest))
				return
			}
			switch layer.MediaType {
			case "application/vnd.docker.image.rootfs.diff.tar.gzip":
				utils.Warn(l, span, "Layer #%d (lID: %d) is tar.gzip", idx, layer.ID)
				layer.Data = lyrBytes
				os, e = layer.EvalOS(ctx)
				if e != nil {
					err = utils.ErrorHandler(l, span, e, "failed to eval layer")
					return
				}
				osFound = true
				// Phase C3: store OS annotations on the Manifest in-memory
				// instead of writing to the wrapper. These annotations are
				// passed through to OCICreateBaseManifest which injects them
				// into the OCI config.
				const annoOSId = "org.qnib.system.os.id"
				const annoOSVersion = "org.qnib.system.os.version"
				if maniData.Annotations.Data == nil {
					maniData.Annotations = types.NewAnnotations()
				}
				if e := maniData.Annotations.AddNewAnnotation(annoOSId, string(os.ID)); e != nil {
					utils.Debug(l, span, "failed to add OS ID annotation: %v", e)
				}
				if e := maniData.Annotations.AddNewAnnotation(annoOSVersion, os.GetVersion()); e != nil {
					utils.Debug(l, span, "failed to add OS Version annotation: %v", e)
				}
				// Also persist to the Manifest node in the graph
				osAnnotations := types.NewAnnotations()
				_ = osAnnotations.AddNewAnnotation(annoOSId, string(os.ID))
				_ = osAnnotations.AddNewAnnotation(annoOSVersion, os.GetVersion())
				if e := mag.Database.ManifestAddAnnotations(ctx, maniData.ID, &osAnnotations); e != nil {
					utils.Debug(l, span, "failed to persist OS annotations: %v", e)
				}
			default:
				err = utils.NewError(l, span, "unknown media-type: %s", layer.MediaType)
			}
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
		"func":    "UpstreamBlobInsertManifest",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
	})
	defer span.End()
	utils.Debug(l, span, "MediaType: '%s'", blob.MediaType)
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

/*
	func (mag Magazine) UpstreamBlobInsertIndex(ctx context.Context, req types.Request, blob types.BlobMeta) (err error) {
		l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
			"package": pkgNameUp,
			"func":    "UpstreamBlobInsertIndex",
		})
		defer span.End()
		if reg, ok := mag.Config.Args["TEST_REGISTRY"]; ok {
			utils.Info(l, span, "Overriding registry to %s", reg)
			req.Host = reg
		} else {
			utils.Info(l, span, "Using registry from request: %s", req.Host)
		}
		switch blob.MediaType {
		case "application/vnd.docker.distribution.manifest.list.v2+json":
			// TODO: Fetch Index and insert all manifest as upstream
			up := upstream.UpstreamTriple{
				Reg:       req.Host,
				Repo:      req.Repo,
				Tag:       req.Tag,
				Digest:    blob.Digest,
				MediaType: blob.MediaType,
			}
			b, blob, err := mag.GetManifestByUpstreamTripleByTag(ctx, up)
			if err != nil {
				l.Error(err.Error())
				span.RecordError(err)
				return err
			}
			utils.Debug(l, span, "Found Blob %v", blob)
			var ml manifestlist.Index
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
*/
func (mag Magazine) HeadUpstream(ctx context.Context, up *types.Upstream) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "HeadUpstream",
	})
	defer span.End()
	if blob, found, err := mag.HeadManifestByRegRepoTag(ctx, up.Reg, up.Repo, up.Tag); err != nil {
		return utils.ErrorHandler(l, span, err, FailHeadManifestByUpstreamTripleByTag)
	} else if !found {
		return utils.NewError(l, span, FailManifestByUpstreamNotFound)
	} else {
		up.Digest = blob.Digest
		up.MediaType = blob.MediaType
		up.Size = blob.Size
	}
	return nil
}

func (mag Magazine) CacheUpstream(ctx context.Context, up types.Upstream) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "CacheUpstream",
	})
	defer span.End()
	_ = l
	utils.Debug(l, span, "attempt to cacheManifestByUpstream %d", up.ID)
	maniData, err := mag.cacheManifestByUpstream(ctx, up)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "while caching manifest")
	}
	utils.Debug(l, span, "Manifest: %v", maniData.Label)
	/// Cache Layers
	if err := mag.CacheLayers(ctx, maniData); err != nil {
		return utils.ErrorHandler(l, span, err, "while caching layers")
	}
	// Ensure config, layers, and manifest exist in the graph.
	// cacheManifestByUpstream may return from MinIO cache without inserting
	// into Neo4j (e.g. after --empty-graph=keep-upstream wipes nodes but
	// MinIO still has the blobs). ManifestUpsertBackends is idempotent and
	// handles the get-or-create for config, layers, and the manifest node.
	if e := mag.ManifestUpsertBackends(ctx, &maniData); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to ManifestUpsertBackends")
	}
	if e := mag.Database.ManifestLinkToConfig(ctx, maniData.ID, maniData.Config); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to ManifestLinkToConfig")
	}
	return nil
}

// AllowsUnauthorizedPull checks whether the upstream registry allows anonymous
// blob pulls by looking up the registry in MagConfig.Registries. Matches on
// both EffectiveURL and EffectiveName (the Upstream node stores EffectiveURL
// as Reg, but legacy data may use the slug name).
// Unknown registries default to denied. The TEST_REGISTRY override is preserved
// for integration tests.
func (mag Magazine) AllowsUnauthorizedPull(up types.Upstream) bool {
	if _, ok := mag.Config.Args["TEST_REGISTRY"]; ok {
		return true
	}
	for _, reg := range mag.Config.Registries {
		if reg.EffectiveURL() == up.Reg || reg.EffectiveName() == up.Reg {
			return reg.AllowAnon
		}
	}
	return false
}
