package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/distribution/manifest/schema2"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManifestCRUD(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
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
	arm_mani := types.FTarmManifest()
	_, found, err := db.ManifestGetExactly(ctx, arm_mani)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	arm_mani_id, err := db.createManifest(ctx, arm_mani)
	if err != nil {
		t.Fatal(err)
	}
	arm_mani_id_got1, found, err := db.ManifestGetExactly(ctx, arm_mani)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, arm_mani_id_got1)
	// Phase 4 (deprecate-manifest-wrapper): no :ManifestWrapper node — operate
	// on the content-leaf created above directly.
	// ManifestGet_ByDigest
	arm_mani_got3, found, err := db.ManifestGetByDigest(ctx, arm_mani.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, arm_mani_got3.ID)

	// ManifestRemove
	reqGot3 := types.Request{
		Repo:   arm_mani_got3.Upstream.Repo,
		Tag:    arm_mani_got3.Upstream.Tag,
		Host:   arm_mani_got3.Upstream.Reg,
		Digest: arm_mani_got3.Digest,
	}
	found, err = db.ManifestRemove(ctx, reqGot3)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	_, found, err = db.ManifestGetByDigest(ctx, arm_mani_got3.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
}

func TestManifestLinks(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("annotation:platform:generic")
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
	// Actual Test
	aPlatGenID, err := br.GetInt64("annotation:platform:generic#id")
	if err != nil {
		t.Fatal(err)
	}
	// Phase 4 (deprecate-manifest-wrapper): annotations attach to the
	// content-leaf :Manifest, not a wrapper node.
	arm_mani := types.FTarmManifest()
	arm_mani_id, err := db.createManifest(ctx, arm_mani)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, arm_mani_id)
	err = db.ManifestLinkToAnnotation(ctx, arm_mani_id, aPlatGenID)
	if err != nil {
		t.Fatal(err)
	}
	// Link Config
	arm_config_id, err := br.GetInt64("config:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_config, found, err := db.ConfigGetByID(ctx, arm_config_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Config not found")
	}
	err = db.ManifestLinkToConfig(ctx, arm_mani_id, arm_config)
	if err != nil {
		t.Fatal(err)
	}
	// Phase 4 (deprecate-manifest-wrapper): assert the content-leaf exists
	// directly instead of via the wrapper-only ManifestGetAll.
	m, found, err := db.ManifestGetByID(ctx, arm_mani_id)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, m.ID)
	// Add Layers
	arm_layer0_id, err := br.GetInt64("layer:0:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_layer0, found, err := db.LayerGetByID(ctx, arm_layer0_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Layer not found")
	}
	// Layer by themselfe do not know what mediaType they are,
	// it's only defined in relation to a manifestData
	// (the sha256 does not change whether it is an DOcker or OCI Image - it's just a tarball)
	arm_layer0.MediaType = schema2.MediaTypeLayer
	err = db.ManifestLinkToLayer(ctx, arm_mani_id, arm_layer0, 0)
	if err != nil {
		t.Fatal(err)
	}
	arm_layer1_id, err := br.GetInt64("layer:1:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_layer1, found, err := db.LayerGetByID(ctx, arm_layer1_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Layer not found")
	}
	arm_layer1.MediaType = schema2.MediaTypeLayer
	err = db.ManifestLinkToLayer(ctx, arm_mani_id, arm_layer1, 1)
	if err != nil {
		t.Fatal(err)
	}
}

