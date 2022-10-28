package magazine

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) NeoInsertUpstream(ctx context.Context, up types.Upstream) (upID int64, err error) {
	upID, success, err := mag.Neo.GetUpstream(up)
	if err != nil {
		return
	}
	if !success {
		upID, err = mag.Neo.CreateUpstream(up)
		if err != nil {
			return
		}
	}
	return
}
