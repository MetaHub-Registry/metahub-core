package sqlite

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (s Sqlite) Manifest_GetID_ByDigest(dig string) (id int64, found bool, err error) {
	err = s.Sql.QueryRow("SELECT id FROM manifest WHERE digest = ?", dig).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return id, false, nil
		}
		return id, false, err
	}
	return id, true, nil
}

func (s Sqlite) Manifest_Insert(ctx context.Context, m types.Manifest) (maniID int64, err error) {
	err = s.Manifest.GetInsert(&m)
	if err != nil {
		return
	}
	maniID = m.ID
	return
}

func (s Sqlite) Manifest_GetTypes_ByRequest(req types.Request) (res types.Manifest, found bool, err error) {
	return s.Manifest.GetTypesManifestByRequest(req)
}

func (s Sqlite) Platform_GetInsert(p *types.Platform) (err error) {
	return s.Platform.Upsert(p)
}

func (s Sqlite) Manifest_GetInsert(ctx context.Context, mani *types.Manifest) (err error) {
	return s.Manifest.GetInsert(mani)
}

func (s Sqlite) Manifest_Remove(ctx context.Context, mani types.Manifest) (found bool, err error) {
	mlIDs, _, err := s.Manifest.Remove(mani)
	for _, mlID := range mlIDs {
		err = s.Collection_Scrub_ByManifestListID(mlID)
		if err != nil {
			err = errors.Wrap(err, "failed to SQlite.Collection.ScrubCollectionsByManifestListID")
			return
		}
	}
	return
}

func (s Sqlite) Manifest_GetManifests_ByManifestListID(mlID int64) (manis []types.Manifest, err error) {
	return s.Manifest.TypesManifest_GetByManifestListID(mlID)
}
