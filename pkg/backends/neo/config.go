package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (n Neo) Config_GetID_ByDigestAndType(ctx context.Context, dig, mediaType string) (cfgID int64, found bool, err error) {
	q := `MATCH (c:Config) WHERE c.digest = "%s" AND c.mediaType = "%s" RETURN id(c) as ID`
	query := fmt.Sprintf(q, dig, mediaType)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return
	}
	cfgID, err = res.GetInt64("ID")
	return
}

func (n Neo) Config_GetByDigest(dig string) (cfgs []types.Config, found bool, err error) {
	q := `MATCH (n:Config) WHERE n.digest = "%s"`
	q += ` RETURN id(n) AS configID, n.digest AS configDigest, n.mediaType AS configMediaType, n.size AS configSize`
	query := fmt.Sprintf(q, dig)
	//fmt.Printf("Query: %s\n", query)
	res, err := n.Query(query)
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	cfgs = make([]types.Config, 0)
	for _, r := range res {
		cfg, e := ConfigFromResult(r)
		if e != nil {
			err = errors.Wrap(e, "failed to create config from DB result")
			return
		}
		cfgs = append(cfgs, cfg)
		/*
			// TODO: Fetch annotations
			ctx := context.Background()
			a, e := n.Anno(ctx, cfg.ID)
			if e != nil {
				err = errors.Wrap(e, "failed to get annotations by config ID")
				return
			}
			cfg.Labels = a
		*/
		found = true
	}
	return
}

func (n Neo) Config_GetByDigestAndType(dig, mediaType string) (cfg types.Config, found bool, err error) {
	q := `MATCH (n:Config) WHERE n.digest = "%s" AND n.mediaType = "%s"`
	q += ` RETURN id(n) AS configID, n.digest AS configDigest, n.mediaType AS configMediaType, n.size AS configSize`
	query := fmt.Sprintf(q, dig, mediaType)
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
		/*
			// TODO: Fetch annotations
			ctx := context.Background()
			a, e := n.GetAnnotationsByConfigID(ctx, cfg.ID)
			if e != nil {
				err = errors.Wrap(e, "failed to get annotations by config ID")
				return
			}
			cfg.Labels = a
		*/
		found = true
	default:
		err = fmt.Errorf("found multiple configs with digest %s and mediaType '%s'", dig, mediaType)
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
	cfg, found, err = n.Config_GetByDigestAndType(ociMani.Config.Digest.String(), ociMani.Config.MediaType)
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		return
	}
	// Create Config from ociMani
	/*
		a, err := types.NewAnnotationsFromMap(ociMani.Config.Annotations)
		if err != nil {
			logrus.Error(err)
			return
		}
	*/
	cfg = types.Config{
		Digest:    ociMani.Config.Digest.String(),
		MediaType: ociMani.Config.MediaType,
		Size:      ociMani.Config.Size,
		//Labels:    a,
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

func (n Neo) createConfig(cfg types.Config) (cfgID int64, err error) {
	label := utils.CreateLabelFromTypeAndDigest(cfg.MediaType, cfg.Digest)
	q := `CREATE (n:Config {digest: "%s", mediaType: "%s", size: %d, label: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, cfg.Digest, cfg.MediaType, cfg.Size, label)
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
	gotCfg, success, err := n.Config_GetByDigestAndType(cfg.Digest, cfg.MediaType)
	if err != nil {
		return
	}
	if !success {
		cfgID, err = n.createConfig(cfg)
		if err != nil {
			return
		}
	} else {
		cfgID = gotCfg.ID
	}
	for k, v := range cfg.Cfg.Config.Labels {
		a := types.NewAnnotation(k, v)
		lID, e := n.Label_GetInsert(ctx, a)
		if e != nil {
			err = errors.Wrap(e, "failed to get or insert label")
			return
		}
		err = n.LinkConfigToLabel(ctx, cfgID, lID)
		if err != nil {
			err = errors.Wrap(err, "failed to link config to label")
			return
		}
	}
	return
}

func ConfigFromResult(res types.Result) (cfg types.Config, err error) {
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

func (n Neo) Config_Update(ctx context.Context, cfgID int64, cfg types.Config) (err error) {
	q := `MATCH (cfg:Config) WHERE id(cfg) = %d SET cfg.digest = "%s", cfg.size = %d`
	query := fmt.Sprintf(q, cfgID, cfg.Digest, cfg.Size)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) LinkConfigToLabel(ctx context.Context, cfgID, labelID int64) (err error) {
	q := `MATCH (cfg:Config), (l:Label) WHERE id(cfg) = %d AND id(l) = %d MERGE (cfg)-[:HAS_LABEL]->(l)`
	query := fmt.Sprintf(q, cfgID, labelID)
	_, err = n.Execute(query)
	if err != nil {
		return
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

func (n Neo) Config_AddLabel(ctx context.Context, cfgID int64, a types.Annotation) (err error) {
	// GetAnnotationID
	labelID, err := n.Label_GetInsert(ctx, a)
	if err != nil {
		logrus.Errorf("AddLabel : %v", err)
		return
	}
	q := `MATCH (cfg:Config) WHERE id(cfg) = %d`
	q += ` MATCH (l:Label) WHERE id(l) = %d`
	q += ` MERGE (cfg)-[:HAS_LABEL]->(l)`
	query := fmt.Sprintf(q, cfgID, labelID)
	_, err = n.Execute(query)
	if err != nil {
		logrus.Errorf("Config_AddLabel: %v", err)
		return
	}
	return
}

func ConfigFromRecord(res neo4j.Result) (c types.Config, err error) {
	r := res.Record()
	v, ok := r.Get("configID")
	if !ok {
		err = fmt.Errorf("no configID found in record: %v", r)
		return
	}
	c.ID = v.(int64)
	v, ok = r.Get("configDigest")
	if !ok {
		err = fmt.Errorf("no configDigest found in record: %v", r)
		return
	}
	c.Digest = v.(string)
	v, ok = r.Get("configMediaType")
	if !ok {
		err = fmt.Errorf("no configMediaType found in record: %v", r)
		return
	}
	c.MediaType = v.(string)
	v, ok = r.Get("configSize")
	if !ok {
		err = fmt.Errorf("no configSize found in record: %v", r)
		return
	}
	c.Size = v.(int64)
	return
}
