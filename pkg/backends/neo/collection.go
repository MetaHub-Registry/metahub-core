package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (n Neo) Collection_GetID_ByYaml(ctx context.Context, ycol yamlCfg.ManifestCollection) (colID, colTagID int64, found bool, err error) {
	ctx, span := tracer.Start(ctx, "Collection_GetID_ByYaml")
	defer span.End()
	col, found, err := n.GetCollection(ctx, ycol)
	if err != nil || !found {
		return 0, 0, false, err
	}
	colID = col.ID
	colTagID = col.ColTag.ID
	return
}

func (n Neo) Collection_Create_ByYaml(ctx context.Context, ycol yamlCfg.ManifestCollection) (colID, colTagID int64, err error) {
	return n.CreateCollection(ctx, ycol)
}

func (n Neo) Collection_Get_ByRepoTag_woManifests(ctx context.Context, repo, tag string) (resCol types.Collection, found bool, err error) {
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t) WHERE c.repo = "%s" AND t.name = "%s" RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	query := fmt.Sprintf(q, repo, tag)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return resCol, found, err
	}
	resCol, err = CollectionFromResult(res)
	return
}

func (n Neo) GetCollection(ctx context.Context, col yamlCfg.ManifestCollection) (resCol types.Collection, found bool, err error) {
	_, span := tracer.Start(ctx, "GetCollection")
	defer span.End()
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t) WHERE c.repo = "%s" AND t.name = "%s" RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	query := fmt.Sprintf(q, col.Name, col.Tag)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return resCol, found, err
	}
	resCol, err = CollectionFromResult(res)
	return
}

func (n Neo) CollectionGetAll(ctx context.Context) (collections []types.Collection, err error) {
	return n.GetAllCollections(ctx)
}

func (n Neo) CollectionExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/collection.go",
		"func":    "CollectionExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	_, span := tracer.Start(ctx, "CollectionExists")
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("digest").String(utils.ShortenDigest(up.Digest.String())),
	)

	q := fmt.Sprintf(`MATCH (c:Collection)-[:HAS_TAG]->(t:Tag) WHERE c.repo = "%s" AND t.name = "%s"`, up.Repo, up.Tag)
	if up.Digest.String() != "" {
		q = `MATCH (md:ManifestData)-[]-(m:Manifest)<-[:UPSTREAM]-(c:Collection)-[:HAS_TAG]->(t:Tag)-[:UPSTREAM]->(m)`
		q += fmt.Sprintf(`WHERE c.repo = "%s" AND md.digest = "%s"`, up.Repo, up.Digest.String())
	}
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, found, err := n.QueryOne(q)
	if err != nil {
		msg := "query execution failed"
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		l.Error(msg)
		err = errors.Wrap(err, msg)
		return found, err
	}
	if !found {
		msg := "collection not found"
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		return found, nil
	}
	colID, err := res.GetInt64("collectionID")
	if err != nil {
		msg := "colID could not be found"
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		err = errors.Wrap(err, msg)
		l.Error(msg)
		return found, err
	}
	msg := fmt.Sprintf("collection found: ID %d", colID)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
	return found, nil
}

