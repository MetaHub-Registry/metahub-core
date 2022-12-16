package magazine

import (
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) Tags_GetList(req types.Request) (res types.TagResponse, err error) {
	res, err = mag.Database.Collection_GetTagsListByRepo(req.Repo)
	if err != nil {
		return
	}
	if req.Last != "" {
		for i, tag := range res.Tags {
			if tag == req.Last {
				res.Tags = res.Tags[i:]
				break
			}
		}
	}
	if req.Number != 0 {
		res.Tags = res.Tags[:req.Number]
	}
	return
}
