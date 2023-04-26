package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_ManifestData_CRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	arm_mani := types.FT_arm_ManifestData()
	_, found, err := db.ManifestData_GetExactly(ctx, arm_mani)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	arm_mani_id, err := db.createManifestData(ctx, arm_mani)
	if err != nil {
		t.Fatal(err)
	}
	arm_mani_id_got1, found, err := db.ManifestData_GetExactly(ctx, arm_mani)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, arm_mani_id_got1)
	// Link Manifest to ManifestData
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	err = db.LinkManifestToManifestData(ctx, maniID, arm_mani_id_got1)
	if err != nil {
		t.Fatal(err)
	}
	// Manifest_Get_ByDigest
	arm_mani_got3, found, err := db.ManifestData_Get_ByDigest(ctx, arm_mani.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, arm_mani_got3.ID)

	// Manifest_Remove
	reqGot3 := types.Request{
		Repo:   arm_mani_got3.Upstream.Repo,
		Tag:    arm_mani_got3.Upstream.Tag,
		Host:   arm_mani_got3.Upstream.Reg,
		Digest: arm_mani_got3.Digest,
	}
	found, err = db.ManifestData_Remove(ctx, reqGot3)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	_, found, err = db.ManifestData_Get_ByDigest(ctx, arm_mani_got3.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
}

func Test_ManifestData_Links(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("annotation:platform:generic")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	aPlatGenID, err := br.GetInt64("annotation:platform:generic#id")
	if err != nil {
		t.Fatal(err)
	}
	err = db.Manifest_LinkToAnnotation(ctx, maniID, aPlatGenID)
	if err != nil {
		t.Fatal(err)
	}
	// Actual test
	arm_mani := types.FT_arm_ManifestData()
	arm_mani_id, err := db.createManifestData(ctx, arm_mani)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, arm_mani_id)
	// Manifest Link To ManifestData
	err = db.LinkManifestToManifestData(ctx, maniID, arm_mani_id)
	if err != nil {
		t.Fatal(err)
	}
	// Link Config
	arm_config_id, err := br.GetInt64("config:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_config, found, err := db.Config_GetByID(ctx, arm_config_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Config not found")
	}
	err = db.ManifestData_LinkToConfig(ctx, arm_mani_id, arm_config)
	if err != nil {
		t.Fatal(err)
	}
	// Query all links?
	manis, err := db.ManifestData_GetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(manis))
	// Add Layers
	arm_layer0_id, err := br.GetInt64("layer:0:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_layer0, found, err := db.Layer_Get_ByID(ctx, arm_layer0_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Layer not found")
	}
	err = db.ManifestData_LinkToLayer(ctx, arm_mani_id, arm_layer0, 0)
	if err != nil {
		t.Fatal(err)
	}
	arm_layer1_id, err := br.GetInt64("layer:1:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_layer1, found, err := db.Layer_Get_ByID(ctx, arm_layer1_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Layer not found")
	}
	err = db.ManifestData_LinkToLayer(ctx, arm_mani_id, arm_layer1, 1)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_ManifestData_Rules(t *testing.T) {
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
	br, err := SetupTest(bs)
	if err != nil {
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	aPlatGenID, err := br.GetInt64("annotation:org.supercontainers.hardware.cpu.target:platform#id")
	if err != nil {
		t.Fatal(err)
	}
	err = db.Manifest_LinkToAnnotation(ctx, maniID, aPlatGenID)
	if err != nil {
		t.Fatal(err)
	}
	// Actual test
	arm_mani := types.FT_arm_ManifestData()
	arm_mani_id, err := db.createManifestData(ctx, arm_mani)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, arm_mani_id)
	// Manifest Link To ManifestData
	err = db.LinkManifestToManifestData(ctx, maniID, arm_mani_id)
	if err != nil {
		t.Fatal(err)
	}
	// Link Config
	arm_config_id, err := br.GetInt64("config:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_config, found, err := db.Config_GetByID(ctx, arm_config_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Config not found")
	}
	err = db.ManifestData_LinkToConfig(ctx, arm_mani_id, arm_config)
	if err != nil {
		t.Fatal(err)
	}
	// Query all links?
	manis, err := db.ManifestData_GetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(manis))
	// Add Layers
	arm_layer0_id, err := br.GetInt64("layer:0:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_layer0, found, err := db.Layer_Get_ByID(ctx, arm_layer0_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Layer not found")
	}
	err = db.ManifestData_LinkToLayer(ctx, arm_mani_id, arm_layer0, 0)
	if err != nil {
		t.Fatal(err)
	}
	arm_layer1_id, err := br.GetInt64("layer:1:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	arm_layer1, found, err := db.Layer_Get_ByID(ctx, arm_layer1_id)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Layer not found")
	}
	err = db.ManifestData_LinkToLayer(ctx, arm_mani_id, arm_layer1, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = db.ManifestData_LinkToAnnotation(ctx, arm_mani_id, aPlatGenID)
	if err != nil {
		t.Fatal(err)
	}
	// Compute New OCI Manifest
	// Eval Rules
	err = db.Rule_EvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Get By Rule
	r, found, err := db.Rule_GetByLabel(ctx, "cpu:generic")
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

func Test_ManifestData_GetAllByManifestID(t *testing.T) {
	ctx := context.Background()
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
		bs.PostTestSleep = 120
	}
	br, err := SetupTest(bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {

		t.Fatal(err)

	}
	defer db.TeardownTest(bs)
	// Actual test
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	prfID, err := br.GetInt64("profile:cpu:generic#id")
	if err != nil {
		t.Fatal(err)
	}

	manis, err := db.ManifestData_GetAllByManifestID_WithProfile(ctx, maniID, prfID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(manis))
	// ManifestData_GetAllByManifestID
	manis, err = db.ManifestData_GetAllByManifestID(ctx, maniID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(manis))
}

func Test_ManifestData_LinkToOCIv1Layer(t *testing.T) {
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
		bs.PostTestSleep = 120
	}
	br, err := SetupTest(bs)
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
	// REMOVE LINK
	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	l0ID, err := br.GetInt64("layer:0:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	l0, found, err := db.Layer_Get_ByID(ctx, l0ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Layer not found")
	}
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[r]->(l:Layer) WHERE ID(md) = %d AND ID(l) = %d DELETE r`, mdID, l0ID)
	_, err = db.Execute(q)
	if err != nil {
		t.Fatal(err)
	}
	ld0 := types.NewLayerDesc_FromLayer(l0)
	err = db.ManifestData_LinkToOCIv1Layer(ctx, mdID, ld0)
	if err != nil {
		t.Fatal(err)
	}

}
func Test_ManifestData_LinkToOCIv1Config(t *testing.T) {
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
		bs.PostTestSleep = 120
	}
	br, err := SetupTest(bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {

		t.Fatal(err)

	}
	defer db.TeardownTest(bs)
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
	cfg, found, err := db.Config_GetByID(ctx, cfgID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Config not found")
	}
	q := fmt.Sprintf(`MATCH (md:ManifestData)-[r]->(c:Config) WHERE ID(md) = %d AND ID(c) = %d DELETE r`, mdID, cfgID)
	_, err = db.Execute(q)
	if err != nil {
		t.Fatal(err)
	}
	err = db.ManifestData_LinkToOCIv1Config(ctx, mdID, cfg)
	if err != nil {
		t.Fatal(err)
	}
}
