package magazine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
//
// For new-schema manipulations the effective OCI config mutation is
// computed by merging `mp.Default` with the matching variant's config
// (variant values override defaults). Legacy-form manipulations
// produce an empty ManipuConfig because env/cmd live on the legacy
// ManipuEnv/ManipuCmd nodes which are handled by the legacy
// `ApplyManipulationToConfig` path for pushed manifests.
func tryApplyManipulation(ctx context.Context, mag Magazine, l *logrus.Entry, span trace.Span, mp *types.Manipulation, md *types.Manifest, result *[]types.Manifest) bool {
	for _, item := range flattenManipuLayers(mp) {
		if !manipuLayerMatchesPlatform(item, md.Platform) {
			continue
		}
		utils.Info(l, span, "ManipuLayer %s matches manifest %s (platform %s, rule=%q)", item.Label, md.Digest, md.Platform.GetLabel(), item.Rule)
		cfg := effectiveConfigForLayer(mp, item)
		manipulatedMd, e := mag.createManipulatedManifest(ctx, *md, item, mp.Label, cfg)
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

// effectiveConfigForLayer builds the OCI config mutation that should be
// applied when `layer` is the matched variant layer for `mp`. Merges
// `mp.Default` with the matching variant's `Config` so variant values
// override defaults. Env maps are union-merged; scalar fields
// (workdir, user) use variant-first-then-default; volumes are unioned.
// Entrypoint and Cmd are variant-first (variant commands replace
// defaults wholesale because they carry their own op).
//
// Returns a zero ManipuConfig when the manipulation has no Default or
// Variants (legacy flat form) so the caller can short-circuit the
// config-mutation step.
func effectiveConfigForLayer(mp *types.Manipulation, layer types.ManipuLayer) types.ManipuConfig {
	if mp.Default.IsZero() && len(mp.Variants) == 0 {
		return types.ManipuConfig{}
	}
	merged := cloneManipuConfig(mp.Default)
	// Find the variant whose rule matches the layer's Rule field
	// (flattenManipuLayers copied the variant rule onto the layer).
	for _, v := range mp.Variants {
		if v.Rule != layer.Rule {
			continue
		}
		mergeManipuConfig(&merged, v.Config)
		break
	}
	return merged
}

// cloneManipuConfig returns a deep-enough copy of cfg so that
// merge operations don't mutate the source manipulation tree.
// Layers are NOT copied — the caller only needs the scalar-shaped
// fields (env/workdir/user/volumes/entrypoint/cmd) for config
// application; layers are applied via the flatten path.
func cloneManipuConfig(cfg types.ManipuConfig) types.ManipuConfig {
	out := types.ManipuConfig{
		Workdir: cfg.Workdir,
		User:    cfg.User,
	}
	if len(cfg.Env) > 0 {
		out.Env = make(map[string]string, len(cfg.Env))
		for k, v := range cfg.Env {
			out.Env[k] = v
		}
	}
	if len(cfg.Volumes) > 0 {
		out.Volumes = append([]string(nil), cfg.Volumes...)
	}
	if cfg.Entrypoint != nil && !cfg.Entrypoint.IsZero() {
		cp := *cfg.Entrypoint
		cp.Value = append([]string(nil), cfg.Entrypoint.Value...)
		out.Entrypoint = &cp
	}
	if cfg.Cmd != nil && !cfg.Cmd.IsZero() {
		cp := *cfg.Cmd
		cp.Value = append([]string(nil), cfg.Cmd.Value...)
		out.Cmd = &cp
	}
	return out
}

// mergeManipuConfig merges `variant` into `base` in-place. Variant
// values override defaults for scalars (workdir, user). Env maps
// are merged key-wise (variant wins on duplicate keys). Volumes
// are union-merged preserving order (defaults first, then
// variant-only entries). Entrypoint / Cmd from the variant
// replace the default wholesale when present.
func mergeManipuConfig(base *types.ManipuConfig, variant types.ManipuConfig) {
	mergeScalars(base, variant)
	mergeEnv(base, variant.Env)
	base.Volumes = mergeVolumes(base.Volumes, variant.Volumes)
	if cmd := cloneCommandIfPresent(variant.Entrypoint); cmd != nil {
		base.Entrypoint = cmd
	}
	if cmd := cloneCommandIfPresent(variant.Cmd); cmd != nil {
		base.Cmd = cmd
	}
}

// mergeScalars applies variant's non-empty scalar fields to base.
func mergeScalars(base *types.ManipuConfig, variant types.ManipuConfig) {
	if variant.Workdir != "" {
		base.Workdir = variant.Workdir
	}
	if variant.User != "" {
		base.User = variant.User
	}
}

// mergeEnv merges variant env entries into base.Env, allocating the
// map on demand. Variant values override defaults on duplicate keys.
func mergeEnv(base *types.ManipuConfig, variantEnv map[string]string) {
	if len(variantEnv) == 0 {
		return
	}
	if base.Env == nil {
		base.Env = make(map[string]string, len(variantEnv))
	}
	for k, v := range variantEnv {
		base.Env[k] = v
	}
}

// mergeVolumes union-merges `variant` into `base`, preserving the
// original ordering of base and appending variant-only entries.
func mergeVolumes(base, variant []string) []string {
	if len(variant) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base))
	for _, v := range base {
		seen[v] = struct{}{}
	}
	for _, v := range variant {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		base = append(base, v)
	}
	return base
}

