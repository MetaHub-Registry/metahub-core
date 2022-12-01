package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func (n Neo) GetCollection(col yamlCfg.ManifestCollection) (colId int64, success bool, err error) {
	q := `MATCH (c:Collection) WHERE c.repo = "%s" AND c.tag = "%s" RETURN id(c) AS collectionID`
	query := fmt.Sprintf(q, col.Name, col.Tag)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		colId = result.Record().GetByIndex(0).(int64)
		success = true
	}
	return
}

func (n Neo) GetAllCollections() (collections []types.Collection, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (c:Collection)<-[:SERVES]-(ml:ManifestList)-[:IMPLEMENTS]->(f) WHERE ml.label IS NOT NULL`
	q += ` RETURN id(c) AS collectionID, id(ml) AS manifestListID, id(f) AS filterID,`
	q += ` c.repo AS collectionRepo, c.tag AS collectionTag,`
	q += ` ml.digest AS manifestListDigest, ml.mediaType AS manifestListMediaType, ml.size AS manifestListSize`
	logrus.Warnf("query: %s", q)
	input := map[string]interface{}{}
	result, err := session.Run(q, input)
	if err != nil {
		return
	}
	cols := map[int64]types.Collection{}
	collections = make([]types.Collection, 0)
	for result.Next() {
		c, e := CollectionFromRecord(result)
		if e != nil {
			err = e
			return
		}
		ml, e := ManifestListFromRecord(result)
		if e != nil {
			err = e
			return
		}
		// Get all manifests for this manifestList
		ml.Manifests, err = n.GetManifestsForManifestList(ml.NeoID)
		if err != nil {
			return
		}
		col, ok := cols[c.NeoID]
		if !ok {
			c.ManifestLists = append(c.ManifestLists, ml)
			cols[c.NeoID] = c
		} else {
			col.ManifestLists = append(cols[c.NeoID].ManifestLists, ml)
			cols[c.NeoID] = col
		}
	}
	for _, c := range cols {
		collections = append(collections, c)
	}
	return
}

func (n Neo) CreateCollection(col yamlCfg.ManifestCollection) (colId int64, err error) {
	q := `CREATE (c:Collection {repo: "%s", tag: "%s", label: "%s"}) RETURN id(c) AS collectionID`
	label := fmt.Sprintf("%s:%s", col.Name, col.Tag)
	query := fmt.Sprintf(q, col.Name, col.Tag, label)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create collection: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("collectionID")
		if !ok {
			err = fmt.Errorf("failed to get collectionID")
			return
		}
		colId = v.(int64)
	}
	return
}

func CollectionFromRecord(res neo4j.Result) (c types.Collection, err error) {
	r := res.Record()
	v, ok := r.Get("collectionID")
	if !ok {
		err = fmt.Errorf("no manifestListID found in record: %v", r)
		return
	}
	c.NeoID = v.(int64)
	v, ok = r.Get("collectionRepo")
	if !ok {
		err = fmt.Errorf("no collectionRepo found in record: %v", r)
		return
	}
	c.Repo = v.(string)
	v, ok = r.Get("collectionTag")
	if !ok {
		err = fmt.Errorf("no collectionTag found in record: %v", r)
		return
	}
	tagStr := v.(string)
	c.ColTag = types.NewCollectionTag(tagStr)
	c.Tag = tagStr
	c.ManifestLists = make([]types.ManifestList, 0)
	return
}

func ManifestListFromRecord(res neo4j.Result) (ml types.ManifestList, err error) {
	r := res.Record()
	v, ok := r.Get("manifestListID")
	if !ok {
		err = fmt.Errorf("no manifestListID found in record: %v", r)
		return
	}
	ml.NeoID = v.(int64)
	v, ok = r.Get("manifestListDigest")
	if !ok {
		err = fmt.Errorf("no manifestListDigest found in record: %v", r)
		return
	}
	ml.Digest = v.(string)
	v, ok = r.Get("manifestListMediaType")
	if !ok {
		err = fmt.Errorf("no manifestListMediaType found in record: %v", r)
		return
	}
	ml.MediaType = v.(string)
	ml.Manifests = make([]types.Manifest, 0)
	return
}

func (n Neo) LinkCollectionToManifest(ctx context.Context, colID, maniID int64) (err error) {
	q := `MATCH (c:Collection), (m:Manifest) WHERE id(c) = %d AND id(m) = %d`
	q += ` MERGE (c)-[:UPSTREAM]->(m)`
	query := fmt.Sprintf(q, colID, maniID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link collection-manifest: %v", err)
		return
	}
	return
}

func (n Neo) GetCollectionFilterPairs() (cfps []types.CollectionFilterPair, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	q := `MATCH (c:Collection)-[:UPSTREAM]-(Manifest)-[:IS]->(Platform)<-[:USES]-(f:Filter)`
	q += ` RETURN DISTINCT id(c) AS collectionID, c.repo AS colRepo, c.tag AS colTag,`
	q += ` id(f) AS filterID, f.label AS filterLabel`
	logrus.Infof("query: %s", q)
	result, err := session.Run(q, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to get collection-filter pairs: %v", err)
		return
	}
	type tupel struct {
		collectionID int64
		colRepo      string
		colTag       string
		filterID     int64
		filterLabel  string
	}
	tupels := make([]tupel, 0)
	for result.Next() {
		t := tupel{}
		r := result.Record()
		v, ok := r.Get("collectionID")
		if !ok {
			err = fmt.Errorf("no collectionID found in record: %v", r)
			return
		}
		t.collectionID = v.(int64)
		v, ok = r.Get("colRepo")
		if !ok {
			err = fmt.Errorf("no colRepo found in record: %v", r)
			return
		}
		t.colRepo = v.(string)
		v, ok = r.Get("colTag")
		if !ok {
			err = fmt.Errorf("no colTag found in record: %v", r)
			return
		}
		t.colTag = v.(string)
		v, ok = r.Get("filterID")
		if !ok {
			err = fmt.Errorf("no filterID found in record: %v", r)
			return
		}
		t.filterID = v.(int64)
		v, ok = r.Get("filterLabel")
		if !ok {
			err = fmt.Errorf("no filterLabel found in record: %v", r)
			return
		}
		t.filterLabel = v.(string)
		tupels = append(tupels, t)
	}
	// Create CollectionFilterPairs
	cfps = make([]types.CollectionFilterPair, 0)
	for _, t := range tupels {
		q := `MATCH (c:Collection)-[:UPSTREAM]-(m:Manifest)-[:IS]->(p:Platform)<-[:USES]-(f:Filter)`
		q += ` MATCH (cfg:Config)<-[:USES]-(m)-[:UPSTREAM]->(u:Upstream)`
		q += ` WHERE id(c) = %d AND id(f) = %d`
		q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant, p.features AS platformFeatures,`
		q += ` id(m) AS manifestID, m.digest AS manifestDigest, m.mediaType AS manifestMediaType, m.size AS manifestSize,`
		q += ` id(u) AS upstreamID, u.reg AS upstreamReg, u.repo AS upstreamRepo, u.tag AS upstreamTag,`
		q += ` id(cfg) AS configID, cfg.digest AS configDigest, cfg.mediaType AS configMediaType, cfg.size AS configSize`
		query := fmt.Sprintf(q, t.collectionID, t.filterID)
		//logrus.Warnf("query: %s", query)
		result, err := session.Run(query, map[string]interface{}{})
		if err != nil {
			logrus.Errorf("failed to get collection-filter pairs: %v", err)
			return nil, err
		}
		cfp := types.CollectionFilterPair{
			Collection: types.Collection{
				NeoID:  t.collectionID,
				Repo:   t.colRepo,
				Tag:    t.colTag,
				ColTag: types.NewCollectionTag(t.colTag),
			},
			Filter: types.Filter{
				NeoID: t.filterID,
				Label: t.filterLabel,
			},
			Manifests: make([]types.Manifest, 0),
		}
		for result.Next() {
			p, e := PlatformFromRecord(result)
			if e != nil {
				err = errors.Wrap(e, "failed to get platform from record")
				return cfps, err
			}
			c, e := ConfigFromRecord(result)
			if e != nil {
				err = errors.Wrap(e, "failed to get config from record")
				return cfps, err
			}
			u, e := UpstreamFromRecord(result)
			if e != nil {
				err = errors.Wrap(e, "failed to get upstream from record")
				return cfps, err
			}
			m, e := ManifestFromRecord(result)
			if e != nil {
				err = errors.Wrap(e, "failed to get manifest from record")
				return cfps, err
			}
			m.Config = c
			m.Platform = p
			m.Upstream = u
			cfp.Manifests = append(cfp.Manifests, m)
		}
		cfps = append(cfps, cfp)
	}
	return
}
