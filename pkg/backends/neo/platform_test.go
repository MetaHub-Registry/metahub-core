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
	bs.AddBootstrap("filter")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 45
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	platform_amd64 := types.Platform{Arch: "amd64", Variant: "", OS: "linux", Features: ""}
	platform_arm64 := types.Platform{Arch: "arm64", Variant: "", OS: "linux", Features: ""}
	_, found, err := db.GetPlatform(platform_amd64)
	assert.NoError(t, err)
	assert.False(t, found)
	platAmd64ID, err := db.CreatePlatform(platform_amd64)
	assert.NoError(t, err)
	assert.NotNil(t, platAmd64ID)
	gotPlatAmd64ID, found, err := db.GetPlatform(platform_amd64)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, platAmd64ID, gotPlatAmd64ID)
	// GetInsert
	err = db.Platform_GetInsert(&platform_amd64)
	assert.NoError(t, err)
	assert.Equal(t, platAmd64ID, platform_amd64.ID)
	// ARM64
	err = db.Platform_GetInsert(&platform_arm64)
	assert.NoError(t, err)
	assert.Greater(t, platform_arm64.ID, platAmd64ID)
	fGenID, err := br.GetInt64("filter:generic#id")
	assert.NoError(t, err)
	// LinkPlatformToFilter
	err = db.LinkPlatformIDToFilterID(ctx, platform_amd64.ID, fGenID)
	assert.NoError(t, err)
	// GetFilter
	f, found, err := db.Filter_Get_ByID(fGenID)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, fGenID, f.ID)
	err = db.LinkPlatformIDToFilter(ctx, platform_arm64.ID, f)
	assert.NoError(t, err)

}
