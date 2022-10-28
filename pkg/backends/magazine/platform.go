package magazine

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) NeoInsertPlatform(ctx context.Context, plat types.Platform) (platID int64, err error) {
	platID, success, err := mag.Neo.GetPlatform(plat)
	if err != nil {
		return
	}
	if !success {
		platID, err = mag.Neo.CreatePlatform(plat)
		if err != nil {
			return
		}
	}
	return
}

func (mag Magazine) NeoLinkPlatformToFilter(ctx context.Context, platID int64, plat types.Platform) (err error) {
	return mag.Neo.LinkPlatformToFilter(ctx, platID, plat)
}
