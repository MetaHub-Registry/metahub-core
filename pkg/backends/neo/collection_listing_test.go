package neo

/*
func TestCollectionListingGetByID(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
}
	defer db.TeardownTest(bs)
	// Actual test
	colID, err := br.GetInt64("collection:ft#id")
	if err != nil {
		t.Fatal(err)
}
	colTagID, err := br.GetInt64("tag:generic#id")
	if err != nil {
		t.Fatal(err)
}
	cl, err := db.CollectionListing_Get_ByID(colID, colTagID)
	if err != nil {
		t.Fatal(err)
}
	assert.Equal(t, colID, cl.ID)
	assert.Equal(t, colTagID, cl.ColTag.ID)
	assert.Equal(t, "generic", cl.ColTag.Tag)
	assert.Equal(t, "qnib/featuretest", cl.Repo)
	assert.Equal(t, int64(2), cl.ManifestCount)
	assert.NotEqual(t, 0, len(cl.Manifests), fmt.Sprintf("Expected manifest to be present: %+v", cl.Manifests))
		// Like
	cls, err := db.CollectionListing_Get_ByRepoLike("")
	if err != nil {
		t.Fatal(err)
}
	assert.Equal(t, 2, len(cls))
	cls, err = db.CollectionListing_Get_ByRepoLike("qnib")
	if err != nil {
		t.Fatal(err)
}
	assert.Equal(t, 2, len(cls))
	cls, err = db.CollectionListing_Get_ByRepoLike("qnib/nope")
	if err != nil {
		t.Fatal(err)
}
	assert.Equal(t, 0, len(cls))
}
*/
