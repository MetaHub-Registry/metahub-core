package neo

import (
	"context"
	"fmt"
	"strings"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

const (
	FailManipuLayerFromResult           = "fail to create ManipuLayer from result"
	pkgNameManipuLayer                  = "neo/manipulayer.go"
	// ManipuLayerReturn is the column alias set used by every
	// ManipuLayer-returning query. `mplSelect` is coalesced to ""
	// so queries against older nodes (which predate the `select`
	// property) keep working.
	ManipuLayerReturn = "id(mpl) AS mplID, mpl.label AS mplLabel, mpl.rule AS mplRule, coalesce(mpl.select, \"\") AS mplSelect"
	FailManipuLayerGetByLabel           = "fail to get ManipuLayer by label"
	FailManipuLayerGetCreate            = "fail to get or create ManipuLayer"
	FailManipuLayerFromUpstream         = "fail to create ManipuLayer from upstream"
	FailManipuLayerLinkToUpstream       = "fail to link ManipuLayer to upstream"
	FailManipuLayersGetByManipulationID = "fail to get ManipuLayers by manipulation ID"
	FailManipuLayerLinkToRule           = "fail to link ManipuLayer to rule"
	FailManipuLayerCreate               = "fail to create ManipuLayer"
	FailManipuLayerFromConfigYAML       = "fail to create ManipuLayer from config YAML"
)

func (n Neo) ManipuLayerCreate(ctx context.Context, mpl *types.ManipuLayer) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerCreate",
	})
	defer span.End()
	_ = ctx
	key := "mplID"
	// Escape the slice expression defensively — users can put
	// literal "[" and ":" etc. in there, and while the current
	// ParseLayerSlice grammar rejects quotes, persisting raw avoids
	// any future surprise if the grammar broadens.
	selectEsc := strings.ReplaceAll(mpl.Select, `"`, `\"`)
	q := fmt.Sprintf(
		`CREATE (mpl:ManipuLayer {label: "%s", rule: "%s", select: "%s"}) RETURN id(mpl) AS %s`,
		mpl.Label, mpl.Rule, selectEsc, key,
	)
	utils.Debug(l, span, q)
	if res, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else {
		if mplID, err := res.GetInt64(key); err != nil {
			return utils.ErrorHandler(l, span, err, types.FailBrGet, key)
		} else {
			mpl.ID = mplID
		}
	}
	// Add Cmd
	utils.Debug(l, span, "ManipuCmd: %d", len(mpl.Cmd))
	for _, mpc := range mpl.Cmd {
		if e := n.ManipuCmdInsert(ctx, mpl.ID, &mpc); e != nil {
			return utils.ErrorHandler(l, span, e, FailManipuCmdCreate)
		}
	}
	// Add Environment
	utils.Debug(l, span, "ManipuEnv: %d", len(mpl.Environment))
	for _, mpe := range mpl.Environment {
		if e := n.ManipuEnvInsert(ctx, mpl.ID, &mpe); e != nil {
			return utils.ErrorHandler(l, span, e, FailManipuEnvInsert)
		}
	}
	// Connect to Manifest
	if md, found, err := n.ManifestGetByDigest(ctx, mpl.Image.Digest); err != nil {
		return utils.ErrorHandler(l, span, err, FailManifestGetByDigest)
	} else if !found {
		return utils.NewError(l, span, "failed to get Manifest: %s", mpl.Image.Digest)
	} else {
		if err = n.ManifestLinkToManipulayer(ctx, md.ID, mpl.ID); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipuLayerLinkToManifest)
		}
	}
	// Connect to Rule
	rLabel := strings.TrimSuffix(mpl.Rule, "=true")
	utils.Debug(l, span, "RuleGetByLabel: %s", rLabel)
	if rl, found, err := n.RuleGetByLabel(ctx, rLabel); err != nil {
		return utils.ErrorHandler(l, span, err, FailRuleGetByLabel)
	} else if !found {
		return utils.NewError(l, span, "failed to get Rule: %s", rLabel)
	} else {
		if err = n.ManipuLayerLinkToRule(ctx, mpl.ID, rl.ID); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipuLayerLinkToRule)
		}
	}

	return nil
}

