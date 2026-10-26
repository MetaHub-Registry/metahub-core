package neo

import (
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"golang.org/x/net/context"
)

const (
	pkgNameManipulation               = "neo/manipulation.go"
	FailManipulationLinkToManipuLayer = "failed to link manipulation to ManipuLayer"
	FailManipulationsGetByProfileID   = "failed to get manipulations by profile ID"
	FailManipulationGetCreate         = "failed to get or get/create manipulation"
)

var (
	ManipulationReturn = `id(mp) AS manipulationID, mp.label AS manipulationLabel`
)

func (n Neo) ManipulationDeleteByID(ctx context.Context, mpID int64) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationDeleteByID",
		"mpID":    mpID,
	})
	defer span.End()
	_ = ctx
	_ = l
	if _, found, e := n.ManipulationGetByID(ctx, mpID); e != nil {
		return found, utils.ErrorHandler(l, span, e, FailManipulationGetByID)
	} else if !found {
		return false, nil
	}
	// TODO: delete potential connections
	q := fmt.Sprintf(`MATCH (mp:Manipulation) WHERE id(mp) = %d`, mpID)
	q += ` DETACH DELETE mp`
	if _, e := n.Execute(q); e != nil {
		return true, utils.ErrorHandler(l, span, e, FailManipulationDeleteByID)
	}
	return true, nil
}

func (n Neo) ManipulationGetCreate(ctx context.Context, mp *types.Manipulation) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationGetCreate",
	})
	defer span.End()
	_ = ctx
	_ = l
	// Check if manipulation exists
	if mpID, found, err := n.ManipulationGetIDByLabel(ctx, mp.Label); err != nil {
		return utils.ErrorHandler(l, span, err, FailManipulationGetID)
	} else if found {
		utils.Debug(l, span, "Found manipulation '%s' with ID %d", mp.Label, mpID)
		mp.ID = mpID
		return nil
	}
	// Insert Manipulation
	if err := n.ManipulationCreate(ctx, mp); err != nil {
		return utils.ErrorHandler(l, span, err, FailManipulationCreate)
	}
	return nil
}

func (n Neo) ManipulationCreate(ctx context.Context, mp *types.Manipulation) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationCreate",
	})
	defer span.End()
	key := "mpID"
	q := fmt.Sprintf(`CREATE (mp:Manipulation {label: "%s"}) RETURN id(mp) AS %s`, mp.Label, key)
	utils.Debug(l, span, "%s", q)
	res, err := n.Execute(q)
	if err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	mpID, err := res.GetInt64(key)
	if err != nil {
		return utils.ErrorHandler(l, span, err, types.FailBrGet, key)
	}
	mp.ID = mpID

	// Legacy flat ManipuLayers: still written the old way. The new
	// default/variants path below is additive.
	utils.Debug(l, span, "Manipulation has %d flat ManipuLayers", len(mp.ManipuLayers))
	for _, mpl := range mp.ManipuLayers {
		if err := n.ManipuLayerGetCreate(ctx, &mpl); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipuLayerGetCreate)
		}
		if err := n.ManipulationLinkToManipuLayer(ctx, mp.ID, mpl.ID); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipulationLinkToManipuLayer)
		}
	}

	// New default block.
	if !mp.Default.IsZero() {
		if err := n.writeManipuConfig(ctx, mp.ID, relHasDefault, "", &mp.Default); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipuConfigCreate)
		}
	}

	// Variants.
	for i := range mp.Variants {
		v := &mp.Variants[i]
		if err := n.writeManipuConfig(ctx, mp.ID, relHasVariant, v.Rule, &v.Config); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipuConfigCreate)
		}
	}
	return nil
}

// writeManipuConfig persists a ManipuConfig node, links it to the owning
// Manipulation via the given relationship, and links each contained
// ManipuLayer by order. Layers are expected to have their ID populated
// (either pre-existing or freshly created by the caller).
func (n Neo) writeManipuConfig(ctx context.Context, mpID int64, rel, ruleKey string, cfg *types.ManipuConfig) error {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuConfig,
		"func":    "writeManipuConfig",
		"mpID":    mpID,
		"rel":     rel,
		"rule":    ruleKey,
	})
	defer span.End()
	if err := n.manipuConfigCreate(ctx, cfg); err != nil {
		return utils.ErrorHandler(l, span, err, FailManipuConfigCreate)
	}
	if err := n.manipuConfigLink(ctx, mpID, cfg.ID, rel, ruleKey); err != nil {
		return utils.ErrorHandler(l, span, err, FailManipuConfigLink)
	}
	for i := range cfg.Layers {
		lyr := &cfg.Layers[i]
		if lyr.ID == 0 {
			// Skip — the caller is responsible for pre-linking layers
			// that were already persisted via ManipuLayerGetCreate.
			utils.Debug(l, span, "skipping layer link for zero-id layer %q", lyr.Label)
			continue
		}
		if err := n.manipuConfigLinkLayer(ctx, cfg.ID, lyr.ID, i); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipuConfigCmdLinkFailed)
		}
		// Also write the legacy (:Manipulation)-[:HAS]->(:ManipuLayer)
		// edge for backward compatibility with tooling that queries
		// it directly (e.g. GoCD e2e-dev verify-ce).
		if err := n.manipulationLinkToManipuLayerLegacy(ctx, mpID, lyr.ID); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipulationLinkToManipuLayer)
		}
	}
	return nil
}

