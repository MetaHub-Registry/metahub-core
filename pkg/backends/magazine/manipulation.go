package magazine

import (
	"context"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
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
					return mplCfgDigest, utils.ErrorHandler(l, span, err, "Failed to get Config for ManifestData of ManipuLayer %d", mpl.Image.ID)
				} else if !found {
					return mplCfgDigest, utils.NewError(l, span, "Did not find config for ManifestData of ManipuLayer %d", mpl.Image.ID)
				}
				mplCfgDigest = cfg.Digest
				for idx, lyr := range mpl.Image.Layers {
					if e := mag.Database.ManifestDataLinkToLayer(ctx, maniDataID, lyr, idx); e != nil {
						return mplCfgDigest, utils.ErrorHandler(l, span, e, neo.FailManifestDataLinkToLayer)
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
