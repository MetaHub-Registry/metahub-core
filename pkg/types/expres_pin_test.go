package types

import (
	"context"
	"fmt"
	"testing"

	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/stretchr/testify/assert"
)

// ---- helpers -------------------------------------------------------------

// mkImg builds a manifest with an OCI image-manifest media type so that
// CollectionTagProfilePair.AddManifest accepts it (AddManifest skips any
// media type that is not an OCI/docker image manifest).
func mkImg(id int64) Manifest {
	return Manifest{
		ID:        id,
		Label:     fmt.Sprintf("md-%d", id),
		MediaType: "application/vnd.oci.image.manifest.v1+json",
	}
}

// mkExprRes builds an ExprRes from a rule -> matching manifest-ID mapping,
// mirroring how ComputeManifestlists populates it from ManifestsMatchingRule.
func mkExprRes(ruleToIDs map[string][]int64) *ExprRes {
	er := NewExprRes()
	for rule, ids := range ruleToIDs {
		for _, id := range ids {
			er.Add(rule, mkImg(id))
		}
	}
	return er
}

// mkCTPP builds a pair for profileLabel over the given rule labels and the
// collection membership mdIDs (the whole collection tag, profile-independent,
// exactly as ManifestGetByCollectionTag hydrates it).
func mkCTPP(profileLabel string, rules []string, mdIDs []int64) CollectionTagProfilePair {
	ctpp := CollectionTagProfilePair{
		Profile:        Profile{Label: profileLabel},
		Rules:          set.NewStringSet(),
		ManifestIDs:    set.NewStringSet(),
		Manifests:      []Manifest{},
		PinnedProfiles: map[string]set.StringSet{},
	}
	for _, r := range rules {
		ctpp.Rules.Add(r)
	}
	for _, id := range mdIDs {
		_ = ctpp.AddManifest(mkImg(id))
	}
	return ctpp
}

// resultIDs collapses FilterByCTPP output into a set of manifest IDs.
func resultIDs(mds []Manifest) map[int64]bool {
	out := map[int64]bool{}
	for _, m := range mds {
		out[m.ID] = true
	}
	return out
}

// ---- FilterByCTPP: explicit-pin exclusivity ------------------------------

// The canonical leak scenario: one runtime collection whose three GPU
// images are all linux/amd64 and therefore all match the broad `os:linux`
// rule every linux profile carries. Without pins they union into every
// profile's index; with explicit `profile:` pins each lands in exactly one.
func TestFilterByCTPP_PinExclusivity_PerProfile(t *testing.T) {
	const (
		rocm    = int64(10) // pinned frame
		cuda86  = int64(11) // pinned bazzite
		cuda90  = int64(12) // pinned hopper
		generic = int64(20) // no pin -> multi-arch fall-through
	)
	er := mkExprRes(map[string][]int64{
		"os:linux":        {rocm, cuda86, cuda90, generic},
		"gpu:amd-gfx1151": {rocm},
		"gpu:nvidia-sm86": {cuda86},
		"gpu:nvidia-sm90": {cuda90},
	})
	pin := func(ctpp *CollectionTagProfilePair) {
		ctpp.AddPinnedProfile(fmt.Sprint(rocm), "frame")
		ctpp.AddPinnedProfile(fmt.Sprint(cuda86), "bazzite")
		ctpp.AddPinnedProfile(fmt.Sprint(cuda90), "hopper")
		// generic image is pushed under the default profile -> ignored,
		// never recorded as a pin.
		ctpp.AddPinnedProfile(fmt.Sprint(generic), "generic")
	}
	all := []int64{rocm, cuda86, cuda90, generic}

	cases := []struct {
		profile string
		rules   []string
		wantIDs []int64
	}{
		{"frame", []string{"os:linux", "gpu:amd-gfx1151"}, []int64{rocm, generic}},
		{"bazzite", []string{"os:linux", "gpu:nvidia-sm86"}, []int64{cuda86, generic}},
		{"hopper", []string{"os:linux", "gpu:nvidia-sm90"}, []int64{cuda90, generic}},
		// even the generic profile's own index excludes the pinned images.
		{"generic", []string{"os:linux"}, []int64{generic}},
	}
	for _, tc := range cases {
		t.Run(tc.profile, func(t *testing.T) {
			ctpp := mkCTPP(tc.profile, tc.rules, all)
			pin(&ctpp)
			res := er.FilterByCTPP(context.Background(), ctpp)
			ids := resultIDs(res)
			assert.Equal(t, len(tc.wantIDs), len(ids), "profile %s membership size", tc.profile)
			for _, want := range tc.wantIDs {
				assert.True(t, ids[want], "profile %s should contain md-%d", tc.profile, want)
			}
		})
	}
}

