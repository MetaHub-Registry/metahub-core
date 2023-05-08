package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManifest_Create_WithLabel(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("upstream:ft_amd64")
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
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	upID, err := br.GetInt64("upstream:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	maniID, err := db.manifestCreateViaUpstream(ctx, "label", upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Greater(t, maniID, upID)

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
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
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
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	expLabel := "registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-amd64-generic"
	m, found, err := db.ManifestGetByLabel(ctx, expLabel)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, expLabel, m.Label)
	// found
	upID, err := br.GetInt64("upstream:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	maniID, err := db.manifestCreateViaUpstream(ctx, expLabel, upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, m.ID, maniID)
	// Fail to link
	newLabel := "registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:NEWLABEL"
	maniID, err = db.manifestCreateViaUpstream(ctx, newLabel, upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Greater(t, maniID, m.ID)
	// Fail to link

}

func TestManifestCR(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.AddBootstrap("config:ft_zen3")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_zen3")
	bs.AddBootstrap("upstream:ft_arm64")
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
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	/// Get Upstream
	upID, err := br.GetInt64("upstream:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	_, found, err := db.Manifest_Get(ctx, upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	/// create Manifest
	maniID, err := db.ManifestCreate(ctx, upID)
	if err != nil {
		t.Fatal(err)
	}
	/// get Manifest
	gotManiID, found, err := db.Manifest_Get(ctx, upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, maniID, gotManiID)

}

func TestManifestGetInsert(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	upID, err := br.GetInt64("upstream:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	_, found, err := db.Manifest_Get(ctx, upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	/// create Manifest
	up, _, err := db.UpstreamGetByID(ctx, upID)
	if err != nil {
		t.Fatal(err)
	}
	mani := types.Manifest{
		Upstream: up,
	}
	// FIXME: No annotations here?
	maniID, err := db.ManifestGetInsert(ctx, mani.Upstream)
	if err != nil {
		t.Fatal(err)
	}
	/// get Manifest
	gotManiID, found, err := db.Manifest_Get(ctx, upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, maniID, gotManiID)
}

func TestManifestUpsertOS(t *testing.T) {
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
		db.TeardownTest(bs)
		t.Fatal(err)

	}
	defer db.TeardownTest(bs)
	// Actual test
	maniID, err := br.GetInt64("manifest:ft_zen3#id")
	if err != nil {
		t.Fatal(err)
	}
	as, err := db.ManifestGetAllSystemAnnos(ctx, maniID)
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
	err = db.ManifestUpsertOS(ctx, maniID, u2204)
	if err != nil {
		t.Fatal(err)
	}
	as, err = db.ManifestGetAllSystemAnnos(ctx, maniID)
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
	err = db.ManifestUpsertOS(ctx, maniID, ku22041)
	if err != nil {
		t.Fatal(err)
	}
	as, err = db.ManifestGetAllSystemAnnos(ctx, maniID)
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

func TestManifest_GetByLabel(t *testing.T) {
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
		db.TeardownTest(bs)
		t.Fatal(err)

	}
	defer db.TeardownTest(bs)
	// Actual test
	m, found, e := db.ManifestGetByLabel(ctx, "registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:3-17-3-amd64-cpu_zen3")
	if e != nil {
		t.Log(e)
		t.Fatal(e)
	}
	if !found {
		t.Fatal("Manifest not found")
	}
	assert.Equal(t, 1, len(m.Manifests))

}
