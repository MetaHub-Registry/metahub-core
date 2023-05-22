package magazine

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameManipulation = "magazine/manipulation.go"
)

func (mag Magazine) ManipulationGetCreate(ctx context.Context, mp types.Manipulation) (mpID int64, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameManipulation,
		"func":    "ManipulationCreate",
	})
	defer span.End()
	// Now the manipulation is inserted into the graph, need to make sure the upstream is cached and linked
	for _, mpl := range mp.ManipuLayers {
		if err = mag.CacheUpstream(ctx, mpl.Image.Upstream); err != nil {
			return -1, utils.ErrorHandler(l, span, err, FailCacheUpstream)
		}
	}
	if e := mag.Database.ManipulationGetCreate(ctx, &mp); e != nil {
		return -1, utils.ErrorHandler(l, span, e, neo.FailManipulationGetCreate)
	}
	return
}
