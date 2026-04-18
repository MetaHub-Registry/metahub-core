package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameManipulation = "magazine/manipulation.go"
)

func (mag Magazine) ManipulationGetCreate(ctx context.Context, mp types.Manipulation) (mpID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationCreate",
	})
	defer span.End()
	// Now the manipulation is inserted into the graph, need to make sure the upstream is cached and linked
	for _, mpl := range mp.ManipuLayers {
		if err = mag.CacheUpstream(ctx, mpl.Image.Upstream); err != nil {
			return -1, utils.ErrorHandler(l, span, err, FailCacheUpstream)
		}
	}
	if e := mag.Database.ManipulationGetCreate(ctx, &mp); e != nil {
		return -1, utils.ErrorHandler(l, span, e, neo.FailManipulationGetCreate)
	}
	mpID = mp.ID
	return
}

func (mag Magazine) ManipulationUnwind(ctx context.Context, maniID, maniDataID, prfID int64) (mplCfgDigest string, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameManipulation,
		"func":       "ManipulationUnwind",
		"prfID":      prfID,
		"maniID":     maniID,
		"maniDataID": maniDataID,
	})
	defer span.End()
	////////   The rules are going to be matched against the ManiPulayer to figure out a match
	maniRls, e := mag.Database.RulesGetByManifestID(ctx, maniID)
	if e != nil {
		return mplCfgDigest, utils.ErrorHandler(l, span, e, neo.FailRulesGetByManifestID)
	}

	// Get Manipulations for Profile
	mps, err := mag.Database.ManipulationsGetByProfileID(ctx, prfID)
	if err != nil {
		return mplCfgDigest, utils.ErrorHandler(l, span, err, neo.FailManipulationsGetByProfileID)
	}
	utils.Warn(l, span, "Manipulations:  %d", len(mps))
	for _, mp := range mps {
		for _, mpl := range mp.ManipuLayers {
			rls, err := mag.Database.RulesGetByManipuLayerID(ctx, mpl.ID)
			if err != nil {
				return mplCfgDigest, utils.ErrorHandler(l, span, err, neo.FailRulesGetByManipuLayerID)
			}
			// Check if mpRules all be matched by maniRules
			if rls.SatisfiedBy(ctx, maniRls) {
				utils.Warn(l, span, "ManipuLayer: %s (%d Layers) is satisfies by manifest ", mpl.Image.Label, len(mpl.Image.Layers))
				cfg, found, err := mag.Database.ConfigGetByManiDataID(ctx, mpl.Image.ID)
				if err != nil {
					return mplCfgDigest, utils.ErrorHandler(l, span, err, "Failed to get Config for Manifest of ManipuLayer %d", mpl.Image.ID)
				} else if !found {
					return mplCfgDigest, utils.NewError(l, span, "Did not find config for Manifest of ManipuLayer %d", mpl.Image.ID)
				}
				mplCfgDigest = cfg.Digest
				for idx, lyr := range mpl.Image.Layers {
					if e := mag.Database.ManifestLinkToLayer(ctx, maniDataID, lyr, idx); e != nil {
						return mplCfgDigest, utils.ErrorHandler(l, span, e, neo.FailManifestLinkToLayer)
					}
				}
			} else {
				utils.Warn(l, span, "ManipuLayer %d is NOT satisfied by Manifest %d", mpl.ID, maniID)
				continue
			}
		}
	}
	return mplCfgDigest, nil
}

