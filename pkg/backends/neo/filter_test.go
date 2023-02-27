package neo

/*
func Test_Filter_CRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	/// Get Nothing
	fGen := types.NewEmptyFilter("generic")
	_, found, err := db.GetFilter(fGen)
	assert.NoError(t, err)
	assert.False(t, found)
	fGenID, err := db.CreateFilter(fGen)
	assert.NoError(t, err)
	assert.NotNil(t, fGenID)
	gotFGenID, found, err := db.GetFilter(fGen)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, fGenID, gotFGenID)
	fGen, found, err = db.Filter_Get_ByID(fGenID)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, fGenID, fGen.ID)
	// NonEmpty Filter
	fGen2 := types.NewEmptyFilter("generic2")
	rGen, err := types.NewRule("anno", "eq", "platform", "generic")
	assert.NoError(t, err)
	err = fGen2.AddRule(rGen)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(fGen2.Rules))
	fGen2ID, err := db.CreateFilter(fGen2)
	assert.NoError(t, err)
	gotFGen2ID, found, err := db.GetFilter(fGen2)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, fGen2ID, gotFGen2ID)
	// Check Rules
	rGen2, err := db.Rules_GetByFilterID(ctx, fGen2ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(rGen2))
}
*/
