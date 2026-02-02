package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
)

const (
	pkgNameUp              = "neo/upstream.go"
	UpstreamReturn         = `id(u) AS upstreamID, u.reg AS upstreamReg, u.repo AS upstreamRepo, u.tag AS upstreamTag, u.digest AS upstreamDigest`
	FailUpstreamFromResult = `failed to get upstream from result`
	FailUpstreamGet        = `failed to get upstream by oject (reg/repo/tag)`
	FailUpstreamCreate     = `failed to create upstream`
	FailUpstreamNotFound   = `upstream not found`
	FailFoundUpstream      = "found upstream w/ ID %d"
)

func (n Neo) UpstreamGetInsert(ctx context.Context, up *types.Upstream) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameUp,
		"func":     "UpstreamGetInsert",
		"upstream": up.String(),
	})
	_ = ctx
	defer span.End()
	utils.Debug(l, span, "Getting Upstream: %s", up.String())
	upID, found, err := n.UpstreamGet(ctx, *up)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, FailUpstreamGet)
		return
	}
	if !found {
		utils.Debug(l, span, "upstream not found")
		upID, err = n.CreateUpstream(ctx, *up)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, FailUpstreamCreate)
			return
		}
	}
	up.ID = upID
	utils.Debug(l, span, "Upstream ID: %d", up.ID)
	return
}

func (n Neo) UpstreamGetInsertByRequest(ctx context.Context, req types.Request) (up types.Upstream, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameUp,
		"func":     "UpstreamGetInsertByRequest",
		"upstream": up.String(),
	})
	_ = l
	defer span.End()
	up = types.Upstream{
		Reg:    req.Host,
		Repo:   req.Repo,
		Tag:    req.Tag,
		Digest: digest.Digest(req.Digest),
	}
	found, err = n.UpstreamGetInsert(ctx, &up)
	return
}

func (n Neo) UpstreamGetByRequest(ctx context.Context, req types.Request) (up types.Upstream, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameUp,
		"func":    "UpstreamGetByRequest",
		"host":    req.Host,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	_ = l
	ctx, span := tracer.Start(ctx, "UpstreamGetByRequest")
	defer span.End()
	span.SetAttributes(
		attribute.Key("host").String(req.Host),
		attribute.Key("repo").String(req.Repo),
		attribute.Key("tag").String(req.Tag),
		attribute.Key("digest").String(utils.ShortenDigest(req.Digest)),
	)
	up = types.Upstream{
		Reg:    req.Host,
		Repo:   req.Repo,
		Tag:    req.Tag,
		Digest: digest.Digest(req.Digest),
	}
	up.ID, found, err = n.UpstreamGet(ctx, up)
	if !found {
		return
	}
	up, found, err = n.UpstreamGetByID(ctx, up.ID)
	return
}

func (n Neo) UpstreamGetByManifestDataID(ctx context.Context, maniDataID int64) (up types.Upstream, relType string, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "UpstreamGetByManifestDataID",
		"maniDataID": maniDataID,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[r]->(u:Upstream) WHERE id(md) = %d`, maniDataID)
	q += ` RETURN type(r) AS relationType,` + UpstreamReturn
	utils.Debug(l, span, q)
	res, found, e := n.QueryOne(q)
	if e != nil {
		return up, relType, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else if !found {
		utils.Debug(l, span, "did not found upstream connected to maniID %d", maniDataID)
		return up, relType, false, nil
	}
	up, e = UpstreamFromResult(res)
	if e != nil {
		return up, relType, false, utils.ErrorHandler(l, span, e, FailUpstreamFromResult)
	}
	relType, err = res.GetStr("relationType")
	if err != nil {
		return up, relType, false, utils.ErrorHandler(l, span, err, types.FailBrGet, "relationType")
	}
	return
}

func (n Neo) UpstreamGetIDByManifestID(ctx context.Context, maniID int64) (upID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "UpstreamGetIDByManifestID",
		"maniID":  maniID,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (m:Manifest)-[]->(up:Upstream) WHERE id(m) = %d`, maniID)
	q += ` RETURN id(up) AS ID`
	utils.Debug(l, span, q)
	if res, found, e := n.QueryOne(q); e != nil {
		return upID, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else {
		if !found {
			utils.Debug(l, span, "did not found upstream connected to maniID %d", maniID)
			return upID, false, nil
		}
		if upID, e = res.GetInt64("ID"); e != nil {
			return upID, false, utils.ErrorHandler(l, span, e, types.FailBrGet, "ID")
		} else {
			utils.Debug(l, span, FailFoundUpstream, upID)

			return upID, true, nil
		}
	}
}

