package magazine

import (
	"context"

	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func (mag Magazine) CreateCollection(col yamlCfg.ManifestCollection) (colId int64, err error) {
	colId, success, err := mag.Neo.GetCollection(col)
	if err != nil {
		return
	}
	if !success {
		colId, err = mag.Neo.CreateCollection(col)
		if err != nil {
			return
		}
	}
	return
}

func (mag Magazine) NeoLinkCollectionToManifest(ctx context.Context, colID, maniID int64) (err error) {
	return mag.Neo.LinkCollectionToManifest(ctx, colID, maniID)
}
