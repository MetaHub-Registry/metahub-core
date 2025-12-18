package neo

// func TestCollectionTagProfilePair_Platform_GetRegexFiltered(t *testing.T) {
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
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
// 	// ManiData Setup
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	br, err := SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		db.TeardownTest(bs)
// 		t.Fatal(err)
// 	}
// 	defer db.TeardownTest(bs)
// 	// Actual test
// 	ctpps, err := db.CollectionTagProfilePairGetAll(ctx)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 3, len(ctpps))
// }
