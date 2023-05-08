package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestProfileCRUD(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	pFoo := types.Profile{
		Name: "foo",
		Tag:  "tag1",
		Rule: "",
	}
	pGen := types.Profile{
		Name: "generic",
		Tag:  "tag2",
		Rule: "",
	}
	_, found, err := db.GetProfile(pFoo)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	_, found, err = db.Profile_Get_ByNameTag(ctx, "foo", "tag1")
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	prfs, err := db.GetProfiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0, len(prfs))
	// Create first profile
	pFooID, err := db.CreateProfile(pFoo)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, pFooID)
	gotPrfFooID, found, err := db.GetProfile(pFoo)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, pFooID, gotPrfFooID)
	gotPrf1, found, err := db.Profile_Get_ByNameTag(ctx, "foo", "tag1")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, pFooID, gotPrf1.ID)
	prfs, err = db.GetProfiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(prfs))
	// GetInsert
	err = db.Profile_GetInsert(ctx, &pFoo)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, pFooID, pFoo.ID)
	err = db.Profile_GetInsert(ctx, &pGen)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, pGen.ID)
}

func TestProfileFromManifestID(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("config:ft_zen3")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("upstream:ft_zen3")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_zen3")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_amd64")
	bs.AddBootstrap("collection:ft_zen3")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("layer:0:ft_zen3")
	bs.AddBootstrap("layer:1:ft_zen3")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_zen3")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 180
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Log(err)
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	prfs, err := db.Profile_FromManifestID(ctx, maniID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(prfs))
	// AMD
	maniID_AMD, err := br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	prfs_AMD, err := db.Profile_FromManifestID(ctx, maniID_AMD)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(prfs_AMD))
	// Zen3

	maniID_zen3, err := br.GetInt64("manifest:ft_zen3#id")
	if err != nil {
		t.Fatal(err)
	}
	prfs_zen3, err := db.Profile_FromManifestID(ctx, maniID_zen3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(prfs_zen3))
}

func TestProfileFromManifestIDZen3(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.AddBootstrap("config:ft_zen3")
	bs.AddBootstrap("upstream:ft_zen3")
	bs.AddBootstrap("manifest:ft_zen3")
	bs.AddBootstrap("collection:ft_zen3")
	bs.AddBootstrap("layer:0:ft_zen3")
	bs.AddBootstrap("layer:1:ft_zen3")
	bs.AddBootstrap("manifestdata:ft_zen3")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 180
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Log(err)
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	err = db.Rule_EvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Zen3
	maniID_zen3, err := br.GetInt64("manifest:ft_zen3#id")
	if err != nil {
		t.Fatal(err)
	}
	prfs_zen3, err := db.Profile_FromManifestID(ctx, maniID_zen3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(prfs_zen3))
}
