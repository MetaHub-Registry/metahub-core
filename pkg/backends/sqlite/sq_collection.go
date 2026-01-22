package sqlite

import (
	"fmt"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

var (
	errNotImplemented = fmt.Errorf("not implemented")
)

func (s Sqlite) CollectionGetAll() (typeCols []types.Collection, err error) {
	cols, err := s.Collection.GetAll()
	if err != nil {
		return
	}
	for _, col := range cols {
		ct := types.CollectionTag{
			Tag: col.ColTag.Tag,
		}
		c := types.Collection{
			ID:            col.ID,
			Repo:          col.Repo,
			Tag:           col.Tag,
			ColTag:        ct,
			ManifestLists: []types.ManifestList{},
		}
		typeCols = append(typeCols, c)
	}
	/// FIXME: transform to types is not implemented yet!
	err = errNotImplemented
	return
}

func (s Sqlite) CollectionGetIDByYaml(ycol yamlCfg.ManifestCollection) (colID int64, found bool, err error) {
	return 0, false, errNotImplemented
}

func (s Sqlite) CollectionCreateByYaml(ycol yamlCfg.ManifestCollection) (colID int64, err error) {
	return 0, errNotImplemented
}

func (s Sqlite) CollectionGetInsertByRequest(req types.Request) (col types.Collection, err error) {
	return s.GetInsertCollectionByRepoTag(req.Repo, req.Tag)
}

func (s Sqlite) CollectionInsertXTable(sqlID, colTagID, mlID int64) (err error) {
	return s.Collection.InsertXTable(sqlID, colTagID, mlID)
}

func (s Sqlite) CollectionManifestByRequest(req types.Request, mani types.ManifestData) (err error) {
	// Tag && Collection & ManifestList
	/*
		tag, err := mag.SQlite.CollectionTag.GetCreateByRequest(req)
		if err != nil {
			err = errors.Wrap(err, "failed to SQlite.CollectionTag.GetCreateByRequest")
			return
		}
		_ = tag
	*/
	//// Fetch current collection or create new one
	col, err := s.GetInsertCollectionByRepoTag(req.Repo, req.Tag)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.GetInsertCollectionByRepo")
		return
	}
	//logrus.Fatalf("col.Tag: %v", col.Tag)
	// Augment ManifestList to collection
	err = s.AddMLtoCollection(&col)
	if err != nil {
		err = errors.Wrap(err, "failed to AddMLtoCollection")
		return
	}
	/*
		// TODO: Add Manifest to ManifestList (which is linked to Collection)
		for _, ml := range col.ManifestLists {
			b := ml.ToOCIBytes()
			logrus.Warnf("ManifestList: %s", string(b))
		}
	*/
	ml := types.ManifestList{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
		Manifests: []types.ManifestData{
			mani,
		},
	}
	ml.Digest = digest.FromBytes(ml.ToOCIBytes()).String()
	// Add ML to SQLite
	err = s.InsertManifestList(&ml)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.ManifestList.Insert")
		return
	}
	col.ManifestLists = append(col.ManifestLists, ml)
	// Update Collection
	//err = mag.SQlite.Collection.InsertXTable(col.SqlID, col.ColTag.SqlID, ml.SqlID)
	if err != nil {
		err = errors.Wrap(err, "failed to SQlite.Collection.InsertXTable")
		return
	}
	return

}

func (s Sqlite) AddMLtoCollection(col *types.Collection) (err error) {
	q := fmt.Sprintf(`SELECT
			colID,
			colRepo,
			colTag,
			manifestlistDigest,
			manifestlistID
	FROM main.v_get_manifestlist
	WHERE colID=%d;`, col.ID)
	rows, err := s.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	type rType struct {
		ColID          int64
		ColRepo        string
		ColTag         string
		ManiListDigest string
		ManiListID     int64
	}
	for rows.Next() {
		mlRes := rType{}
		err = rows.Scan(
			&mlRes.ColID,
			&mlRes.ColRepo,
			&mlRes.ColTag,
			&mlRes.ManiListDigest,
			&mlRes.ManiListID,
		)
		if err != nil {
			logrus.Error(err)
		}

		ml := types.ManifestList{
			SchemaVersion: 2,
			MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
			ID:            int64(mlRes.ManiListID),
			Digest:        mlRes.ManiListDigest,
			Manifests:     []types.ManifestData{},
		}
		/// Add manifests to manifestlist
		manis, err := s.Manifest.TypesManifestGetByManifestListID(ml.ID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		ml.Manifests = manis
		col.ManifestLists = append(col.ManifestLists, ml)
	}
	return
}

func (s Sqlite) CollectionScrubByManifestListID(mlID int64) (err error) {
	return s.Collection.ScrubCollectionsByManifestListID(mlID)
}

func (s Sqlite) ComputeManifestlists() (err error) {
	return errNotImplemented
}

func (s Sqlite) CollectionGetTagsListByRepo(repo string) (tags types.TagResponse, err error) {
	return s.Collection.GetTagsListByRepo(repo)
}
