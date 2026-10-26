package neo

import (
	"context"
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameManipuLayerCmd            = "neo/manipulayer_cmd.go"
	returnManipuCmd                  = `id(mpc) AS mpcID, mpc.rule AS mpcRule, mpc.cmd AS mpcCmd`
	FailManipuCmdCreate              = "failed to create ManipuCmd"
	FailManipuCmdsGetByManipulayerID = "failed to get ManipuCmds by ManipuLayerID"
	ManipuCmdRulePrepend             = "prepend"
	ManipuCmdRuleAppend              = "append"
	ManipuCmdRuleReplace             = "replace"
)

func (n Neo) ManipuCmdInsert(ctx context.Context, mplID int64, mpc *types.ManipuCmd) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayerCmd,
		"func":    "ManipuCmdInsert",
	})
	defer span.End()
	if ok, e := n.ManipuCmdGet(ctx, mpc); e != nil {
		return utils.ErrorHandler(l, span, e, FailManipuCmdCreate)
	} else if ok {
		return nil
	} else {
		if e := n.ManipuCmdCreate(ctx, mplID, mpc); e != nil {
			return utils.ErrorHandler(l, span, e, FailManipuCmdCreate)
		}
	}
	return nil
}

func (n Neo) ManipuCmdCreate(ctx context.Context, mplID int64, mpc *types.ManipuCmd) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayerCmd,
		"func":    "ManipuLayerCmdCreate",
	})
	defer span.End()
	_ = ctx
	key := "mpcID"
	q := fmt.Sprintf(`CREATE (mpc:ManipuCmd {rule: "%s", cmd: "%s"}) RETURN id(mpc) AS %s`, mpc.Rule, utils.Base64Encode(mpc.Cmd), key)
	utils.Debug(l, span, "%s", q)
	if res, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else {
		mpcID, err := res.GetInt64(key)
		if err != nil {
			return utils.ErrorHandler(l, span, err, types.FailBrGet, key)
		}
		mpc.ID = mpcID
		q := `MATCH (mpl:ManipuLayer),(mpc:ManipuCmd)`
		q += fmt.Sprintf(` WHERE id(mpl) = %d AND id(mpc) = %d MERGE (mpl)-[:CMD_%s]->(mpc)`, mplID, mpcID, mpc.Rule)
		utils.Debug(l, span, "%s", q)
		if _, err := n.Execute(q); err != nil {
			return utils.ErrorHandler(l, span, err, FailNeoExecute)
		}
	}
	return
}

func (n Neo) ManipuCmdGet(ctx context.Context, mpc *types.ManipuCmd) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayerCmd,
		"func":    "ManipuCmdGet",
		"rule":    mpc.Rule,
		"cmd":     mpc.Cmd,
	})
	defer span.End()
	_ = ctx
	key := "mpcID"
	q := fmt.Sprintf(`MATCH (mpc:ManipuCmd) WHERE mpc.rule = "%s" AND mpc.cmd = "%s" RETURN id(mpc) AS %s`, mpc.Rule, utils.Base64Encode(mpc.Cmd), key)
	utils.Debug(l, span, "%s", q)
	if res, found, err := n.QueryOne(q); err != nil {
		return false, utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else if !found {
		return false, nil
	} else {
		mpcID, err := res.GetInt64(key)
		if err != nil {
			return false, utils.ErrorHandler(l, span, err, types.FailBrGet, key)
		}
		mpc.ID = mpcID
		return true, nil
	}
}

func (n Neo) ManipuCmdsGetByManipulayerID(ctx context.Context, mplyrID int64) (mpcs []types.ManipuCmd, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayerCmd,
		"func":    "ManipuCmdsGetByManipulayerID",
		"mplyrID": mplyrID,
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (mpc:ManipuCmd)<-[]-(mpl:ManipuLayer) WHERE id(mpl) = %d RETURN %s`, mplyrID, returnManipuCmd)
	utils.Debug(l, span, "%s", q)
	res, err := n.Query(q)
	if err != nil {
		return mpcs, utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else {
		for _, r := range res {
			mpc, err := ManipuCmdFromResult(r)
			if err != nil {
				return mpcs, utils.ErrorHandler(l, span, err, FailManipuEnvFromResult)
			}
			mpcs = append(mpcs, mpc)
		}
	}
	return
}

func ManipuCmdFromResult(res types.Result) (mpc types.ManipuCmd, err error) {
	err = res.CheckKeys("mpcID", "mpcRule", "mpcCmd")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("mpcID")
	mpcRule, _ := res.GetStr("mpcRule")
	mpcCmd, _ := res.GetStr("mpcCmd")
	mpc = types.ManipuCmd{
		ID:   id,
		Rule: mpcRule,
		Cmd:  mpcCmd,
	}
	return
}