// cloneCommandIfPresent returns a deep copy of cmd when it is non-nil
// and non-zero, otherwise nil. Callers use the returned value to
// replace an existing Entrypoint or Cmd only when the variant
// actually provides one.
func cloneCommandIfPresent(cmd *types.ManipuCommand) *types.ManipuCommand {
	if cmd == nil || cmd.IsZero() {
		return nil
	}
	cp := *cmd
	cp.Value = append([]string(nil), cmd.Value...)
	return &cp
}

// applyManipuConfigToV1 mutates a v1.Image config in place by applying
// the env / workdir / user / volumes / entrypoint / cmd from the given
// ManipuConfig. Env keys are upserted into the Env slice. Entrypoint
// and Cmd respect the `Op` field (replace / prepend / append). Volumes
// are union-merged into the Volumes map. Zero-valued fields are skipped.
func applyManipuConfigToV1(cfg types.ManipuConfig, v1Cfg *v1.Image) {
	if v1Cfg == nil || cfg.IsZero() {
		return
	}
	if cfg.Workdir != "" {
		v1Cfg.Config.WorkingDir = cfg.Workdir
	}
	if cfg.User != "" {
		v1Cfg.Config.User = cfg.User
	}
	if len(cfg.Env) > 0 {
		v1Cfg.Config.Env = upsertEnv(v1Cfg.Config.Env, cfg.Env)
	}
	if len(cfg.Volumes) > 0 {
		if v1Cfg.Config.Volumes == nil {
			v1Cfg.Config.Volumes = make(map[string]struct{}, len(cfg.Volumes))
		}
		for _, v := range cfg.Volumes {
			v1Cfg.Config.Volumes[v] = struct{}{}
		}
	}
	if cfg.Entrypoint != nil && !cfg.Entrypoint.IsZero() {
		v1Cfg.Config.Entrypoint = applyManipuCommand(v1Cfg.Config.Entrypoint, cfg.Entrypoint)
	}
	if cfg.Cmd != nil && !cfg.Cmd.IsZero() {
		v1Cfg.Config.Cmd = applyManipuCommand(v1Cfg.Config.Cmd, cfg.Cmd)
	}
}

