package magazine

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) InsertFilter(ctx context.Context, filter types.Filter) (filterID int64, err error) {
	return mag.Database.Filter_Insert(ctx, filter)
}

func (mag Magazine) NeoInsertFilter(ctx context.Context, f types.Filter) (filterID int64, err error) {
	/*
		filterID, success, err := mag.Neo.GetFilter(f)
		if err != nil {
			return
		}
		if !success {
			filterID, err = mag.Neo.CreateFilter(f)
			if err != nil {
				return
			}
		}
	*/
	return mag.InsertFilter(ctx, f)
}

func (mag Magazine) SQliteInsertFilter(ctx context.Context, f types.Filter) (filterID int64, err error) {
	/*
		filterID, success, err := mag.SQlite.Filter.GetFilter(f)
		if err != nil {
			return
		}
		if !success {
			err = mag.SQlite.Filter.InsertFilter(&f)
			if err != nil {
				return
			}
			filterID = f.SqlID
		}
	*/
	return mag.InsertFilter(ctx, f)
}
