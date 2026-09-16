package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

// discoverRegList builds the ordered upstream registry endpoints to try for a
// discovery request and the (possibly prefix-stripped) upstream repo path.
func (mag Magazine) discoverRegList(ctx context.Context, repo string, targetReg ...string) (regList []string, upstreamRepo string, err error) {
	_, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "discoverRegList",
		"repo":    repo,
	})
	defer span.End()
	upstreamRepo = repo
	if len(targetReg) > 0 && targetReg[0] != "" {
		return mag.targetRegList(repo, targetReg[0])
	}
	// No target — every opted-in registry, or the legacy flat-string config.
	if urls := mag.eligibleRegistryURLs(); len(urls) > 0 {
		return urls, upstreamRepo, nil
	}
	reg, ok := mag.GetConfig("registries")
	if !ok {
		return nil, upstreamRepo, fmt.Errorf("Failed to get 'registries' from config, please check your config")
	}
	return strings.Split(reg, ","), upstreamRepo, nil
}

// targetRegList validates an explicitly targeted registry against the
// configured, pull-through-eligible set and strips its slug prefix from repo.
func (mag Magazine) targetRegList(repo, target string) (regList []string, upstreamRepo string, err error) {
	upstreamRepo = repo
	for _, reg := range mag.Config.Registries {
		if !reg.PullThroughEligible() {
			continue
		}
		if reg.EffectiveName() != target && reg.EffectiveURL() != target {
			continue
		}
		regList = []string{reg.EffectiveURL()}
		// Strip the registry prefix so the upstream reference is well-formed
		// (e.g. "os/alpine", not "registry:5050/os/alpine").
		if _, stripped, ok := mag.ResolveUpstreamRegistry(repo); ok {
			upstreamRepo = stripped
		}
		return regList, upstreamRepo, nil
	}
	return nil, upstreamRepo, fmt.Errorf("target registry '%s' is not configured", target)
}

// eligibleRegistryURLs returns the endpoints of every opted-in registry.
func (mag Magazine) eligibleRegistryURLs() []string {
	var out []string
	for _, reg := range mag.Config.Registries {
		if !reg.PullThroughEligible() {
			continue
		}
		out = append(out, reg.EffectiveURL())
	}
	return out
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

	// Build the ordered registry list to try and the (possibly prefix-stripped)
	// upstream repo. Extracted to keep this method's cognitive complexity low.
	regList, upstreamRepo, err := mag.discoverRegList(ctx, repo, targetReg...)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to build discovery registry list")
		return
	}
	utils.Info(l, span, "Using registries: %v (upstreamRepo: %s)", regList, upstreamRepo)

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

// getOrFetchManifest returns manifest bytes + metadata from MinIO if already
// cached, otherwise fetches from the upstream registry and stores in MinIO.
func (mag Magazine) getOrFetchManifest(ctx context.Context, ups types.Upstream) ([]byte, types.BlobMeta, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp, "func": "getOrFetchManifest",
	})
	defer span.End()
	cached, cacheErr := mag.Minio.ManifestGetObj(ctx, ups.Digest.String())
	if cacheErr == nil && len(cached.Data) > 0 {
		utils.Info(l, span, "manifest %s already in MinIO, skipping upstream fetch", utils.ShortenDigest(ups.Digest.String()))
		meta := types.BlobMeta{Digest: digest.Digest(cached.Digest), Size: int64(len(cached.Data)), MediaType: cached.MediaType}
		return cached.Data, meta, nil
	}
	maniBytes, maniMeta, e := mag.GetManifestByUpstreamByDigest(ctx, ups)
	if e != nil {
		return nil, types.BlobMeta{}, utils.ErrorHandler(l, span, e, "while getting manifest")
	}
	utils.Debug(l, span, "found manifest upstream: %s", utils.ShortenDigest(maniMeta.Digest.String()))
	if e := mag.Minio.PutManifestByUpstream(ctx, maniBytes, ups); e != nil {
		return nil, types.BlobMeta{}, utils.ErrorHandler(l, span, e, "failed to put manifest in minio")
	}
	return maniBytes, maniMeta, nil
}

// ensureConfigCached fetches the image config blob from the upstream registry
// and stores it in MinIO, skipping the fetch if the blob is already present.
func (mag Magazine) ensureConfigCached(ctx context.Context, configUpt upstream.UpstreamTriple) error {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp, "func": "ensureConfigCached",
	})
	defer span.End()
	if _, cfgStatErr := mag.Minio.BlobStatObj(ctx, configUpt.Digest.String()); cfgStatErr == nil {
		utils.Info(l, span, "config blob %s already in MinIO, skipping upstream fetch", utils.ShortenDigest(configUpt.Digest.String()))
		return nil
	}
	cfgByte, cfgBlob, e := mag.GetBlobByUpstreamTriple(ctx, configUpt)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to get config via GetBlobByUpstreamTriple")
	}
	utils.Debug(l, span, "found config upstream: %s", utils.ShortenDigest(cfgBlob.Digest.String()))
	if e := mag.Minio.PutBlob(ctx, cfgByte, cfgBlob.Digest.String()); e != nil {
		return utils.ErrorHandler(l, span, e, "while putting config in Minio")
	}
	return nil
}