func (mag Magazine) ApplyManipulationToConfig(ctx context.Context, maniID, prfID int64, v1Cfg *v1.Image, mplCfg types.Config) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ApplyManipulationToConfig",
		"prfID":   prfID,
		"maniID":  maniID,
	})
	defer span.End()
	////////   The rules are going to be matched against the ManiPulayer to figure out a match
	maniRls, e := mag.Database.RulesGetByManifestID(ctx, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, neo.FailRulesGetByManifestID)
	}

	// Get Manipulations for Profile
	mps, err := mag.Database.ManipulationsGetByProfileID(ctx, prfID)
	if err != nil {
		return utils.ErrorHandler(l, span, err, neo.FailManipulationsGetByProfileID)
	}
	utils.Warn(l, span, "Manipulations:  %d", len(mps))
	for _, mp := range mps {
		for _, mpl := range mp.ManipuLayers {
			utils.Debug(l, span, "ManipuLayer: %s (%d Layers)", mpl.Image.Label, len(mpl.Image.Layers))
			rls, err := mag.Database.RulesGetByManipuLayerID(ctx, mpl.ID)
			if err != nil {
				return utils.ErrorHandler(l, span, err, neo.FailRulesGetByManipuLayerID)
			}
			// Check if mpRules all be matched by maniRules
			if rls.SatisfiedBy(ctx, maniRls) {
				utils.Warn(l, span, "ManipuLayer: %s (%d Layers) is satisfies by manifest ", mpl.Image.Label, len(mpl.Image.Layers))
				// Apply Cmd manipulations
				mpcs, err := mag.Database.ManipuCmdsGetByManipulayerID(ctx, mpl.ID)
				if err != nil {
					return utils.ErrorHandler(l, span, err, neo.FailManipuCmdsGetByManipulayerID)
				}
				utils.Debug(l, span, "ManipuCmds: %d", len(mpcs))
				for _, mpc := range mpcs {
					utils.Debug(l, span, "ManipuCmd: %s", mpc.ToString())
					cmd := utils.Base64Decode(mpc.Cmd)
					switch mpc.Rule {
					case neo.ManipuCmdRulePrepend:
						v1Cfg.Config.Cmd = append([]string{cmd}, v1Cfg.Config.Cmd...)
					case neo.ManipuCmdRuleAppend:
						v1Cfg.Config.Cmd = append(v1Cfg.Config.Cmd, cmd)
					case neo.ManipuCmdRuleReplace:
						v1Cfg.Config.Cmd = []string{cmd}
					default:
						return utils.NewError(l, span, "ManipuCmdRule %s not implemented", mpc.Rule)
					}
				}
				// Apply Env manipulations
				mpes, err := mag.Database.ManipuEnvsGetByManipulayerID(ctx, mpl.ID)
				if err != nil {
					return utils.ErrorHandler(l, span, err, neo.FailManipuCmdsGetByManipulayerID)
				}
				utils.Debug(l, span, "ManipuEnvs: %d", len(mpes))
				for _, mpe := range mpes {
					utils.Debug(l, span, "ManipuEnv: %s", mpe.ToString())
					switch mpe.Rule {
					case neo.ManipuEnvRuleIfNULL:
						v1Cfg.Config.Env, err = mag.ApplyManipuEnv(ctx, mpe, mplCfg, v1Cfg.Config.Env)
						if err != nil {
							return utils.ErrorHandler(l, span, err, "Failed to apply ManipuEnv %d", mpe.ID)
						}
					default:
						return utils.NewError(l, span, "ManipuCmdRule %s not implemented", mpe.Rule)
					}
				}
			} else {
				utils.Warn(l, span, "ManipuLayer %d is NOT satisfied by Manifest %d", mpl.ID, maniID)
				continue
			}
		}
	}
	return nil
}

func (mag Magazine) ApplyManipuEnv(ctx context.Context, mpe types.ManipuEnv, mplCfg types.Config, in []string) (out []string, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ApplyManipuEnv",
	})
	defer span.End()
	_ = ctx
	mplCfgES := utils.NewEnvStore()
	for _, ele := range mplCfg.Cfg.Config.Env {
		if e := mplCfgES.AddEnv(ele); e != nil {
			utils.NewError(l, span, "Failed to parse Env %s", ele)
			continue
		}
	}
	es := utils.NewEnvStore()
	for _, ele := range in {
		if e := es.AddEnv(ele); e != nil {
			utils.NewError(l, span, "Failed to parse Env %s", ele)
			continue
		}
	}
	key := utils.Base64Decode(mpe.Key)
	if !es.Keys.Contains(key) {
		if !mplCfgES.Keys.Contains(key) {
			return in, utils.NewError(l, span, "Key %s not found in ManipuLayer Config", key)
		}
		val := mplCfgES.Map[key]
		es.Add(key, val)
	}
	return es.ToSlice(), nil
}

// manipuLayerMatchesPlatform checks if a ManipuLayer's platform rule matches a manifest's platform.
// Rules follow the pattern "arch:<architecture>=true" or "os:<os>=true".
// An empty rule matches all platforms.
func manipuLayerMatchesPlatform(mpl types.ManipuLayer, plat types.Platform) bool {
	rule := mpl.Rule
	if rule == "" {
		return true // No rule means match all
	}
	// Parse rule: "arch:arm64=true" -> key=arch, value=arm64
	// Strip "=true" suffix
	rule = strings.TrimSuffix(rule, "=true")
	parts := strings.SplitN(rule, ":", 2)
	if len(parts) != 2 {
		return false
	}
	key, value := parts[0], parts[1]
	switch key {
	case "arch":
		return plat.Arch == value
	case "os":
		return plat.OS == value
	default:
		return false
	}
}

