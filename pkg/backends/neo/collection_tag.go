package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
)

const (
	pkgNameColTag = "neo/collection_tag.go"
)

func (n Neo) CollectionTagGetIDByManifestID(ctx context.Context, maniID int64) (tagID int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameColTag,
		"func":    "CollectionTagGetIDByManifestID",
		"maniID":  maniID,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (m:ManifestWrapper)<-[]-(t:Tag) WHERE id(m) = %d`, maniID)
	q += ` RETURN id(t) AS ID`
	utils.Debug(l, span, "%s", q)
	if res, found, e := n.QueryOne(q); e != nil {
		return tagID, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else {
		if !found {
			utils.Debug(l, span, "did not found tag connected to maniID %d", maniID)
			return tagID, false, nil
		}
		if tagID, e = res.GetInt64("ID"); e != nil {
			return tagID, false, utils.ErrorHandler(l, span, e, types.FailBrGet, "ID")
		} else {
			utils.Debug(l, span, "found tag w/ ID %d", tagID)

			return tagID, true, nil
		}
	}
}

func (n Neo) CollectionTagCleanupByID(ctx context.Context, tagID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameColTag,
		"func":    "CollectionTagCleanupByID",
		"tagID":   tagID,
	})
	_ = ctx
	defer span.End()
	key := "countTag"
	q := fmt.Sprintf(`MATCH (m:ManifestWrapper)<-[]-(t:Tag) WHERE id(t) = %d`, tagID)
	q += fmt.Sprintf(` RETURN count(t) AS %s`, key)
	var countUpstream int
	if res, _, e := n.QueryOne(q); e != nil {
		return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else {
		if countUpstream, e = res.GetInt(key); e != nil {
			return utils.ErrorHandler(l, span, e, types.FailBrGet, key)
		}
	}
	utils.Debug(l, span, "Tag has '%d' relationships to Manifests", countUpstream)
	if countUpstream == 0 {
		q := fmt.Sprintf(`MATCH (t:Tag) WHERE id(t) = %d DETACH DELETE t`, tagID)
		if _, e := n.Execute(q); e != nil {
			return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
		}
	}
	return nil
}

func (n Neo) CollectionTagCreate(repo, tag string) (ct types.CollectionTag, err error) {
	// Tag identity is per-collection ({name, repo}). A bare `latest` must NOT be
	// shared across collections: a global Tag node made every collection list
	// every other collection's manifests via Repo→HAS_TAG→Tag→REF→Index. MERGE
	// (not CREATE) so this converges with the Tag that SlotGetCreate/RepoLinkTag
	// MERGE for the same (repo, tag).
	q := `MERGE (t:Tag {name: "%s", repo: "%s"}) RETURN id(t) AS tagID, t.name AS tagName`
	query := fmt.Sprintf(q, tag, repo)
	res, err := n.Execute(query)
	if err != nil {
		return
	}
	return CollectionTagFromResult(res)
}

func (n Neo) CollectionTagGetCreate(ctx context.Context, repo, tag string) (ct types.CollectionTag, err error) {
	_, span := tracer.Start(ctx, "CollectionTagGetCreate")
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(repo),
		attribute.Key("tag").String(tag),
	)
	ct, found, err := n.CollectionTagGetByTag(repo, tag)
	if err != nil {
		return
	}
	if found {
		return ct, nil
	}
	return n.CollectionTagCreate(repo, tag)
}

func (n Neo) LinkCollectionTagToManifest(ctx context.Context, colTagID, maniID int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameColTag,
		"func":     "LinkCollectionTagToManifest",
		"colTagID": colTagID,
		"maniID":   maniID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (t:Tag), (m:Manifest) WHERE id(t) = %d AND id(m) = %d`
	q += ` MERGE (t)-[:UPSTREAM]->(m)`
	query := fmt.Sprintf(q, colTagID, maniID)
	utils.Debug(l, span, "%s", query)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link collectionTag->manifest: %v", err)
		return
	}
	return
}

func (n Neo) LinkCollectionTagToManifestRemove(ctx context.Context, colTagID, maniID int64) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameColTag,
		"func":     "LinkCollectionTagToManifestRemove",
		"colTagID": colTagID,
		"maniID":   maniID,
	})
	_ = ctx
	defer span.End()
	q := `MATCH (t:Tag)-[u:UPSTREAM]->(m:ManifestWrapper) WHERE id(t) = %d AND id(m) = %d`
	q += ` DELETE u`
	query := fmt.Sprintf(q, colTagID, maniID)
	utils.Debug(l, span, "%s", query)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to delete link collectionTag->manifest: %v", err)
		return
	}
	return
}

func (n Neo) CollectionTagGetByTag(repo, tag string) (ct types.CollectionTag, found bool, err error) {
	q := `MATCH (t:Tag {name: "%s", repo: "%s"}) RETURN id(t) AS tagID, t.name AS tagName`
	query := fmt.Sprintf(q, tag, repo)
	res, err := n.Query(query)
	if err != nil {
		return
	}
	switch len(res) {
	case 0:
		return
	case 1:
		found = true
		ct, err = CollectionTagFromResult(res[0])
	default:
		err = fmt.Errorf("found %d tags with name '%s'", len(res), tag)
		return
	}
	return
}

