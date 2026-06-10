package neo

import (
	"context"
	"fmt"

	"github.com/docker/distribution/manifest/manifestlist"
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

const (
	pkgNameCol  = "neo/collection.go"
	failInsMani = "failed to insert manifest"
)

func (n Neo) CollectionGetIDByYaml(ctx context.Context, ycol yamlCfg.ManifestCollection) (colID, colTagID int64, found bool, err error) {
	ctx, span := tracer.Start(ctx, "CollectionGetIDByYaml")
	defer span.End()
	col, found, err := n.GetCollection(ctx, ycol)
	if err != nil || !found {
		return 0, 0, false, err
	}
	colID = col.ID
	colTagID = col.ColTag.ID
	return
}

func (n Neo) CollectionCreateByYaml(ctx context.Context, ycol yamlCfg.ManifestCollection) (colID, colTagID int64, err error) {
	return n.CreateCollection(ctx, ycol)
}

func (n Neo) CollectionGetByRepoTagWoManifests(ctx context.Context, repo, tag string) (resCol types.Repo, found bool, err error) {
	q := `MATCH (c:Repo)-[:HAS_TAG]->(t) WHERE c.repo = "%s" AND t.name = "%s" RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	query := fmt.Sprintf(q, repo, tag)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return resCol, found, err
	}
	resCol, err = CollectionFromResult(res)
	return
}

func (n Neo) GetCollection(ctx context.Context, col yamlCfg.ManifestCollection) (resCol types.Repo, found bool, err error) {
	_, span := tracer.Start(ctx, "GetCollection")
	defer span.End()
	q := `MATCH (c:Repo)-[:HAS_TAG]->(t) WHERE c.repo = "%s" AND t.name = "%s" RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	query := fmt.Sprintf(q, col.Name, col.Tag)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return resCol, found, err
	}
	resCol, err = CollectionFromResult(res)
	return
}

func (n Neo) CollectionGetAll(ctx context.Context) (collections []types.Repo, err error) {
	return n.GetAllCollections(ctx)
}

func (n Neo) CollectionExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "CollectionExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (c:Repo)-[:HAS_TAG]->(t:Tag) WHERE c.repo = "%s" AND t.name = "%s"`, up.Repo, up.Tag)
	if up.Digest.String() != "" {
		q = `MATCH (md:Manifest)-[]-(m:ManifestWrapper)<-[:UPSTREAM]-(c:Repo)-[:HAS_TAG]->(t:Tag)-[:UPSTREAM]->(m)`
		q += fmt.Sprintf(`WHERE c.repo = "%s" AND md.digest = "%s"`, up.Repo, up.Digest.String())
	}
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return found, utils.NewError(l, span, types.FailNeoExecute)

	}
	if !found {
		utils.Debug(l, span, types.FailCollectionNotFound)
		return found, nil
	}
	colID, err := res.GetInt64("collectionID")
	if err != nil {
		return found, utils.NewError(l, span, types.FailCollectionNotFound)
	}
	utils.Debug(l, span, "collection found: ID %d", colID)
	return found, nil
}

func (n Neo) CollectionGetByRepo(ctx context.Context, repo string) (col types.Repo, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "CollectionGetByRepo",
		"repo":    repo,
	})
	defer span.End()
	colID, colFound, e := n.CollectionGetIDByRepo(ctx, repo)
	if e != nil {
		return col, false, utils.ErrorHandler(l, span, e, types.FailCollectionNotFound)
	} else if !colFound {
		return col, false, nil
	}
	col = types.NewCollection(repo, "")
	col.ID = colID
	if found, e = n.CollectionUpdate(ctx, &col); e != nil {
		return col, false, utils.ErrorHandler(l, span, e, types.FailCollectionUpdate)
	} else if !found {
		return col, false, nil
	}
	return col, true, nil
}