// upsertEnv merges `add` into the existing KEY=VALUE slice `base`,
// overriding any existing entry with the same key. The order of
// existing keys is preserved; newly-added keys are appended.
func upsertEnv(base []string, add map[string]string) []string {
	// Index existing entries by key so lookups during the merge are O(1).
	idx := make(map[string]int, len(base))
	for i, kv := range base {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		idx[kv[:eq]] = i
	}
	// Deterministic iteration: sort keys so the emitted slice stays
	// stable across parses. Without this the manipulated manifest's
	// digest would flap every rebuild.
	keys := make([]string, 0, len(add))
	for k := range add {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := append([]string(nil), base...)
	for _, k := range keys {
		entry := k + "=" + add[k]
		if i, ok := idx[k]; ok {
			out[i] = entry
			continue
		}
		out = append(out, entry)
	}
	return out
}

// applyManipuCommand returns the result of applying a ManipuCommand
// to an existing string-slice value. Replace overwrites the slice,
// prepend inserts the command value at the start, append at the end.
// Unknown ops fall back to the identity (i.e. leave existing unchanged).
func applyManipuCommand(existing []string, cmd *types.ManipuCommand) []string {
	if cmd == nil || cmd.IsZero() {
		return existing
	}
	switch cmd.Op {
	case types.ManipuCmdOpReplace:
		out := append([]string(nil), cmd.Value...)
		return out
	case types.ManipuCmdOpPrepend:
		out := append([]string(nil), cmd.Value...)
		out = append(out, existing...)
		return out
	case types.ManipuCmdOpAppend:
		out := append([]string(nil), existing...)
		out = append(out, cmd.Value...)
		return out
	default:
		return existing
	}
}

// flattenManipuLayers returns every ManipuLayer reachable from the
// manipulation tree. When the manipulation uses the new schema
// (Default or Variants populated), the legacy flat `mp.ManipuLayers`
// slice is ignored because it is a compatibility mirror of the
// variant layers (wired via the `:HAS` edge for old tooling) and
// re-including it would produce duplicate entries with empty rules,
// which the matcher would spuriously pick before the rule-keyed
// variant entries.
//
// Order: legacy-only manipulations → legacy flat layers first.
// New-schema manipulations → default block's layers (rule inherited
// as-is, typically empty), then each variant's layers with the
// variant's rule string copied onto the `Rule` field so
// `manipuLayerMatchesPlatform` can match on it.
func flattenManipuLayers(mp *types.Manipulation) []types.ManipuLayer {
	hasExtended := !mp.Default.IsZero() || len(mp.Variants) > 0
	out := make([]types.ManipuLayer, 0, len(mp.ManipuLayers))
	if !hasExtended {
		out = append(out, mp.ManipuLayers...)
		return out
	}
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
//
// cfg carries the OCI config mutation (env / workdir / user / volumes /
// entrypoint / cmd) to apply to the rebuilt v1.Image config. Pass a
// zero ManipuConfig to skip config mutation — only layers will be
// appended in that case.
func (mag Magazine) createManipulatedManifest(ctx context.Context, orig types.Manifest, mpl types.ManipuLayer, mpLabel string, cfg types.ManipuConfig) (types.Manifest, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameManipulation,
		"func":     "createManipulatedManifest",
		"origID":   orig.ID,
		"mplID":    mpl.ID,
		"mplLabel": mpl.Label,
	})
	defer span.End()

	manipLabel := fmt.Sprintf("%s#%s", orig.Label, mpLabel)
	if existing, hit := mag.reuseExistingManipulated(ctx, l, span, orig.Digest, mpLabel, manipLabel); hit {
		return existing, nil
	}
	parts, err := mag.gatherManipulationParts(ctx, l, span, orig, mpl)
	if err != nil {
		return types.Manifest{}, err
	}
	newCfgType, err := mag.buildAndStoreManipulatedConfig(ctx, l, span, parts, cfg)
	if err != nil {
		return types.Manifest{}, err
	}
	maniBytes, maniDigest, allLayers, err := mag.buildAndStoreManipulatedManifestBlob(ctx, l, span, parts, newCfgType)
	if err != nil {
		return types.Manifest{}, err
	}
	manipMd := types.Manifest{
		Digest:    maniDigest.String(),
		MediaType: v1.MediaTypeImageManifest,
		Size:      int64(len(maniBytes)),
		Label:     manipLabel,
		Platform:  orig.Platform,
		Config:    newCfgType,
		Layers:    allLayers,
	}
	if err := mag.persistManipulatedManifestGraph(ctx, l, span, &manipMd, orig, allLayers, mpLabel); err != nil {
		return types.Manifest{}, err
	}
	utils.Info(l, span, "Created manipulated manifest %d (%s) with %d layers from original %d", manipMd.ID, maniDigest.String(), len(allLayers), orig.ID)
	return manipMd, nil
}

// reuseExistingManipulated returns an already-persisted manipulated
// manifest for the (origDigest, manipulationLabel) pair when one
// exists, so repeat pulls don't regenerate identical blobs.
func (mag Magazine) reuseExistingManipulated(ctx context.Context, l *logrus.Entry, span trace.Span, origDigest, mpLabel, manipLabel string) (types.Manifest, bool) {
	existing, found, err := mag.Database.ManifestGetManipulatedByOriginalAndLabel(ctx, origDigest, mpLabel)
	if err != nil {
		utils.Warn(l, span, "ManifestGetManipulatedByOriginalAndLabel check failed (non-fatal): %v", err)
		return types.Manifest{}, false
	}
	if !found {
		return types.Manifest{}, false
	}
	utils.Info(l, span, "Reusing existing manipulated manifest %d (digest %s) for %s", existing.ID, existing.Digest, manipLabel)
	return existing, true
}

