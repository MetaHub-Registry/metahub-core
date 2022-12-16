package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) CollectionTag_Create(tag string) (ct types.CollectionTag, err error) {
	q := `CREATE (t:Tag {name: "%s"}) RETURN id(t) AS tagID, t.name AS tagName`
	query := fmt.Sprintf(q, tag)
	res, err := n.Execute(query)
	if err != nil {
		return
	}
	return CollectionTagFromResult(res)
}

func (n Neo) CollectionTag_GetCreate(tag string) (ct types.CollectionTag, err error) {
	ct, found, err := n.CollectionTag_GetByTag(tag)
	if err != nil {
		return
	}
	if found {
		return ct, nil
	}
	return n.CollectionTag_Create(tag)
}

func (n Neo) LinkCollectionTagToManifest(ctx context.Context, colTagID, maniID int64) (err error) {
	q := `MATCH (t:Tag), (m:Manifest) WHERE id(t) = %d AND id(m) = %d`
	q += ` MERGE (t)-[:UPSTREAM]->(m)`
	query := fmt.Sprintf(q, colTagID, maniID)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	_, err = session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create link collectionTag->manifest: %v", err)
		return
	}
	return
}

func (n Neo) CollectionTag_GetByTag(tag string) (ct types.CollectionTag, found bool, err error) {
	q := `MATCH (t:Tag {name: "%s"}) RETURN id(t) AS tagID, t.name AS tagName`
	query := fmt.Sprintf(q, tag)
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

func (n Neo) Collection_GetTagsListByRepo(repo string) (tags types.TagResponse, err error) {
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t:Tag) WHERE c.repo = "%s"`
	q += ` RETURN id(t) AS tagID, t.name AS tagName`
	query := fmt.Sprintf(q, repo)
	res, err := n.Query(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	tags = types.TagResponse{
		Name: repo,
		Tags: make([]string, 0),
	}
	for _, r := range res {
		var ct types.CollectionTag
		ct, err = CollectionTagFromResult(r)
		if err != nil {
			logrus.Error(err)
			return
		}
		tags.Tags = append(tags.Tags, ct.Tag)
	}
	return
}

func CollectionTagFromResult(res Result) (ct types.CollectionTag, err error) {
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
