package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Platform_CRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 45
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
	// Actual test
	platform_amd64 := types.Platform{Arch: "amd64", Variant: "", OS: "linux"}
	platform_arm64 := types.Platform{Arch: "arm64", Variant: "", OS: "linux"}
	_, found, err := db.GetPlatform(ctx, platform_amd64)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	platAmd64ID, err := db.CreatePlatform(ctx, platform_amd64)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, platAmd64ID)
	gotPlatAmd64ID, found, err := db.GetPlatform(ctx, platform_amd64)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, platAmd64ID, gotPlatAmd64ID)
	// GetInsert
	err = db.Platform_GetInsert(ctx, &platform_amd64)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, platAmd64ID, platform_amd64.ID)
	// ARM64
	err = db.Platform_GetInsert(ctx, &platform_arm64)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, platform_arm64.ID, platAmd64ID)
}