// manipulationParts bundles the original + ManipuLayer layers and
// configs that createManipulatedManifest needs across its stages.
//
// `mplSelectedIdx` carries the indices chosen by the ManipuLayer's
// `select` slice expression (see add-manipulayer-select openspec
// change). It is nil for identity selections — callers treat nil as
// "use mplLayers as-is" so the happy path stays a pure copy. When
// non-nil, both the layer descriptor slice and the DiffIDs in
// `mplV1Cfg.RootFS.DiffIDs` MUST be projected through the same
// indices at apply time, or docker's diff check will reject the
// manipulated manifest.
type manipulationParts struct {
	origLayers     []types.Layer
	mplLayers      []types.Layer
	origV1Cfg      *v1.Image
	mplV1Cfg       *v1.Image
	mplSelectedIdx []int
}

// gatherManipulationParts loads the original manifest's layers + config
// and the ManipuLayer image's layers + config, pulling upstreams
// on-demand when blobs are missing from MinIO.
func (mag Magazine) gatherManipulationParts(ctx context.Context, l *logrus.Entry, span trace.Span, orig types.Manifest, mpl types.ManipuLayer) (manipulationParts, error) {
	origLayers, err := mag.Database.GetLayersForManifestID(ctx, orig.ID)
	if err != nil {
		return manipulationParts{}, utils.ErrorHandler(l, span, err, "failed to get layers for original manifest %d", orig.ID)
	}
	mplLayersFull, err := mag.loadManipuLayerLayers(ctx, l, span, mpl)
	if err != nil {
		return manipulationParts{}, err
	}
	// Apply the ManipuLayer.Select slice (if any). Resolving here —
	// before loading the V1 config — means an out-of-range slice
	// surfaces before we do the expensive minio fetch.
	mplLayers, selectedIdx, err := applyManipuLayerSelect(l, span, mpl, mplLayersFull)
	if err != nil {
		return manipulationParts{}, err
	}
	utils.Info(l, span, "Original has %d layers, ManipuLayer adds %d (of %d) layers", len(origLayers), len(mplLayers), len(mplLayersFull))
	origV1Cfg, err := mag.loadOrPullManifestConfig(ctx, l, span, orig.ID, "original")
	if err != nil {
		return manipulationParts{}, err
	}
	mplV1Cfg, err := mag.loadOrPullManifestConfig(ctx, l, span, mpl.Image.ID, "ManipuLayer")
	if err != nil {
		return manipulationParts{}, err
	}
	return manipulationParts{
		origLayers:     origLayers,
		mplLayers:      mplLayers,
		origV1Cfg:      origV1Cfg,
		mplV1Cfg:       mplV1Cfg,
		mplSelectedIdx: selectedIdx,
	}, nil
}

// applyManipuLayerSelect parses and applies the ManipuLayer's slice
// expression. Returns the selected layers plus the indices used, so
// buildAndStoreManipulatedConfig can project the DiffIDs through the
// same projection. An identity select (empty / "[:]") returns the
// input slice and a nil index slice so downstream code can
// short-circuit on `selectedIdx == nil`.
func applyManipuLayerSelect(l *logrus.Entry, span trace.Span, mpl types.ManipuLayer, layers []types.Layer) ([]types.Layer, []int, error) {
	if mpl.Select == "" {
		return layers, nil, nil
	}
	ls, err := utils.ParseLayerSlice(mpl.Select)
	if err != nil {
		return nil, nil, utils.ErrorHandler(l, span, err, "ManipuLayer %q: invalid select expression", mpl.Label)
	}
	if ls.IsIdentity() {
		return layers, nil, nil
	}
	idx, err := ls.ApplyIndices(len(layers))
	if err != nil {
		return nil, nil, utils.ErrorHandler(l, span, err, "ManipuLayer %q: select %q against %d layers", mpl.Label, mpl.Select, len(layers))
	}
	out := make([]types.Layer, 0, len(idx))
	for _, i := range idx {
		out = append(out, layers[i])
	}
	utils.Info(l, span, "ManipuLayer %q select=%q kept %d/%d layers", mpl.Label, mpl.Select, len(out), len(layers))
	return out, idx, nil
}