func TestManifestRules(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target:platform")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	aPlatGenID, err := br.GetInt64("annotation:org.supercontainers.hardware.cpu.target:platform#id")
	if err != nil {
		t.Fatal(err)
	}
	// Phase 4 (deprecate-manifest-wrapper): operate on the content-leaf.
	arm_mani := types.FTarmManifest()
	arm_mani_id, err := db.createManifest(ctx, arm_mani)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, arm_mani_id)
	// Link Config
	arm_config_id, err := br.GetInt64("config:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_config, found, err := db.ConfigGetByID(ctx, arm_config_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Config not found")
	}
	err = db.ManifestLinkToConfig(ctx, arm_mani_id, arm_config)
	if err != nil {
		t.Fatal(err)
	}
	// Phase 4 (deprecate-manifest-wrapper): assert the content-leaf exists
	// directly instead of via the wrapper-only ManifestGetAll.
	if _, found, e := db.ManifestGetByID(ctx, arm_mani_id); e != nil {
		t.Fatal(e)
	} else {
		assert.True(t, found)
	}
	// Add Layers
	arm_layer0_id, err := br.GetInt64("layer:0:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_layer0, found, err := db.LayerGetByID(ctx, arm_layer0_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Layer not found")
	}
	arm_layer0.MediaType = schema2.MediaTypeLayer
	err = db.ManifestLinkToLayer(ctx, arm_mani_id, arm_layer0, 0)
	if err != nil {
		t.Fatal(err)
	}
	arm_layer1_id, err := br.GetInt64("layer:1:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_layer1, found, err := db.LayerGetByID(ctx, arm_layer1_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Layer not found")
	}
	arm_layer1.MediaType = schema2.MediaTypeLayer
	err = db.ManifestLinkToLayer(ctx, arm_mani_id, arm_layer1, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = db.ManifestLinkToAnnotation(ctx, arm_mani_id, aPlatGenID)
	if err != nil {
		t.Fatal(err)
	}
	// Compute New OCI Manifest
	// Eval Rules
	err = db.RuleEvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Path Z task 7.7: D5 invariant canary after every RuleEvalAll.
	assertRuleShortcutInvariantsOrFatal(t, ctx)
	// Get By Rule
	r, found, err := db.RuleGetByLabel(ctx, "cpu:generic")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Rule not found")
	}
	q := fmt.Sprintf(`MATCH (r:Rule)-[:%s]-(a:Annotation) WHERE ID(r) = %d RETURN a.value AS val`, types.RelationSuccess, r.ID)
	res, found, err := db.QueryOne(q)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Annotation not found")
	}
	v, err := res.GetStr("val")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "platform", v)
}

// func TestManifestGetAllByManifestID(t *testing.T) {
// 	ctx := context.Background()
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_amd64")
// 	bs.AddBootstrap("collection:ft_generic")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_amd64")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	br, err := SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer db.TeardownTest(bs, br)
// 	// Actual test
// 	//// ARM64
// 	maniArm64ID, err := br.GetInt64("manifest:ft_arm64#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	prfID, err := br.GetInt64("profile:cpu:generic#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	manis, err := db.ManifestGetAllByManifestIDWithProfile(ctx, maniArm64ID, prfID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 1, len(manis))
// 	// ManifestGetAllByManifestID
// 	manis, err = db.ManifestGetAllByManifestID(ctx, maniArm64ID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 1, len(manis))
// 	/// Delete
// 	maniDataDig, err := br.Get("manifestdata:ft_arm64#digest")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	err = db.ManifestDeleteByDigest(ctx, maniDataDig)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	/// Check if Items are deleted
// 	//// Manifest
// 	if maniDataArm64ID, err := br.GetInt64("manifestdata:ft_arm64#id"); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		if _, found, err := db.ManifestGetByID(ctx, maniDataArm64ID); err != nil {
// 			t.Fatal(err)
// 		} else {
// 			assert.False(t, found)
// 		}
// 	}
// 	//// Manifest
// 	if _, foundArm64, e := db.ManifestWrapperGetByID(ctx, maniArm64ID); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		assert.False(t, foundArm64)
// 	}
// 	//// Upstream
// 	if upArm64ID, err := br.GetInt64("upstream:ft_arm64#id"); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		if _, found, err := db.UpstreamGetByID(ctx, upArm64ID); err != nil {
// 			t.Fatal(err)
// 		} else {
// 			assert.False(t, found)
// 		}
// 	}
// 	//// Tag (if no other manifest is present)
// 	q := `MATCH (t:Tag) RETURN count(t) AS tagCount`
// 	res, _, e := db.QueryOne(q)
// 	if e != nil {
// 		t.Fatal(e)
// 	}
// 	tagCnt1, _ := res.GetInt("tagCount")
// 	assert.Equal(t, 1, tagCnt1)

