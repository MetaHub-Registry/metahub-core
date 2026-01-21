package sqlite

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (s Sqlite) ManifestListInsert(ctx context.Context, ml *types.ManifestList) (err error) {
	return s.InsertManifestList(ml)
}

func (s Sqlite) ManifestList_Get_ByRequest(req types.Request) (res types.ManifestList, found bool, err error) {
	return s.GetTypesManifestListByRequest(req)
}
