package neo

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (n Neo) CollectionListing_Get_ByID(ctx context.Context, colID, colTagID int64) (col types.CollectionListing, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package":  "neo/collection_listing.go",
		"func":     "CollectionListing_Get_ByID",
		"colID":    colID,
		"colTagID": colTagID,
	})
	ctx, span := tracer.Start(ctx, "CollectionListing_Get_ByID")
	defer span.End()
	_ = ctx
	_ = l
	q := fmt.Sprintf(`MATCH (c:Collection)-[:HAS_TAG]->(t) WHERE id(c) = %d AND id(t) = %d`, colID, colTagID)
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	l.Debug(q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return col, err
	}
	if !found {
		err = utils.NewError(l, span, fmt.Sprintf("collection with ID '%d' npt found", colID))
		return col, err
	}
	col, err = CollectionListingFromResult(res)
	if err != nil {
		utils.ErrorHandler(l, span, err, "while executing CollectionListingFromResult")
		return
	}
	// Fill Manifests
	manis, err := n.ManifestData_GetByCollectionTag(ctx, col.ID, col.ColTag.ID)
	if err != nil {
		utils.ErrorHandler(l, span, err, "while executing ManifestData_GetByCollectionTag")
		return
	}
	for _, m := range manis {
		k := m.Label
		utils.Debug(l, span, fmt.Sprintf("Add manifest '%s'", k))
		col.Manifests[k] = m
		col.ManifestCount++
	}
	return
}

func (n Neo) CollectionListing_Get_ByName(ctx context.Context, repoName, colTagName string) (col types.CollectionListing, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/collection_listing.go",
		"func":    "CollectionListing_Get_ByName",
		"repo":    repoName,
		"tag":     colTagName,
	})
	ctx, span := tracer.Start(ctx, "CollectionListing_Get_ByName")
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MATCH (c:Collection)-[:HAS_TAG]->(t) WHERE c.repo = "%s" AND t.name = "%s"`, repoName, colTagName)
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil {
		return col, err
	}
	if !found {
		return col, fmt.Errorf("collection with name '%s' and tag '%s' not found", repoName, colTagName)
	}
	col, err = CollectionListingFromResult(res)
	if err != nil {
		utils.ErrorHandler(l, span, err, "while executing CollectionListingFromResult")
		return
	}
	msg := fmt.Sprintf("CL > %d / %d", col.ID, col.ColTag.ID)
	utils.Debug(l, span, msg)
	// Fill Manifests
	manis, err := n.ManifestData_GetByCollectionTag(ctx, col.ID, col.ColTag.ID)
	if err != nil {
		utils.ErrorHandler(l, span, err, "while executing ManifestData_GetByCollectionTag")
		return
	}
	for _, m := range manis {
		k := m.Label //fmt.Sprintf("%s/%s:%s@%s-%s", m.Upstream.Reg, m.Upstream.Repo, m.Upstream.Tag, m.Profile, m.Platform.Arch)
		utils.Debug(l, span, fmt.Sprintf("Add manifest '%s'", k))
		col.Manifests[k] = m
		col.ManifestCount++
	}
	return
}

func (n Neo) CollectionListingGetByRepoLike(ctx context.Context, repo string) (cols []types.CollectionListing, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "neo/collection_listing.go",
		"func":    "CollectionListingGetByRepoLike",
		"repo":    repo,
	})
	ctx, span := tracer.Start(ctx, "CollectionListingGetByRepoLike")
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(repo),
	)
	q := `MATCH (c:Collection)-[:HAS_TAG]->(t) WHERE c.repo =~ "^%s.*"`
	q += ` RETURN id(c) AS collectionID, id(t) AS tagID, t.name AS tagName, c.repo AS collectionRepo`
	query := fmt.Sprintf(q, repo)
	utils.Debug(l, span, query)
	res, err := n.Query(query)
	if err != nil {
		utils.ErrorHandler(l, span, err, "while executing Query")
		return cols, err
	}
	for _, r := range res {
		col, err := CollectionListingFromResult(r)
		if err != nil {
			utils.ErrorHandler(l, span, err, "while executing CollectionListingFromResult")
			return cols, err
		}
		// Fill Manifests
		manis, e := n.ManifestData_GetByCollectionTag(ctx, col.ID, col.ColTag.ID)
		if e != nil {
			utils.ErrorHandler(l, span, err, "while executing ManifestData_GetByCollectionTag")
			return cols, e
		}
		for _, m := range manis {
			k := fmt.Sprintf("%s/%s:%s-%s", m.Upstream.Reg, m.Upstream.Repo, m.Upstream.Tag, m.Platform.Arch)
			utils.Debug(l, span, fmt.Sprintf("Add manifest '%s'", k))
			col.Manifests[k] = m
			col.ManifestCount++
		}
		cols = append(cols, col)
	}
	return
}
