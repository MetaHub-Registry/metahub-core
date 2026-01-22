package sqlite

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (s Sqlite) ProfileGetInsert(ctx context.Context, p *types.Profile) (err error) {
	return errNotImplemented
}

func (s Sqlite) ProfileGetByName(ctx context.Context, name string) (p types.Profile, found bool, err error) {
	p.ID, found, err = s.Profile.HasProfile(name)
	return
}
