package magazine

import (
	"context"

	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func (mag Magazine) CreateCollection(col yamlCfg.ManifestCollection) (colId, colTagID int64, err error) {
	colId, colTagID, success, err := mag.Database.Collection_GetID_ByYaml(col)
	if err != nil {
		return
	}
	if !success {
		colId, colTagID, err = mag.Database.Collection_Create_ByYaml(col)
		if err != nil {
			return
		}
	}
	return
}

func (mag Magazine) NeoLinkCollectionToManifest(ctx context.Context, colID, colTagID, maniID int64) (err error) {
	err = mag.Database.LinkCollectionToManifest(ctx, colID, maniID)
	if err != nil {
		return
	}
	err = mag.Database.LinkCollectionTagToManifest(ctx, colTagID, maniID)
	return
}
