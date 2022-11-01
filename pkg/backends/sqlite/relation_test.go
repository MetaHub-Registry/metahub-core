package sqlite

/*
func Test_Relations_InsertRelation(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("profiles")
	bs.SetSqlite("Desktop/metahub.db")
	bs.SetSqlEmptyAll()
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	req := types.Request{
		Profile: "broadwell",
	}
	p1 := types.Example_Platform_broadwell()
	p1.Features = ""
	err = sq.InsertRelation(req, &p1)
	assert.NoError(t, err)
	assert.NotEqual(t, "", p1.Features)
}
*/
