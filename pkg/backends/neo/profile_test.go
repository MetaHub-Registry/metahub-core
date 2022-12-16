package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Profile_CRUD(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	pFoo := types.Profile{
		Name: "foo",
	}
	pGen := types.Profile{
		Name: "generic",
	}
	_, found, err := db.GetProfile(pFoo)
	assert.NoError(t, err)
	assert.False(t, found)
	_, found, err = db.Profile_Get_ByName(ctx, "foo")
	assert.NoError(t, err)
	assert.False(t, found)
	prfs, err := db.GetProfiles()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(prfs))
	// Create first profile
	pFooID, err := db.CreateProfile(pFoo)
	assert.NoError(t, err)
	assert.NotNil(t, pFooID)
	gotPrfFooID, found, err := db.GetProfile(pFoo)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, pFooID, gotPrfFooID)
	gotPrf1, found, err := db.Profile_Get_ByName(ctx, "foo")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, pFooID, gotPrf1.ID)
	prfs, err = db.GetProfiles()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(prfs))
	// GetInsert
	err = db.Profile_GetInsert(ctx, &pFoo)
	assert.NoError(t, err)
	assert.Equal(t, pFooID, pFoo.ID)
	err = db.Profile_GetInsert(ctx, &pGen)
	assert.NoError(t, err)
	assert.NotNil(t, pGen.ID)
}

func Test_GetProfileFilterPlatformPairs(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	pfs, err := db.GetProfileFilterPlatformPairs()
	assert.NoError(t, err)
	assert.Equal(t, 18, len(pfs))
}
