package magazine

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) InsertUser(ctx context.Context, user types.User) (err error) {
	_, err = mag.SQlite.InsertUser(ctx, user)
	return err
}
