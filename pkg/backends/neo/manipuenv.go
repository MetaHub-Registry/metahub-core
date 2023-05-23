package neo

import (
	"context"
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameManipuEnv                 = "neo/manipuev.go"
	returnManipuEnv                  = `id(mpe) AS mpeID, mpe.rule AS mpeRule, mpe.key AS mpeKey`
	FailManipuEnvInsert              = "failed to insert ManipuEnv"
	FailManipuEnvCreate              = "failed to create ManipuEnv"
	FailManipuEnvGet                 = "failed to get ManipuEnv"
	FailManipuEnvFromResult          = "failed to get ManipuEnv from result"
	ManipuEnvRuleIfNULL              = "ifNULL"
	FailManipuEnvsGetByManipulayerID = "failed to get ManipuEnvs by ManipuLayerID"
)

func (n Neo) ManipuEnvInsert(ctx context.Context, mplID int64, mpe *types.ManipuEnv) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuEnv,
		"func":    "ManipuEnvInsert",
	})
	defer span.End()
	if ok, e := n.ManipuEnvGet(ctx, mpe); e != nil {
		return utils.ErrorHandler(l, span, e, FailManipuEnvGet)
	} else if ok {
		return nil
	} else {
		if e := n.ManipuEnvCreate(ctx, mplID, mpe); e != nil {
			return utils.ErrorHandler(l, span, e, FailManipuEnvCreate)
		}
	}
	return nil
}

func (n Neo) ManipuEnvCreate(ctx context.Context, mplID int64, mpe *types.ManipuEnv) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuEnv,
		"func":    "ManipuEnvCreate",
	})
	defer span.End()
	_ = ctx
	key := "mpeID"
	q := fmt.Sprintf(`CREATE (mpe:ManipuEnv {rule: "%s", key: "%s"}) RETURN id(mpe) AS %s`, mpe.Rule, utils.Base64Encode(mpe.Key), key)
	utils.Debug(l, span, q)
	if res, err := n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else {
		mpeID, err := res.GetInt64(key)
		if err != nil {
			return utils.ErrorHandler(l, span, err, types.FailBrGet, key)
		}
		mpe.ID = mpeID
		q := `MATCH (mpl:ManipuLayer),(mpe:ManipuEnv)`
		q += fmt.Sprintf(` WHERE id(mpl) = %d AND id(mpe) = %d MERGE (mpl)-[:ENV_%s]->(mpe)`, mplID, mpeID, mpe.Rule)
		utils.Debug(l, span, q)
		if _, err := n.Execute(q); err != nil {
			return utils.ErrorHandler(l, span, err, FailNeoExecute)
		}
	}
	return
}

func (n Neo) ManipuEnvGet(ctx context.Context, mpe *types.ManipuEnv) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameManipuEnv,
		"func":       "ManipuEnvGet",
		"rule":       mpe.Rule,
		"keyEncoded": mpe.Key,
		"key":        utils.Base64Decode(mpe.Key),
	})
	defer span.End()
	_ = ctx
	key := "mpeID"
	q := fmt.Sprintf(`MATCH (mpe:ManipuEnv) WHERE mpe.rule = "%s" AND mpe.key = "%s" RETURN id(mpe) AS %s`, mpe.Rule, utils.Base64Encode(mpe.Key), key)
	utils.Debug(l, span, q)
	if res, found, err := n.QueryOne(q); err != nil {
		return false, utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else if !found {
		return false, nil
	} else {
		mpeID, err := res.GetInt64(key)
		if err != nil {
			return false, utils.ErrorHandler(l, span, err, types.FailBrGet, key)
		}
		mpe.ID = mpeID
		return true, nil
	}
}

func (n Neo) ManipuEnvsGetByManipulayerID(ctx context.Context, mplyrID int64) (mpes []types.ManipuEnv, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipuLayerCmd,
		"func":    "ManipuEnvsGetByManipulayerID",
		"mplyrID": mplyrID,
	})
	defer span.End()
	_ = ctx
	_ = l
	q := fmt.Sprintf(`MATCH (mpe:ManipuEnv)<-[]-(mpl:ManipuLayer) WHERE id(mpl) = %d RETURN %s`, mplyrID, returnManipuEnv)
	utils.Debug(l, span, q)
	res, err := n.Query(q)
	if err != nil {
		return mpes, utils.ErrorHandler(l, span, err, FailNeoExecute)
	} else {
		for _, r := range res {
			mpe, err := ManipuEnvFromResult(r)
			if err != nil {
				return mpes, utils.ErrorHandler(l, span, err, FailManipuEnvFromResult)
			}
			mpes = append(mpes, mpe)
		}
	}
	return
}

func ManipuEnvFromResult(res types.Result) (mpe types.ManipuEnv, err error) {
	err = res.CheckKeys("mpeID", "mpeRule", "mpeKey")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("mpeID")
	mpeRule, _ := res.GetStr("mpeRule")
	mpeKey, _ := res.GetStr("mpeKey")
	mpe = types.ManipuEnv{
		ID:   id,
		Rule: mpeRule,
		Key:  mpeKey,
	}
	return
}
