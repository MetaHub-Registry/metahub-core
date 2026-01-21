package magazine

/*
func TestBasic(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("cypher:collection:ft_tag1")
	bs.AddBootstrap("cypher:collection_only:ft_generic")
	bs.AddBootstrap("ComputeManifestlists")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	_, err := mag.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Test
	cols, err := mag.Database.CollectionGetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(cols), cols)
	req := types.Request{
		Repo: "qnib",
	}
	res, err := mag.CollectionsGetCatalog(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(res.Repos))
	//assert.Equal(t, 3, len(res.Repos))
}

/*
func TestCatalogARM64(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
	bs.AddBootstrap("ComputeManifestlists")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	_, err := mag.SetupTest(bs)
	if err != nil {
		t.Fatal(err)
}
	defer TeardownTest(bs, br)
	// Test
	cols, err := mag.Database.CollectionGetAll()
	if err != nil {
		t.Fatal(err)
}
	assert.Equal(t, 1, len(cols), cols)
	req := types.Request{
		Repo: "qnib",
	}
	res, err := mag.CollectionsGetCatalog(req)
	if err != nil {
		t.Fatal(err)
}
	assert.Equal(t, 1, len(res.Repos))

}
*/
