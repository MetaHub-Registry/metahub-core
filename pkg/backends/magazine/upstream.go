package magazine

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) NeoInsertUpstream(ctx context.Context, up types.Upstream) (upID int64, err error) {
	req := types.Request{
		Host:   up.Reg,
		Repo:   up.Repo,
		Tag:    up.Tag,
		Digest: up.Digest.String(),
	}

	res, _, err := mag.Database.Upstream_GetInsert_ByRequest(req)
	if err != nil {
		return
	}
	upID = res.ID
	return
}
