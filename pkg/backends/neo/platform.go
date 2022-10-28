package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) GetPlatform(plat types.Platform) (platID int64, success bool, err error) {
	q := `MATCH (n:Platform) WHERE n.arch = "%s" AND n.os = "%s" AND n.variant = "%s" AND n.features = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, plat.Arch, plat.OS, plat.Variant, plat.Features)
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
		platID = v.(int64)
		success = true
	}
	return
}

func (n Neo) CreatePlatform(plat types.Platform) (platID int64, err error) {
	q := `CREATE (n:Platform {arch: "%s", os: "%s", variant: "%s", features: "%s", label: "%s"}) RETURN id(n) AS ID`
	label := fmt.Sprintf("%s/%s", plat.OS, plat.Arch)
	if plat.Variant != "" {
		label = fmt.Sprintf("%s/%s", label, plat.Variant)
	}
	if plat.Features != "" {
		label = fmt.Sprintf("%s:%s", label, plat.Features)
	}
	query := fmt.Sprintf(q, plat.Arch, plat.OS, plat.Variant, plat.Features, label)
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
		platID = v.(int64)
	}
	return
}

func (n Neo) LinkPlatformToFilter(ctx context.Context, platID int64, plat types.Platform) (err error) {
	q := `MATCH (p:Platform), (f:Filter) WHERE id(p) = %d`
	if plat.Features != "" {
		q += fmt.Sprintf(" AND f.feature = '%s'", plat.Features)
	} else {
		q += " AND f.label = 'generic'"
	}
	q += ` MERGE (f)-[:USES]->(p)`
	query := fmt.Sprintf(q, platID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (f:Filter)-[:USES]->(p:Platform): %v", err)
		return
	}
	return
}
