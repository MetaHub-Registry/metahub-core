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

func (n Neo) Collection_GetID_ByYaml(ycol yamlCfg.ManifestCollection) (colID, colTagID int64, found bool, err error) {
	col, found, err := n.GetCollection(ycol)
	if err != nil || !found {
		return 0, 0, false, err
	}
	colID = col.ID
	colTagID = col.ColTag.ID
	return
}

func (n Neo) Collection_Create_ByYaml(ycol yamlCfg.ManifestCollection) (colID, colTagID int64, err error) {
	return n.CreateCollection(ycol)
}

func (n Neo) GetCollectionTag(repo, tag string) (resCol types.Collection, found bool, err error) {
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t) WHERE c.repo = "%s" AND t.name = "%s" RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	query := fmt.Sprintf(q, repo, tag)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return resCol, found, err
	}
	resCol, err = CollectionFromResult(res)
	return
}

func (n Neo) GetCollection(col yamlCfg.ManifestCollection) (resCol types.Collection, found bool, err error) {
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t) WHERE c.repo = "%s" AND t.name = "%s" RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	query := fmt.Sprintf(q, col.Name, col.Tag)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return resCol, found, err
	}
	resCol, err = CollectionFromResult(res)
	return
}

func (n Neo) CollectionGetAll() (collections []types.Collection, err error) {
	return n.GetAllCollections()
}

func (n Neo) Collection_Get_ByRepoTag(repo, tag string) (c types.Collection, found bool, err error) {
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t:Tag)-[:REF]->(ml:ManifestList)-[:IMPLEMENTS]->(f) WHERE c.repo = "%s" AND t.name = "%s"`
	q += ` RETURN id(c) AS collectionID, id(ml) AS manifestListID, id(f) AS filterID,`
	q += ` c.repo AS collectionRepo, t.name AS tagName, id(t) AS tagID,`
	q += ` ml.digest AS manifestListDigest, ml.mediaType AS manifestListMediaType, ml.size AS manifestListSize`
	query := fmt.Sprintf(q, repo, tag)
	//fmt.Printf("Query: %s\n", query)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	input := map[string]interface{}{}
	result, err := session.Run(query, input)
	if err != nil {
		return
	}
	for result.Next() {
		found = true
		c, err = CollectionFromRecord(result)
		if err != nil {
			return
		}
		ml, e := ManifestListFromRecord(result)
		if e != nil {
			err = e
			return
		}
		// Get all manifests for this manifestList
		ml.Manifests, err = n.ManifestData_GetByManifestList(ml.ID)
		if err != nil {
			return
		}
		c.ManifestLists = append(c.ManifestLists, ml)

	}
	return
}

func (n Neo) GetAllCollections() (collections []types.Collection, err error) {
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t:Tag)-[:REF]->(ml:ManifestList)-[:IMPLEMENTS]->(f) WHERE ml.label IS NOT NULL`
	q += ` MATCH (c)-[:REF]->(ml:ManifestList)`
	q += ` RETURN id(c) AS collectionID, id(ml) AS manifestListID, id(f) AS filterID,`
	q += ` c.repo AS collectionRepo, t.name AS tagName, id(t) AS tagID,`
	q += ` ml.digest AS manifestListDigest, ml.mediaType AS manifestListMediaType, ml.size AS manifestListSize`
	//fmt.Printf("Query: GetAllCollections \n%s\n", q)
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
		ml.Manifests, err = n.ManifestData_GetByManifestList(ml.ID)
		if err != nil {
			return
		}
		col, ok := cols[c.ID]
		if !ok {
			c.ManifestLists = append(c.ManifestLists, ml)
			cols[c.ID] = c
		} else {
			col.ManifestLists = append(cols[c.ID].ManifestLists, ml)
			cols[c.ID] = col
		}
	}
	for _, c := range cols {
		collections = append(collections, c)
	}
	return
}

func (n Neo) Collection_Create_ByRequest(req types.Request) (col types.Collection, err error) {
	ctx := context.Background()
	col, _, err = n.Collection_GetCreate_ByRepoTag(ctx, req.Repo, req.Tag)
	return
}

func (n Neo) Collection_GetCreate_ByRepoTag(ctx context.Context, repo, tag string) (c types.Collection, found bool, err error) {
	ct, err := n.CollectionTag_GetCreate(tag)
	if err != nil {
		return
	}
	c, found, err = n.Collection_GetCreate_ByRepo(repo)
	if err != nil {
		return
	}
	c.ColTag = ct
	c.Tag = tag
	q := `MATCH (c:Collection), (t:Tag) WHERE id(c) = %d AND id(t) = %d`
	q += ` MERGE (c)-[:HAS_TAG]->(t)`
	query := fmt.Sprintf(q, c.ID, ct.ID)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	return
}

