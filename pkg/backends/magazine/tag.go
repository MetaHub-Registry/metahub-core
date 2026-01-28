package magazine

import (
	"context"
	"strings"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const pkgNameTag = "magazine/tag.go"

func (mag Magazine) TagsGetList(ctx context.Context, req types.Request) (res types.TagResponse, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameTag,
		"func":    "TagsGetList",
		"repo":    req.Repo,
		"n":       req.Number,
		"last":    req.Last,
	})
	defer span.End()

	res, err = mag.Database.CollectionGetTagsListByRepo(ctx, req.Repo)
	if err != nil {
		return res, utils.ErrorHandler(l, span, err, types.FailMagGetTags)
	}
	utils.Debug(l, span, "Found %d tags", len(res.Tags))
	res.Tags = utils.SortTags(res.Tags)
	utils.Debug(l, span, "Before last/n: '%s'", strings.Join(res.Tags, "', '"))
	if req.Last != "" {
		for i, tag := range res.Tags {
			if tag == req.Last {
				if i+1 < len(res.Tags) {
					res.Tags = res.Tags[i+1:]
				} else {
					res.Tags = []string{}
				}
				break
			}
		}
	}
	utils.Debug(l, span, "After last: '%s'", strings.Join(res.Tags, "', '"))
	if req.Number != 0 {
		if len(res.Tags) > req.Number {
			res.Tags = res.Tags[:req.Number]
		}
	}
	utils.Debug(l, span, "Before n: '%s'", strings.Join(res.Tags, "', '"))
	return
}