// 	//// AMD64
// 	maniAmd64ID, err := br.GetInt64("manifest:ft_amd64#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	manisAmd64, err := db.ManifestGetAllByManifestIDWithProfile(ctx, maniAmd64ID, prfID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 1, len(manisAmd64))
// 	// ManifestGetAllByManifestID
// 	manis, err = db.ManifestGetAllByManifestID(ctx, maniAmd64ID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 1, len(manis))
// 	/// Delete
// 	maniDataDigAmd64, err := br.Get("manifestdata:ft_amd64#digest")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if err = db.ManifestDeleteByDigest(ctx, maniDataDigAmd64); err != nil {
// 		t.Fatal(err)
// 	}
// 	/// Check if Items are deleted
// 	//// Manifest
// 	if maniDataAmd64ID, err := br.GetInt64("manifestdata:ft_amd64#id"); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		if _, found, err := db.ManifestGetByID(ctx, maniDataAmd64ID); err != nil {
// 			t.Fatal(err)
// 		} else {
// 			assert.False(t, found)
// 		}
// 	}
// 	//// Manifest
// 	if _, foundAmd64, e := db.ManifestWrapperGetByID(ctx, maniAmd64ID); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		assert.False(t, foundAmd64)
// 	}
// 	//// Upstream
// 	if upAmd64ID, err := br.GetInt64("upstream:ft_amd64#id"); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		if _, found, err := db.UpstreamGetByID(ctx, upAmd64ID); err != nil {
// 			t.Fatal(err)
// 		} else {
// 			assert.False(t, found)
// 		}
// 	}
// 	//// Tag (if no other manifest is present)
// 	q = `MATCH (t:Tag) RETURN count(t) AS tagCount`
// 	res, _, e = db.QueryOne(q)
// 	if e != nil {
// 		t.Fatal(e)
// 	}
// 	tagCnt, _ := res.GetInt("tagCount")
// 	assert.Equal(t, 0, tagCnt)
// }

func TestManifestGetPulledByRepoTag(t *testing.T) {
	ctx := context.Background()
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestBlobHandlerLayerTest",
	})
	defer span.End()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
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
	// Linking
	colID, err := br.GetInt64("collection:ft#id")
	if err != nil {
		t.Fatal(err)
	}
	tagID, err := br.GetInt64("tag:amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	maniID, err := br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	mdID, err := br.GetInt64("manifestdata:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	_ = maniID
	// Phase 4 (deprecate-manifest-wrapper): wrapper-node linking removed.
	// ManifestGetPulledByRepoTag walks via Slot instead of wrapper, so link
	// the content-leaf to Repo/Tag and wire a Slot+HAS_PUSHED edge.
	if e := db.LinkCollectionToManifest(ctx, colID, mdID); e != nil {
		t.Fatal(e)
	}
	if e := db.LinkCollectionTagToManifest(ctx, tagID, mdID); e != nil {
		t.Fatal(e)
	}
	slot, e := db.SlotGetCreate(ctx, "qnib/featuretest", "amd64", "cpu#generic")
	if e != nil {
		t.Fatal(e)
	}
	if e := db.SlotAppendPushedManifest(ctx, slot.ID, mdID); e != nil {
		t.Fatal(e)
	}
	// Actual test
	repo := "qnib/featuretest"
	tag := "amd64"
	//
	amd64Dig, err := br.Get("manifestdata:ft_amd64#digest")
	if err != nil {
		t.Fatal(err)
	}

	if md, found, err := db.ManifestGetPulledByRepoTag(ctx, repo, tag); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Did not find manifestdata")
	} else {
		assert.Equal(t, amd64Dig, md.Digest)
	}
	span.End()
}

