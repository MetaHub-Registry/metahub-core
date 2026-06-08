package magazine

import (
	"context"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const pkgNameCompMani = "backend/magazine/compute_manifest.go"

func (mag Magazine) ComputeAllManifests(ctx context.Context) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCompMani,
		"func":    "ComputeAllManifests",
	})
	defer span.End()
	e := mag.Database.RuleEvalAll(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while RuleEvalAll")
		return
	}
	// 	1. Get all manifest(standins)
	manis, e := mag.Database.ManifestWrapperGetAll(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while getting all manifests")
		return
	}
	for _, mani := range manis {
		utils.Info(l, span, ">> Manifest: %s", mani.Upstream.String())
		// Get profiles connected to this manifest?
		prfs, e := mag.Database.ProfileFromManifestWrapperID(ctx, mani.ID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "while getting profiles for manifest %d", mani.ID)
		}
		for _, prf := range prfs {
			utils.Info(l, span, " - Computing Index for profile %s", prf.Label)
			e := mag.ComputeManifestForProfile(ctx, mani, prf)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while in ComputeManifest for %s", mani.Upstream.String())
				return
			}
		}
	}
	return
}

// ComputeManifestForLeaf is the Slot/leaf-based push-reshape entrypoint. It
// reshapes a pushed Docker v2 manifest into OCI v1 per profile WITHOUT
// requiring the caller to fetch a types.ManifestWrapper first. The caller
// (notably metahub-api's push handler) passes the content-leaf types.Manifest
// returned by StoreManifestUpload; this severs the last cross-repo wrapper
// coupling.
//
// Behaviour is identical to the wrapper-typed ComputeManifest: the wrapper
// identity is resolved internally from the content-leaf (via
// ManifestWrapperGetIDByManifestID + ManifestWrapperGetByID) and reshaping is
// delegated to ComputeManifest. The internal wrapper resolution is an
// implementation detail scheduled for removal in Phase 4 once the read-side
// Cypher (profile/manipulation evaluation) is rekeyed onto the Slot/leaf; the
// public contract here is leaf-only and will not change.
//
// If the leaf has no associated wrapper identity (e.g. nothing was actually
// stored), reshaping is skipped — matching the prior `found` guard at the call
// site.
func (mag Magazine) ComputeManifestForLeaf(ctx context.Context, md types.Manifest) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCompMani,
		"func":    "ComputeManifestForLeaf",
		"leaf.ID": md.ID,
		"digest":  md.Digest,
	})
	defer span.End()
	maniID, found, e := mag.Database.ManifestWrapperGetIDByManifestID(ctx, md.ID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while resolving wrapper id for content-leaf %d", md.ID)
	}
	if !found {
		utils.Debug(l, span, "no wrapper identity for content-leaf %d; skipping reshape", md.ID)
		return nil
	}
	mani, found, e := mag.Database.ManifestWrapperGetByID(ctx, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "while fetching wrapper %d for content-leaf %d", maniID, md.ID)
	}
	if !found {
		utils.Debug(l, span, "wrapper %d not found for content-leaf %d; skipping reshape", maniID, md.ID)
		return nil
	}
	return mag.ComputeManifest(ctx, mani)
}

// ComputeManifests looks for changes to a manifest (e.g. new annotations), creates new config blobs and subsequently updates the manifest.
func (mag Magazine) ComputeManifest(ctx context.Context, mani types.ManifestWrapper) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCompMani,
		"func":    "ComputeManifest",
		"mani":    mani.ID,
		"name":    mani.Upstream.String(),
	})
	defer span.End()
	l.Warnf("Manifest: %s/%s:%s", mani.Upstream.Reg, mani.Upstream.Repo, mani.Upstream.Tag)
	for _, maniData := range mani.Manifests {
		if maniData.MediaType == v1.MediaTypeImageManifest {
			utils.Warn(l, span, "[SKIP] Manifest %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
			continue
		}
		utils.Warn(l, span, ">> Manifest %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
		if len(mani.Annotations.Keys) > 0 {
			utils.Warn(l, span, "  >> Annotations: %v", mani.Annotations.Keys)
		}
		///TODO: Need to add the annotations to the config
		_, e := mag.CreateOCIv1(ctx, mani, maniData)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "while in ComputeManifest")
		}
	}
	return
}

func (mag Magazine) ComputeManifestForProfile(ctx context.Context, mani types.ManifestWrapper, prf types.Profile) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCompMani,
		"func":    "ComputeManifestForProfile",
		"mani":    mani.ID,
		"name":    mani.Upstream.ShortString(),
		"prfID":   prf.ID,
	})
	defer span.End()
	utils.Warn(l, span, "> Manifest [prf:%s]: %s/%s:%s", prf.Label, mani.Upstream.Reg, mani.Upstream.Repo, mani.Upstream.Tag)
	for _, maniData := range mani.Manifests {
		if maniData.MediaType == v1.MediaTypeImageManifest {
			utils.Warn(l, span, "[SKIP] Manifest %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
			continue
		}
		utils.Info(l, span, ">> Manifest %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
		if len(mani.Annotations.Keys) > 0 {
			l.Infof("  >> Annotations: %v", mani.Annotations.Keys)
		}
		///TODO: Need to add the annotations to the config
		if prf.ID == 0 {
			utils.NewError(l, span, "  >> No profile found, skipping")
			continue
		}
		for _, lyr := range maniData.Layers {
			utils.Debug(l, span, "   >> Layer: %s: %s", utils.ShortenDigest(lyr.Digest), lyr.MediaType)
		}
		utils.Debug(l, span, "Config: %v", maniData.Config)
		ociManiDataID, e := mag.CreateOCIv1ForProfile(ctx, mani, maniData, prf)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "while in ComputeManifest")
		}
		utils.Warn(l, span, "  >> ociManiDataID: %d", ociManiDataID)
		// Link Profile and new Manifest
		if e = mag.Database.ManifestLinkToProfile(ctx, ociManiDataID, prf.ID); e != nil {
			return utils.ErrorHandler(l, span, e, "while linking manifestdata to profile")
		}
	}
	return
}