// ApplyManipulationsToManifests checks a profile for manipulations and, for each
// candidate manifest that matches a ManipuLayer's platform rule, creates a manipulated
// copy (original layers + ManipuLayer layers). Returns a new candidate list where
// matched originals are replaced by their manipulated copies.
// If the profile has no manipulations, the original list is returned unchanged.
func (mag Magazine) ApplyManipulationsToManifests(ctx context.Context, prf types.Profile, candidates []types.Manifest) ([]types.Manifest, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameManipulation,
		"func":       "ApplyManipulationsToManifests",
		"prfID":      prf.ID,
		"prfLabel":   prf.Label,
		"candidates": len(candidates),
	})
	defer span.End()

	// Fetch manipulations for this profile, hydrated with Default + Variants
	// so both legacy (flat ManipuLayers) and new (defaults + variants) forms
	// are fully represented.
	mps, err := mag.Database.ManipulationsWithConfigByProfileID(ctx, prf.ID)
	if err != nil {
		return candidates, utils.ErrorHandler(l, span, err, neo.FailManipulationsGetByProfileID)
	}
	if len(mps) == 0 {
		utils.Debug(l, span, "Profile %s has no manipulations, returning candidates unchanged", prf.Label)
		return candidates, nil
	}
	utils.Info(l, span, "Profile %s has %d manipulation(s), processing %d candidates", prf.Label, len(mps), len(candidates))

	result := make([]types.Manifest, 0, len(candidates))
	for _, md := range candidates {
		manipulated := false
		for _, mp := range mps {
			if tryApplyManipulation(ctx, mag, l, span, &mp, &md, &result) {
				manipulated = true
				break // Only apply the first matching manipulation per manifest
			}
		}
		if !manipulated {
			// No manipulation matched or manipulation failed — keep the original
			result = append(result, md)
		}
	}
	utils.Info(l, span, "Manipulation complete: %d candidates → %d results", len(candidates), len(result))
	return result, nil
}

// tryApplyManipulation iterates every ManipuLayer reachable from the
// manipulation (legacy flat slice + Default + each variant) and applies
// the first one that matches the given manifest's platform. Returns true
// when a manipulated manifest was appended to `result`.
func tryApplyManipulation(ctx context.Context, mag Magazine, l *logrus.Entry, span trace.Span, mp *types.Manipulation, md *types.Manifest, result *[]types.Manifest) bool {
	for _, item := range flattenManipuLayers(mp) {
		if !manipuLayerMatchesPlatform(item, md.Platform) {
			continue
		}
		utils.Info(l, span, "ManipuLayer %s matches manifest %s (platform %s, rule=%q)", item.Label, md.Digest, md.Platform.GetLabel(), item.Rule)
		manipulatedMd, e := mag.createManipulatedManifest(ctx, *md, item, mp.Label)
		if e != nil {
			// Non-fatal: keep the original manifest.
			utils.Warn(l, span, "Manipulation failed for %s (non-fatal, using original): %v", md.Digest, e)
			return false
		}
		*result = append(*result, manipulatedMd)
		return true
	}
	return false
}

// flattenManipuLayers returns every ManipuLayer reachable from the
// manipulation tree. Legacy flat layers come first (in declaration
// order), then the default block's layers (rule is empty → matches
// every platform), then each variant's layers with the variant's rule
// propagated onto the `Rule` field of the flattened entry so
// `manipuLayerMatchesPlatform` can match on it.
func flattenManipuLayers(mp *types.Manipulation) []types.ManipuLayer {
	out := make([]types.ManipuLayer, 0, len(mp.ManipuLayers))
	out = append(out, mp.ManipuLayers...)
	for _, lyr := range mp.Default.Layers {
		out = append(out, lyr) // rule stays as-is (typically empty)
	}
	for _, v := range mp.Variants {
		for _, lyr := range v.Config.Layers {
			// The variant key IS the rule that selects this layer; copy
			// it onto the layer so the matcher has it.
			lyrCopy := lyr
			lyrCopy.Rule = v.Rule
			out = append(out, lyrCopy)
		}
	}
	return out
}

