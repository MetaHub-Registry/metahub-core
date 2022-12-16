package sqlite

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (s Sqlite) Layer_PlainInsert(ctx context.Context, l types.Layer) (layerID int64, err error) {
	return 0, errNotImplemented
}

func (s Sqlite) LayerGetByDigest(dig string) (l types.Layer, found bool, err error) {
	return types.Layer{}, false, errNotImplemented
}

func (s Sqlite) LayerDeleteByDigest(dig string) error {
	return errNotImplemented
}

func (s Sqlite) Layer_Insert(ctx context.Context, l types.Layer, up types.Upstream) (lID int64, err error) {
	return 0, errNotImplemented
}
