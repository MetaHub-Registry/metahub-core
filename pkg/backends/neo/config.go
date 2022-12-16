package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) ConfigGetByDigest(dig string) (cfg types.Config, found bool, err error) {
	q := `MATCH (n:Config) WHERE n.digest = "%s"`
	q += ` RETURN id(n) AS configID, n.digest AS configDigest, n.mediaType AS configMediaType, n.size AS configSize`
	query := fmt.Sprintf(q, dig)
	//fmt.Printf("Query: %s\n", query)
	res, err := n.Query(query)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	switch len(res) {
	case 0:
		return
	case 1:
		r := res[0]
		cfg, err = ConfigFromResult(r)
		if err != nil {
			err = errors.Wrap(err, "failed to create config from DB result")
			return
		}
		found = true
	default:
		err = fmt.Errorf("found multiple configs with digest %s", dig)
		return
	}
	return
}

func (n Neo) ConfigDeleteByDigest(dig string) (err error) {
	q := `MATCH (n:Config) WHERE n.digest = "%s" DETACH DELETE n`
	query := fmt.Sprintf(q, dig)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	return err
}

func (n Neo) Config_GetInsert_ByOCIManifest(ociMani types.OCIManifest, label string) (cfg types.Config, found bool, err error) {
	cfg.MediaType = ociMani.Config.MediaType
	cfg.Size = ociMani.Config.Size
	cfg.Digest = ociMani.Config.Digest.String()
	cfgId, suc, err := n.GetConfig(cfg)
	if err != nil {
		logrus.Error(err)
		return
	}
	if suc {
		found = true
		cfg.ID = cfgId
		return
	}
	cfgId, err = n.createConfig(cfg)
	//err = fmt.Errorf("need more info to create config with Upstream to create label")
	if err != nil {
		logrus.Error(err)
		return
	}
	cfg.ID = cfgId
	return
}

func (n Neo) GetConfig(cfg types.Config) (cfgID int64, success bool, err error) {
	q := `MATCH (n:Config) WHERE n.digest = "%s" AND n.mediaType = "%s" AND n.size = %d RETURN id(n) AS ID`
	query := fmt.Sprintf(q, cfg.Digest, cfg.MediaType, cfg.Size)
	//fmt.Printf("QueryGet: %s\n", query)
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
			err = fmt.Errorf("GetConfig() failed to get ID")
			return
		}
		cfgID = v.(int64)
		success = true
	}
	return
}

func (n Neo) createConfig(cfg types.Config) (cfgID int64, err error) {
	q := `CREATE (n:Config {digest: "%s", mediaType: "%s", size: %d}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, cfg.Digest, cfg.MediaType, cfg.Size)
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
			err = fmt.Errorf("createConfig: failed to get ID")
			return
		}
		cfgID = v.(int64)
	}
	return
}

func (n Neo) Config_Insert(ctx context.Context, cfg types.Config) (cfgID int64, err error) {
	cfgID, success, err := n.GetConfig(cfg)
	if err != nil {
		return
	}
	if !success {
		cfgID, err = n.createConfig(cfg)
		if err != nil {
			return
		}
	}
	return
}

func ConfigFromResult(res Result) (cfg types.Config, err error) {
	err = res.CheckKeys("configID", "configDigest", "configMediaType", "configSize")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("configID")
	dig, _ := res.GetStr("configDigest")
	mt, _ := res.GetStr("configMediaType")
	size, _ := res.GetInt64("configSize")
	cfg = types.Config{
		ID:        id,
		Digest:    dig,
		MediaType: mt,
		Size:      size,
	}
	return
}
