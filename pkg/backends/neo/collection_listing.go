package neo

import (
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) CollectionListing_Get_ByID(colID, colTagID int64) (col types.CollectionListing, err error) {
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t) WHERE id(c) = %d AND id(t) = %d`
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	query := fmt.Sprintf(q, colID, colTagID)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return col, err
	}
	if !found {
		return col, fmt.Errorf("collection with ID '%d' npt found", colID)
	}
	col, err = CollectionListingFromResult(res)
	if err != nil {
		return
	}
	// Fill Manifests
	manis, err := n.Manifest_GetByCollectionTag(col.ID, col.ColTag.ID)
	if err != nil {
		return
	}
	for _, m := range manis {
		k := fmt.Sprintf("%s/%s:%s", m.Upstream.Reg, m.Upstream.Repo, m.Upstream.Tag)
		col.Manifests[k] = m
		col.ManifestCount++
	}
	return
}

func (n Neo) CollectionListing_Get_ByRepoLike(repo string) (cols []types.CollectionListing, err error) {
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t) WHERE c.repo =~ "^%s.*"`
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	query := fmt.Sprintf(q, repo)
	res, err := n.Query(query)
	if err != nil {
		return cols, err
	}
	for _, r := range res {
		col, err := CollectionListingFromResult(r)
		if err != nil {
			return cols, err
		}
		// Fill Manifests
		manis, e := n.Manifest_GetByCollectionTag(col.ID, col.ColTag.ID)
		if e != nil {
			return cols, e
		}
		for _, m := range manis {
			k := fmt.Sprintf("%s/%s:%s", m.Upstream.Reg, m.Upstream.Repo, m.Upstream.Tag)
			col.Manifests[k] = m
			col.ManifestCount++
		}
		cols = append(cols, col)
	}
	return
}