func (n Neo) CollectionUpdate(ctx context.Context, col *types.Repo) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "CollectionUpdate",
		"colID":   col.ID,
		"repo":    col.Repo,
		"tag":     col.Tag,
	})
	defer span.End()
	if col.ID == -1 {
		if colID, colFound, e := n.CollectionGetIDByRepo(ctx, col.Repo); e != nil {
			return false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
		} else if !colFound {
			return false, nil
		} else {
			col.ID = colID
		}
	}
	q := fmt.Sprintf(`MATCH (c:Repo)-[:HAS_TAG]->(t:Tag) WHERE id(c) = %d AND t.name = "%s"`, col.ID, col.Tag)
	q += ` RETURN t.name AS tagName, id(t) AS tagID`
	utils.Debug(l, span, q)
	if res, found, e := n.QueryOne(q); e != nil {
		return false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else if found {
		if ct, e := CollectionTagFromResult(res); e != nil {
			return false, utils.ErrorHandler(l, span, e, "failed to derive collectionTag from result")
		} else {
			col.ColTag = ct
		}
	}
	// Index adding
	q = `MATCH (c:Repo)-[:HAS_TAG]->(t:Tag)-[:REF]->(ml:Index)-[:IMPLEMENTS]->(f)`
	q += fmt.Sprintf(` WHERE id(c) = %d AND t.name = "%s"`, col.ID, col.Tag)
	q += ` RETURN id(ml) AS manifestListID, id(f) AS filterID,`
	q += ` c.repo AS collectionRepo, t.name AS tagName, id(t) AS tagID,`
	q += ` ml.digest AS manifestListDigest, ml.mediaType AS manifestListMediaType, ml.size AS manifestListSize`
	utils.Debug(l, span, q)
	if res, e := n.Query(q); e != nil {
		return false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else {
		for _, r := range res {
			if ml, e := n.IndexFromResult(ctx, r); e != nil {
				return false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
			} else {
				ml.Manifests, e = n.ManifestGetByManifestList(ctx, ml.ID)
				if e != nil {
					return false, utils.ErrorHandler(l, span, e, types.FailManifestGetByManifestList)
				}
				col.ManifestLists = append(col.ManifestLists, ml)
			}
		}
	}
	return true, nil
}

func (n Neo) CollectionGetIDByRepo(ctx context.Context, repo string) (colID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "CollectionGetIDByRepo",
		"repo":    repo,
	})
	defer span.End()
	_ = ctx
	key := "colID"
	q := fmt.Sprintf(`MATCH (c:Repo) WHERE c.repo = "%s" RETURN id(c) AS %s`, repo, key)
	utils.Debug(l, span, q)
	if res, found, e := n.QueryOne(q); e != nil {
		return colID, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else if !found {
		return colID, false, nil
	} else {
		if colID, e = res.GetInt64(key); e != nil {
			return colID, false, utils.ErrorHandler(l, span, e, types.FailBrGet, key)
		}
	}
	return colID, true, nil
}

func (n Neo) CollectionGetByRepoTag(ctx context.Context, repo, tag string) (c types.Repo, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "CollectionGetByRepoTag",
		"repo":    repo,
		"tag":     tag,
	})
	_ = l
	defer span.End()
	q := `MATCH (c:Repo)-[:HAS_TAG]->(t:Tag)-[:REF]->(ml:Index)-[:IMPLEMENTS]->(f) WHERE c.repo = "%s" AND t.name = "%s"`
	q += ` RETURN id(c) AS collectionID, id(ml) AS manifestListID, id(f) AS filterID,`
	q += ` c.repo AS collectionRepo, t.name AS tagName, id(t) AS tagID,`
	q += ` ml.digest AS manifestListDigest, ml.mediaType AS manifestListMediaType, ml.size AS manifestListSize`
	query := fmt.Sprintf(q, repo, tag)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, query),
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
				attribute.String(attrLogSeverity, attrSevError),
				attribute.String(attrLogMessage, err.Error()),
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
				attribute.String(attrLogSeverity, attrSevError),
				attribute.String(attrLogMessage, err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
		// Get all manifests for this manifestList
		ml.Manifests, err = n.ManifestGetByManifestList(ctx, ml.ID)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.AddEvent("log", trace.WithAttributes(
				attribute.String(attrLogSeverity, attrSevError),
				attribute.String(attrLogMessage, err.Error()),
			))
			span.RecordError(err)
			l.Error(err.Error())
			return
		}
		c.ManifestLists = append(c.ManifestLists, ml)

	}
	return
}

