package magazine

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) ComputeManifestlists() (err error) {
	return mag.Database.ComputeManifestlists()
}

func (mag Magazine) ManifestListGetByRequest(req types.Request) (ml types.ManifestList, found bool, err error) {
	// Fetch ML from SQLite
	ml, found, err = mag.Database.ManifestList_Get_ByRequest(req)
	if err != nil {
		logrus.Errorf("Failed to get ManifestList from Database: %v", err)
		return
	}
	if !found {
		// We did not find a ManifestList
		return
	}
	return
}

func (mag Magazine) ManifestList_GetByHTTPRequest(r *http.Request) (ml types.ManifestList, found bool, err error) {
	req := types.NewRequest(r)
	msg := fmt.Sprintf("%-5s ManifestListHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	if req.Profile != "generic" {
		msg += fmt.Sprintf(" - profile: %s", req.Profile)
	}
	logrus.Info(msg)
	ml, found, err = mag.ManifestListGetByRequest(req)
	if err != nil {
		return
	}
	return
}

/*
func (mag Magazine) AddMLtoCollection(col *types.Collection) (err error) {
	q := fmt.Sprintf(`SELECT
			colID,
			colRepo,
			colTag,
			manifestlistDigest,
			manifestlistID
	FROM main.v_get_manifestlist
	WHERE colID=%d;`, col.SqlID)
	rows, err := mag.SQlite.Query(q)
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
			SqlID:         int64(mlRes.ManiListID),
			Digest:        mlRes.ManiListDigest,
			Manifests:     []types.Manifest{},
		}
		/// Add manifests to manifestlist
		manis, err := mag.SQlite.Manifest.TypesManifest_GetByManifestListID(ml.SqlID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		ml.Manifests = manis
		col.ManifestLists = append(col.ManifestLists, ml)
	}
	return
}
*/
