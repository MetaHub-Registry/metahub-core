package magazine

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) ComputeManifestlists() (err error) {
	colFilPairs, err := mag.Neo.GetCollectionFilterPairs()
	if err != nil {
		return
	}
	for _, colFilPair := range colFilPairs {
		colID := colFilPair.Collection.NeoID
		filterID := colFilPair.Filter.NeoID
		_ = filterID
		logrus.Warnf("Collection %s:%s - %s", colFilPair.Collection.Repo, colFilPair.Collection.Tag.Tag, colFilPair.Filter.Label)
		ml := types.ManifestList{
			SchemaVersion: 2,
			MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
			Manifests:     []types.Manifest{},
		}

		for _, mani := range colFilPair.Manifests {
			logrus.Warnf("  Manifest %s:%s", mani.Upstream.Repo, mani.Upstream.Tag)
			ml.Manifests = append(ml.Manifests, mani)
		}
		mlBlob := ml.ToOCIBytes()
		mlDigest := digest.FromBytes(mlBlob)
		//label := fmt.Sprintf("%s:%s-%s", colFilPair.Collection.Repo, colFilPair.Collection.Tag, colFilPair.Filter.Label)
		mlID, err := mag.NeoInsertManifestList(mlDigest, ml.MediaType, len(mlBlob))
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to Collection
		err = mag.NeoLinkManifestListToCollection(mlID, colID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to Filter
		err = mag.NeoLinkManifestListToFilter(mlID, filterID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to Manifests
		err = mag.NeoLinkManifestListToManifests(mlID, colFilPair.Manifests)
		if err != nil {
			logrus.Error(err)
			continue
		}
	}
	return
}

func (mag Magazine) NeoInsertManifestList(dig digest.Digest, mt string, size int) (mlID int64, err error) {
	mlID, success, err := mag.Neo.GetManifestList(dig, mt, size)
	if err != nil {
		return
	}
	if !success {
		mlID, err = mag.Neo.CreateManifestList(dig, mt, size)
		if err != nil {
			return
		}
	}
	return
}

func (mag Magazine) NeoLinkManifestListToCollection(mlID, colID int64) (err error) {
	return mag.Neo.LinkManifestListToCollection(mlID, colID)
}

func (mag Magazine) NeoLinkManifestListToFilter(mlID, colID int64) (err error) {
	return mag.Neo.LinkManifestListToFilter(mlID, colID)
}

func (mag Magazine) NeoLinkManifestListToManifests(mlID int64, manifests []types.Manifest) (err error) {
	return mag.Neo.LinkManifestListToManifests(mlID, manifests)
}

func (mag Magazine) ManifestListGetByRequest(req types.Request) (ml types.ManifestList, found bool, err error) {
	logrus.Warnf("ManifestListGetByReqest: %v", req)
	ctx := context.Background()
	// Fetch ML from SQLite
	ml, found, err = mag.SQlite.GetTypesManifestListByRequest(req)
	if err != nil {
		logrus.Errorf("Failed to get ManifestList from SQLite: %v", err)
		return
	}
	if !found {
		// We did not find a ManifestList in SQLite
		return
	}
	manis, err := mag.SQlite.Manifest.TypesManifest_GetByManifestListID(ml.SqlID)
	for _, m := range manis {
		// Fetch Manifest from Minio
		miniMani, err := mag.Minio.ManifestGetObj(ctx, m.Digest)
		if err != nil {
			logrus.Errorf("Failed to fetch Manifest from Object store: %v", err)
			return ml, found, err
		}
		ml.Manifests = append(ml.Manifests, miniMani)
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
