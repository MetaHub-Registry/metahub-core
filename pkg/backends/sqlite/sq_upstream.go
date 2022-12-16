package sqlite

import "github.com/metahub-registry/metahub-core/pkg/types"

func (s Sqlite) Upstream_GetInsert_ByRequest(req types.Request) (up types.Upstream, err error) {
	return s.Upstream.GetCreateByRequest(req)
}