func TestManifestLinkToOCIv1Layer(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
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
	// REMOVE LINK
	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	l0ID, err := br.GetInt64("layer:0:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	l0, found, err := db.LayerGetByLayerAndManifestID(ctx, l0ID, mdID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Layer not found")
	}
	q := fmt.Sprintf(`MATCH (md:Manifest)-[r]->(l:Blob) WHERE ID(md) = %d AND ID(l) = %d DELETE r`, mdID, l0ID)
	_, err = db.Execute(q)
	if err != nil {
		t.Fatal(err)
	}
	ld0 := types.NewLayerDescFromLayer(l0)
	err = db.ManifestLinkToOCIv1Layer(ctx, mdID, ld0)
	if err != nil {
		t.Fatal(err)
	}

}

func TestManifestLinkToOCIv1Config(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
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
	// REMOVE LINK
	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	cfgID, err := br.GetInt64("config:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	cfg, found, err := db.ConfigGetByID(ctx, cfgID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Config not found")
	}
	q := fmt.Sprintf(`MATCH (md:Manifest)-[r]->(c:Blob) WHERE ID(md) = %d AND ID(c) = %d DELETE r`, mdID, cfgID)
	_, err = db.Execute(q)
	if err != nil {
		t.Fatal(err)
	}
	err = db.ManifestLinkToOCIv1Config(ctx, mdID, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestManifestInsert(t *testing.T) {
	c := context.Background()
	l, ctx, span := utils.InitLogging(c, tracer, map[string]any{
		"func": "TestBlobHandlerLayerTest",
	})
	defer span.End()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
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

	// Actual Tests
	mdArm64 := types.ManifestFeaturetestArm64
	upID, err := br.GetInt64("upstream:ft_arm64#id")
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	if up, found, err := db.UpstreamGetByID(ctx, upID); err != nil {
		span.End()
		t.Fatal(err)
	} else if !found {
		span.End()
		t.Fatalf("Upstream not found by ID: %d", upID)
	} else {
		utils.Debug(l, span, "Upstream found: %+v", up)
		mdArm64.Upstream.ID = upID
	}
	if _, e := db.ManifestInsert(ctx, &mdArm64); e != nil {
		t.Fatal(e)
	}
	// Test the insert
	/// MD<>Config<>Label
	q := `MATCH (md:Manifest)-[]->(c:Blob)-[]->(l:Label) WHERE l.value = "arm64" AND c.type = "config" RETURN md.label AS mdLabel`
	utils.Debug(l, span, "Querying Manifest Label: %s", q)
	if res, found, err := db.QueryOne(q); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Not found")
	} else {
		if label, err := res.GetStr("mdLabel"); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, "dkr:ecb2919cf", label)
		}
	}
	// Count Blobs (Layers/Config)
	q = `MATCH (md:Manifest)-[]->(l:Blob) RETURN COUNT(l) AS count`
	utils.Debug(l, span, "Querying Manifest Blobs: %s", q)
	if res, found, err := db.QueryOne(q); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Not found")
	} else {
		if cnt, err := res.GetInt("count"); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 3, cnt)
		}
	}
	// // Again to trigger found
	if _, e := db.ManifestInsert(ctx, &mdArm64); e != nil {
		t.Fatal(e)
	}
	span.End()
}

