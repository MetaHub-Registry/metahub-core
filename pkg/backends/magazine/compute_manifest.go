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
	manis, e := mag.Database.Manifest_GetAll(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while getting all manifests")
		return
	}
	for _, mani := range manis {
		utils.Info(l, span, ">> Manifest: %s", mani.Upstream.String())
		// Get profiles connected to this manifest?
		prfs, e := mag.Database.ProfileFromManifestID(ctx, mani.ID)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "while getting profiles for manifest %d", mani.ID)
		}
		for _, prf := range prfs {
			utils.Info(l, span, " - Computing ManifestList for profile %s", prf.Label)
			e := mag.ComputeManifestForProfile(ctx, mani, prf)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, "while in ComputeManifest for %s", mani.Upstream.String())
				return
			}
		}
	}
	return
}

// ComputeManifests looks for changes to a manifest (e.g. new annotations), creates new config blobs and subsequently updates the manifest.
func (mag Magazine) ComputeManifest(ctx context.Context, mani types.Manifest) (err error) {
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
			utils.Warn(l, span, "[SKIP] ManifestData %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
			continue
		}
		utils.Warn(l, span, ">> ManifestData %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
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

func (mag Magazine) ComputeManifestForProfile(ctx context.Context, mani types.Manifest, prf types.Profile) (err error) {
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
			utils.Warn(l, span, "[SKIP] ManifestData %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
			continue
		}
		utils.Info(l, span, ">> ManifestData %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
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
		// Link Profile and new ManifestData
		if e = mag.Database.ManifestDataLinkToProfile(ctx, ociManiDataID, prf.ID); e != nil {
			return utils.ErrorHandler(l, span, e, "while linking manifestdata to profile")
		}
	}
	return
}