func (n Neo) Collection_Get_ByRepo(repo string) (c types.Collection, found bool, err error) {
	q := `MATCH (c:Collection {repo: "%s"}) RETURN id(c) AS collectionID`
	query := fmt.Sprintf(q, repo)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return
	}
	if found {
		colId, e := res.GetInt64("collectionID")
		if e != nil {
			err = errors.Wrap(e, "failed to get collectionID from result")
			return c, found, err
		}
		c.ID = colId
		c.Repo = repo
		return
	}
	return
}

func (n Neo) Collection_GetCreate_ByRepo(repo string) (c types.Collection, found bool, err error) {
	c, found, err = n.Collection_Get_ByRepo(repo)
	if err != nil {
		return
	}
	if !found {
		q := `CREATE (c:Collection {repo: "%s"}) RETURN id(c) AS collectionID`
		query := fmt.Sprintf(q, repo)
		res, err := n.Execute(query)
		if err != nil {
			return c, found, err
		}
		colId, e := res.GetInt64("collectionID")
		if e != nil {
			err = errors.Wrap(e, "failed to get collectionID from result")
			return c, found, err
		}
		c.ID = colId
		c.Repo = repo
	}
	return
}

func (n Neo) CreateCollection(col yamlCfg.ManifestCollection) (colId, colTagId int64, err error) {
	ctx := context.Background()
	c, _, err := n.Collection_GetCreate_ByRepoTag(ctx, col.Name, col.Tag)
	if err != nil {
		return
	}
	colId = c.ID
	colTagId = c.ColTag.ID
	return
}

func CollectionFromRecord(res neo4j.Result) (c types.Collection, err error) {
	r := res.Record()
	v, ok := r.Get("collectionID")
	if !ok {
		err = fmt.Errorf("no manifestListID found in record: %v", r)
		return
	}
	c.ID = v.(int64)
	v, ok = r.Get("collectionRepo")
	if !ok {
		err = fmt.Errorf("no collectionRepo found in record: %v", r)
		return
	}
	c.Repo = v.(string)
	v, ok = r.Get("tagName")
	if !ok {
		err = fmt.Errorf("no tagName found in record: %v", r)
		return
	}
	tagStr := v.(string)
	v, ok = r.Get("tagID")
	if !ok {
		err = fmt.Errorf("no tagName found in record: %v", r)
		return
	}
	tagID := v.(int64)
	c.ColTag = types.NewCollectionTag(tagStr)
	c.ColTag.ID = tagID
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
	ml.ID = v.(int64)
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
	ml.Manifests = make([]types.ManifestData, 0)
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
		logrus.Errorf("failed to create link collection->manifest: %v", err)
		return
	}
	return
}

func (n Neo) GetCollectionFilterPairs_ForCollection(col types.Collection) (cfps []types.CollectionFilterPair, err error) {
	// Platform and filters need to be reworked
	q := `MATCH (t:Tag)-[:UPSTREAM]-(m:Manifest)-[:IS]->(Platform)<-[:USES]-(f:Filter)`
	q += ` MATCH (c:Collection)-[:UPSTREAM]->(m)`
	q += ` WHERE c.repo = "%s" AND t.name = "%s"`
	query := fmt.Sprintf(q, col.Repo, col.ColTag.Tag)
	//fmt.Printf("Query GetCollectionFilterPairs_ForCollection: \n%s\n", query)
	return n.getCollectionFilterPairs(query)
}

