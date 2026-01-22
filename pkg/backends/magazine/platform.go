package magazine

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) NeoInsertPlatform(ctx context.Context, plat types.Platform) (platID int64, err error) {
	err = mag.Database.PlatformGetInsert(ctx, &plat)
	if err != nil {
		return
	}
	platID = plat.ID
	return
}

/*
func (mag Magazine) NeoLinkPlatformIDToFilter(ctx context.Context, platID int64, fltr types.Filter) (err error) {
	return mag.Database.LinkPlatformIDToFilter(ctx, platID, fltr)
}
*/
