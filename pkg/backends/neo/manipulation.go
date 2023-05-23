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
	_ = ctx
	key := "mpID"
	q := fmt.Sprintf(`CREATE (mp:Manipulation {label: "%s"}) RETURN id(mp) AS %s`, mp.Label, key)
	utils.Debug(l, span, q)
	if res, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else {
		if mpID, err := res.GetInt64(key); err != nil {
			return utils.ErrorHandler(l, span, err, types.FailBrGet, key)
		} else {
			mp.ID = mpID
		}
	}
	utils.Debug(l, span, "Manipulation has %d layers", len(mp.ManipuLayers))
	for _, mpl := range mp.ManipuLayers {
		if err := n.ManipuLayerGetCreate(ctx, &mpl); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipuLayerGetCreate)
		} else {
			if err := n.ManipulationLinkToManipuLayer(ctx, mp.ID, mpl.ID); err != nil {
				return utils.ErrorHandler(l, span, err, FailManipulationLinkToManipuLayer)
			}
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
	utils.Debug(l, span, q)
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
	utils.Debug(l, span, q)
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
	utils.Debug(l, span, q)
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

func (n Neo) ManipulationsGetByProfileID(ctx context.Context, prfID int64) (mps []types.Manipulation, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationGetByProfileID",
		"prfID":   prfID,
	})
	defer span.End()
	mps = make([]types.Manipulation, 0)
	_ = ctx
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:USES]->(mp:Manipulation) WHERE id(prf) = %d`, prfID)
	q += fmt.Sprintf(` RETURN %s`, ManipulationReturn)
	utils.Debug(l, span, q)
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
	}
	if mpl, e := n.ManipuLayersGetByManipulationID(ctx, mp.ID); e != nil {
		return mp, utils.ErrorHandler(l, span, e, FailManipuLayersGetByManipulationID)
	} else {
		mp.ManipuLayers = mpl
	}
	return mp, nil
}

func (n Neo) ManipulationFromConfigManipulation(ctx context.Context, m config.Manipulation) (mpl types.Manipulation, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationFromConfigManipulation",
	})
	defer span.End()
	mp := types.Manipulation{
		Label:        m.Label,
		ManipuLayers: make([]types.ManipuLayer, 0),
	}
	for _, cfgMppl := range m.ManipuLayers {
		utils.Debug(l, span, "ManipuLayer: %s (%d Cmds, %d Envs)", cfgMppl.Image, len(cfgMppl.Cmd), len(cfgMppl.Environment))
		up, e := types.NewUpstreamByName(cfgMppl.Image)
		if e != nil {
			return mp, utils.ErrorHandler(l, span, e, types.FailNewUpstreamByName)
		}
		if mpl, err := n.ManipuLayerFromUpstream(ctx, up, cfgMppl); err != nil {
			return mp, utils.ErrorHandler(l, span, err, FailManipuLayerFromUpstream)
		} else {
			utils.Debug(l, span, "Add Env and Cmd to ManipuLayer")
			mpl.Environment = make([]types.ManipuEnv, 0)
			for _, cfgEnv := range cfgMppl.Environment {
				mpe := types.ManipuEnv{
					Key:  cfgEnv.Key,
					Rule: cfgEnv.Rule,
				}
				utils.Debug(l, span, "Adding %s", mpe.ToString())
				mpl.Environment = append(mpl.Environment, mpe)
			}
			mpl.Cmd = make([]types.ManipuCmd, 0)
			for _, cfgCmd := range cfgMppl.Cmd {
				mpc := types.ManipuCmd{
					Cmd:  cfgCmd.Value,
					Rule: cfgCmd.Rule,
				}
				utils.Debug(l, span, "Adding %s", mpc.ToString())
				mpl.Cmd = append(mpl.Cmd, mpc)
			}
			mp.ManipuLayers = append(mp.ManipuLayers, mpl)
		}
	}
	return mp, nil
}

func (n Neo) ManinpulationCheckRules(ctx context.Context, prfID, maniID int64) (ok bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManinpulationCheckRules",
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
	prfRls, err := n.Rules_GetByProfileID(ctx, prfID)
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
				return ok, utils.ErrorHandler(l, span, err, FailRulesGetByManipuLayerID)
			}
			if rls.SatisfiedBy(ctx, maniRls) {
				utils.Warn(l, span, "Manipulation %d (ManipuLayers: %d) is satisfied by Manifest %d", mp.ID, len(mp.ManipuLayers), maniID)
				ok = true
				// FIXME: multiple manipulayers are going to fail here.
			}
		}
	}
	return ok, nil
}

func (n Neo) ManipulationFromYAML(ctx context.Context, cmp config.Manipulation) (mp types.Manipulation, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationFromYAML",
		"Label":   cmp.Label,
	})
	defer span.End()
	mp = types.Manipulation{
		Label: cmp.Label,
	}
	for _, layer := range cmp.ManipuLayers {
		if tl, e := n.ManipuLayerFromConfigYAML(ctx, layer); e != nil {
			return mp, utils.ErrorHandler(l, span, e, FailManipuLayerFromConfigYAML)
		} else {
			mp.ManipuLayers = append(mp.ManipuLayers, tl)
		}
	}
	return
}
