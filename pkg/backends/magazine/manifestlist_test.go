package magazine

// func TestManifestlistCompute(t *testing.T) {
// 	ctx := context.Background()
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_amd64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_amd64")
// 	bs.AddBootstrap("collection:ft_generic")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}

// 	br, err := mag.SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		TeardownTest(bs)
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs)
// 	// Actual Test
// 	err = mag.Database.Rule_EvalAll(ctx)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	maniID, err := br.GetInt64("manifest:ft_amd64#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	prfID, err := br.GetInt64("profile:cpu:generic#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	prf, found, err := mag.Database.Profile_Get_ByID(ctx, prfID)
// 	if err != nil {
// 		t.Fatal(err)
// 	} else if !found {
// 		t.Fatal("Profile not found")
// 	}
// 	t.Logf("Profile ID: %d // ManiID: %d", prfID, maniID)
// 	mani, found, err := mag.Database.Manifest_GetByID(ctx, maniID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !found {
// 		t.Fatal("Manifest not found")
// 	}
// 	// Actual test
// 	err = mag.ComputeManifestForProfile(ctx, mani, prf)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	err = mag.Database.Rule_EvalAll(ctx)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if e := mag.ComputeManifestlists(ctx); e != nil {
// 		t.Fatal(e)
// 	}
// 	// TODO: Remove Collection w/ Manifestlist
// 	req := types.Request{
// 		Repo: "qnib/featuretest",
// 		Tag:  "generic",
// 	}
// 	found, err = mag.RemoveCollectionByRequest(ctx, req)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.True(t, found)
// }

// func TestManifestlistComputeWithManipulation(t *testing.T) {
// 	ctx := context.Background()
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("empty:all")
// 	bs.AddBootstrap("upstream:glow150arm64")
// 	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.AddBootstrap("layer:0:glow150arm64_2660")
// 	bs.AddBootstrap("layer:1:glow150arm64_3571")
// 	bs.AddBootstrap("layer:2:glow150arm64_d54e")
// 	bs.AddBootstrap("config:glow150arm64")
// 	bs.AddBootstrap("manifestdata:glow150arm64")
// 	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
// 	//bs.AddBootstrap("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#glow150arm64_.*")
// 	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
// 	// Collection
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_amd64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_amd64")
// 	bs.AddBootstrap("collection:ft_generic")

// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}

// 	br, err := mag.SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		TeardownTest(bs)
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs)
// 	// Actual Test
// 	err = mag.Database.Rule_EvalAll(ctx)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	maniID, err := br.GetInt64("manifest:ft_arm64#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	prfID, err := br.GetInt64("profile:glow:1.5.0-arm64#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	prf, found, err := mag.Database.Profile_Get_ByID(ctx, prfID)
// 	if err != nil {
// 		t.Fatal(err)
// 	} else if !found {
// 		t.Fatal("Profile not found")
// 	}
// 	t.Logf("Profile ID: %d // ManiID: %d", prfID, maniID)
// 	mani, found, err := mag.Database.Manifest_GetByID(ctx, maniID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !found {
// 		t.Fatal("Manifest not found")
// 	}
// 	// Actual test
// 	err = mag.ComputeManifestForProfile(ctx, mani, prf)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if e := mag.ComputeManifestlists(ctx); e != nil {
// 		t.Fatal(e)
// 	}
// 	req := types.Request{
// 		Repo:    "qnib/featuretest",
// 		Tag:     "generic",
// 		Profile: "glow#1.5.0-arm64",
// 	}

// 	if ml, found, err := mag.Database.ManifestList_Get_ByRequest(ctx, req); err != nil {
// 		t.Fatal(err)
// 	} else if !found {
// 		t.Fatal("ManifestList not found")
// 	} else {
// 		assert.Equal(t, 1, len(ml.Manifests))
// 	}
// }