// projectDiffIDs returns the subset of `in` at the indices `idx`. A
// nil `idx` means "no projection" and the input is returned
// unchanged. When the source config has fewer diffIDs than the
// layer descriptor list (malformed image), the function returns an
// error naming the mismatch so the operator sees the real problem
// rather than a confusing "diffID count != layer count" docker pull
// failure downstream.
func projectDiffIDs(in []digest.Digest, idx []int) ([]digest.Digest, error) {
	if idx == nil {
		return in, nil
	}
	out := make([]digest.Digest, 0, len(idx))
	for _, i := range idx {
		if i < 0 || i >= len(in) {
			return nil, fmt.Errorf("DiffID projection: index %d out of range for %d diffIDs (image layers and config DiffIDs likely disagree)", i, len(in))
		}
		out = append(out, in[i])
	}
	return out, nil
}

// loadManipuLayerLayers returns the ManipuLayer image's layer slice,
// preferring the pre-populated `mpl.Image.Layers` and falling back to
// a graph query when the slice is empty.
func (mag Magazine) loadManipuLayerLayers(ctx context.Context, l *logrus.Entry, span trace.Span, mpl types.ManipuLayer) ([]types.Layer, error) {
	if len(mpl.Image.Layers) > 0 {
		return mpl.Image.Layers, nil
	}
	mplLayers, err := mag.Database.GetLayersForManifestID(ctx, mpl.Image.ID)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "failed to get layers for ManipuLayer image %d", mpl.Image.ID)
	}
	return mplLayers, nil
}

// loadOrPullManifestConfig fetches the config blob for a manifest from
// MinIO. When the blob is absent the helper issues a best-effort
// CacheUpstream and retries once. `kind` is used in log messages to
// distinguish original vs ManipuLayer callers.
func (mag Magazine) loadOrPullManifestConfig(ctx context.Context, l *logrus.Entry, span trace.Span, maniID int64, kind string) (*v1.Image, error) {
	cfg, cfgFound, err := mag.Database.ConfigGetByManiDataID(ctx, maniID)
	if err != nil || !cfgFound {
		return nil, utils.ErrorHandler(l, span, err, "failed to get config for %s manifest %d", kind, maniID)
	}
	v1Cfg, err := mag.Minio.GetConfig(ctx, cfg.Digest)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "failed to get %s config from MinIO %s", kind, cfg.Digest)
	}
	if v1Cfg != nil {
		return v1Cfg, nil
	}
	utils.Info(l, span, "%s config blob %s not in MinIO, attempting on-demand pull for manifest %d", kind, cfg.Digest, maniID)
	up, _, upFound, upErr := mag.Database.UpstreamGetByManifestID(ctx, maniID)
	if upErr != nil || !upFound {
		return nil, utils.NewError(l, span, "%s config blob %s not found in MinIO and no upstream available for manifest %d", kind, cfg.Digest, maniID)
	}
	if cacheErr := mag.CacheUpstream(ctx, up); cacheErr != nil {
		return nil, utils.ErrorHandler(l, span, cacheErr, "failed to pull %s image from upstream %s/%s:%s", kind, up.Reg, up.Repo, up.Tag)
	}
	v1Cfg, err = mag.Minio.GetConfig(ctx, cfg.Digest)
	if err != nil || v1Cfg == nil {
		return nil, utils.NewError(l, span, "%s config blob %s still not found in MinIO after pull", kind, cfg.Digest)
	}
	return v1Cfg, nil
}