func (n Neo) UpstreamGetIDByManifestDataID(ctx context.Context, maniDataID int64) (upID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNameUp,
		"func":       "UpstreamGetIDByManifestDataID",
		"maniDataID": maniDataID,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[]->(up:Upstream) WHERE id(md) = %d`, maniDataID)
	q += ` RETURN id(up) AS ID`
	utils.Debug(l, span, q)
	if res, found, e := n.QueryOne(q); e != nil {
		return upID, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else {
		if !found {
			utils.Debug(l, span, "did not found upstream connected to maniDataID %d", maniDataID)
			return upID, false, nil
		}
		if upID, e = res.GetInt64("ID"); e != nil {
			return upID, false, utils.ErrorHandler(l, span, e, types.FailBrGet, "ID")
		} else {
			utils.Debug(l, span, FailFoundUpstream, upID)

			return upID, true, nil
		}
	}
}

func (n Neo) UpstreamGetIDByDigest(ctx context.Context, dig string) (upID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "UpstreamGetIDByDigest",
		"digest":  dig,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (up:Upstream) WHERE up.digest = "%s"`, dig)
	q += ` RETURN id(up) AS ID`
	utils.Debug(l, span, q)
	if res, found, e := n.QueryOne(q); e != nil {
		return upID, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else {
		if !found {
			utils.Debug(l, span, "did not found upstream with digest")
			return upID, false, nil
		}
		if upID, e = res.GetInt64("ID"); e != nil {
			return upID, false, utils.ErrorHandler(l, span, e, types.FailBrGet, "ID")
		} else {
			utils.Debug(l, span, FailFoundUpstream, upID)

			return upID, true, nil
		}
	}
}

func (n Neo) UpstreamGetByManipuLayerID(ctx context.Context, mplID int64) (up types.Upstream, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":       pkgNameUp,
		"func":          "UpstreamGetByManipuLayerID",
		"manipuLayerID": mplID,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (mpl:ManipuLayer)-[]->(u:Upstream) WHERE id(mpl) = %d`, mplID)
	q += fmt.Sprintf(` RETURN %s`, UpstreamReturn)
	utils.Debug(l, span, q)
	if res, found, e := n.QueryOne(q); e != nil {
		return up, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else {
		if !found {
			utils.Debug(l, span, "did not found upstream connected to ManipuLayerID %d", mplID)
			return up, false, nil
		}
		up, err = UpstreamFromResult(res)
		if err != nil {
			logrus.Errorf("Failed to get upstream: %s", err)
			return up, false, utils.ErrorHandler(l, span, err, FailUpstreamFromResult)
		}
		return up, true, nil
	}
}

func (n Neo) UpstreamCleanupByID(ctx context.Context, upID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "UpstreamCleanupByID",
		"upID":    upID,
	})
	_ = ctx
	defer span.End()
	key := "countUpstream"
	q := fmt.Sprintf(`MATCH (m:Manifest)-[]->(up:Upstream) WHERE id(up) = %d`, upID)
	q += fmt.Sprintf(` RETURN count(up) AS %s`, key)
	countUpstream := -1
	if res, _, e := n.QueryOne(q); e != nil {
		return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else {
		if countUpstream, e = res.GetInt(key); e != nil {
			return utils.ErrorHandler(l, span, e, types.FailBrGet, key)
		}
	}
	utils.Debug(l, span, "Upstream has '%d' relationships to Manifests", countUpstream)
	if countUpstream == 0 {
		q := fmt.Sprintf(`MATCH (up:Upstream) WHERE id(up) = %d DETACH DELETE up`, upID)
		if _, e := n.Execute(q); e != nil {
			return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
		}
	}
	return
}

func (n Neo) UpstreamGetByID(ctx context.Context, upID int64) (up types.Upstream, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "UpstreamGetByID",
		"upID":    upID,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (u:Upstream) WHERE id(u) = %d`, upID)
	q += ` RETURN ` + UpstreamReturn
	utils.Debug(l, span, q)
	res, err := n.Execute(q)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if len(res.Strings) == 0 {
		found = false
		return
	}
	found = true
	up, err = UpstreamFromResult(res)
	if err != nil {
		logrus.Errorf("Failed to get upstream: %s", err)
		return
	}
	return
}

