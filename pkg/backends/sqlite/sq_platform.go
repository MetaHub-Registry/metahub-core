package sqlite

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (s Sqlite) LinkPlatformIDToFilter(ctx context.Context, platID int64, filter types.Filter) (err error) {
	return errNotImplemented
}
