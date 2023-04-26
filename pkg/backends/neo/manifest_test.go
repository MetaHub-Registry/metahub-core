package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Manifest_CR(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	/// Get Upstream
	upID, err := br.GetInt64("upstream:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	_, found, err := db.Manifest_Get(upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	/// create Manifest
	maniID, err := db.Manifest_Create(ctx, upID)
	if err != nil {
		t.Fatal(err)
	}
	/// get Manifest
	gotManiID, found, err := db.Manifest_Get(upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, maniID, gotManiID)
}

func Test_Manifest_GetInsert(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
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
	_, found, err := db.Manifest_Get(upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	/// create Manifest
	up, err := db.Upstream_Get_ByID(ctx, upID)
	if err != nil {
		t.Fatal(err)
	}
	mani := types.Manifest{
		Upstream: up,
	}
	maniID, err := db.Manifest_GetInsert(ctx, mani)
	if err != nil {
		t.Fatal(err)
	}
	/// get Manifest
	gotManiID, found, err := db.Manifest_Get(upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, maniID, gotManiID)
}