// Guard the fix's premise: with NO pins recorded, the same broad rule leaks
// every same-arch image into the profile index (the pre-fix behaviour). This
// documents that PinnedProfiles is the sole lever gating exclusivity.
func TestFilterByCTPP_NoPins_UnionUnchanged(t *testing.T) {
	all := []int64{10, 11, 12, 20}
	er := mkExprRes(map[string][]int64{"os:linux": all})
	ctpp := mkCTPP("frame", []string{"os:linux"}, all)
	// no AddPinnedProfile calls
	res := er.FilterByCTPP(context.Background(), ctpp)
	assert.Equal(t, 4, len(res), "without pins the broad rule unions all same-arch images")
}

// Pin must beat rule specificity: a manifest pinned to bazzite is dropped
// from the frame index even when it matched frame's NARROWEST rule.
func TestFilterByCTPP_PinOverridesNarrowerRule(t *testing.T) {
	const (
		rocm   = int64(10) // pinned frame, matches broad os:linux only
		cuda86 = int64(11) // pinned bazzite, but ALSO matches frame's narrow rule
	)
	er := mkExprRes(map[string][]int64{
		"os:linux":        {rocm, cuda86},
		"gpu:amd-gfx1151": {cuda86}, // narrow, frame-specific, matches the bazzite image
	})
	ctpp := mkCTPP("frame", []string{"os:linux", "gpu:amd-gfx1151"}, []int64{rocm, cuda86})
	ctpp.AddPinnedProfile(fmt.Sprint(rocm), "frame")
	ctpp.AddPinnedProfile(fmt.Sprint(cuda86), "bazzite")

	res := er.FilterByCTPP(context.Background(), ctpp)
	ids := resultIDs(res)
	assert.True(t, ids[rocm], "frame-pinned image stays")
	assert.False(t, ids[cuda86], "bazzite-pinned image excluded despite matching frame's narrow rule")
	assert.Equal(t, 1, len(ids))
}

// A manifest whose pin set includes THIS profile is kept (co-pinned images).
func TestFilterByCTPP_PinnedToThisProfileKept(t *testing.T) {
	const shared = int64(10)
	er := mkExprRes(map[string][]int64{"os:linux": {shared}})
	ctpp := mkCTPP("frame", []string{"os:linux"}, []int64{shared})
	ctpp.AddPinnedProfile(fmt.Sprint(shared), "bazzite")
	ctpp.AddPinnedProfile(fmt.Sprint(shared), "frame") // co-pinned
	res := er.FilterByCTPP(context.Background(), ctpp)
	assert.Equal(t, 1, len(res), "a manifest co-pinned to this profile is retained")
}

// Multi-arch collections (generic pushes only) keep the union behaviour.
func TestFilterByCTPP_GenericPushIsNotAPin(t *testing.T) {
	all := []int64{30, 31}
	er := mkExprRes(map[string][]int64{"os:linux": all})
	ctpp := mkCTPP("bazzite", []string{"os:linux"}, all)
	// generic label is explicitly ignored by AddPinnedProfile
	ctpp.AddPinnedProfile("30", "generic")
	ctpp.AddPinnedProfile("31", "")
	res := er.FilterByCTPP(context.Background(), ctpp)
	assert.Equal(t, 2, len(res), "generic/empty labels are not exclusive pins")
}

// ---- unit tests for the pin helpers on the pair --------------------------

func TestAddPinnedProfile_IgnoresGenericAndEmpty(t *testing.T) {
	ctpp, _ := NewCollectionTagProfilePair(Repo{}, CollectionTag{}, Profile{Label: "frame", Rule: "os:linux=true"})
	ctpp.AddPinnedProfile("1", "generic")
	ctpp.AddPinnedProfile("1", "")
	assert.Empty(t, ctpp.PinnedProfiles["1"], "generic/empty must never be recorded as a pin")

	ctpp.AddPinnedProfile("1", "frame")
	ctpp.AddPinnedProfile("1", "frame") // idempotent
	assert.Equal(t, 1, len(ctpp.PinnedProfiles["1"]))
	assert.True(t, ctpp.PinnedProfiles["1"].Contains("frame"))
}

func TestPinnedElsewhere(t *testing.T) {
	ctpp := mkCTPP("frame", []string{"os:linux"}, []int64{1, 2, 3})
	// 1: unpinned, 2: pinned frame, 3: pinned bazzite
	ctpp.AddPinnedProfile("2", "frame")
	ctpp.AddPinnedProfile("3", "bazzite")

	assert.False(t, ctpp.pinnedElsewhere("1"), "unpinned -> not elsewhere")
	assert.False(t, ctpp.pinnedElsewhere("2"), "pinned to this profile -> not elsewhere")
	assert.True(t, ctpp.pinnedElsewhere("3"), "pinned to another profile -> elsewhere")
	assert.False(t, ctpp.pinnedElsewhere("999"), "unknown id -> not elsewhere")
}
