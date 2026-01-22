package neo

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func TestProfileCRUD(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
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
		t.Fatal(err)

	}
	defer db.TeardownTest(bs, br)
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
	_, found, err = db.ProfileGetByNameTag(ctx, "foo", "tag1")
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
	gotPrf1, found, err := db.ProfileGetByNameTag(ctx, "foo", "tag1")
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
	err = db.ProfileGetInsert(ctx, &pFoo)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, pFooID, pFoo.ID)
	err = db.ProfileGetInsert(ctx, &pGen)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, pGen.ID)
}

func TestProfileFromManifestID(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
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
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	prfs, err := db.ProfileFromManifestID(ctx, maniID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(prfs))
	// AMD
	maniID_AMD, err := br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	prfs_AMD, err := db.ProfileFromManifestID(ctx, maniID_AMD)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(prfs_AMD))
	// Zen3

	maniID_zen3, err := br.GetInt64("manifest:ft_zen3#id")
	if err != nil {
		t.Fatal(err)
	}
	prfs_zen3, err := db.ProfileFromManifestID(ctx, maniID_zen3)
	if err != nil {
		t.Fatal(err)
	}
	if len(prfs_zen3) != 2 {
		for _, prf := range prfs_zen3 {
			t.Log(prf)
		}
		t.Fatal("Expected 2 profile, got", len(prfs_zen3))
	}
	// Rules
	if e := db.RuleEvalAll(ctx); e != nil {
		t.Fatal(e)
	}
	prfZen3 := prfs_zen3[0]
	if rls, err := db.ProfileGetPlatformRules(ctx, prfZen3.ID); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 0, len(rls))
	}
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
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	err = db.RuleEvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Zen3
	maniID_zen3, err := br.GetInt64("manifest:ft_zen3#id")
	if err != nil {
		t.Fatal(err)
	}
	prfs_zen3, err := db.ProfileFromManifestID(ctx, maniID_zen3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(prfs_zen3))
	prfZen3 := prfs_zen3[0]
	if rls, err := db.ProfileGetPlatformRules(ctx, prfZen3.ID); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 0, len(rls))
	}
	if rls, err := db.ProfileGetAnnoEqRules(ctx, prfZen3.ID); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 1, len(rls))
	}
	if err = db.ProfileUnwindRules(ctx, prfZen3.ID, maniID_zen3); err != nil {
		t.Fatal(err)
	}
}

func TestFilterReadFilterConfig(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetYAMLPrefix("../../../misc/yaml")
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
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	pGlow, err := config.LoadProfile(ctx, filepath.Join("../../../misc/yaml", "profiles/glow/1.5.0-arm64.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "1.5.0-arm64", pGlow.Tag)
	tpGlow, err := db.ProfileFromYAML(ctx, pGlow)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(tpGlow.Manipulations))
}

func TestProfileFromManifestIDManipulations(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
	// ManiData Setup
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	//ARM64
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
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
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Connect Collection/Tag
	maniAmd64ID, err := br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		t.Fatalf("Failed to get manifest:ft_amd64#id: %v", err)
	}
	colID, err := br.GetInt64("collection:ft#id")
	if err != nil {
		t.Fatalf("Failed to get collection:ft#id: %v", err)
	}
	tagAmd64ID, err := br.GetInt64("tag:amd64#id")
	if err != nil {
		t.Fatalf("Failed to get tag:amd64#id: %v", err)
	}
	if e := db.LinkCollectionTagToManifest(ctx, tagAmd64ID, maniAmd64ID); e != nil {
		t.Fatalf("Failed to LinkCollectionTagToManifest: %v", e)
	}
	if e := db.LinkCollectionToManifest(ctx, colID, maniAmd64ID); e != nil {
		t.Fatalf("Failed to LinkCollectionToManifest: %v", e)
	}
	maniArm64ID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatalf("Failed to get manifest:ft_arm64#id: %v", err)
	}
	tagArm64ID, err := br.GetInt64("tag:arm64#id")
	if err != nil {
		t.Fatalf("Failed to get tag:amd64#id: %v", err)
	}
	if e := db.LinkCollectionTagToManifest(ctx, tagArm64ID, maniArm64ID); e != nil {
		t.Fatalf("Failed to LinkCollectionTagToManifest: %v", e)
	}
	if e := db.LinkCollectionToManifest(ctx, colID, maniArm64ID); e != nil {
		t.Fatalf("Failed to LinkCollectionToManifest: %v", e)
	}
	/// Link Platform to rules
	platArm64ID, err := br.GetInt64("platform:linux/arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	platAmd64ID, err := br.GetInt64("platform:linux/amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	//// Linux to both
	ruleLinuxID, err := br.GetInt64("rule:os:linux#id")
	if err != nil {
		t.Fatalf("Failed to get 'rule:os:linux#id': %v", err)
	}
	q := fmt.Sprintf(`MATCH (p:Platform),(r:Rule) WHERE id(p) = %d AND id(r) = %d`, platArm64ID, ruleLinuxID)
	q += ` CREATE (r)-[:TRUE]->(p)`
	if _, err = db.Execute(q); err != nil {
		t.Fatal(err)
	}
	q = fmt.Sprintf(`MATCH (p:Platform),(r:Rule) WHERE id(p) = %d AND id(r) = %d`, platAmd64ID, ruleLinuxID)
	q += ` CREATE (r)-[:TRUE]->(p)`
	if _, err = db.Execute(q); err != nil {
		t.Fatal(err)
	}
	/// Arm64 to only one
	ruleArm64ID, err := br.GetInt64("rule:arch:arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	q = fmt.Sprintf(`MATCH (p:Platform),(r:Rule) WHERE id(p) = %d AND id(r) = %d`, platArm64ID, ruleArm64ID)
	q += ` CREATE (r)-[:TRUE]->(p)`
	if _, err = db.Execute(q); err != nil {
		t.Fatal(err)
	}
	// Actual Test

}