func (n Neo) GetAllCollections(ctx context.Context) (collections []types.Repo, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "GetAllCollections",
	})
	_ = l
	defer span.End()
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (c:Repo)-[:HAS_TAG]->(t:Tag)-[:REF]->(ml:Index)-[:IMPLEMENTS]->(f) WHERE ml.label IS NOT NULL`
	q += ` MATCH (c)-[:REF]->(ml:Index)`
	q += ` RETURN id(c) AS collectionID, id(ml) AS manifestListID, id(f) AS filterID,`
	q += ` c.repo AS collectionRepo, t.name AS tagName, id(t) AS tagID,`
	q += ` ml.digest AS manifestListDigest, ml.mediaType AS manifestListMediaType, ml.size AS manifestListSize`
	utils.Debug(l, span, q)
	input := map[string]interface{}{}
	result, e := session.Run(q, input)
	if e != nil {
		return collections, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	}
	cols := map[int64]types.Repo{}
	collections = make([]types.Repo, 0)
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
		ml.Manifests, err = n.ManifestGetByManifestList(ctx, ml.ID)
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

func (n Neo) CollectionCreateByRequest(req types.Request) (col types.Repo, err error) {
	ctx := context.Background()
	col, _, err = n.CollectionGetCreateByRepoTag(ctx, req.Repo, req.Tag)
	return
}

func (n Neo) CollectionGetCreateByRepoTag(ctx context.Context, repo, tag string) (c types.Repo, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "neo/collection.go",
		"func":    "CollectionGetCreateByRepoTag",
		"repo":    repo,
		"tag":     tag,
	})
	defer span.End()
	ct, err := n.CollectionTagGetCreate(ctx, tag)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get collection tag")
		return
	}
	c, found, err = n.CollectionGetCreateByRepo(ctx, repo)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to get collection")
		return
	}
	c.ColTag = ct
	c.Tag = tag
	q := `MATCH (c:Repo), (t:Tag) WHERE id(c) = %d AND id(t) = %d`
	q += ` MERGE (c)-[:HAS_TAG]->(t)`
	query := fmt.Sprintf(q, c.ID, ct.ID)
	utils.Debug(l, span, query)
	_, err = n.Execute(query)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to connect collection and tag")
		return
	}
	utils.Debug(l, span, "Found Repo and tag")
	span.SetStatus(codes.Ok, "Found collection and tag")
	return
}

// RepoLinkTag ensures a Repo -[:HAS_TAG]-> Tag edge exists. Both nodes are
// MERGEd first so the function is safe to call even before any other path
// has created them. Idempotent — calling it twice is a no-op.
func (n Neo) RepoLinkTag(ctx context.Context, repo, tag string) error {
	q := fmt.Sprintf(`MERGE (c:Repo {repo: "%s"}) MERGE (t:Tag {name: "%s"}) MERGE (c)-[:HAS_TAG]->(t)`, repo, tag)
	if _, err := n.Execute(q); err != nil {
		return fmt.Errorf("RepoLinkTag: %w", err)
	}
	return nil
}

func (n Neo) CollectionGetCreateByRepo(ctx context.Context, repo string) (c types.Repo, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "CollectionGetCreateByRepo",
		"repo":    repo,
	})
	defer span.End()
	c, found, err = n.CollectionGetByRepo(ctx, repo)
	if err != nil {
		return
	}
	if !found {
		q := `CREATE (c:Repo {repo: "%s"}) RETURN id(c) AS collectionID`
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
	c, _, err := n.CollectionGetCreateByRepoTag(ctx, col.Name, col.Tag)
	if err != nil {
		return
	}
	colId = c.ID
	colTagId = c.ColTag.ID
	return
}

func CollectionFromRecord(res neo4j.Result) (c types.Repo, err error) {
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
	c.ManifestLists = make([]types.Index, 0)
	return
}

func ManifestListFromRecord(res neo4j.Result) (ml types.Index, err error) {
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
	ml.Manifests = make([]types.Manifest, 0)
	return
}

func (n Neo) LinkCollectionToManifest(ctx context.Context, colID, maniID int64) (err error) {
	_, span := tracer.Start(ctx, "LinkCollectionToManifest")
	defer span.End()
	span.SetAttributes(
		attribute.Key("colID").Int64(colID),
		attribute.Key("maniID").Int64(maniID),
	)
	q := `MATCH (c:Repo), (m:Manifest) WHERE id(c) = %d AND id(m) = %d`
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

func (n Neo) GetCollectionFilterPairsForCollection(ctx context.Context, col types.Repo) (cfps []types.CollectionFilterPair, err error) {
	// Platform and filters need to be reworked
	q := `MATCH (t:Tag)-[:UPSTREAM]-(m:ManifestWrapper)-[:HAS_PLATFORM]->(Platform)<-[:USES]-(f:Filter)`
	q += ` MATCH (c:Repo)-[:UPSTREAM]->(m)`
	q += ` WHERE c.repo = "%s" AND t.name = "%s"`
	query := fmt.Sprintf(q, col.Repo, col.ColTag.Tag)
	//fmt.Printf("Query GetCollectionFilterPairsForCollection: \n%s\n", query)
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
		q := `MATCH (c:Repo)-[:HAS_TAG]->(t:Tag)-[:UPSTREAM]-(m:ManifestWrapper)-[:HAS_PLATFORM]->(p:Platform)<-[:USES]-(f:Filter)`
		q += ` MATCH (cfg:Blob)<-[:USES]-(md:Manifest)-[]-(m)-[:UPSTREAM]->(u:Upstream)`
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
			Repo: types.Repo{
				ID:     t.collectionID,
				Repo:   t.colRepo,
				Tag:    t.tagName,
				ColTag: ct,
			},
			Filter: types.Filter{
				ID:   t.filterID,
				Name: t.filterLabel,
			},
			Manifests: make([]types.Manifest, 0),
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

func (n Neo) GetCollectionFilterPairs(ctx context.Context) (cfps []types.CollectionFilterPair, err error) {
	q := `MATCH (c:Repo)-[:HAS_TAG]->(t:Tag)-[:UPSTREAM]-(m:ManifestWrapper)-[:HAS_PLATFORM]->(Platform)<-[:USES]-(f:Filter)`
	q += ` MATCH (c)-[:UPSTREAM]->(m)`
	//logrus.Infof("query: %s", q)
	return n.getCollectionFilterPairs(ctx, q)
}

func (n Neo) CollectionGetInsertByRequest(ctx context.Context, req types.Request) (col types.Repo, err error) {
	col, found, err := n.CollectionGetByRepoTag(ctx, req.Repo, req.Tag)
	if err != nil {
		logrus.Error(err)
		return
	}
	if found {
		return col, nil
	}
	col, err = n.CollectionCreateByRequest(req)
	return
}

func CollectionFromResult(res types.Result) (col types.Repo, err error) {
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
	col = types.Repo{
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
		Manifests:     map[string]types.Manifest{},
		ManifestCount: 0,
	}
	return
}

func (n Neo) CollectionRemove(ctx context.Context, col types.Repo) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "CollectionRemove",
		"repo":    col.Repo,
		"tag":     col.Tag,
	})
	defer span.End()
	if found, e := n.CollectionUpdate(ctx, &col); e != nil {
		return false, utils.ErrorHandler(l, span, e, "failed to get collection id")
	} else if !found {
		return false, nil
	}
	// Remove CollectionTag link to Manifest
	for _, ml := range col.ManifestLists {
		err = n.UnLinkManifestListToCollection(ml.ID, col.ID)
		if err != nil {
			err = errors.Wrap(err, "failed to UnLinkManifestListToCollection")
			logrus.Error(err)
			return
		}
		err = n.UnLinkManifestListToCollectionTag(ml.ID, col.ColTag.ID)
		if err != nil {
			err = errors.Wrap(err, "failed to UnLinkManifestListToCollectionTag")
			logrus.Error(err)
			return
		}
	}
	// Remove HAS_TAG between Repo and CollectionTag
	err = n.CollectionTagDetachFromCollection(col.ID, col.ColTag.ID)
	if err != nil {
		err = errors.Wrap(err, "failed to remove HAS_TAG between Repo and CollectionTag")
		logrus.Error(err)
		return
	}
	found = true
	return
}

func (n Neo) CollectionGetCreateUpstreamTriple(ctx context.Context, ups types.Upstream, maniIn manifestlist.ManifestDescriptor) (upID, maniID int64, prf types.Profile, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "CollectionGetCreateUpstreamTriple",
		"repo":    ups.Repo,
		"tag":     ups.Tag,
		"profile": ups.Profile,
	})
	defer span.End()
	_, e := n.UpstreamGetInsert(ctx, &ups)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to UpstreamGetInsert")
		return
	}
	upID = ups.ID

	prf, prfFound, e := n.ProfileGetByLabel(ctx, ups.Profile)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to create collection")
		return
	}
	if !prfFound {
		err = errors.Errorf("profile '%s' not found", ups.Profile)
		l.Error(err.Error())
		span.RecordError(err)
		return
	}
	// deprecate-manifest-wrapper Phase 3: the legacy wrapper creation
	// (ManifestWrapperGetInsertWithDesc → CREATE (:ManifestWrapper)) is removed.
	// This entrypoint now resolves only Upstream + Profile; the content-leaf
	// :Manifest (carrying the descriptor's platform) and its Slot are wired at
	// ingest time via wireSlotOnPush, not here. We deliberately do NOT MERGE a
	// :Repo/:Slot in this resolver so the caller's later
	// CollectionGetCreateByRepoTag keeps its create-vs-find semantics.
	//
	// `maniID` is left zero: its sole production caller
	// (upstreamBlobInsertManifestDesc) is disabled, and no caller (incl. the
	// CollectionGetCreateUpstreamTriple test) consumes it.
	_ = maniIn
	return

}