func (n Neo) CollectionGetTagsListByRepo(ctx context.Context, repo string) (tags types.TagResponse, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameColTag,
		"func":    "CollectionGetTagsListByRepo",
	})
	defer span.End()
	_ = ctx
	tags = types.TagResponse{
		Name: repo,
		Tags: []string{},
	}
	var colID int64
	key := "colID"
	q := fmt.Sprintf(`MATCH (c:Repo) WHERE c.repo = "%s" RETURN id(c) AS %s`, repo, key)
	utils.Debug(l, span, "%s", q)
	if res, found, e := n.QueryOne(q); e != nil {
		return tags, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	} else if !found {
		return tags, utils.NewError(l, span, "collection '%s' not found", repo)
	} else {
		if colID, e = res.GetInt64(key); e != nil {
			return tags, utils.ErrorHandler(l, span, e, types.FailBrGet, key)
		}
	}
	q = fmt.Sprintf(`MATCH (c:Repo)-[:HAS_TAG]->(t:Tag) WHERE id(c) = %d`, colID)
	q += ` RETURN id(t) AS tagID, t.name AS tagName`
	utils.Debug(l, span, "%s", q)
	if res, err := n.Query(q); err != nil {
		return tags, utils.ErrorHandler(l, span, err, types.FailNeoExecute)
	} else {
		for _, r := range res {
			var ct types.CollectionTag
			ct, e := CollectionTagFromResult(r)
			if e != nil {
				err = utils.ErrorHandler(l, span, e, types.FailCollectionTagFromResult)
				return tags, err
			}
			if ct.Tag == "" {
				continue
			}
			utils.Debug(l, span, "found tag '%s'", ct.Tag)
			tags.Tags = append(tags.Tags, ct.Tag)
		}
	}
	return
}

func CollectionTagFromResult(res types.Result) (ct types.CollectionTag, err error) {
	err = res.CheckKeys("tagID", "tagName")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("tagID")
	name, _ := res.GetStr("tagName")
	ct = types.CollectionTag{
		ID:  id,
		Tag: name,
	}
	return
}

func (n Neo) CollectionTagCountRelationships(ctID int64) (count int64, err error) {
	q := `MATCH (t:Tag)-[r]->() WHERE id(t) = %d RETURN count(r) AS count`
	query := fmt.Sprintf(q, ctID)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return
	}
	if !found {
		err = errors.New("failed to count relationships from Tag")
		logrus.Error(err)
		return
	}
	count, err = res.GetInt64("count")
	if err != nil {
		err = errors.Wrap(err, "failed to get count from result")
		return
	}
	return
}

func (n Neo) CollectionTagDetachFromCollection(cID, ctID int64) (err error) {
	q := `MATCH (c:Repo)-[r:HAS_TAG]->(t:Tag) WHERE id(c) = %d AND id(t) = %d`
	q += ` DELETE r`
	query := fmt.Sprintf(q, cID, ctID)
	//fmt.Printf("!!! Query: \n%s\n", query)
	_, err = n.Execute(query)
	if err != nil {
		return
	}
	// Count relationships from Tag
	count, err := n.CollectionTagCountRelationships(ctID)
	if count == 0 {
		// Delete Tag
		q = `MATCH (t:Tag) WHERE id(t) = %d DELETE t`
		query = fmt.Sprintf(q, ctID)
		_, err = n.Execute(query)
	}
	return
}

// CollectionTagSetLastRefreshed stamps last_refreshed (Unix seconds) on the
// (repo, tag) Tag node. Called at pull-through ingest so the TTL logic has a
// per-alias freshness marker (decision D4). Additive: push-path nodes simply
// never carry the property.
func (n Neo) CollectionTagSetLastRefreshed(ctx context.Context, repo, tag string, ts int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameColTag,
		"func":    "CollectionTagSetLastRefreshed",
		"repo":    repo,
		"tag":     tag,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (t:Tag {name: "%s", repo: "%s"}) SET t.last_refreshed = %d`, tag, repo, ts)
	utils.Debug(l, span, "%s", q)
	if _, e := n.Execute(q); e != nil {
		return utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	}
	return nil
}

// CollectionTagGetLastRefreshed returns last_refreshed (Unix seconds) for the
// (repo, tag) Tag. found is false when the Tag node is absent. A returned 0
// means "never fetched" and MUST be treated as TTL-expired (decision D4).
func (n Neo) CollectionTagGetLastRefreshed(ctx context.Context, repo, tag string) (ts int64, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameColTag,
		"func":    "CollectionTagGetLastRefreshed",
		"repo":    repo,
		"tag":     tag,
	})
	_ = ctx
	defer span.End()
	q := fmt.Sprintf(`MATCH (t:Tag {name: "%s", repo: "%s"}) RETURN coalesce(t.last_refreshed, 0) AS last_refreshed`, tag, repo)
	utils.Debug(l, span, "%s", q)
	res, f, e := n.QueryOne(q)
	if e != nil {
		return 0, false, utils.ErrorHandler(l, span, e, types.FailNeoExecute)
	}
	if !f {
		return 0, false, nil
	}
	ts, e = res.GetInt64("last_refreshed")
	if e != nil {
		return 0, false, utils.ErrorHandler(l, span, e, types.FailBrGet, "last_refreshed")
	}
	return ts, true, nil
}