func (n Neo) UpstreamGet(ctx context.Context, up types.Upstream) (upID int64, success bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "UpstreamGet",
	})
	defer span.End()
	_ = ctx
	q := `MATCH (u:Upstream)`
	q += fmt.Sprintf(` WHERE u.reg = "%s" AND u.repo = "%s" AND u.tag = "%s"`, up.Reg, up.Repo, up.Tag)
	if up.Digest.String() != "" {
		q += fmt.Sprintf(` AND u.digest = "%s"`, up.Digest.String())
	}
	q += ` RETURN id(u) AS ID, u.digest AS upstreamDigest`
	utils.Debug(l, span, q)
	if res, found, e := n.QueryOne(q); e != nil {
		return upID, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else if !found {
		utils.Debug(l, span, "did not found upstream: %s", up.String())
		return upID, false, nil
	} else {
		if upID, e = res.GetInt64("ID"); e != nil {
			return upID, false, utils.ErrorHandler(l, span, e, types.FailBrGet, "ID")
		}
	}
	utils.Debug(l, span, FailFoundUpstream, upID)
	return upID, true, nil
}

func (n Neo) CreateUpstream(ctx context.Context, up types.Upstream) (upID int64, err error) {
	label := fmt.Sprintf("%s/%s:%s", up.Reg, up.Repo, up.Tag)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "CreateUpstream",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	defer span.End()
	if up.Digest.String() == "" {
		err = utils.ErrorHandler(l, span, err, "CreateUpstream: digest is empty")
		return
	}
	q := fmt.Sprintf(`CREATE (u:Upstream {reg: "%s", repo: "%s", tag: "%s", label: "%s"`, up.Reg, up.Repo, up.Tag, label)
	if up.Digest.String() != "" {
		q += fmt.Sprintf(`,digest: "%s"`, up.Digest.String())
	}
	q += `}) RETURN id(u) AS ID`
	utils.Debug(l, span, q)
	res, err := n.Execute(q)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "Failed to run query")
		return
	}
	upID, err = res.GetInt64("ID")
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "CreateUpstream: failed to get ID")
		return
	}
	utils.Debug(l, span, "upstreamID: %d", upID)
	return
}

func UpstreamFromResult(res types.Result) (u types.Upstream, err error) {
	err = res.CheckKeys("upstreamID", "upstreamReg", "upstreamRepo", "upstreamTag", "upstreamDigest")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("upstreamID")
	reg, _ := res.GetStr("upstreamReg")
	repo, _ := res.GetStr("upstreamRepo")
	tag, _ := res.GetStr("upstreamTag")
	dig, _ := res.GetStr("upstreamDigest")
	mt, _ := res.GetStr("upstreamMediaType")
	u = types.Upstream{
		ID:        id,
		Reg:       reg,
		Repo:      repo,
		Tag:       tag,
		MediaType: utils.ExpandMediaType(mt),
		Digest:    digest.Digest(dig),
	}
	return
}

func UpstreamFromRecord(res neo4j.Result) (u types.Upstream, err error) {
	r := types.NewResult()
	record := res.Record()
	keys, err := res.Keys()
	if err != nil {
		logrus.Errorf("Error getting keys: %s\n", err)
		return
	}
	for idx, key := range keys {
		v := record.Values[idx]
		err = r.Add(key, v)
		if err != nil {
			logrus.Errorf("Error adding key: %s\n", err)
			return
		}
	}
	return UpstreamFromResult(r)
}

func (n Neo) UpstreamGetByLayerDigest(ctx context.Context, dig string) (up types.Upstream, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUp,
		"func":    "mag.UpstreamGetByLayerDigest",
		"digest":  dig,
	})
	defer span.End()
	_ = l
	_ = ctx
	q := `MATCH (l:Blob)<-[r]-(md:ManifestData)-[:ORIGINAL]->(u:Upstream)`
	q += fmt.Sprintf(` WHERE l.digest = "%s" RETURN l.mediaType AS upstreamMediaType, %s`, dig, UpstreamReturn)
	if res, found, err := n.QueryOne(q); err != nil {
		err = utils.ErrorHandler(l, span, err, FailNeoExecute)
		return up, false, err
	} else if !found {
		return up, false, nil
	} else {
		up, err = UpstreamFromResult(res)
		if err != nil {
			return up, false, utils.ErrorHandler(l, span, err, FailNeoExecute)
		}
	}
	return up, true, nil
}