func (n Neo) ManipulationLinkToManipuLayer(ctx context.Context, mpID, mplID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationLinkToManipuLayer",
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (mp:Manipulation), (mpl:ManipuLayer) WHERE id(mp) = %d AND id(mpl) = %d`, mpID, mplID)
	q += " MERGE (mp)-[:HAS]->(mpl)"
	utils.Debug(l, span, "%s", q)
	if _, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	return nil
}

func (n Neo) ManipulationGetIDByLabel(ctx context.Context, label string) (mpID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationGetIDByLabel",
	})
	defer span.End()
	_ = ctx
	key := "mpID"
	q := fmt.Sprintf(`MATCH (mp:Manipulation) WHERE mp.label = "%s" RETURN id(mp) AS %s`, label, key)
	utils.Debug(l, span, "%s", q)
	if res, found, err := n.QueryOne(q); err != nil {
		return mpID, false, utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else if !found {
		return mpID, false, nil
	} else {
		if mpID, err = res.GetInt64(key); err != nil {
			return mpID, false, utils.ErrorHandler(l, span, err, types.FailBrGet, key)
		}
	}
	utils.Debug(l, span, "Manipulation with label '%s' has ID %d", label, mpID)
	return mpID, true, nil
}

func (n Neo) ManipulationGetByID(ctx context.Context, mpID int64) (mp types.Manipulation, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationGetByID",
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (mp:Manipulation) WHERE id(mp) = %d`, mpID)
	q += fmt.Sprintf(` RETURN %s`, ManipulationReturn)
	utils.Debug(l, span, "%s", q)
	if res, found, err := n.QueryOne(q); err != nil {
		return mp, false, utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else if !found {
		return mp, false, nil
	} else {
		if mp, err = n.ManipulationFromResult(ctx, res); err != nil {
			return mp, false, utils.ErrorHandler(l, span, err, FailManipulationFromResult)
		} else {
			return mp, true, nil
		}

	}
}

// ManipulationsWithConfigByProfileID returns every manipulation linked
// to a profile with Default + Variants fully hydrated from the graph.
// It is a convenience wrapper over `ManipulationsGetByProfileID`
// documenting that the hydration is already performed by
// `ManipulationFromResult`.
func (n Neo) ManipulationsWithConfigByProfileID(ctx context.Context, prfID int64) ([]types.Manipulation, error) {
	return n.ManipulationsGetByProfileID(ctx, prfID)
}

func (n Neo) ManipulationsGetByProfileID(ctx context.Context, prfID int64) (mps []types.Manipulation, err error) {
	return n.manipulationsGetByProfileWhere(ctx, fmt.Sprintf(`id(prf) = %d`, prfID), map[string]any{
		"func":  "ManipulationGetByProfileID",
		"prfID": prfID,
	})
}

// ManipulationsWithConfigByProfileLabel returns every manipulation linked to
// the profile with the given (stored) label. Public callers address a profile
// by label; the element ID stays inside this query.
func (n Neo) ManipulationsWithConfigByProfileLabel(ctx context.Context, label string) ([]types.Manipulation, error) {
	return n.manipulationsGetByProfileWhere(ctx, fmt.Sprintf(`prf.label = "%s"`, label), map[string]any{
		"func":  "ManipulationGetByProfileLabel",
		"label": label,
	})
}

