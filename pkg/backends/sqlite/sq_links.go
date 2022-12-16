package sqlite

import "context"

func (s Sqlite) LinkCollectionToManifest(ctx context.Context, colID, maniID int64) error {
	return errNotImplemented
}

func (s Sqlite) LinkManifestToLayer(ctx context.Context, maniID, layerID int64, idx int) error {
	return errNotImplemented
}

func (s Sqlite) LinkManifestToUpstream(ctx context.Context, maniID, upID int64) error {
	return errNotImplemented
}
func (s Sqlite) LinkManifestToConfig(ctx context.Context, maniID, cfgID int64) error {
	return errNotImplemented
}

func (s Sqlite) LinkManifestToPlatform(ctx context.Context, maniID, platID int64) error {
	return errNotImplemented
}
