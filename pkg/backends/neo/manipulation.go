package neo

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"golang.org/x/net/context"
)

const (
	pkgNameManipulation               = "neo/manipulation.go"
	FailManipulationLinkToManipuLayer = "failed to link manipulation to ManipuLayer"
	FailManipulationsGetByProfileID   = "failed to get manipulations by profile ID"
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

func (n Neo) ManipulationGetCreate(ctx context.Context, mp types.Manipulation) (mpID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationGetCreate",
	})
	defer span.End()
	_ = ctx
	_ = l
	// Check if manipulation exists
	if mpID, found, err := n.ManipulationGetIDByLabel(ctx, mp.Label); err != nil {
		return mpID, utils.ErrorHandler(l, span, err, FailManipulationGetID)
	} else if found {
		return mpID, nil
	}
	// Insert Manipulation
	if mpID, err = n.ManipulationCreate(ctx, mp); err != nil {
		return mpID, utils.ErrorHandler(l, span, err, FailManipulationCreate)
	}
	return mpID, nil
}

func (n Neo) ManipulationCreate(ctx context.Context, mp types.Manipulation) (mpID int64, err error) {
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
		return mpID, utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else {
		if mpID, err = res.GetInt64(key); err != nil {
			return mpID, utils.ErrorHandler(l, span, err, types.FailBrGet, key)
		}
	}
	utils.Debug(l, span, "Manipulation has %d layers", len(mp.ManipuLayers))
	for _, mpl := range mp.ManipuLayers {
		if mplID, err := n.ManipuLayerGetCreate(ctx, mpl); err != nil {
			return mpID, utils.ErrorHandler(l, span, err, FailManipuLayerGetCreate)
		} else {
			if err := n.ManipulationLinkToManipuLayer(ctx, mpID, mplID); err != nil {
				return mpID, utils.ErrorHandler(l, span, err, FailManipulationLinkToManipuLayer)
			}
		}

	}
	return mpID, nil
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
		return mp, errors.Wrap(e, FailManipuLayersGetByManipulationID)
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
		Label: m.Label,
	}
	for _, cfgMppl := range m.ManipuLayers {
		utils.Debug(l, span, "ManipuLayer: %s", cfgMppl.Image)
		up, e := types.NewUpstreamByName(cfgMppl.Image)
		if e != nil {
			return mp, utils.ErrorHandler(l, span, e, types.FailNewUpstreamByName)
		}
		if mpl, err := n.ManipuLayerFromUpstream(ctx, up, cfgMppl.Rule); err != nil {
			return mp, utils.ErrorHandler(l, span, err, FailManipuLayerFromUpstream)
		} else {
			mp.ManipuLayers = append(mp.ManipuLayers, mpl)
		}

	}
	return mp, nil
}

// ManipulationGetByProfileManiID returns a manipulation which is connected through rules to a profile and a manifest
func (n Neo) ManipulationUnwind(ctx context.Context, maniID, maniDataID, prfID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationUnwind",
	})
	defer span.End()
	////////   The rules are going to be matched against the ManiPulayer to figure out a match
	maniRls, e := n.RulesGetByManifestID(ctx, maniID)
	if e != nil {
		return utils.ErrorHandler(l, span, e, FailRulesGetByManifestID)
	}
	// Get Manipulations for Profile
	mps, err := n.ManipulationsGetByProfileID(ctx, prfID)
	if err != nil {
		return utils.ErrorHandler(l, span, err, FailManipulationsGetByProfileID)
	}
	utils.Warn(l, span, "Manipulations:  %d", len(mps))
	for _, mp := range mps {
		rls, err := n.RulesGetByManipuLayerID(ctx, mp.ID)
		if err != nil {
			return utils.ErrorHandler(l, span, err, FailRulesGetByManipuLayerID)
		}
		// Check if mpRules all be matched by maniRules
		if rls.SatisfiedBy(maniRls) {
			utils.Warn(l, span, "Manipulation %d (ManipuLayers: %d) is satisfied by Manifest %d", mp.ID, len(mp.ManipuLayers), maniID)
			for _, mpl := range mp.ManipuLayers {
				utils.Warn(l, span, "ManipuLayer: %s (%d Layers)", mpl.Image, len(mpl.Layers))
				for idx, lyr := range mpl.Layers {
					if e := n.ManifestData_LinkToLayer(ctx, maniDataID, lyr, idx); e != nil {
						return utils.ErrorHandler(l, span, e, FailManifestDataLinkToLayer)
					}
				}
			}
		} else {
			utils.Warn(l, span, "Manipulation %d is NOT satisfied by Manifest %d", mp.ID, maniID)
			continue
		}
	}

	return
}
