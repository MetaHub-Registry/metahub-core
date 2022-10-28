package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) GetProfiles() (ps []types.Profile, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (prf:Profile) RETURN id(prf) AS id, prf.name AS name`
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	ps = make([]types.Profile, 0)
	for result.Next() {
		p := types.Profile{}
		record := result.Record()
		id, ok := record.Get("id")
		if !ok {
			logrus.Errorf("No id found in record: %v", record)
			continue
		}
		p.NeoID = id.(int64)
		v, ok := record.Get("name")
		if !ok {
			logrus.Errorf("No name found in record: %v", record)
			continue
		}
		p.Name = v.(string)
		ps = append(ps, p)
	}
	return
}

func (n Neo) GetProfileFilterPlatformPairs() (pfs []types.ProfileFilterPlatform, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (p:Profile)-[]-(f:Filter)-[]-(plat:Platform)`
	q += ` RETURN id(p) AS profileID, p.name AS profileName, id(f) AS filterID, f.label AS filterLabel,`
	q += ` id(plat) AS platformID, plat.os AS platformOS, plat.arch AS platformArch, plat.variant AS platformVariant, plat.features AS platformFeatures`
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	pfs = make([]types.ProfileFilterPlatform, 0)
	for result.Next() {
		p := types.Profile{}
		f := types.Filter{}
		plat := types.Platform{}
		record := result.Record()
		v, ok := record.Get("profileID")
		if !ok {
			logrus.Errorf("No profileID found in record: %v", record)
			continue
		}
		p.NeoID = v.(int64)
		v, ok = record.Get("profileName")
		if !ok {
			logrus.Errorf("No profileName found in record: %v", record)
			continue
		}
		p.Name = v.(string)
		v, ok = record.Get("filterID")
		if !ok {
			logrus.Errorf("No filterID found in record: %v", record)
			continue
		}
		f.NeoID = v.(int64)
		v, ok = record.Get("filterLabel")
		if !ok {
			logrus.Errorf("No filterLabel found in record: %v", record)
			continue
		}
		f.Label = v.(string)
		// Platform
		v, ok = record.Get("platformID")
		if !ok {
			logrus.Errorf("No platformID found in record: %v", record)
			continue
		}
		plat.NeoID = v.(int64)
		v, ok = record.Get("platformOS")
		if !ok {
			logrus.Errorf("No platformOS found in record: %v", record)
			continue
		}
		plat.OS = v.(string)
		v, ok = record.Get("platformArch")
		if !ok {
			logrus.Errorf("No platformArch found in record: %v", record)
			continue
		}
		plat.Arch = v.(string)
		v, ok = record.Get("platformVariant")
		if !ok {
			logrus.Errorf("No platformVariant found in record: %v", record)
			continue
		}
		plat.Variant = v.(string)
		v, ok = record.Get("platformFeatures")
		if !ok {
			logrus.Errorf("No platformFeatures found in record: %v", record)
			continue
		}
		plat.Features = v.(string)
		pfs = append(pfs, types.ProfileFilterPlatform{Profile: p, Filter: f, Platform: plat})
	}
	return
}

func (n Neo) GetProfile(prf types.Profile) (prfID int64, success bool, err error) {
	q := `MATCH (n:Profile) WHERE n.name = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, prf.Name)
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
		prfID = v.(int64)
		success = true
	}
	return
}

func (n Neo) CreateProfile(prf types.Profile) (prfID int64, err error) {
	q := `CREATE (n:Profile {name: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, prf.Name)
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
		prfID = v.(int64)
	}
	return
}

func (n Neo) LinkProfileToFilter(ctx context.Context, profileID, filterID int64) (err error) {
	q := `MATCH (p:Profile),(f:Filter) WHERE id(p) = %d AND id(f) = %d`
	q += ` MERGE (p)-[:USES]->(f)`
	query := fmt.Sprintf(q, profileID, filterID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link (p:Profile)-[:USES]->(f:Filter): %v", err)
		return
	}
	return
}
