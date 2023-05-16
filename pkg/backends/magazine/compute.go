package magazine

import (
	"context"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const pkgNameCompute = "backend/magazine/compute.go"

func (mag Magazine) ComputeManifestForProfile(ctx context.Context, mani types.Manifest, prfID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCompute,
		"func":    "ComputeManifest",
		"mani":    mani.ID,
		"name":    mani.Upstream.String(),
	})
	defer span.End()
	// Get Profile
	prf, found, e := mag.Database.Profile_Get_ByID(ctx, prfID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to Database.Profile_Get_ByID")

		return
	}
	if !found {
		err = utils.NewError(l, span, "Profile not found: %d", prfID)
		return
	}
	l.Warnf("Manifest: %s/%s:%s", mani.Upstream.Reg, mani.Upstream.Repo, mani.Upstream.Tag)
	for _, maniData := range mani.Manifests {
		if maniData.MediaType == v1.MediaTypeImageManifest {
			utils.Warn(l, span, "[SKIP] ManifestData %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
			continue
		}
		utils.Debug(l, span, ">> ManifestData %s (%-10s): %s (Annotations: %d, Layers:%d)", utils.ShortenDigest(maniData.Digest), maniData.Origin, maniData.MediaType, len(mani.Annotations.Keys), len(maniData.Layers))
		if len(mani.Annotations.Keys) > 0 {
			l.Infof("  >> Annotations: %v", mani.Annotations.Keys)
		}
		///TODO: Need to add the annotations to the config
		_, e := mag.CreateOCIv1ForProfile(ctx, mani, maniData, prf)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while in ComputeManifest")
			l.Error(err)
			return
		}
	}
	return
}

// ComputeManifests looks for changes to a manifest (e.g. new annotations), creates new config blobs and subsequently updates the manifest.
func (mag Magazine) ComputeManifest(ctx context.Context, mani types.Manifest) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCompute,
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
			err = utils.ErrorHandler(l, span, e, "while in ComputeManifest")
			l.Error(err)
			return
		}
	}
	return
}

// ComputeManifests looks for changes to a manifest (e.g. new annotations), creates new config blobs and subsequently updates the manifest.
func (mag Magazine) ComputeManifests(ctx context.Context) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCompute,
		"func":    "ComputeManifests",
	})
	defer span.End()
	e := mag.Database.Rule_EvalAll(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while Rule_EvalAll")
		return
	}
	// 	1. Get all manifest(standins)
	manis, e := mag.Database.Manifest_GetAll(ctx)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "while getting all manifests")
		return
	}
	for _, mani := range manis {
		e := mag.ComputeManifest(ctx, mani)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "while in ComputeManifest for %s", mani.Upstream.String())
			return
		}
	}
	return
}
