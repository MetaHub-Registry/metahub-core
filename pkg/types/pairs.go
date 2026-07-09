package types

import (
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// genericProfileLabel is the default, non-exclusive profile every push
// lands under when a collection entry carries no explicit `profile:`
// field (see types.NewRequest, which defaults req.Profile to "generic").
// A Slot pinned to this label is NOT treated as an exclusive pin: it is
// the union-eligible multi-arch fall-through, not an author's "this image
// is only for profile X" statement.
const genericProfileLabel = "generic"

type CollectionTagProfilePair struct {
	Repo        Repo
	Tag         CollectionTag
	Profile     Profile
	Rules       set.StringSet
	ManifestIDs set.StringSet
	Manifests   []Manifest
	// PinnedProfiles maps a manifest ID (decimal string, matching the
	// ManifestIDs key format) to the set of NON-generic profile labels it
	// is explicitly bound to within THIS collection, i.e. the collection
	// entries that carried an explicit `profile:` field (graph:
	// (Slot{repo,tag,profile≠generic})-[:HAS_PUSHED]->(md)). An explicit
	// pin is EXCLUSIVE: a manifest pinned to profile X is a member of X's
	// computed index only, and never leaks into another profile's index
	// via a broad rule-union match (e.g. os:linux) — even one more
	// specific than the profile it is pinned to. A manifest with no entry
	// here (unpinned / generic-only) keeps the historical rule-union
	// membership so genuinely multi-arch collections are unaffected.
	PinnedProfiles map[string]set.StringSet
}

func NewCollectionTagProfilePair(c Repo, t CollectionTag, p Profile) (pp CollectionTagProfilePair, err error) {
	pp = CollectionTagProfilePair{
		Repo:           c,
		Tag:            t,
		Profile:        p,
		Rules:          set.NewStringSet(),
		ManifestIDs:    set.NewStringSet(),
		Manifests:      []Manifest{},
		PinnedProfiles: map[string]set.StringSet{},
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

func (pmdps *CollectionTagProfilePair) AddManifest(md Manifest) error {
	if md.MediaType != "application/vnd.oci.image.manifest.v1+json" &&
		md.MediaType != "application/vnd.docker.distribution.manifest.v2+json" {
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

// AddPinnedProfile records that the manifest identified by mdID (decimal
// string) is explicitly bound to profileLabel within this collection. The
// generic (empty / "generic") label is intentionally ignored: it is the
// non-exclusive multi-arch default, not an author's exclusive pin, so
// recording it would wrongly make an otherwise union-eligible manifest
// look pinned. Idempotent — repeated calls collapse into the set.
func (ctpp *CollectionTagProfilePair) AddPinnedProfile(mdID, profileLabel string) {
	if profileLabel == "" || profileLabel == genericProfileLabel {
		return
	}
	if ctpp.PinnedProfiles == nil {
		ctpp.PinnedProfiles = map[string]set.StringSet{}
	}
	pins, ok := ctpp.PinnedProfiles[mdID]
	if !ok {
		pins = set.NewStringSet()
		ctpp.PinnedProfiles[mdID] = pins
	}
	pins.Add(profileLabel)
}

// pinnedElsewhere reports whether the manifest mdID carries an explicit
// (non-generic) pin that EXCLUDES this pair's profile — i.e. it is pinned
// to at least one profile and this pair's profile is not among them. When
// true the manifest must be dropped from this profile's computed index
// regardless of which (broad or narrow) rule matched it. A manifest with
// no pins, or one whose pin set contains this profile, returns false and
// follows the ordinary rule-union membership.
func (ctpp CollectionTagProfilePair) pinnedElsewhere(mdID string) bool {
	pins, ok := ctpp.PinnedProfiles[mdID]
	if !ok || len(pins) == 0 {
		return false
	}
	return !pins.Contains(ctpp.Profile.Label)
}

func (ctpp CollectionTagProfilePair) String() string {
	return fmt.Sprintf("[%d]%s:%s[%d] [%s:%s] (manifests:'%s')", ctpp.Repo.ID, ctpp.Repo.Repo, ctpp.Tag.Tag, ctpp.Tag.ID, ctpp.Profile.Name, ctpp.Profile.Tag, strings.Join(ctpp.ManifestIDs.ToSlice(), "', '"))
}
