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
		// TODO: Fetch annotations
		ctx := context.Background()
		a, e := n.GetAnnotationsByConfigID(ctx, cfg.ID)
		if e != nil {
			err = errors.Wrap(e, "failed to get annotations by config ID")
			return
		}
		cfg.Labels = a
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
	cfg, found, err = n.ConfigGetByDigest(ociMani.Config.Digest.String())
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		return
	}
	// Create Config from ociMani
	a, err := types.NewAnnotationsFromMap(ociMani.Config.Annotations)
	if err != nil {
		logrus.Error(err)
		return
	}
	cfg = types.Config{
		Digest:    ociMani.Config.Digest.String(),
		MediaType: ociMani.Config.MediaType,
		Size:      ociMani.Config.Size,
		Labels:    a,
	}
	cfgId, err := n.createConfig(cfg)
	//err = fmt.Errorf("need more info to create config with Upstream to create label")
	if err != nil {
		logrus.Error(err)
		return
	}
	cfg.ID = cfgId
	return
}

/*
func (n Neo) GetConfig(cfg types.Config) (cfgID int64, success bool, err error) {
	q := `MATCH (n:Config) WHERE n.digest = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, cfg.Digest)
	fmt.Printf("QueryGet: %s\n", query)
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
		logrus.Warnf("found config with ID: %d", cfgID)
		success = true
		// Get Annotations
		ctx := context.Background()
		a, e := n.GetAnnotationsByConfigID(ctx, cfgID)
		if e != nil {
			err = errors.Wrap(e, "failed to get annotations by config ID")
			return
		}
		logrus.Warnf("  -> found %d annotations", len(a.Map))
		cfg.Labels = a
	}
	return
}
*/

func (n Neo) createConfig(cfg types.Config) (cfgID int64, err error) {
	q := `CREATE (n:Config {digest: "%s", mediaType: "%s", size: %d}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, cfg.Digest, cfg.MediaType, cfg.Size)
	//fmt.Printf("QueryCreate: \n%s\n", query)
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
	} else {
		err = fmt.Errorf("createConfig: failed to get ID")
	}
	return
}

func (n Neo) Config_Insert(ctx context.Context, cfg types.Config) (cfgID int64, err error) {
	gotCfg, success, err := n.ConfigGetByDigest(cfg.Digest)
	if err != nil {
		return
	}
	if !success {
		cfgID, err = n.createConfig(cfg)
		if err != nil {
			return
		}
		//logrus.Errorf("Config_Insert ID: %d", cfgID)
		// Add Annotations
		a, e := cfg.LabelsToAnnotations()
		if e != nil {
			err = e
			return
		}
		//logrus.Errorf("Config_Insert Annotations: %v", a)
		err = n.Config_AddLabels(ctx, cfgID, &a)
		return
	}
	cfgID = gotCfg.ID
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

func (n Neo) LinkConfigToAnnotation(ctx context.Context, cfgID, annotationID int64) (err error) {
	q := `MATCH (cfg:Config), (a:Annotation) WHERE id(cfg) = %d AND id(a) = %d MERGE (cfg)-[:HAS_ANNOTATION]->(a)`
	query := fmt.Sprintf(q, cfgID, annotationID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}
