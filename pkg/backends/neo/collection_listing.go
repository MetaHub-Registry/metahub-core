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
		cols = append(cols, col)
	}
	return
}
