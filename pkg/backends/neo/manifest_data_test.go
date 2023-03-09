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
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	arm_mani := types.FT_arm_ManifestData()
	_, found, err := db.ManifestData_GetExactly(arm_mani)
	assert.NoError(t, err)
	assert.False(t, found)
	arm_mani_id, err := db.createManifestData(ctx, arm_mani)
	assert.NoError(t, err)
	arm_mani_id_got1, found, err := db.ManifestData_GetExactly(arm_mani)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, arm_mani_id_got1)
	// Link Manifest to ManifestData
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	assert.NoError(t, err)
	err = db.LinkManifestToManifestData(ctx, maniID, arm_mani_id_got1)
	assert.NoError(t, err)
	// Manifest_Get_ByDigest
	arm_mani_got3, found, err := db.ManifestData_Get_ByDigest(ctx, arm_mani.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, arm_mani_id, arm_mani_got3.ID)
	/*
		// Manifest_Remove
		reqGot3 := types.Request{
			Repo:   arm_mani_got3.Upstream.Repo,
			Tag:    arm_mani_got3.Upstream.Tag,
			Host:   arm_mani_got3.Upstream.Reg,
			Digest: arm_mani_got3.Digest,
		}
		found, err = db.ManifestData_Remove(ctx, reqGot3)
		assert.NoError(t, err)
		assert.True(t, found)
		_, found, err = db.ManifestData_Get_ByDigest(arm_mani_got3.Digest)
		assert.NoError(t, err)
		assert.False(t, found)
		// Manifest_GetInsert
		err = db.ManifestData_GetInsert(ctx, &arm_mani)
		assert.NoError(t, err)
		/// Add Upstream
		found, err = db.Upstream_GetInsert(&up)
		assert.NoError(t, err)
		assert.True(t, found)
		err = db.Manifest_LinkToUpstream(ctx, arm_mani.ID, up.ID)
		assert.NoError(t, err)
		arm_mani_got4, found, err := db.ManifestData_Get_ByDigest(arm_mani.Digest)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, arm_mani.ID, arm_mani_got4.ID)
		arm_mani.ID = -1
		err = db.ManifestData_GetInsert(ctx, &arm_mani)
		assert.NoError(t, err)
		assert.Equal(t, arm_mani_got4.ID, arm_mani.ID)
		arm_mani_got5, err := db.ManifestData_Insert(ctx, arm_mani)
		assert.NoError(t, err)
		assert.Equal(t, arm_mani_got4.ID, arm_mani_got5)
		reqArm := types.Request{
			Repo:   arm_mani.Upstream.Repo,
			Tag:    arm_mani.Upstream.Tag,
			Host:   arm_mani.Upstream.Reg,
			Digest: arm_mani.Digest,
		}
		found, err = db.ManifestData_Remove(ctx, reqArm)
		assert.NoError(t, err)
		assert.True(t, found)
		arm_mani_got6, err := db.ManifestData_Insert(ctx, arm_mani)
		assert.NoError(t, err)
		assert.NotEqual(t, arm_mani_got4.ID, arm_mani_got6)
	*/
}

func Test_ManifestData_Links(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("config:ft_arm")
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
	assert.NoError(t, err)
	aPlatGenID, err := br.GetInt64("annotation:platform:generic#id")
	assert.NoError(t, err)
	err = db.Manifest_LinkToAnnotation(ctx, maniID, aPlatGenID)
	assert.NoError(t, err)
	// Actual test
	arm_mani := types.FT_arm_ManifestData()
	arm_mani_id, err := db.createManifestData(ctx, arm_mani)
	assert.NoError(t, err)
	assert.NotNil(t, arm_mani_id)
	// Manifest Link To ManifestData
	err = db.LinkManifestToManifestData(ctx, maniID, arm_mani_id)
	assert.NoError(t, err)
	// Link Config
	arm_config_id, err := br.GetInt64("config:ft_arm64#id")
	assert.NoError(t, err)
	err = db.ManifestData_LinkToConfig(ctx, arm_mani_id, arm_config_id)
	assert.NoError(t, err)
	// Query all links?
	manis, err := db.ManifestData_GetAll()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(manis))
	// Add Layers
	arm_layer0_id, err := br.GetInt64("layer:0:ft_arm64#id")
	assert.NoError(t, err)
	err = db.ManifestData_LinkToLayer(ctx, arm_mani_id, arm_layer0_id, 0)
	assert.NoError(t, err)
	arm_layer1_id, err := br.GetInt64("layer:1:ft_arm64#id")
	assert.NoError(t, err)
	err = db.ManifestData_LinkToLayer(ctx, arm_mani_id, arm_layer1_id, 1)
	assert.NoError(t, err)
}

func Test_ManifestData_Rules(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target:platform")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	assert.NoError(t, err)
	aPlatGenID, err := br.GetInt64("annotation:org.supercontainers.hardware.cpu.target:platform#id")
	assert.NoError(t, err)
	err = db.Manifest_LinkToAnnotation(ctx, maniID, aPlatGenID)
	assert.NoError(t, err)
	// Actual test
	arm_mani := types.FT_arm_ManifestData()
	arm_mani_id, err := db.createManifestData(ctx, arm_mani)
	assert.NoError(t, err)
	assert.NotNil(t, arm_mani_id)
	// Manifest Link To ManifestData
	err = db.LinkManifestToManifestData(ctx, maniID, arm_mani_id)
	assert.NoError(t, err)
	// Link Config
	arm_config_id, err := br.GetInt64("config:ft_arm64#id")
	assert.NoError(t, err)
	err = db.ManifestData_LinkToConfig(ctx, arm_mani_id, arm_config_id)
	assert.NoError(t, err)
	// Query all links?
	manis, err := db.ManifestData_GetAll()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(manis))
	// Add Layers
	arm_layer0_id, err := br.GetInt64("layer:0:ft_arm64#id")
	assert.NoError(t, err)
	err = db.ManifestData_LinkToLayer(ctx, arm_mani_id, arm_layer0_id, 0)
	assert.NoError(t, err)
	arm_layer1_id, err := br.GetInt64("layer:1:ft_arm64#id")
	assert.NoError(t, err)
	err = db.ManifestData_LinkToLayer(ctx, arm_mani_id, arm_layer1_id, 1)
	assert.NoError(t, err)
	err = db.ManifestData_LinkToAnnotation(ctx, arm_mani_id, aPlatGenID)
	assert.NoError(t, err)
	// Compute New OCI Manifest
	// Eval Rules
	err = db.Rule_EvalAll(ctx)
	assert.NoError(t, err)
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
