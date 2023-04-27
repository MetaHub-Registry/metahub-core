package types

import (
	"fmt"

	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type CollectionTagProfilePair struct {
	Collection  Collection
	Tag         CollectionTag
	Profile     Profile
	Rules       set.StringSet
	ManifestIDs set.StringSet
	Manifests   []ManifestData
}

func NewCollectionTagProfilePair(c Collection, t CollectionTag, p Profile) (pp CollectionTagProfilePair, err error) {
	pp = CollectionTagProfilePair{
		Collection:  c,
		Tag:         t,
		Profile:     p,
		Rules:       set.NewStringSet(),
		ManifestIDs: set.NewStringSet(),
		Manifests:   []ManifestData{},
	}
	lbs, err := utils.ExprGetLabels(p.Rule)
	if err != nil {
		err = errors.Wrap(err, "failed to get labels from rule")
	}
	for _, lb := range lbs.ToSlice() {
		pp.Rules.Add(lb)
	}
	return
}

func (pmdps *CollectionTagProfilePair) AddRule(label string) error {
	if !pmdps.Rules.Contains(label) {
		pmdps.Rules.Add(label)
	}
	return nil
}

func (pmdps *CollectionTagProfilePair) AddManifest(md ManifestData) error {
	if md.MediaType != "application/vnd.oci.image.manifest.v1+json" {
		logrus.Warnf("Skip manifest with type: %s", md.MediaType)
		return nil
	}
	id := fmt.Sprintf("%d", md.ID)
	if !pmdps.ManifestIDs.Contains(id) {
		pmdps.ManifestIDs.Add(id)
		pmdps.Manifests = append(pmdps.Manifests, md)
	}
	return nil
}

func (ctpp CollectionTagProfilePair) String() string {
	return fmt.Sprintf("[%d]%s:%s[%d] [%s:%s]", ctpp.Collection.ID, ctpp.Collection.Repo, ctpp.Tag.Tag, ctpp.Tag.ID, ctpp.Profile.Name, ctpp.Profile.Tag)
}
