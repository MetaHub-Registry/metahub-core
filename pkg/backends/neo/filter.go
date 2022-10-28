package neo

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) GetFilter(f types.Filter) (filterID int64, success bool, err error) {
	q := `MATCH (n:Filter) WHERE n.label = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, f.Label)
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
		filterID = v.(int64)
		success = true
	}
	return
}

func (n Neo) CreateFilter(f types.Filter) (filterID int64, err error) {
	q := `CREATE (n:Filter {label: "%s", feature: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, f.Label, f.Feature)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create platform: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("failed to get ID")
			return
		}
		filterID = v.(int64)
	}
	return
}
