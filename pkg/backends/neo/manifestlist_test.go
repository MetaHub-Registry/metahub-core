package neo

/*
func TestManifestListCRUD(t *testing.T) {
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
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 120
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
}
	defer db.TeardownTest(bs)
	// Actual test
	ml_arm64 := types.FT_arm_Manifestlist()
	_, found, err := db.GetManifestList(digest.Digest(ml_arm64.Digest), ml_arm64.MediaType, int(ml_arm64.Size))
	if err != nil {
		t.Fatal(err)
}
	assert.False(t, found)
	// Add ManifestList
	ml_arm64_id, err := db.CreateManifestList(digest.Digest(ml_arm64.Digest), ml_arm64.MediaType, int(ml_arm64.Size))
	if err != nil {
		t.Fatal(err)
}
	assert.NotNil(t, ml_arm64_id)
	ml_arm64_id_got, found, err := db.GetManifestList(digest.Digest(ml_arm64.Digest), ml_arm64.MediaType, int(ml_arm64.Size))
	if err != nil {
		t.Fatal(err)
}
	assert.True(t, found)
	assert.Equal(t, ml_arm64_id, ml_arm64_id_got)
	// LINKS
	//// Filter
	flt_arm64_id, err := br.GetInt64("filter:generic#id")
	if err != nil {
		t.Fatal(err)
}
	err = db.LinkManifestListToFilter(ml_arm64_id, flt_arm64_id)
	if err != nil {
		t.Fatal(err)
}
	//// Manifest
	arm_ml := types.FT_arm_ManifestData()
	arm_ml.ID, err = br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
}
	amd_ml := types.FT_arm_ManifestData()
	amd_ml.ID, err = br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
}
	//// GENERIC ML
	ml_generic := types.FT_generic_Manifestlist()
	ml_generic_id, err := db.CreateManifestList(digest.Digest(ml_generic.Digest), ml_generic.MediaType, int(ml_generic.Size))
	if err != nil {
		t.Fatal(err)
}
	assert.NotNil(t, ml_generic_id)
	err = db.LinkManifestListToManifests(ml_generic_id, []types.ManifestData{arm_ml, amd_ml})
	if err != nil {
		t.Fatal(err)
}
}

func TestManifestListByRequestSeedNeoSkylake(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("cypher:manifestlist:ft_generic")
	bs.AddBootstrap("cypher:manifestlist:ft_skylake")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := SetupTest(bs)
	_ = br
	if err != nil {
		t.Fatal(err)
}
	defer db.TeardownTest(bs)
	// Actual Test
	//// Generic
	req := types.Request{
		Host:    "metahub-registry.org",
		Repo:    "qnib/featuretest",
		Tag:     "tag1",
		Profile: "generic",
	}
	ml, found, err := db.ManifestList_Get_ByRequest(req)
	if err != nil {
		t.Fatal(err)
}
	assert.True(t, found)
	assert.Equal(t, 2, len(ml.Manifests))
	//// Skylake
	req.Profile = "skylake"
	ml, found, err = db.ManifestList_Get_ByRequest(req)
	if err != nil {
		t.Fatal(err)
}
	assert.True(t, found)
	assert.Equal(t, 1, len(ml.Manifests))
}
*/
