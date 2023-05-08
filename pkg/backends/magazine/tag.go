package magazine

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const pkgNameTag = "magazine/tag.go"

func (mag Magazine) TagsGetList(ctx context.Context, req types.Request) (res types.TagResponse, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameTag,
		"func":    "TagsGetList",
	})
	defer span.End()
	res, err = mag.Database.CollectionGetTagsListByRepo(ctx, req.Repo)
	if err != nil {
		return res, utils.ErrorHandler(l, span, err, types.FailMagGetTags)
	}
	utils.Debug(l, span, "Found %d tags", len(res.Tags))
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