func (n Neo) manipulationsGetByProfileWhere(ctx context.Context, where string, fields map[string]any) (mps []types.Manipulation, err error) {
	fields["package"] = pkgNameManipulation
	l, ctx, span := utils.InitLogging(ctx, tracer, fields)
	defer span.End()
	mps = make([]types.Manipulation, 0)
	_ = ctx
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(mp:Manipulation) WHERE %s`, where)
	q += fmt.Sprintf(` RETURN %s`, ManipulationReturn)
	utils.Debug(l, span, "%s", q)
	if res, err := n.Query(q); err != nil {
		return mps, utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else {
		for _, r := range res {
			if mp, err := n.ManipulationFromResult(ctx, r); err != nil {
				return mps, utils.ErrorHandler(l, span, err, FailManipulationFromResult)
			} else {
				mps = append(mps, mp)
			}
		}
	}
	return mps, nil
}

func (n Neo) ManipulationFromResult(ctx context.Context, res types.Result) (mp types.Manipulation, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationFromResult",
	})
	defer span.End()
	err = res.CheckKeys("manipulationID", "manipulationLabel")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("manipulationID")
	label, _ := res.GetStr("manipulationLabel")
	mp = types.Manipulation{
		ID:           id,
		Label:        label,
		ManipuLayers: make([]types.ManipuLayer, 0),
		Variants:     make([]types.ManipuVariant, 0),
	}
	if mpl, e := n.ManipuLayersGetByManipulationID(ctx, mp.ID); e != nil {
		return mp, utils.ErrorHandler(l, span, e, FailManipuLayersGetByManipulationID)
	} else {
		mp.ManipuLayers = mpl
	}
	// Hydrate the default / variant configs when present.
	links, e := n.manipuConfigsByManipulationID(ctx, mp.ID)
	if e != nil {
		return mp, utils.ErrorHandler(l, span, e, FailManipuConfigGetByManipID)
	}
	for _, lnk := range links {
		cfg, ce := n.manipuConfigHydrate(ctx, lnk.CfgID)
		if ce != nil {
			return mp, utils.ErrorHandler(l, span, ce, FailManipuConfigFromResult)
		}
		switch lnk.Rel {
		case relHasDefault:
			mp.Default = cfg
		case relHasVariant:
			mp.Variants = append(mp.Variants, types.ManipuVariant{Rule: lnk.Rule, Config: cfg})
		}
	}
	return mp, nil
}

func (n Neo) ManipulationFromConfigManipulation(ctx context.Context, m config.Manipulation) (types.Manipulation, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationFromConfigManipulation",
	})
	defer span.End()
	mp := types.Manipulation{
		Label:        m.Label,
		ManipuLayers: make([]types.ManipuLayer, 0),
		Variants:     make([]types.ManipuVariant, 0),
	}
	// New extended schema path.
	if m.Variants != nil || !m.DefaultConfig().IsZero() {
		if err := n.populateExtendedManipulation(ctx, &mp, m); err != nil {
			return mp, utils.ErrorHandler(l, span, err, FailManipuConfigCreate)
		}
		return mp, nil
	}
	// Legacy flat layers: preserve old behaviour for backward compatibility.
	for _, cfgMppl := range m.ManipuLayers {
		mpl, err := n.buildLegacyManipuLayer(ctx, cfgMppl)
		if err != nil {
			return mp, utils.ErrorHandler(l, span, err, FailManipuLayerFromUpstream)
		}
		mp.ManipuLayers = append(mp.ManipuLayers, mpl)
	}
	return mp, nil
}

// populateExtendedManipulation fills mp.Default and mp.Variants from the
// config-level manipulation.
func (n Neo) populateExtendedManipulation(ctx context.Context, mp *types.Manipulation, m config.Manipulation) error {
	defCfg, err := n.buildTypesConfigFromYAML(ctx, m.DefaultConfig())
	if err != nil {
		return err
	}
	mp.Default = defCfg
	for rule, cfgIn := range m.Variants {
		cfg, e := n.buildTypesConfigFromYAML(ctx, cfgIn)
		if e != nil {
			return e
		}
		mp.Variants = append(mp.Variants, types.ManipuVariant{Rule: rule, Config: cfg})
	}
	return nil
}

// buildLegacyManipuLayer walks a single config.ManipuLayer entry (legacy
// flat form) through the Upstream+Manifest pipeline and attaches
// Environment + Cmd entries.
func (n Neo) buildLegacyManipuLayer(ctx context.Context, cfgMppl config.ManipuLayer) (types.ManipuLayer, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "buildLegacyManipuLayer",
	})
	defer span.End()
	utils.Debug(l, span, "ManipuLayer: %s (%d Cmds, %d Envs)", cfgMppl.Image, len(cfgMppl.Cmd), len(cfgMppl.Environment))
	up, err := types.NewUpstreamByName(cfgMppl.Image)
	if err != nil {
		return types.ManipuLayer{}, utils.ErrorHandler(l, span, err, types.FailNewUpstreamByName)
	}
	mpl, err := n.ManipuLayerFromUpstream(ctx, up, cfgMppl)
	if err != nil {
		return types.ManipuLayer{}, utils.ErrorHandler(l, span, err, FailManipuLayerFromUpstream)
	}
	mpl.Environment = make([]types.ManipuEnv, 0, len(cfgMppl.Environment))
	for _, cfgEnv := range cfgMppl.Environment {
		mpl.Environment = append(mpl.Environment, types.ManipuEnv{Key: cfgEnv.Key, Rule: cfgEnv.Rule})
	}
	mpl.Cmd = make([]types.ManipuCmd, 0, len(cfgMppl.Cmd))
	for _, cfgCmd := range cfgMppl.Cmd {
		mpl.Cmd = append(mpl.Cmd, types.ManipuCmd{Cmd: cfgCmd.Value, Rule: cfgCmd.Rule})
	}
	return mpl, nil
}

// buildTypesConfigFromYAML converts a config.ManipuConfig into its
// types.ManipuConfig equivalent, resolving each `Layers` entry into a
// graph-persisted ManipuLayer node linked to its Upstream and Manifest.
//
// Unlike the legacy `ManipuLayerFromUpstream` pipeline, this helper does
// NOT expect a per-layer `rule:` — variant layers inherit their rule
// from the `HAS_VARIANT {rule}` edge on their owning ManipuConfig.
func (n Neo) buildTypesConfigFromYAML(ctx context.Context, cfgIn config.ManipuConfig) (types.ManipuConfig, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "buildTypesConfigFromYAML",
	})
	defer span.End()
	out := cfgIn.ToTypesManipuConfig()
	if len(cfgIn.Layers) == 0 {
		return out, nil
	}
	layers := make([]types.ManipuLayer, 0, len(cfgIn.Layers))
	for _, cfgMppl := range cfgIn.Layers {
		if cfgMppl.Image == "" {
			utils.Debug(l, span, "skipping image-less layer rule=%q", cfgMppl.Rule)
			continue
		}
		up, e := types.NewUpstreamByName(cfgMppl.Image)
		if e != nil {
			return out, utils.ErrorHandler(l, span, e, types.FailNewUpstreamByName)
		}
		mpl, e := n.manipuLayerCreateForConfig(ctx, up, cfgMppl.Select)
		if e != nil {
			return out, utils.ErrorHandler(l, span, e, FailManipuLayerFromUpstream)
		}
		layers = append(layers, mpl)
	}
	out.Layers = layers
	return out, nil
}

func (n Neo) ManipulationCheckRules(ctx context.Context, prfID, maniID int64) (ok bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationCheckRules",
		"prfID":   prfID,
		"maniID":  maniID,
	})
	defer span.End()
	// TODO: Check if the manifest/profile pair is actually a fit (do all rules evaluate true?)
	//// TODO #1: In case a profile rule exist, does it check out?
	maniRls, e := n.RulesGetByManifestID(ctx, maniID)
	if e != nil {
		return ok, utils.ErrorHandler(l, span, e, FailRulesGetByManifestID)
	}
	prfRls, err := n.RulesGetByProfileID(ctx, prfID)
	if !prfRls.SatisfiedBy(ctx, maniRls) {
		utils.Warn(l, span, "Profile rules '%s' are not satisfied by manifest", prfRls.ToString())
		return false, nil
	}
	//// TODO #2: Check the ManipuLayer rule
	mps, err := n.ManipulationsGetByProfileID(ctx, prfID)
	if err != nil {
		return ok, utils.ErrorHandler(l, span, err, FailManipulationsGetByProfileID)
	}
	utils.Warn(l, span, "Manipulations:  %d", len(mps))
	for _, mp := range mps {
		utils.Debug(l, span, "Manipulation [%-2d]: %s", mp.ID, mp.Label)
		for _, mpl := range mp.ManipuLayers {
			rls, err := n.RulesGetByManipuLayerID(ctx, mpl.ID)
			if err != nil {
				return false, utils.ErrorHandler(l, span, err, FailRulesGetByManipuLayerID)
			}
			if rls.SatisfiedBy(ctx, maniRls) {
				utils.Warn(l, span, "Manipulation %d, ManipuLayer %d is satisfied by Manifest %d", mp.ID, mpl.ID, maniID)
				return true, nil
			}
		}
	}
	return false, nil
}

func (n Neo) ManipulationFromYAML(ctx context.Context, cmp config.Manipulation) (mp types.Manipulation, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationFromYAML",
		"Label":   cmp.Label,
	})
	defer span.End()
	mp = types.Manipulation{
		Label:    cmp.Label,
		Variants: make([]types.ManipuVariant, 0),
	}
	// New extended schema path: transcribe defaults + variants. Layers
	// inside any config block are created via the existing ManipuLayer
	// pipeline so upstream/manifest linkage is established.
	if cmp.Variants != nil || !cmp.DefaultConfig().IsZero() {
		if err = n.populateExtendedManipulation(ctx, &mp, cmp); err != nil {
			return mp, utils.ErrorHandler(l, span, err, FailManipuConfigCreate)
		}
		return
	}
	// Legacy flat layers form.
	for _, layer := range cmp.ManipuLayers {
		tl, e := n.ManipuLayerFromConfigYAML(ctx, layer)
		if e != nil {
			return mp, utils.ErrorHandler(l, span, e, FailManipuLayerFromConfigYAML)
		}
		mp.ManipuLayers = append(mp.ManipuLayers, tl)
	}
	return
}
