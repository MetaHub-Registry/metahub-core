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
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
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
	assert.NoError(t, err)
	assert.False(t, found)
	_, found, err = db.Profile_Get_ByNameTag(ctx, "foo", "tag1")
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
	gotPrf1, found, err := db.Profile_Get_ByNameTag(ctx, "foo", "tag1")
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
