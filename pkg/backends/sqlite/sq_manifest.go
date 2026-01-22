package sqlite

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (s Sqlite) ManifestDataGetIDByDigest(dig string) (id int64, found bool, err error) {
	err = s.Sql.QueryRow("SELECT id FROM manifest WHERE digest = ?", dig).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return id, false, nil
		}
		return id, false, err
	}
	return id, true, nil
}

func (s Sqlite) ManifestInsert(ctx context.Context, m types.ManifestData) (maniID int64, err error) {
	err = s.Manifest.GetInsert(&m)
	if err != nil {
		return
	}
	maniID = m.ID
	return
}

func (s Sqlite) ManifestGetTypesByRequest(req types.Request) (res types.ManifestData, found bool, err error) {
	return s.Manifest.GetTypesManifestByRequest(req)
}

func (s Sqlite) PlatformGetInsert(p *types.Platform) (err error) {
	//return s.Platform.Upsert(p)
	return
}

func (s Sqlite) ManifestGetInsert(ctx context.Context, mani *types.ManifestData) (err error) {
	return s.Manifest.GetInsert(mani)
}

func (s Sqlite) ManifestRemove(ctx context.Context, mani types.ManifestData) (found bool, err error) {
	mlIDs, _, err := s.Manifest.Remove(mani)
	for _, mlID := range mlIDs {
		err = s.CollectionScrubByManifestListID(mlID)
		if err != nil {
			err = errors.Wrap(err, "failed to SQlite.Collection.ScrubCollectionsByManifestListID")
			return
		}
	}
	return
}