// buildAndStoreManipulatedConfig assembles the manipulated v1 config
// (origV1Cfg + ManipuLayer DiffIDs + OCI config mutation), stores the
// resulting blob in MinIO, and inserts the :Blob node into the graph.
func (mag Magazine) buildAndStoreManipulatedConfig(ctx context.Context, l *logrus.Entry, span trace.Span, parts manipulationParts, cfg types.ManipuConfig) (types.Config, error) {
	newV1Cfg := *parts.origV1Cfg
	// DiffIDs follow exactly the same projection as the layer
	// descriptors. Keep them in lockstep or docker refuses the pull
	// with a "layer digest doesn't match config diffID" error.
	mplDiffIDs, err := projectDiffIDs(parts.mplV1Cfg.RootFS.DiffIDs, parts.mplSelectedIdx)
	if err != nil {
		return types.Config{}, utils.ErrorHandler(l, span, err, "ManipuLayer select projection against DiffIDs")
	}
	newV1Cfg.RootFS.DiffIDs = append(newV1Cfg.RootFS.DiffIDs, mplDiffIDs...)
	// Apply the manipulation's OCI config mutations (env / workdir /
	// user / volumes / entrypoint / cmd) when the new-schema path has
	// produced a non-zero cfg. Legacy-form manipulations get a zero
	// cfg here; their env / cmd are applied via the separate legacy
	// ApplyManipulationToConfig path at push time.
	applyManipuConfigToV1(cfg, &newV1Cfg)
	cfgBytes, err := json.Marshal(newV1Cfg)
	if err != nil {
		return types.Config{}, utils.ErrorHandler(l, span, err, "failed to marshal new config")
	}
	cfgDigest := digest.FromBytes(cfgBytes)
	if err := mag.Minio.PutBlob(ctx, cfgBytes, cfgDigest.String()); err != nil {
		return types.Config{}, utils.ErrorHandler(l, span, err, "failed to store new config in MinIO")
	}
	newCfgType := types.Config{
		Digest:    cfgDigest.String(),
		MediaType: v1.MediaTypeImageConfig,
		Size:      int64(len(cfgBytes)),
	}
	cfgID, err := mag.Database.ConfigInsert(ctx, newCfgType)
	if err != nil {
		return types.Config{}, utils.ErrorHandler(l, span, err, "failed to insert new config into graph")
	}
	newCfgType.ID = cfgID
	return newCfgType, nil
}

// buildAndStoreManipulatedManifestBlob serialises the OCI v1 manifest
// that points at the concatenated layer set, stores it in MinIO and
// returns the bytes / digest along with the flat layer slice. The
// caller owns the subsequent graph-linking step.
func (mag Magazine) buildAndStoreManipulatedManifestBlob(ctx context.Context, l *logrus.Entry, span trace.Span, parts manipulationParts, newCfgType types.Config) ([]byte, digest.Digest, []types.Layer, error) {
	allLayers := make([]types.Layer, 0, len(parts.origLayers)+len(parts.mplLayers))
	allLayers = append(allLayers, parts.origLayers...)
	allLayers = append(allLayers, parts.mplLayers...)
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
			Size:      newCfgType.Size,
			Digest:    digest.Digest(newCfgType.Digest),
		},
		Layers: ociLayers,
	}
	maniBytes, err := json.Marshal(ociMani)
	if err != nil {
		return nil, "", nil, utils.ErrorHandler(l, span, err, "failed to marshal manipulated OCI manifest")
	}
	maniDigest := digest.FromBytes(maniBytes)
	if err := mag.Minio.ManifestsPutObj(ctx, maniDigest.String(), maniBytes); err != nil {
		return nil, "", nil, utils.ErrorHandler(l, span, err, "failed to store manipulated manifest in MinIO")
	}
	return maniBytes, maniDigest, allLayers, nil
}

// persistManipulatedManifestGraph creates the :Manifest node for the
// manipulated manifest and links it to its config, layers, platform
// and [:MANIPULATED_FROM {mpLabel}] origin. manipMd.ID is populated
// in place.
//
// mpLabel is the manipulation label (e.g. "layer/glow-extended#1.5.0")
// that gets written onto the MANIPULATED_FROM edge so the dedup
// lookup in reuseExistingManipulated can distinguish manipulations
// that share the same origin but produce different outputs.
func (mag Magazine) persistManipulatedManifestGraph(ctx context.Context, l *logrus.Entry, span trace.Span, manipMd *types.Manifest, orig types.Manifest, allLayers []types.Layer, mpLabel string) error {
	if err := mag.Database.ManifestGetInsert(ctx, manipMd); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to insert manipulated manifest into graph")
	}
	if err := mag.Database.ManifestLinkToConfig(ctx, manipMd.ID, manipMd.Config); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to link manipulated manifest to config")
	}
	for idx, lyr := range allLayers {
		if err := mag.Database.ManifestLinkToLayer(ctx, manipMd.ID, lyr, idx); err != nil {
			return utils.ErrorHandler(l, span, err, "failed to link manipulated manifest to layer %d", idx)
		}
	}
	if orig.Platform.ID > 0 {
		if err := mag.Database.LinkManifestToPlatform(ctx, manipMd.ID, orig.Platform.ID); err != nil {
			return utils.ErrorHandler(l, span, err, "failed to link manipulated manifest to platform")
		}
	}
	if err := mag.Database.ManifestLinkManipulatedFromWithLabel(ctx, manipMd.ID, orig.ID, mpLabel); err != nil {
		return utils.ErrorHandler(l, span, err, "failed to link MANIPULATED_FROM")
	}
	return nil
}
