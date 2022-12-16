package sqlite

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

type UserManagement struct {
	SQ Sqlite
}

func (um UserManagement) InsertUser(ctx context.Context, u types.User) (userID int64, err error) {
	return um.SQ.InsertUser(ctx, u)
}