func (n Neo) getCollectionFilterPairs(q string) (cfps []types.CollectionFilterPair, err error) {
	q += ` RETURN DISTINCT id(c) AS collectionID, c.repo AS colRepo, id(t) AS tagID, t.name AS tagName,`
	q += ` id(f) AS filterID, f.label AS filterLabel`
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	result, err := session.Run(q, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to get collection-filter pairs: %v", err)
		return
	}
	type tupel struct {
		collectionID int64
		colRepo      string
		tagID        int64
		tagName      string
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
		v, ok = r.Get("tagID")
		if !ok {
			err = fmt.Errorf("no collectionID found in record: %v", r)
			return
		}
		t.tagID = v.(int64)
		v, ok = r.Get("tagName")
		if !ok {
			err = fmt.Errorf("no colTag found in record: %v", r)
			return
		}
		t.tagName = v.(string)
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
		q := `MATCH (c:Collection)-[:HAS_TAG]->(t:Tag)-[:UPSTREAM]-(m:Manifest)-[:IS]->(p:Platform)<-[:USES]-(f:Filter)`
		q += ` MATCH (cfg:Config)<-[:USES]-(md:ManifestData)-[]-(m)-[:UPSTREAM]->(u:Upstream)`
		q += ` WHERE id(c) = %d AND id(f) = %d AND id(t) = %d`
		q += ` RETURN id(p) AS platformID, p.arch AS platformArch, p.os AS platformOS, p.variant AS platformVariant,`
		q += ` id(m) AS manifestID, m.digest AS manifestDigest, m.mediaType AS manifestMediaType, m.size AS manifestSize,`
		q += ` id(u) AS upstreamID, u.reg AS upstreamReg, u.repo AS upstreamRepo, u.tag AS upstreamTag,`
		q += ` id(cfg) AS configID, cfg.digest AS configDigest, cfg.mediaType AS configMediaType, cfg.size AS configSize`
		query := fmt.Sprintf(q, t.collectionID, t.filterID, t.tagID)
		//logrus.Warnf("query: %s", query)
		result, err := session.Run(query, map[string]interface{}{})
		if err != nil {
			logrus.Errorf("failed to get collection-filter pairs: %v", err)
			return nil, err
		}
		ct := types.NewCollectionTag(t.tagName)
		ct.ID = t.tagID
		cfp := types.CollectionFilterPair{
			Collection: types.Collection{
				ID:     t.collectionID,
				Repo:   t.colRepo,
				Tag:    t.tagName,
				ColTag: ct,
			},
			Filter: types.Filter{
				ID:   t.filterID,
				Name: t.filterLabel,
			},
			Manifests: make([]types.ManifestData, 0),
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
			m, e := ManifestDataFromRecord(result)
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

func (n Neo) GetCollectionFilterPairs() (cfps []types.CollectionFilterPair, err error) {
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t:Tag)-[:UPSTREAM]-(m:Manifest)-[:IS]->(Platform)<-[:USES]-(f:Filter)`
	q += ` MATCH (c)-[:UPSTREAM]->(m)`
	//logrus.Infof("query: %s", q)
	return n.getCollectionFilterPairs(q)
}

func (n Neo) Collection_GetInsert_ByRequest(req types.Request) (col types.Collection, err error) {
	col, found, err := n.Collection_Get_ByRepoTag(req.Repo, req.Tag)
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		return col, nil
	}
	col, err = n.Collection_Create_ByRequest(req)
	return
}

func (n Neo) CollectionManifest_ByRequest(req types.Request, mani types.ManifestData) (err error) {
	/*
		ctx := context.Background()
		DOes not work anymore
		// Add Manifest
		err = n.Manifest_GetInsert(ctx, &mani)
		if err != nil {
			logrus.Error(err)
			return
		}
		// Config
		cfgID, err := n.Config_Insert(ctx, mani.Config)
		if err != nil {
			logrus.Error(err)
			return
		}
		err = n.LinkManifestToConfig(ctx, mani.ID, cfgID)
		if err != nil {
			logrus.Error(err)
			return
		}

		// Platform
		err = n.Platform_GetInsert(&mani.Platform)
		if err != nil {
			logrus.Error(err)
			return
		}
		err = n.LinkManifestToPlatform(ctx, mani.ID, mani.Platform.ID)
		if err != nil {
			logrus.Error(err)
			return
		}
		//  Deal with upstream / collection
		// 1. create upstream that is unique to the repo:tag-profile
		maniUp := mani.Upstream
		if maniUp != (types.Upstream{}) {
			// Let's only do upstream if the manifest provides one
			maniUp.Tag = fmt.Sprintf("%s-%s", req.Tag, mani.Platform.GetTag())
			found, err := n.Upstream_GetInsert(&maniUp)
			if err != nil {
				err = errors.Wrap(err, "failed to Upstream_GetInsert")
				logrus.Error(err)
				return err
			}
			if found {
				//Do we need to create a new upstream?
				logrus.Warnf("Upstream already exists: %s/%s:%s", maniUp.Reg, maniUp.Repo, maniUp.Tag)
			}
		} else {
			// Manifest does not come with upstream, we'll create a fake one with the current profile
			// -> <repo>:<tag>-<PlatformTag>
			maniUp = types.Upstream{
				Reg:  req.Host,
				Repo: req.Repo,
				Tag:  fmt.Sprintf("%s-%s", req.Tag, mani.Platform.GetTag()),
			}
			found, err := n.Upstream_GetInsert(&maniUp)
			if err != nil {
				err = errors.Wrap(err, "failed to Upstream_GetInsert")
				logrus.Error(err)
				return err
			}
			if found {
				// Do we need to create a new upstream?
				logrus.Warnf("Upstream already exists: %s/%s:%s", maniUp.Reg, maniUp.Repo, maniUp.Tag)
			}
		}
		// 1.1 Link Manifest To Upstream
		err = n.LinkManifestToUpstream(ctx, mani.ID, maniUp.ID)
		if err != nil {
			logrus.Error(err)
			return
		}
		// 2. create collection for the repo:tag
		// Collection and Tag
		colTag, foundColTag, err := n.CollectionTag_GetByTag(req.Tag)
		if err != nil {
			logrus.Error(err)
			return
		}
		if !foundColTag {
			colTag, err = n.CollectionTag_Create(req.Tag)
			if err != nil {
				logrus.Error(err)
				return
			}
		}
		col, foundCol, err := n.Collection_Get_ByRepo(req.Repo)
		if err != nil {
			logrus.Error(err)
			return
		}
		if !foundCol {
			col, err = n.Collection_Create_ByRequest(req)
			if err != nil {
				logrus.Error(err)
				return
			}
		}
		col.ColTag = colTag
		// 2.1 Link Collection to Tag
		err = n.LinkCollectionTagToManifest(ctx, colTag.ID, mani.ID)
		if err != nil {
			logrus.Error(err)
			return
		}
		err = n.LinkCollectionToManifest(ctx, col.ID, mani.ID)
		if err != nil {
			logrus.Error(err)
			return
		}
		// 3. create/update manifestlist for given collection with additional manifest in it
		err = n.ComputeManifestlists_ForCollection(ctx, col)
		if err != nil {
			logrus.Error(err)
			return
		}
	*/
	return
}

func CollectionFromResult(res types.Result) (col types.Collection, err error) {
	err = res.CheckKeys("tagID", "tagName", "collectionID", "collectionRepo")
	if err != nil {
		return
	}
	tagName, _ := res.GetStr("tagName")
	colId, _ := res.GetInt64("collectionID")
	colRepo, _ := res.GetStr("collectionRepo")
	ct, err := CollectionTagFromResult(res)
	if err != nil {
		return
	}
	col = types.Collection{
		ID:     colId,
		Repo:   colRepo,
		Tag:    tagName,
		ColTag: ct,
	}
	return
}

func CollectionListingFromResult(res types.Result) (col types.CollectionListing, err error) {
	err = res.CheckKeys("tagID", "tagName", "collectionID", "collectionRepo")
	if err != nil {
		return
	}
	tagName, _ := res.GetStr("tagName")
	colId, _ := res.GetInt64("collectionID")
	colRepo, _ := res.GetStr("collectionRepo")
	ct, err := CollectionTagFromResult(res)
	if err != nil {
		return
	}
	col = types.CollectionListing{
		ID:            colId,
		Repo:          colRepo,
		Tag:           tagName,
		ColTag:        ct,
		Manifests:     map[string]types.ManifestData{},
		ManifestCount: 0,
	}
	return
}

func (n Neo) Collection_Remove(ctx context.Context, col types.Collection) (found bool, err error) {
	c, found, err := n.Collection_Get_ByRepoTag(col.Repo, col.Tag)
	if err != nil {
		err = errors.Wrap(err, "failed to Collection_GetCreate_ByRepoTag")
		logrus.Error(err)
		return
	}
	if !found {
		// Might not be a full collection
		c, found, err = n.GetCollectionTag(col.Repo, col.Tag)
		if err != nil {
			err = errors.Wrap(err, "failed to GetCollectionTag")
			logrus.Error(err)
			return
		}
		if !found {
			err = errors.New("collection not found")
			logrus.Error(err)
			return
		}
		fmt.Printf("Simple Collection found: \n%v\n", c)
	}
	// Remove CollectionTag link to Manifest
	for _, ml := range c.ManifestLists {
		err = n.UnLinkManifestListToCollection(ml.ID, c.ID)
		if err != nil {
			err = errors.Wrap(err, "failed to UnLinkManifestListToCollection")
			logrus.Error(err)
			return
		}
		err = n.UnLinkManifestListToCollectionTag(ml.ID, c.ColTag.ID)
		if err != nil {
			err = errors.Wrap(err, "failed to UnLinkManifestListToCollectionTag")
			logrus.Error(err)
			return
		}
	}
	// Remove HAS_TAG between Collection and CollectionTag
	err = n.CollectionTag_DetachFromCollection(c.ID, c.ColTag.ID)
	if err != nil {
		err = errors.Wrap(err, "failed to remove HAS_TAG between Collection and CollectionTag")
		logrus.Error(err)
		return
	}

	return
}