// func TestManifestGetByUpstream(t *testing.T) {
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_amd64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_amd64")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	br, err := SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer db.TeardownTest(bs, br)
// 	// Preperation
// 	upArmID, err := br.GetInt64("upstream:ft_arm64#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	_ = upArmID
// 	// Actual test
// 	if upArm, found, err := db.UpstreamGetByID(ctx, upArmID); err != nil || !found {
// 		t.Fatalf("Upstream not found or through an error: %v", err)
// 	} else {
// 		md, found, err := db.ManifestGetByUpstream(ctx, upArm)
// 		if err != nil || !found {
// 			t.Fatalf("ManifestGetByUpstream not found or through an error: %v", err)
// 		} else {
// 			assert.Equal(t, 2, len(md.Layers))
// 			assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", md.Digest)
// 		}
// 	}
// 	// Profile>Rule
// 	prfID, _ := br.GetInt64("profile:cpu:generic#id")
// 	rID, _ := br.GetInt64("rule:cpu:generic#id")
// 	q := fmt.Sprintf(`MATCH (prf:Profile),(r:Rule) WHERE id(prf) = %d AND id(r) = %d`, prfID, rID)
// 	q += ` CREATE (prf)-[:USES]->(r)`
// 	if _, err = db.Execute(q); err != nil {
// 		t.Fatal(err)
// 	}
// 	// Rule>Platform
// 	pID, _ := br.GetInt64("platform:linux/amd64#id")
// 	q = fmt.Sprintf(`MATCH (p:Platform),(r:Rule) WHERE id(p) = %d AND id(r) = %d`, pID, rID)
// 	q += ` CREATE (r)-[:TRUE]->(p)`
// 	if _, err = db.Execute(q); err != nil {
// 		t.Fatal(err)
// 	}

// 	/// Get By Upstream
// 	upAmdID, err := br.GetInt64("upstream:ft_amd64#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if upAmd, found, err := db.UpstreamGetByID(ctx, upAmdID); err != nil || !found {
// 		t.Fatalf("Upstream not found or through an error: %v", err)
// 	} else {
// 		upAmd.Profile = "cpu#generic"
// 		md, found, err := db.ManifestGetByUpstream(ctx, upAmd)
// 		if err != nil || !found {
// 			t.Fatalf("ManifestGetByUpstream not found or through an error: %v", err)
// 		} else {
// 			assert.Equal(t, 2, len(md.Layers))
// 			assert.Equal(t, "sha256:5bf8eee20fde14eb6e3e924c18fafa4d816e131f315810875136f0438ff7beed", md.Digest)
// 		}
// 	}

// }

// func TestManifestIsLinkedWithProfile(t *testing.T) {
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_amd64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_amd64")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	br, err := SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		t.Fatal(err)

// 	}
// 	defer db.TeardownTest(bs, br)
// 	// Preperation
// 	upArmID, err := br.GetInt64("upstream:ft_arm64#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	_ = upArmID
// 	// Actual test
// 	if upArm, found, err := db.UpstreamGetByID(ctx, upArmID); err != nil || !found {
// 		t.Fatalf("Upstream not found or through an error: %v", err)
// 	} else {
// 		md, found, err := db.ManifestGetByUpstream(ctx, upArm)
// 		if err != nil || !found {
// 			t.Fatalf("ManifestGetByUpstream not found or through an error: %v", err)
// 		} else {
// 			assert.Equal(t, 2, len(md.Layers))
// 			assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", md.Digest)
// 		}
// 	}
// 	// Profile>Rule
// 	prfID, _ := br.GetInt64("profile:cpu:generic#id")
// 	rID, _ := br.GetInt64("rule:cpu:generic#id")
// 	q := fmt.Sprintf(`MATCH (prf:Profile),(r:Rule) WHERE id(prf) = %d AND id(r) = %d`, prfID, rID)
// 	q += ` CREATE (prf)-[:USES]->(r)`
// 	if _, err = db.Execute(q); err != nil {
// 		t.Fatal(err)
// 	}
// 	// Rule>Platform
// 	pID, _ := br.GetInt64("platform:linux/amd64#id")
// 	q = fmt.Sprintf(`MATCH (p:Platform),(r:Rule) WHERE id(p) = %d AND id(r) = %d`, pID, rID)
// 	q += ` CREATE (r)-[:TRUE]->(p)`
// 	if _, err = db.Execute(q); err != nil {
// 		t.Fatal(err)
// 	}
// 	// ManifestIsLinkedWIthProfile
// 	if prf, found, err := db.ProfileGetByLabel(ctx, "cpu#generic"); err != nil || !found {
// 		t.Fatalf("Profile not found or through an error: %v", err)
// 	} else {
// 		maniDataID, err := br.GetInt64("manifestdata:ft_amd64#id")
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		linked, err := db.ManifestIsLinkedWithProfile(ctx, maniDataID, prf)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		assert.Equal(t, true, linked)
// 	}
// }