// insertManifestGraph writes config, platform, layers, and the manifest node
// to the graph, wires ORIGINAL + Slot edges, and updates IDs in maniData.
func (mag Magazine) insertManifestGraph(ctx context.Context, req types.Request, maniData *types.Manifest, ups types.Upstream) error {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp, "func": "insertManifestGraph",
	})
	defer span.End()
	cfgID, e := mag.Database.ConfigInsert(ctx, maniData.Config)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to insert config")
	}
	maniData.Config.ID = cfgID
	utils.Debug(l, span, "Config ID: %d / Platform: %s/%s", cfgID, maniData.Platform.OS, maniData.Platform.Arch)
	if e := mag.Database.PlatformGetInsert(ctx, &maniData.Platform); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to insert platform")
	}
	if e := mag.NeoLinkManifestToPlatform(ctx, maniData.ID, maniData.Platform.ID); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to link manifest to platform")
	}
	for idx, layer := range maniData.Layers {
		lID, e := mag.Database.LayerPlainInsert(ctx, layer)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to insert layer #%d w/ digest '%s'", idx, utils.ShortenDigest(layer.Digest))
		}
		maniData.Layers[idx].ID = lID
	}
	maniData.ID, e = mag.Database.ManifestInsert(ctx, maniData)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to insert manifest data")
	}
	if e := mag.Database.ManifestLinkToUpstream(ctx, maniData.ID, ups.ID); e != nil {
		return utils.ErrorHandler(l, span, e, "while linking manifest to upstream")
	}
	mag.wireSlotOnPush(ctx, l, span, req, maniData.ID)
	return nil
}

// addOSAnnotations stores OS id/version annotations on maniData in-memory and
// persists them to the Manifest graph node. Errors are non-fatal (debug only).
func (mag Magazine) addOSAnnotations(ctx context.Context, maniData *types.Manifest, os types.OS) {
	const annoOSId = "org.qnib.system.os.id"
	const annoOSVersion = "org.qnib.system.os.version"
	if maniData.Annotations.Data == nil {
		maniData.Annotations = types.NewAnnotations()
	}
	_ = maniData.Annotations.AddNewAnnotation(annoOSId, string(os.ID))
	_ = maniData.Annotations.AddNewAnnotation(annoOSVersion, os.GetVersion())
	osAnnotations := types.NewAnnotations()
	_ = osAnnotations.AddNewAnnotation(annoOSId, string(os.ID))
	_ = osAnnotations.AddNewAnnotation(annoOSVersion, os.GetVersion())
	_ = mag.Database.ManifestAddAnnotations(ctx, maniData.ID, &osAnnotations)
}

