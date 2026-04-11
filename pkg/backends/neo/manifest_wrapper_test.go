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

func TestManifestWrapperUpsertOS(t *testing.T) {
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
	// Actual test
	maniID, err := br.GetInt64("manifest:ft_zen3#id")
	if err != nil {
		t.Fatal(err)
	}
	as, err := db.ManifestWrapperGetAllSystemAnnos(ctx, maniID)
	if err != nil {
		t.Fatal(err)
	}
	if len(as.Keys) != 0 {
		t.Errorf("Expected 0 annotations, but got %d", len(as.Keys))
	}
	// Insert OS Annotation
	u2204 := types.OS{
		ID:    "ubuntu",
		Major: 22,
		Minor: 4,
		Patch: 0,
	}
	err = db.ManifestWrapperUpsertOS(ctx, maniID, u2204)
	if err != nil {
		t.Fatal(err)
	}
	as, err = db.ManifestWrapperGetAllSystemAnnos(ctx, maniID)
	if err != nil {
		t.Fatal(err)
	}
	if len(as.Keys) != 2 {
		t.Errorf("Expected 2 annotations, but got %d", len(as.Keys))
	}
	ku22041 := types.OS{
		ID:    "kubuntu",
		Major: 22,
		Minor: 4,
		Patch: 1,
	}
	err = db.ManifestWrapperUpsertOS(ctx, maniID, ku22041)
	if err != nil {
		t.Fatal(err)
	}
	as, err = db.ManifestWrapperGetAllSystemAnnos(ctx, maniID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as.ToSlice() {
		if a.Key == AnnoOSVersion {
			if a.Value != "22.4.1" {
				t.Errorf("Expected 22.4.1, but got %s", a.Value)
			}
		}
		if a.Key == AnnoOSId {
			if a.Value != "kubuntu" {
				t.Errorf("Expected kubuntu, but got %s", a.Value)
			}
		}
	}
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
	// Actual test
	m, found, e := db.ManifestWrapperGetByLabel(ctx, "registry:5050/qnib-pub-containers/qnib/plain-featuretest:3-17-3-amd64-cpu_zen3")
	if e != nil {
		t.Log(e)
		t.Fatal(e)
	}
	if !found {
		t.Fatal("Manifest not found")
	}
	assert.Equal(t, 1, len(m.Manifests))

}
