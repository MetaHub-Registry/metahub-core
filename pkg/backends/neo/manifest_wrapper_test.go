package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// NOTE: Path Z Phase C3 step 3 deleted three wrapper-era tests that
// used to live at the top of this file:
//
//  - TestManifest_Create_WithLabel — exercised manifestCreateViaUpstream
//    twice, asserting that two identically-labelled wrapper :Manifest
//    nodes get distinct IDs (i.e. the CREATE is idempotent-per-call
//    rather than idempotent-per-label). The assertion is meaningless
//    in the Slot-centred model because :ManifestWrapper is gone — the Slot
//    node carries the (repo, tag, profile) identity and is MERGEd by
//    property key.
//
//  - TestManifestCR — exercised ManifestCreate / ManifestGet /
//    ManifestGetIDByManifest / LinkManifestToManifest /
//    GetManifestLinksManifestByID / GetManifestLinksUpstreamByID,
//    all of which are wrapper-graph walks. LinkManifestWrapperToManifest
//    and its helpers survive (they still attach Manifest to the
//    wrapper in the upstream-pull flow) but the other three are being
//    deleted in the same commit.
//
//  - TestManifestGetInsert — exercised ManifestGet + ManifestGetInsert.
//    ManifestGetInsert is still live (called from the upstream-pull
//    flow and test fixtures) but ManifestGet is wrapper-only and
//    deleted in the same commit.

// TestManifestLeafOSAnnotations asserts that OS id + version annotations
// resolve from the content-leaf :Manifest node. This replaces the
// wrapper-era TestManifestWrapperUpsertOS: the ManifestWrapperUpsertOS /
// ManifestWrapperGetAllSystemAnnos family was deleted because OS
// annotations now live on the content-leaf (written via
// ManifestAddAnnotations in the upstream-pull flow, read via
// GetAnnotationsByManifestID).
func TestManifestLeafOSAnnotations(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.AddBootstrap("config:ft_zen3")
	bs.AddBootstrap("upstream:ft_zen3")
	bs.AddBootstrap("manifest:ft_zen3")
	bs.AddBootstrap("layer:0:ft_zen3")
	bs.AddBootstrap("layer:1:ft_zen3")
	bs.AddBootstrap("manifestdata:ft_zen3")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)

	}
	defer db.TeardownTest(bs, br)
	// Actual test: write OS annotations to the content-leaf and read
	// them back, asserting OS id + version resolve from the leaf.
	// NOTE: the content-leaf :Manifest node is keyed "manifestdata:...#id"
	// (inserted via ManifestGetInsert). "manifest:...#id" is the legacy
	// wrapper node and must NOT be used here.
	maniID, err := br.GetInt64("manifestdata:ft_zen3#id")
	if err != nil {
		t.Fatal(err)
	}
	osAnnos := types.NewAnnotations()
	if e := osAnnos.AddNewAnnotation(AnnoOSId, "ubuntu"); e != nil {
		t.Fatal(e)
	}
	if e := osAnnos.AddNewAnnotation(AnnoOSVersion, "22.4.1"); e != nil {
		t.Fatal(e)
	}
	if e := db.ManifestAddAnnotations(ctx, maniID, &osAnnos); e != nil {
		t.Fatal(e)
	}
	as, err := db.GetAnnotationsByManifestID(ctx, maniID)
	if err != nil {
		t.Fatal(err)
	}
	var gotID, gotVersion string
	for _, a := range as.ToSlice() {
		switch a.Key {
		case AnnoOSId:
			gotID = a.Value
		case AnnoOSVersion:
			gotVersion = a.Value
		}
	}
	assert.Equal(t, "ubuntu", gotID, "OS id should resolve from content-leaf")
	assert.Equal(t, "22.4.1", gotVersion, "OS version should resolve from content-leaf")
}

func TestManifestGetByLabel(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.AddBootstrap("config:ft_zen3")
	bs.AddBootstrap("upstream:ft_zen3")
	bs.AddBootstrap("manifest:ft_zen3")
	bs.AddBootstrap("layer:0:ft_zen3")
	bs.AddBootstrap("layer:1:ft_zen3")
	bs.AddBootstrap("manifestdata:ft_zen3")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)

	}
	defer db.TeardownTest(bs, br)
	// Phase 4 (deprecate-manifest-wrapper): :ManifestWrapper nodes are no longer
	// created, so the wrapper-by-label lookup is obsolete. Assert instead that
	// the content-leaf :Manifest resolves by its digest — the Slot-centred
	// canonical lookup that replaced the wrapper-by-label walk.
	mdDigest, err := br.Get("manifestdata:ft_zen3#digest")
	if err != nil {
		t.Fatal(err)
	}
	m, found, e := db.ManifestGetByDigest(ctx, mdDigest)
	if e != nil {
		t.Log(e)
		t.Fatal(e)
	}
	if !found {
		t.Fatal("content-leaf Manifest not found by digest")
	}
	assert.Equal(t, mdDigest, m.Digest)
}
