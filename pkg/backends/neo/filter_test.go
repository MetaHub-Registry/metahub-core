package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var (
	fGen = types.Filter{
		Label:   "generic",
		Feature: "",
	}
	fFoo = types.Filter{
		Label:   "foo",
		Feature: "foo",
	}
	pFoo = types.Platform{
		OS:       "linux",
		Arch:     "amd64",
		Variant:  "",
		Features: "foo",
	}
	pGen = types.Platform{
		OS:       "linux",
		Arch:     "amd64",
		Variant:  "",
		Features: "",
	}
)

func Test_Filter_CRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	/// Get Nothing
	_, found, err := db.GetFilter(fGen)
	assert.NoError(t, err)
	assert.False(t, found)
	fGenID, err := db.CreateFilter(fGen)
	assert.NoError(t, err)
	assert.NotNil(t, fGenID)
	getFltGenID, found, err := db.GetFilter(fGen)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, fGenID, getFltGenID)
	fFooID, err := db.Filter_Insert(ctx, fFoo)
	assert.NoError(t, err)
	assert.NotNil(t, fFooID)
	getFltFooID, found, err := db.GetFilter(fFoo)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, fFooID, getFltFooID)
	r, err := db.Query("MATCH (n:Filter) WHERE n.label = 'foo' RETURN id(n) AS filterID, n.label AS filterLabel, n.feature AS filterFeature")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(r))
	r0 := r[0]
	fltFoo, err := FilterFromResult(r0)
	assert.NoError(t, err)
	assert.Equal(t, fFooID, fltFoo.ID)
	// GetByID
	flt, found, err := db.Filter_Get_ByID(fFooID)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, fFooID, flt.ID)
}

func Test_Filter_GetByFeature(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 45
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	fltrs, err := db.Filter_Get_ByFeatureRegex("foo")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(fltrs))
	fltrs, err = db.Filter_Get_ByFeatureRegex("^$")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(fltrs))
	_, err = db.CreateFilter(fFoo)
	assert.NoError(t, err)
	_, err = db.CreateFilter(fGen)
	assert.NoError(t, err)
	fltrs, err = db.Filter_Get_ByFeatureRegex("foo")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(fltrs))
	fltrs, err = db.Filter_Get_ByFeatureRegex("^$")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(fltrs))
	fGenGot := fltrs[0]
	// Check Platform link
	err = db.Platform_GetInsert(&pGen)
	assert.NoError(t, err)
	assert.NotNil(t, pGen.ID)
	err = db.Filter_UpdatePlatform(ctx, pGen)
	assert.NoError(t, err)
	q := `MATCH (f:Filter)-[:USES]->(p:Platform) WHERE id(f) = %d RETURN id(p) AS platformID`
	q = fmt.Sprintf(q, fGenGot.ID)
	r, err := db.Query(q)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(r))
	// Filters with features
	err = db.Platform_GetInsert(&pFoo)
	assert.NoError(t, err)
	assert.NotNil(t, pFoo.ID)
	err = db.Filter_UpdatePlatform(ctx, pFoo)
	assert.Error(t, err)
	/* TODO: Need to implement
	q = `MATCH (f:Filter)-[:USES]->(p:Platform) WHERE id(p) = %d RETURN id(f) AS filterID`
	q = fmt.Sprintf(q, pFoo.ID)
	r, err = db.Query(q)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(r))
	*/
}