func TestManifestV1GetInsert(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("empty:all")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	// Zen3
	bs.AddBootstrap("config:ft_zen3")
	bs.AddBootstrap("upstream:ft_zen3")
	bs.AddBootstrap("manifest:ft_zen3")
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
	// Test
	expManiDataDigest, err := br.Get("manifestdata:ft_arm64#digest")
	if err != nil {
		t.Fatal(err)
	}
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	expManiDataID, err := br.GetInt64("manifestdata:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	//n.ManifestV1Get
	v1Desc := v1.Descriptor{
		Digest:    digest.Digest(expManiDataDigest),
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Platform: &v1.Platform{
			Architecture: "arm64",
			OS:           "linux",
		},
	}
	if maniDataID, found, err := db.ManifestV1Get(ctx, v1Desc); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("ManifestV1Get not found")
	} else if maniDataID != expManiDataID {
		t.Fatalf("ID do not match got:%d != %d:exp", maniDataID, expManiDataID)
	}
	var maniDataID int64
	var found bool
	if maniDataID, found, err = db.ManifestV1GetInsert(ctx, maniID, v1Desc); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("ManifestV1Get not found")
	} else if maniDataID != expManiDataID {
		t.Fatalf("ID do not match got:%d != %d:exp", maniDataID, expManiDataID)
	}
	// Zen3 insert (not found). Phase 4 (deprecate-manifest-wrapper): the
	// maniSiID parameter is ignored (no wrapper node), so a sentinel id is fine
	// — the zen3 content leaf is intentionally not set up, so the insert path
	// must report the manifest as not-yet-present.
	maniZen3ID := int64(0)
	mdZen3 := types.ManifestAmd64Zen
	zen3Desc := v1.Descriptor{
		Digest:    digest.Digest(mdZen3.Digest),
		MediaType: mdZen3.MediaType,
		Platform: &v1.Platform{
			Architecture: "amd64",
			OS:           "linux",
		},
		Size: mdZen3.Size,
		Data: mdZen3.Data,
	}
	if _, found, err := db.ManifestV1GetInsert(ctx, maniZen3ID, zen3Desc); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("Manifest for zen3 should not be found")
	}
}

func TestManifestUpdateDesc(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	// ManiData Setup
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
	maniDataID, err := br.GetInt64("manifestdata:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	maniDesc := v1.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      123,
		Digest:    digest.Digest("sha256:1234567890123456789012345678901234567890123456789012345678901234"),
	}
	if err = db.ManifestUpdateDesc(ctx, maniDataID, maniDesc); err != nil {
		t.Fatal(err)
	}
	if _, found, err := db.ManifestGetIDByDigest(ctx, maniDesc.Digest.String()); err != nil {
		t.Fatal(err)
	} else {
		assert.True(t, found, "should be updated")
	}
	// Update ARM64 manifestdata with the same digest (which would not happen in real life)
	// -> manifestData should be removed in the process
	maniDataID2, err := br.GetInt64("manifestdata:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.ManifestUpdateDesc(ctx, maniDataID2, maniDesc); err != nil {
		t.Fatal(err)
	}
	q := fmt.Sprintf(`MATCH (md:Manifest) WHERE ID(md) = %d RETURN count(md) AS count`, maniDataID2)
	if res, _, err := db.QueryOne(q); err != nil {
		t.Fatal(err)
	} else {
		count, err := res.GetInt("count")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 0, count)
	}
}

