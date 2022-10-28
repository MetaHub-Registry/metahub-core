package neo

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) GetConfig(cfg types.Config) (cfgID int64, success bool, err error) {
	q := `MATCH (n:Config) WHERE n.digest = "%s" AND n.mediaType = "%s" AND n.size = %d RETURN id(n) AS ID`
	query := fmt.Sprintf(q, cfg.Digest, cfg.MediaType, cfg.Size)
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
		cfgID = v.(int64)
		success = true
	}
	return
}

func (n Neo) CreateConfig(cfg types.Config, up types.Upstream) (cfgID int64, err error) {
	q := `CREATE (n:Config {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	label := fmt.Sprintf("%s/%s:%s", up.Reg, up.Repo, up.Tag)
	query := fmt.Sprintf(q, cfg.Digest, cfg.MediaType, cfg.Size, label)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create conifg: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("failed to get ID")
			return
		}
		cfgID = v.(int64)
	}
	return
}
