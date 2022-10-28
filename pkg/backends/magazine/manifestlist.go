package magazine

import (
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (mag Magazine) ComputeManifestlists() (err error) {
	colFilPairs, err := mag.Neo.GetCollectionFilterPairs()
	if err != nil {
		return
	}
	for _, colFilPair := range colFilPairs {
		colID := colFilPair.Collection.NeoID
		filterID := colFilPair.Filter.NeoID
		_ = filterID
		logrus.Warnf("Collection %s:%s - %s", colFilPair.Collection.Repo, colFilPair.Collection.Tag, colFilPair.Filter.Label)
		ml := types.ManifestList{
			SchemaVersion: 2,
			MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
			Manifests:     []types.Manifest{},
		}

		for _, mani := range colFilPair.Manifests {
			logrus.Warnf("  Manifest %s:%s", mani.Upstream.Repo, mani.Upstream.Tag)
			ml.Manifests = append(ml.Manifests, mani)
		}
		mlBlob := ml.ToOCIBytes()
		mlDigest := digest.FromBytes(mlBlob)
		//label := fmt.Sprintf("%s:%s-%s", colFilPair.Collection.Repo, colFilPair.Collection.Tag, colFilPair.Filter.Label)
		mlID, err := mag.NeoInsertManifestList(mlDigest, ml.MediaType, len(mlBlob))
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to Collection
		err = mag.NeoLinkManifestListToCollection(mlID, colID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to Filter
		err = mag.NeoLinkManifestListToFilter(mlID, filterID)
		if err != nil {
			logrus.Error(err)
			continue
		}
		// Link ManifestList to Manifests
		err = mag.NeoLinkManifestListToManifests(mlID, colFilPair.Manifests)
		if err != nil {
			logrus.Error(err)
			continue
		}
	}
	return
}

func (mag Magazine) NeoInsertManifestList(dig digest.Digest, mt string, size int) (mlID int64, err error) {
	mlID, success, err := mag.Neo.GetManifestList(dig, mt, size)
	if err != nil {
		return
	}
	if !success {
		mlID, err = mag.Neo.CreateManifestList(dig, mt, size)
		if err != nil {
			return
		}
	}
	return
}

func (mag Magazine) NeoLinkManifestListToCollection(mlID, colID int64) (err error) {
	return mag.Neo.LinkManifestListToCollection(mlID, colID)
}

func (mag Magazine) NeoLinkManifestListToFilter(mlID, colID int64) (err error) {
	return mag.Neo.LinkManifestListToFilter(mlID, colID)
}

func (mag Magazine) NeoLinkManifestListToManifests(mlID int64, manifests []types.Manifest) (err error) {
	return mag.Neo.LinkManifestListToManifests(mlID, manifests)
}