// createManipulatedManifest creates a new :Manifest node that is a copy of the original
// with the ManipuLayer's image layers appended. The new manifest is stored in MinIO and
// linked in the graph with proper edges including [:MANIPULATED_FROM].
func (mag Magazine) createManipulatedManifest(ctx context.Context, orig types.Manifest, mpl types.ManipuLayer, mpLabel string) (types.Manifest, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameManipulation,
		"func":     "createManipulatedManifest",
		"origID":   orig.ID,
		"mplID":    mpl.ID,
		"mplLabel": mpl.Label,
	})
	defer span.End()

	manipLabel := fmt.Sprintf("%s#%s", orig.Label, mpLabel)

	// Check if a manipulated manifest already exists for this original+manipulation (dedup)
	existing, found, e := mag.Database.ManifestGetManipulatedByOriginalAndLabel(ctx, orig.Digest, mpLabel)
	if e != nil {
		utils.Warn(l, span, "ManifestGetManipulatedByOriginalAndLabel check failed (non-fatal): %v", e)
	} else if found {
		utils.Info(l, span, "Reusing existing manipulated manifest %d (digest %s) for %s", existing.ID, existing.Digest, manipLabel)
		return existing, nil
	}
	_ = manipLabel // Used for logging

	// 1. Get original's layers
	origLayers, e := mag.Database.GetLayersForManifestID(ctx, orig.ID)
	if e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to get layers for original manifest %d", orig.ID)
	}

	// 2. Get ManipuLayer image's layers
	mplLayers := mpl.Image.Layers
	if len(mplLayers) == 0 {
		// Fetch from graph if not populated
		mplLayers, e = mag.Database.GetLayersForManifestID(ctx, mpl.Image.ID)
		if e != nil {
			return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to get layers for ManipuLayer image %d", mpl.Image.ID)
		}
	}
	utils.Info(l, span, "Original has %d layers, ManipuLayer adds %d layers", len(origLayers), len(mplLayers))

	// 3. Get original config from MinIO to read DiffIDs
	origCfg, origCfgFound, e := mag.Database.ConfigGetByManiDataID(ctx, orig.ID)
	if e != nil || !origCfgFound {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to get config for original manifest %d", orig.ID)
	}
	origV1Cfg, e := mag.Minio.GetConfig(ctx, origCfg.Digest)
	if e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to get original config from MinIO %s", origCfg.Digest)
	}
	if origV1Cfg == nil {
		// Config blob missing — try to pull the original image from upstream
		utils.Info(l, span, "original config blob %s not in MinIO, attempting on-demand pull for manifest %d", origCfg.Digest, orig.ID)
		up, _, upFound, upErr := mag.Database.UpstreamGetByManifestID(ctx, orig.ID)
		if upErr == nil && upFound {
			if cacheErr := mag.CacheUpstream(ctx, up); cacheErr == nil {
				origV1Cfg, e = mag.Minio.GetConfig(ctx, origCfg.Digest)
			}
		}
		if origV1Cfg == nil {
			return types.Manifest{}, utils.NewError(l, span, "original config blob %s not found in MinIO (pull also failed)", origCfg.Digest)
		}
	}

	// 4. Get ManipuLayer image config from MinIO to read DiffIDs.
	// If the config blob is missing, attempt to pull the ManipuLayer image
	// from its upstream registry on-demand (the image reference is stored
	// as an :ORIGINAL edge from the ManipuLayer's Manifest to an Upstream node).
	mplCfg, mplCfgFound, e := mag.Database.ConfigGetByManiDataID(ctx, mpl.Image.ID)
	if e != nil || !mplCfgFound {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to get config for ManipuLayer image %d", mpl.Image.ID)
	}
	mplV1Cfg, e := mag.Minio.GetConfig(ctx, mplCfg.Digest)
	if e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to get ManipuLayer config from MinIO %s", mplCfg.Digest)
	}
	if mplV1Cfg == nil {
		// Config blob missing from MinIO — try to pull the ManipuLayer image from upstream
		utils.Info(l, span, "ManipuLayer config blob %s not in MinIO, attempting on-demand pull for image %d", mplCfg.Digest, mpl.Image.ID)
		up, _, upFound, upErr := mag.Database.UpstreamGetByManifestID(ctx, mpl.Image.ID)
		if upErr != nil || !upFound {
			return types.Manifest{}, utils.NewError(l, span, "ManipuLayer config blob %s not found in MinIO and no upstream available for image %d", mplCfg.Digest, mpl.Image.ID)
		}
		utils.Info(l, span, "Pulling ManipuLayer image from %s/%s:%s", up.Reg, up.Repo, up.Tag)
		if cacheErr := mag.CacheUpstream(ctx, up); cacheErr != nil {
			return types.Manifest{}, utils.ErrorHandler(l, span, cacheErr, "failed to pull ManipuLayer image from upstream %s/%s:%s", up.Reg, up.Repo, up.Tag)
		}
		// Retry config fetch after pull
		mplV1Cfg, e = mag.Minio.GetConfig(ctx, mplCfg.Digest)
		if e != nil {
			return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to get ManipuLayer config from MinIO after pull %s", mplCfg.Digest)
		}
		if mplV1Cfg == nil {
			return types.Manifest{}, utils.NewError(l, span, "ManipuLayer config blob %s still not found in MinIO after pull", mplCfg.Digest)
		}
		utils.Info(l, span, "ManipuLayer image pulled successfully, config now available")
	}

	// 5. Build new v1 config: copy original and append ManipuLayer DiffIDs
	newV1Cfg := *origV1Cfg
	for _, diffID := range mplV1Cfg.RootFS.DiffIDs {
		newV1Cfg.RootFS.DiffIDs = append(newV1Cfg.RootFS.DiffIDs, diffID)
	}

	// 6. Serialize and store new config blob in MinIO
	cfgBytes, e := json.Marshal(newV1Cfg)
	if e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to marshal new config")
	}
	cfgDigest := digest.FromBytes(cfgBytes)
	if e := mag.Minio.PutBlob(ctx, cfgBytes, cfgDigest.String()); e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to store new config in MinIO")
	}

	// 7. Insert config into graph
	newCfgType := types.Config{
		Digest:    cfgDigest.String(),
		MediaType: v1.MediaTypeImageConfig,
		Size:      int64(len(cfgBytes)),
	}
	cfgID, e := mag.Database.ConfigInsert(ctx, newCfgType)
	if e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to insert new config into graph")
	}
	newCfgType.ID = cfgID

	// 8. Build the OCI v1 manifest with all layers (original + manipulation)
	allLayers := make([]types.Layer, 0, len(origLayers)+len(mplLayers))
	allLayers = append(allLayers, origLayers...)
	allLayers = append(allLayers, mplLayers...)

	ociLayers := make([]v1.Descriptor, 0, len(allLayers))
	for _, lyr := range allLayers {
		ociLayers = append(ociLayers, v1.Descriptor{
			MediaType: lyr.MediaType,
			Size:      lyr.Size,
			Digest:    digest.Digest(lyr.Digest),
		})
	}

	ociMani := v1.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: v1.MediaTypeImageManifest,
		Config: v1.Descriptor{
			MediaType: v1.MediaTypeImageConfig,
			Size:      int64(len(cfgBytes)),
			Digest:    cfgDigest,
		},
		Layers: ociLayers,
	}

	// 9. Serialize OCI manifest and compute digest
	maniBytes, e := json.Marshal(ociMani)
	if e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to marshal manipulated OCI manifest")
	}
	maniDigest := digest.FromBytes(maniBytes)

	// 10. Store in MinIO
	if e := mag.Minio.ManifestsPutObj(ctx, maniDigest.String(), maniBytes); e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to store manipulated manifest in MinIO")
	}

	// 11. Create :Manifest node in graph
	manipMd := types.Manifest{
		Digest:    maniDigest.String(),
		MediaType: v1.MediaTypeImageManifest,
		Size:      int64(len(maniBytes)),
		Label:     manipLabel,
		Platform:  orig.Platform,
		Config:    newCfgType,
		Layers:    allLayers,
	}
	if e := mag.Database.ManifestGetInsert(ctx, &manipMd); e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to insert manipulated manifest into graph")
	}

	// 12. Link config
	if e := mag.Database.ManifestLinkToConfig(ctx, manipMd.ID, newCfgType); e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to link manipulated manifest to config")
	}

	// 13. Link layers
	for idx, lyr := range allLayers {
		if e := mag.Database.ManifestLinkToLayer(ctx, manipMd.ID, lyr, idx); e != nil {
			return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to link manipulated manifest to layer %d", idx)
		}
	}

	// 14. Link platform (same as original)
	if orig.Platform.ID > 0 {
		if e := mag.Database.LinkManifestToPlatform(ctx, manipMd.ID, orig.Platform.ID); e != nil {
			return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to link manipulated manifest to platform")
		}
	}

	// 15. Link MANIPULATED_FROM
	if e := mag.Database.ManifestLinkManipulatedFrom(ctx, manipMd.ID, orig.ID); e != nil {
		return types.Manifest{}, utils.ErrorHandler(l, span, e, "failed to link MANIPULATED_FROM")
	}

	utils.Info(l, span, "Created manipulated manifest %d (%s) with %d layers from original %d", manipMd.ID, maniDigest.String(), len(allLayers), orig.ID)
	return manipMd, nil
}