func TestManifestCheckProfile(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
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
	// Prepare
	// Profile>Rule
	prfID, _ := br.GetInt64("profile:cpu:generic#id")
	rID, _ := br.GetInt64("rule:cpu:generic#id")
	q := fmt.Sprintf(`MATCH (prf:Profile),(r:Rule) WHERE id(prf) = %d AND id(r) = %d`, prfID, rID)
	q += ` CREATE (prf)-[:USES]->(r)`
	if _, err = db.Execute(q); err != nil {
		t.Fatal(err)
	}
	// Rule>Platform
	pID, _ := br.GetInt64("platform:linux/amd64#id")
	q = fmt.Sprintf(`MATCH (p:Platform),(r:Rule) WHERE id(p) = %d AND id(r) = %d`, pID, rID)
	q += ` CREATE (r)-[:TRUE]->(p)`
	if _, err = db.Execute(q); err != nil {
		t.Fatal(err)
	}
	pArmID, _ := br.GetInt64("platform:linux/arm64#id")
	q = fmt.Sprintf(`MATCH (p:Platform),(r:Rule) WHERE id(p) = %d AND id(r) = %d`, pArmID, rID)
	q += ` CREATE (r)-[:TRUE]->(p)`
	if _, err = db.Execute(q); err != nil {
		t.Fatal(err)
	}
	if e := db.RuleEvalAll(ctx); e != nil {
		t.Fatal(e)
	}
	// Path Z task 7.7: no D5 invariant check here — this test deliberately
	// manually materialises :TRUE edges from an `anno null` rule
	// (cpu:generic) to :Platform nodes via raw cypher at lines 1205-1213
	// above, which violates invariant 4 (anno rules should point at
	// :Annotation, not :Platform). It's a legacy wrapper-era stub for
	// the evaluator, not a real evaluator output, so the invariant
	// canary does not apply. This test will be rewritten or deleted
	// in Phase C3 when the wrapper-era ManifestCheckProfile helper
	// goes away.
	// Actual tests
	mdArm64, err := db.getManiData(ctx, br, "manifestdata:ft_arm64")
	if err != nil {
		t.Fatal(err)
	}
	prfGen, err := db.getProfile(ctx, br, "cpu#generic")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := db.ManifestCheckProfile(ctx, mdArm64, prfGen); err != nil {
		t.Fatal(err)
	} else {
		assert.False(t, ok)
	}
}

func TestManifestCheckProfileCreationRelationship(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
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
	// Prepare
	prfGenID, err := br.GetInt64("profile:cpu:generic#id")
	if err != nil {
		t.Fatal(err)
	}
	mdID, err := br.GetInt64("manifestdata:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	q := fmt.Sprintf(`MATCH (prf:Profile), (md:Manifest) WHERE id(md) = %d AND id(prf) = %d`, mdID, prfGenID)
	q += ` CREATE (md)-[:CREATED_BY]->(prf)`
	if _, err = db.Execute(q); err != nil {
		t.Fatal(err)
	}
	// Profile>Rule
	prfGen, err := db.getProfile(ctx, br, "cpu#generic")
	if err != nil {
		t.Fatal(err)
	}
	md64, err := db.getManiData(ctx, br, "manifestdata:ft_amd64")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := db.ManifestCheckProfileCreationRelationship(ctx, md64, prfGen); err != nil {
		t.Fatal(err)
	} else {
		assert.True(t, ok)
	}
	if ok, err := db.ManifestCheckProfile(ctx, md64, prfGen); err != nil {
		t.Fatal(err)
	} else {
		assert.True(t, ok)
	}
	// Create annotations
	annID, err := br.GetInt64("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3#id")
	if err != nil {
		t.Fatal(err)
	}
	q = fmt.Sprintf(`MATCH (md:Manifest), (ann:Annotation) WHERE id(ann) = %d AND id(md) = %d`, annID, mdID)
	q += ` CREATE (md)-[:HAS_ANNOTATION]->(ann)`
	if _, err = db.Execute(q); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.ManifestCheckProfile(ctx, md64, prfGen); err != nil {
		t.Fatal(err)
	} else {
		assert.True(t, ok)
	}

}
