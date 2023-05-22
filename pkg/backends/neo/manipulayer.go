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
	pkgNameManipuLayer                  = "neo/manipulation_layer.go"
	ManipuLayerReturn                   = "id(mpl) AS mplID, mpl.label AS mplLabel, mpl.rule AS mplRule"
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
	q := fmt.Sprintf(`CREATE (mpl:ManipuLayer {label: "%s", rule: "%s"}) RETURN id(mpl) AS %s`, mpl.Label, mpl.Rule, key)
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
	// Connect to ManifestData
	if md, found, err := n.ManifestDataGetByDigest(ctx, mpl.Image.Digest); err != nil {
		return utils.ErrorHandler(l, span, err, FailManifestDataGetByDigest)
	} else if !found {
		return utils.NewError(l, span, "failed to get ManifestData: %s", mpl.Image.Digest)
	} else {
		if err = n.ManifestDataLinkToManipulayer(ctx, md.ID, mpl.ID); err != nil {
			return utils.ErrorHandler(l, span, err, FailManipuLayerLinkToManifestData)
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
	md, err := n.ManifestDataForManipuLayerID(ctx, id)
	if err != nil {
		return mpl, utils.ErrorHandler(l, span, err, FailGetLayersForManipuLayerID)
	}
	mpl = types.ManipuLayer{
		ID:    id,
		Label: label,
		Rule:  rule,
		Image: md,
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

func (n Neo) ManipuLayerLinkToLayer(ctx context.Context, mplID, lyrID int64, idx int) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerLinkToLayer",
		"mplID":   mplID,
		"lyrID":   lyrID,
		"order":   idx,
	})
	defer span.End()
	_ = ctx
	rel := ""
	if lyr, found, err := n.Layer_Get_ByID(ctx, lyrID); err != nil || !found {
		return utils.ErrorHandler(l, span, err, FailLayerGetByID)
	} else {
		rel = utils.ShortenMediaType(lyr.MediaType)
	}
	q := fmt.Sprintf(`MATCH (mpl:ManipuLayer), (l:Layer) WHERE id(mpl) = %d AND id(l) = %d`, mplID, lyrID)
	q += fmt.Sprintf(` MERGE (mpl)-[:%s {order: %d}]->(l)`, rel, idx)
	utils.Debug(l, span, q)
	if _, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	}
	return nil
}

func (n Neo) ManipuLayerFromConfigYAML(ctx context.Context, cmpl config.ManipuLayer) (mpl types.ManipuLayer, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayer,
		"func":    "ManipuLayerFromUpstream",
	})
	defer span.End()
	up, err := types.NewUpstreamByName(cmpl.Image)
	if err != nil {
		return mpl, utils.ErrorHandler(l, span, err, types.FailNewUpstreamByName)
	}
	return n.ManipuLayerFromUpstream(ctx, up, cmpl.Rule)
}

func (n Neo) ManipuLayerFromUpstream(ctx context.Context, up types.Upstream, rule string) (mpl types.ManipuLayer, err error) {
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
	md, found, err := n.ManifestDataGetByUpstream(ctx, up)
	if err != nil {
		return mpl, utils.ErrorHandler(l, span, err, FailManifestDataGetByUpstream)
	} else if !found {
		return mpl, utils.NewError(l, span, FailManifestDataNotFound)
	}
	mpl = types.ManipuLayer{
		Label: up.GetLabel(),
		Rule:  rule,
		Image: md,
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
