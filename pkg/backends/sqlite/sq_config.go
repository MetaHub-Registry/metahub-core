package sqlite

import (
	"context"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (s Sqlite) ConfigInsert(ctx context.Context, cfg types.Config) (cfgID int64, err error) {
	return 0, errNotImplemented
}

func (s Sqlite) ConfigDeleteByDigest(dig string) error {
	return errNotImplemented
}

func (s Sqlite) ConfigGetByDigest(dig string) (cfg types.Config, found bool, err error) {
	return types.Config{}, false, errNotImplemented
}

func (s Sqlite) ConfigGetInsertByOCIManifest(ociMani types.OCIManifest, label string) (cfg types.Config, found bool, err error) {
	cfg, err = s.Config.GetCreateByRequest(ociMani)
	// TODO: we expect a create to take place when it is not found - do we need that than?
	found = true
	return
}
