package neo

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (n Neo) GetManifestList(dig digest.Digest, mt string, size int) (mlID int64, success bool, err error) {
	q := `MATCH (n:ManifestList) WHERE n.digest = "%s" AND n.mediaType = "%s" AND n.size = %d RETURN id(n) AS ID`
	query := fmt.Sprintf(q, dig, mt, size)
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
		mlID = v.(int64)
		success = true
	}
	return
}

func (n Neo) CreateManifestList(dig digest.Digest, mt string, size int) (mlID int64, err error) {
	q := `CREATE (n:ManifestList {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, dig, mt, size, utils.ShortenDigest(dig.String()))
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
		mlID = v.(int64)
	}
	return
}

func (n Neo) LinkManifestListToCollection(mlID, colID int64) (err error) {
	q := `MATCH (c:Collection), (ml:ManifestList) WHERE id(c) = %d AND id(ml) = %d`
	q += ` MERGE (ml)-[:SERVES]->(c)`
	query := fmt.Sprintf(q, colID, mlID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:ManifestList)-[:SERVES]->(c:Collection): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToFilter(mlID, filterID int64) (err error) {
	q := `MATCH (ml:ManifestList),(f:Filter) WHERE id(ml) = %d AND id(f) = %d`
	q += ` MERGE (ml)-[:IMPLEMENTS]->(f)`
	query := fmt.Sprintf(q, mlID, filterID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (ml:ManifestList)-[:IMPLEMENTS]->(f:Filter): %v", err)
		return
	}
	return
}

func (n Neo) LinkManifestListToManifests(mlID int64, manifests []types.Manifest) (err error) {
	for _, m := range manifests {
		q := `MATCH (ml:ManifestList),(m:Manifest) WHERE id(ml) = %d AND id(m) = %d`
		q += ` MERGE (ml)-[:INCLUDES]->(m)`
		query := fmt.Sprintf(q, mlID, m.NeoID)
		session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		defer session.Close()
		_, err = session.Run(query, map[string]interface{}{})
		if err != nil {
			logrus.Errorf("failed to create link (ml:ManifestList)-[:INCLUDES]->(m:Manifest): %v\n%s", err, query)
			return
		}
	}
	return
}
