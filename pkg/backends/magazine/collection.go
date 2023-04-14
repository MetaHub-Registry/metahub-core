package magazine

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	yamlCfg "github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func (mag Magazine) CreateCollection(col yamlCfg.ManifestCollection) (colID, colTagID int64, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "magazine/collection.go",
		"func":    "CreateCollection",
		"repo":    col.Name,
		"tag":     col.Tag,
	})
	_ = l
	colID, colTagID, success, err := mag.Database.Collection_GetID_ByYaml(col)
	if err != nil {
		return
	}
	if !success {
		colID, colTagID, err = mag.Database.Collection_Create_ByYaml(col)
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

func (mag Magazine) RemoveCollectionByRequest(req types.Request) (found bool, err error) {
	ctx := context.Background()
	c := types.Collection{
		Repo: req.Repo,
		Tag:  req.Tag,
	}
	return mag.Database.Collection_Remove(ctx, c)
}