func (n Neo) Collection_Get_ByRepoTag(ctx context.Context, repo, tag string) (c types.Collection, found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/collection.go",
		"func":    "Collection_Get_ByRepoTag",
		"repo":    repo,
		"tag":     tag,
	})
	_ = l
	ctx, span := tracer.Start(ctx, "ManifestData_GetByManifestList")
	defer span.End()
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t:Tag)-[:REF]->(ml:ManifestList)-[:IMPLEMENTS]->(f) WHERE c.repo = "%s" AND t.name = "%s"`
	q += ` RETURN id(c) AS collectionID, id(ml) AS manifestListID, id(f) AS filterID,`
	q += ` c.repo AS collectionRepo, t.name AS tagName, id(t) AS tagID,`
	q += ` ml.digest AS manifestListDigest, ml.mediaType AS manifestListMediaType, ml.size AS manifestListSize`
	query := fmt.Sprintf(q, repo, tag)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", query),
	))
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
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
		ml, e := ManifestListFromRecord(result)
		if e != nil {
			err = e
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
		// Get all manifests for this manifestList
		ml.Manifests, err = n.ManifestData_GetByManifestList(ctx, ml.ID)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "error"),
				attribute.String("log.message", err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
		c.ManifestLists = append(c.ManifestLists, ml)

	}
	return
}

func (n Neo) GetAllCollections(ctx context.Context) (collections []types.Collection, err error) {
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
		ml.Manifests, err = n.ManifestData_GetByManifestList(ctx, ml.ID)
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
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/collection.go",
		"func":    "Collection_GetCreate_ByRepoTag",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()
	ct, err := n.CollectionTag_GetCreate(ctx, tag)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get collection tag")
		return
	}
	c, found, err = n.Collection_GetCreate_ByRepo(ctx, repo)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get collection")
		return
	}
	c.ColTag = ct
	c.Tag = tag
	q := `MATCH (c:Collection), (t:Tag) WHERE id(c) = %d AND id(t) = %d`
	q += ` MERGE (c)-[:HAS_TAG]->(t)`
	query := fmt.Sprintf(q, c.ID, ct.ID)
	_, err = n.Execute(query)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to connect collection and tag")
		return
	}
	span.SetStatus(codes.Ok, "Found collection and tag")
	return
}

func (n Neo) Collection_Get_ByRepo(ctx context.Context, repo string) (c types.Collection, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/collection.go",
		"func":    "Collection_Get_ByRepo",
		"repo":    repo,
	})
	defer span.End()
	_ = ctx
	q := `MATCH (c:Collection {repo: "%s"}) RETURN id(c) AS collectionID`
	query := fmt.Sprintf(q, repo)
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(query)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to execute query")
		return
	}
	if found {
		colId, e := res.GetInt64("collectionID")
		if e != nil {
			err = utils.ErrorHandler(l, span, err, "failed to get collectionID from result")
			return c, found, err
		}
		c.ID = colId
		c.Repo = repo
		return
	}
	return
}

func (n Neo) Collection_GetCreate_ByRepo(ctx context.Context, repo string) (c types.Collection, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/collection.go",
		"func":    "Collection_GetCreate_ByRepo",
		"repo":    repo,
	})
	defer span.End()
	c, found, err = n.Collection_Get_ByRepo(ctx, repo)
	if err != nil {
		return
	}
	if !found {
		q := `CREATE (c:Collection {repo: "%s"}) RETURN id(c) AS collectionID`
		query := fmt.Sprintf(q, repo)
		res, err := n.Execute(query)
		if err != nil {
			err = utils.ErrorHandler(l, span, err, "failed to create collection")
			return c, found, err
		}
		colId, e := res.GetInt64("collectionID")
		if e != nil {
			err = utils.ErrorHandler(l, span, err, "failed to get collectionID from result")
			return c, found, err
		}
		c.ID = colId
		c.Repo = repo
	}
	utils.Debug(l, span, "found or created collection w/ id: %d", c.ID)
	return
}

func (n Neo) CreateCollection(ctx context.Context, col yamlCfg.ManifestCollection) (colId, colTagId int64, err error) {
	ctx, span := tracer.Start(ctx, "CreateCollection")
	defer span.End()
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
	_, span := tracer.Start(ctx, "LinkCollectionToManifest")
	defer span.End()
	span.SetAttributes(
		attribute.Key("colID").Int64(colID),
		attribute.Key("maniID").Int64(maniID),
	)
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

func (n Neo) GetCollectionFilterPairs_ForCollection(ctx context.Context, col types.Collection) (cfps []types.CollectionFilterPair, err error) {
	// Platform and filters need to be reworked
	q := `MATCH (t:Tag)-[:UPSTREAM]-(m:Manifest)-[:IS]->(Platform)<-[:USES]-(f:Filter)`
	q += ` MATCH (c:Collection)-[:UPSTREAM]->(m)`
	q += ` WHERE c.repo = "%s" AND t.name = "%s"`
	query := fmt.Sprintf(q, col.Repo, col.ColTag.Tag)
	//fmt.Printf("Query GetCollectionFilterPairs_ForCollection: \n%s\n", query)
	return n.getCollectionFilterPairs(ctx, query)
}

func (n Neo) getCollectionFilterPairs(ctx context.Context, q string) (cfps []types.CollectionFilterPair, err error) {
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
		q += ` ` + UpstreamReturn + `,`
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
			c, e := n.ConfigFromRecord(ctx, result)
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

func (n Neo) GetCollectionFilterPairs(ctx context.Context) (cfps []types.CollectionFilterPair, err error) {
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t:Tag)-[:UPSTREAM]-(m:Manifest)-[:IS]->(Platform)<-[:USES]-(f:Filter)`
	q += ` MATCH (c)-[:UPSTREAM]->(m)`
	//logrus.Infof("query: %s", q)
	return n.getCollectionFilterPairs(ctx, q)
}

func (n Neo) Collection_GetInsert_ByRequest(ctx context.Context, req types.Request) (col types.Collection, err error) {
	col, found, err := n.Collection_Get_ByRepoTag(ctx, req.Repo, req.Tag)
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
		logrus.Error(err)
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
	c, found, err := n.Collection_Get_ByRepoTag(ctx, col.Repo, col.Tag)
	if err != nil {
		err = errors.Wrap(err, "failed to Collection_GetCreate_ByRepoTag")
		logrus.Error(err)
		return
	}
	if !found {
		// Might not be a full collection
		c, found, err = n.Collection_Get_ByRepoTag_woManifests(ctx, col.Repo, col.Tag)
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