// evalFirstLayerOS confirms the first layer blob exists, resolves its media
// type from the graph, evaluates the OS from the tar content, and stores OS
// annotations on the manifest.
func (mag Magazine) evalFirstLayerOS(ctx context.Context, req types.Request, upstreamRepo string, maniData *types.Manifest) (types.OS, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp, "func": "evalFirstLayerOS",
	})
	defer span.End()
	if len(maniData.Layers) == 0 {
		return types.OS{}, utils.NewError(l, span, "manifest has no layers")
	}
	layer := maniData.Layers[0]
	utils.Warn(l, span, "pulling layer #0 w/ digest '%s' (%s)", utils.ShortenDigest(layer.Digest), layer.MediaType)
	up := upstream.UpstreamTriple{
		MediaType: layer.MediaType,
		Reg:       req.Host,
		Repo:      upstreamRepo,
		Tag:       req.Tag,
		Digest:    digest.Digest(layer.Digest),
	}
	blobFound, e := mag.BlobExists(ctx, up)
	if e != nil {
		return types.OS{}, utils.ErrorHandler(l, span, e, "failed to check if blob exists")
	}
	if !blobFound {
		return types.OS{}, utils.NewError(l, span, "blob not found")
	}
	lyr, found, e := mag.Database.LayerGetByLayerAndManifestID(ctx, layer.ID, maniData.ID)
	if e != nil {
		return types.OS{}, utils.ErrorHandler(l, span, e, "failed to get layer by layer and manifest data id")
	} else if !found {
		return types.OS{}, utils.NewError(l, span, "layer not found")
	}
	layer.MediaType = lyr.MediaType
	utils.Debug(l, span, "layer #0 (lID: %d) is %s", layer.ID, layer.MediaType)
	switch utils.ExpandMediaType(layer.MediaType) {
	case "application/vnd.docker.image.rootfs.diff.tar.gzip",
		"application/vnd.oci.image.layer.v1.tar+gzip":
		utils.Warn(l, span, "layer #0 (lID: %d) is tar.gzip", layer.ID)
		os, e := mag.EvalOSFromBlob(ctx, layer.Digest)
		if e != nil {
			return types.OS{}, utils.ErrorHandler(l, span, e, "failed to eval layer")
		}
		mag.addOSAnnotations(ctx, maniData, os)
		return os, nil
	default:
		return types.OS{}, utils.NewError(l, span, "unknown media-type: %s", layer.MediaType)
	}
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
	ups = types.Upstream{Reg: req.Host, Repo: upstreamRepo, Tag: req.Tag, Digest: dig, Profile: req.Profile}
	ups.ID, e = mag.NeoInsertUpstream(ctx, ups)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while inserting upstream")
		return
	}
	maniBytes, maniMeta, e := mag.getOrFetchManifest(ctx, ups)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while fetching manifest")
		return
	}
	rawMani, e := minio.RawManifestFromBlob(maniBytes)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to RawManifestFromBlob")
		return
	}
	// Fetch Upstream Config — use upstreamRepo (stripped) to ensure the
	// reference targets the real upstream path, not the prefix-preserved alias.
	configUpt := upstream.UpstreamTriple{Reg: req.Host, Repo: upstreamRepo, Tag: req.Tag, Digest: dig, Profile: req.Profile}
	switch maniMeta.MediaType {
	case "application/vnd.docker.distribution.manifest.v1+prettyjws":
		err = fmt.Errorf("manifest type is not supported: %s", maniMeta.MediaType)
		return
	case "application/vnd.docker.distribution.manifest.v2+json":
		configUpt.Digest = rawMani.ConfigInfo().Digest
	default:
		l.Fatalf("Unknown media type: %s", maniMeta.MediaType)
	}
	if e := mag.ensureConfigCached(ctx, configUpt); e != nil {
		err = utils.ErrorHandler(l, span, e, "while ensuring config is cached")
		return
	}
	maniData, e = mag.Minio.NewManifestFromBytes(ctx, maniBytes)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while Minio.NewManifest_FromBytes")
		return
	}
	maniData.Type = "PULLED_BY"
	if e := mag.insertManifestGraph(ctx, req, &maniData, ups); e != nil {
		err = utils.ErrorHandler(l, span, e, "while inserting manifest graph")
		return
	}
	os, e := mag.evalFirstLayerOS(ctx, req, upstreamRepo, &maniData)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while evaluating first layer OS")
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
	// Stamp the (repo, tag) freshness marker at pull-through ingest so the
	// metahub-api TTL logic knows when this alias was last refreshed (D4).
	if e = mag.Database.CollectionTagSetLastRefreshed(ctx, req.Repo, req.Tag, time.Now().Unix()); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to stamp CollectionTag.last_refreshed")
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
	case "application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json":
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
// UpstreamBlobInsertIndex expands a multi-arch index discovered upstream — a
// Docker manifest list or an OCI image index — by ingesting every supported
// child manifest (linux/amd64, linux/arm64) and its config metadata, and
// skipping unsupported architectures without failing (decision D5). Layer bytes
// are never downloaded during ingest; the per-profile compute filters by
// RuleTypePlatform downstream.
func (mag Magazine) UpstreamBlobInsertIndex(ctx context.Context, req types.Request, blob types.BlobMeta) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "UpstreamBlobInsertIndex",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  blob.Digest.String(),
	})
	defer span.End()

	// The Upstream node stores the repo as the upstream registry knows it
	// (prefix stripped); mirrors upstreamInsertGeneric.
	upstreamRepo := req.Repo
	if _, stripped, ok := mag.ResolveUpstreamRegistry(req.Repo); ok {
		upstreamRepo = stripped
	}
	up := upstream.UpstreamTriple{
		Reg:       req.Host,
		Repo:      upstreamRepo,
		Tag:       req.Tag,
		Digest:    blob.Digest,
		MediaType: blob.MediaType,
	}
	indexBytes, _, e := mag.GetManifestByUpstreamTripleByDigest(ctx, up)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to fetch upstream index")
	}
	var idx types.OCIManifestList
	if e := json.Unmarshal(indexBytes, &idx); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to parse upstream index")
	}
	utils.Info(l, span, "index %s carries %d manifest(s)", utils.ShortenDigest(blob.Digest.String()), len(idx.Manifests))

	children := supportedIndexChildren(idx)
	for _, dig := range children {
		// Metadata-only ingest for the child (manifest + config + layer
		// metadata); layer bytes stay lazy.
		if e := mag.upstreamBlobInsert(ctx, req, dig); e != nil {
			return utils.ErrorHandler(l, span, e, "failed to ingest child manifest")
		}
	}
	utils.Info(l, span, "ingested %d child manifest(s) for %s:%s", len(children), req.Repo, req.Tag)
	return nil
}

// supportedIndexChildren returns the child manifest digests MetaHub ingests
// from a multi-arch index: linux/amd64 and linux/arm64 only. Unsupported
// architectures (and non-linux OSes) are skipped (D5).
func supportedIndexChildren(idx types.OCIManifestList) []digest.Digest {
	var out []digest.Digest
	for _, m := range idx.Manifests {
		if m.Platform.OS != "" && m.Platform.OS != "linux" {
			continue
		}
		switch m.Platform.Architecture {
		case "amd64", "arm64":
			out = append(out, m.Digest)
		}
	}
	return out
}

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
