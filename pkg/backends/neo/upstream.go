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
	"go.opentelemetry.io/otel/trace"
)

var UpstreamReturn = `id(u) AS upstreamID, u.reg AS upstreamReg, u.repo AS upstreamRepo, u.tag AS upstreamTag, u.digest AS upstreamDigest`

func (n Neo) Upstream_GetInsert(ctx context.Context, up *types.Upstream) (found bool, err error) {
	ctx, span := tracer.Start(ctx, "Upstream_GetInsert")
	defer span.End()
	span.SetAttributes(
		attribute.Key("upstream").String(up.String()),
	)
	upID, found, err := n.GetUpstream(ctx, *up)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !found {
		upID, err = n.CreateUpstream(ctx, *up)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	up.ID = upID
	return
}

func (n Neo) Upstream_GetInsert_ByRequest(ctx context.Context, req types.Request) (up types.Upstream, found bool, err error) {
	up = types.Upstream{
		Reg:    req.Host,
		Repo:   req.Repo,
		Tag:    req.Tag,
		Digest: digest.Digest(req.Digest),
	}
	found, err = n.Upstream_GetInsert(ctx, &up)
	return
}

func (n Neo) Upstream_Get_ByRequest(ctx context.Context, req types.Request) (up types.Upstream, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/upstream.go",
		"func":    "Upstream_Get_ByRequest",
		"host":    req.Host,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	_ = l
	ctx, span := tracer.Start(ctx, "Upstream_Get_ByRequest")
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
	up.ID, found, err = n.Upstream_Get(ctx, up)
	if !found {
		return
	}
	up, err = n.Upstream_Get_ByID(ctx, up.ID)
	return
}

func (n Neo) Upstream_Get_ByID(ctx context.Context, upID int64) (up types.Upstream, err error) {
	q := fmt.Sprintf(`MATCH (u:Upstream) WHERE id(u) = %d`, upID)
	q += ` RETURN ` + UpstreamReturn

	res, err := n.Execute(q)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	up, err = UpstreamFromResult(res)
	if err != nil {
		logrus.Errorf("Failed to get upstream: %s", err)
		return
	}
	return
}

func (n Neo) Upstream_Get(ctx context.Context, up types.Upstream) (upID int64, success bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/upstream.go",
		"func":    "Upstream_Get",
	})
	_, span := tracer.Start(ctx, "Upstream_Get")
	defer span.End()
	span.SetAttributes(
		attribute.Key("upstream").String(up.String()),
	)
	q := `MATCH (u:Upstream)`
	q += fmt.Sprintf(` WHERE u.reg = "%s" AND u.repo = "%s" AND u.tag = "%s"`, up.Reg, up.Repo, up.Tag)
	if up.Digest.String() != "" {
		q += fmt.Sprintf(` AND u.digest = "%s"`, up.Digest.String())
	}
	q += ` RETURN id(u) AS ID`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	l.Debug(q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	result, err := session.Run(q, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("Upstream_Get: failed to get ID")
			return
		}
		upID = v.(int64)
		success = true
	}
	l.Debugf("upstream: %s, found: %t", up.String(), success)
	return
}

func (n Neo) GetUpstream(ctx context.Context, up types.Upstream) (upID int64, success bool, err error) {
	return n.Upstream_Get(ctx, up)
}

func (n Neo) CreateUpstream(ctx context.Context, up types.Upstream) (upID int64, err error) {
	label := fmt.Sprintf("%s/%s:%s", up.Reg, up.Repo, up.Tag)
	l := logrus.WithFields(logrus.Fields{
		"package":  "neo/upstream.go",
		"func":     "CreateUpstream",
		"upstream": label,
	})
	_, span := tracer.Start(ctx, "CreateUpstream")
	defer span.End()
	if up.Digest.String() == "" {
		err = fmt.Errorf("CreateUpstream: digest is empty")
		return
	}
	q := fmt.Sprintf(`CREATE (u:Upstream {reg: "%s", repo: "%s", tag: "%s", label: "%s"`, up.Reg, up.Repo, up.Tag, label)
	if up.Digest.String() != "" {
		q += fmt.Sprintf(`,digest: "%s"`, up.Digest.String())
	}
	q += `}) RETURN id(u) AS ID`
	l.Debug(q)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, err := n.Execute(q)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	upID, err = res.GetInt64("ID")
	if err != nil {
		logrus.Errorf("CreateUpstream: failed to get ID: %s", err)
		return
	}
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
	u = types.Upstream{
		ID:     id,
		Reg:    reg,
		Repo:   repo,
		Tag:    tag,
		Digest: digest.Digest(dig),
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
