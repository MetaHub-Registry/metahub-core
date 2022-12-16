package neo

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) Upstream_GetInsert(up *types.Upstream) (found bool, err error) {
	upID, found, err := n.GetUpstream(*up)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !found {
		upID, err = n.CreateUpstream(*up)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	up.ID = upID
	return
}

func (n Neo) Upstream_GetInsert_ByRequest(req types.Request) (up types.Upstream, found bool, err error) {
	up = types.Upstream{
		Reg:  req.Host,
		Repo: req.Repo,
		Tag:  req.Tag,
	}
	found, err = n.Upstream_GetInsert(&up)
	return
}

func (n Neo) Upstream_Get_ByID(upID int64) (up types.Upstream, err error) {
	q := `MATCH (n:Upstream) WHERE id(n) = %d RETURN id(n) AS upstreamID, n.reg AS upstreamReg, n.repo AS upstreamRepo, n.tag AS upstreamTag`
	query := fmt.Sprintf(q, upID)
	res, err := n.Execute(query)
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

func (n Neo) Upstream_Get(up types.Upstream) (upID int64, success bool, err error) {
	q := `MATCH (n:Upstream) WHERE n.reg = "%s" AND n.repo = "%s" AND n.tag = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, up.Reg, up.Repo, up.Tag)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
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
	return
}

func (n Neo) GetUpstream(up types.Upstream) (upID int64, success bool, err error) {
	return n.Upstream_Get(up)
}

func (n Neo) CreateUpstream(up types.Upstream) (upID int64, err error) {
	q := `CREATE (n:Upstream {reg: "%s", repo: "%s", tag: "%s", label: "%s"}) RETURN id(n) AS ID`
	label := fmt.Sprintf("%s/%s:%s", up.Reg, up.Repo, up.Tag)
	query := fmt.Sprintf(q, up.Reg, up.Repo, up.Tag, label)
	res, err := n.Execute(query)
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

func UpstreamFromResult(res Result) (u types.Upstream, err error) {
	err = res.CheckKeys("upstreamID", "upstreamReg", "upstreamRepo", "upstreamTag")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("upstreamID")
	reg, _ := res.GetStr("upstreamReg")
	repo, _ := res.GetStr("upstreamRepo")
	tag, _ := res.GetStr("upstreamTag")
	u = types.Upstream{
		ID:   id,
		Reg:  reg,
		Repo: repo,
		Tag:  tag,
	}
	return
}

func UpstreamFromRecord(res neo4j.Result) (u types.Upstream, err error) {
	r := NewResult()
	record := res.Record()
	keys, err := res.Keys()
	if err != nil {
		logrus.Errorf("Error getting keys: %s\n", err)
		return
	}
	for idx, key := range keys {
		v := record.Values[idx]
		r.Add(key, v)
	}
	return UpstreamFromResult(r)
}
