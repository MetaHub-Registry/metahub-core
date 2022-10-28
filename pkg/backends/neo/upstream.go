package neo

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) GetUpstream(up types.Upstream) (upID int64, success bool, err error) {
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
			err = fmt.Errorf("failed to get ID")
			return
		}
		upID = v.(int64)
		success = true
	}
	return
}

func (n Neo) CreateUpstream(up types.Upstream) (upID int64, err error) {
	q := `CREATE (n:Upstream {reg: "%s", repo: "%s", tag: "%s", label: "%s"}) RETURN id(n) AS ID`
	label := fmt.Sprintf("%s/%s:%s", up.Reg, up.Repo, up.Tag)
	query := fmt.Sprintf(q, up.Reg, up.Repo, up.Tag, label)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create manifest: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("failed to get ID")
			return
		}
		upID = v.(int64)
	}
	return
}