func (n Neo) ManipuLayerGetCreate(ctx context.Context, mpl *types.ManipuLayer) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerGetCreate",
	})
	defer span.End()
	if mplGot, found, e := n.ManipuLayerGetByLabel(ctx, mpl.Label); e != nil {
		return utils.ErrorHandler(l, span, e, FailManipuLayerGetByLabel)
	} else if found {
		utils.Debug(l, span, "ManipuLayer found: %d", mplGot.ID)
		mpl.ID = mplGot.ID
		return nil
	} else {
		if err := n.ManipuLayerCreate(ctx, mpl); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipuLayerCreate)
		}
		return
	}
}
func (n Neo) ManipuLayersGetByManipulationID(ctx context.Context, mpID int64) (mpls []types.ManipuLayer, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerGetByManipulationID",
		"mpID":    mpID,
	})
	defer span.End()
	q := fmt.Sprintf(`MATCH (mp:Manipulation)-[]->(mpl:ManipuLayer) WHERE id(mp) = %d RETURN  %s`, mpID, ManipuLayerReturn)
	utils.Debug(l, span, q)
	if res, err := n.Query(q); err != nil {
		return mpls, utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else {
		for _, r := range res {
			if mpl, err := n.ManipuLayerFromResult(ctx, r); err != nil {
				return mpls, utils.ErrorHandler(l, span, err, FailManipuLayerFromResult)
			} else {
				mpls = append(mpls, mpl)
			}
		}
	}
	return
}

func (n Neo) ManipuLayerGetByLabel(ctx context.Context, label string) (mpl types.ManipuLayer, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerGetByLabel",
	})
	defer span.End()

	q := fmt.Sprintf(`MATCH (mpl:ManipuLayer) WHERE mpl.label = "%s"`, label)
	q += fmt.Sprintf(` RETURN  %s`, ManipuLayerReturn)
	utils.Debug(l, span, q)
	if res, found, err := n.QueryOne(q); err != nil {
		return mpl, false, utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else if !found {
		return mpl, false, nil
	} else {
		if mpl, err = n.ManipuLayerFromResult(ctx, res); err != nil {
			return mpl, false, utils.ErrorHandler(l, span, err, FailManipuLayerFromResult)
		}

	}
	return mpl, true, nil
}

func (n Neo) ManipuLayerFromResult(ctx context.Context, res types.Result) (mpl types.ManipuLayer, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerFromResult",
	})
	defer span.End()
	err = res.CheckKeys("mplID", "mplRule", "mplLabel")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("mplID")
	label, _ := res.GetStr("mplLabel")
	rule, _ := res.GetStr("mplRule")
	// `mplSelect` is optional so older nodes (pre add-manipulayer-select)
	// still round-trip — missing key is handled by CheckKeys above so
	// we can read it safely without another CheckKeys call.
	sel, _ := res.GetStr("mplSelect")
	md, err := n.ManifestForManipuLayerID(ctx, id)
	if err != nil {
		return mpl, utils.ErrorHandler(l, span, err, FailGetLayersForManipuLayerID)
	}
	mpcs, err := n.ManipuCmdsGetByManipulayerID(ctx, id)
	if err != nil {
		return mpl, utils.ErrorHandler(l, span, err, FailManipuCmdsGetByManipulayerID)
	}
	mpes, err := n.ManipuEnvsGetByManipulayerID(ctx, id)
	if err != nil {
		return mpl, utils.ErrorHandler(l, span, err, FailManipuEnvsGetByManipulayerID)
	}

	mpl = types.ManipuLayer{
		ID:          id,
		Label:       label,
		Rule:        rule,
		Select:      sel,
		Image:       md,
		Cmd:         mpcs,
		Environment: mpes,
	}

	return mpl, nil
}

func (n Neo) ManipuLayerLinkToUpstream(ctx context.Context, mplID, upID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerLinkToUpstream",
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (mpl:ManipuLayer), (up:Upstream) WHERE id(mpl) = %d AND id(up) = %d MERGE (mpl)-[:UPSTREAM]->(up)`, mplID, upID)
	utils.Debug(l, span, q)
	if _, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	return nil
}

func (n Neo) ManipuLayerLinkToLayer(ctx context.Context, mplID, lyrID int64, mediaType string, idx int) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerLinkToLayer",
		"mplID":   mplID,
		"lyrID":   lyrID,
		"order":   idx,
	})
	defer span.End()
	_ = ctx
	rel := utils.ShortenMediaType(mediaType)
	q := fmt.Sprintf(`MATCH (mpl:ManipuLayer), (l:Blob) WHERE id(mpl) = %d AND id(l) = %d`, mplID, lyrID)
	q += fmt.Sprintf(` MERGE (mpl)-[:LayerLink {mediaType: "%s", order: %d}]->(l)`, rel, idx)
	utils.Debug(l, span, q)
	if _, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	return nil
}

func (n Neo) ManipuLayerFromConfigYAML(ctx context.Context, cmpl config.ManipuLayer) (mpl types.ManipuLayer, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerFromConfigYAML",
	})
	defer span.End()
	up, err := types.NewUpstreamByName(cmpl.Image)
	if err != nil {
		return mpl, utils.ErrorHandler(l, span, err, types.FailNewUpstreamByName)
	}
	return n.ManipuLayerFromUpstream(ctx, up, cmpl)
}

func (n Neo) ManipuLayerFromUpstream(ctx context.Context, up types.Upstream, cfgMpl config.ManipuLayer) (mpl types.ManipuLayer, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerFromUpstream",
	})
	defer span.End()
	if upID, found, err := n.UpstreamGet(ctx, up); err != nil {
		return mpl, utils.ErrorHandler(l, span, err, FailUpstreamGet)
	} else if !found {
		return mpl, utils.NewError(l, span, FailUpstreamNotFound)
	} else {
		up.ID = upID
	}
	md, found, err := n.ManifestGetByUpstream(ctx, up)
	if err != nil {
		return mpl, utils.ErrorHandler(l, span, err, FailManifestGetByUpstream)
	} else if !found {
		return mpl, utils.NewError(l, span, FailManifestNotFound)
	}
	mpes := make([]types.ManipuEnv, 0)
	for _, e := range cfgMpl.Environment {
		mpes = append(mpes, types.ManipuEnv{
			Key:  e.Key,
			Rule: e.Rule,
		})
	}
	mpcs := make([]types.ManipuCmd, 0)
	for _, c := range cfgMpl.Cmd {
		mpcs = append(mpcs, types.ManipuCmd{
			Cmd:  c.Value,
			Rule: c.Rule,
		})
	}
	mpl = types.ManipuLayer{
		Label:       up.GetLabel(),
		Rule:        cfgMpl.Rule,
		Select:      cfgMpl.Select,
		Image:       md,
		Environment: mpes,
		Cmd:         mpcs,
	}

	err = n.ManipuLayerGetCreate(ctx, &mpl)
	if err != nil {
		return mpl, utils.ErrorHandler(l, span, err, FailManipuLayerGetByLabel)
	}
	if err = n.ManipuLayerLinkToUpstream(ctx, mpl.ID, up.ID); err != nil {
		return mpl, utils.ErrorHandler(l, span, err, FailManipuLayerLinkToUpstream)
	}
	return mpl, nil
}

func (n Neo) ManipuLayerLinkToRule(ctx context.Context, mplID, ruleID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerLinkToRule",
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (mpl:ManipuLayer), (r:Rule) WHERE id(mpl) = %d AND id(r) = %d MERGE (mpl)-[:USES]->(r)`, mplID, ruleID)
	utils.Debug(l, span, q)
	if _, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	return nil
}
