package neo

// func TestManipuCmdCRUD(t *testing.T) {
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
// 	bs.AddBootstrap("upstream:glow150arm64")
// 	bs.AddBootstrap("layer:0:glow150arm64_2660")
// 	bs.AddBootstrap("layer:1:glow150arm64_3571")
// 	bs.AddBootstrap("layer:2:glow150arm64_d54e")
// 	bs.AddBootstrap("config:glow150arm64")
// 	bs.AddBootstrap("manifestdata:glow150arm64")
// 	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
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
// 		db.TeardownTest(bs, br)
// 		t.Fatal(err)
// 	}
// 	defer db.TeardownTest(bs, br)
// 	// Actual test
// 	mplID, err := br.GetInt64("manipulation:layer/glow#1.5.0-arm64#id")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	var mplyrID int64
// 	if mpls, err := db.ManipuLayersGetByManipulationID(ctx, mplID); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		if len(mpls) != 1 {
// 			t.Fatalf("Expected 1 ManipuLayer, but got %d", len(mpls))
// 		}
// 		mplyrID = mpls[0].ID
// 	}

// 	mpc := types.ManipuCmdPrependHelp
// 	if ok, e := db.ManipuCmdGet(ctx, &mpc); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		if ok {
// 			t.Fatal("Should not have found ManipuCmd")
// 		}
// 	}
// 	if e := db.ManipuCmdCreate(ctx, mplyrID, &mpc); e != nil {
// 		t.Fatal(e)
// 	}
// 	if ok, e := db.ManipuCmdGet(ctx, &mpc); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		if !ok {
// 			t.Fatal("Should HAVE been found ManipuCmd")
// 		}
// 	}
// 	// GetCreate
// 	mpc2 := types.ManipuCmdAppendHelp
// 	if e := db.ManipuCmdInsert(ctx, mplyrID, &mpc2); e != nil {
// 		t.Fatal(e)
// 	}
// 	if ok, e := db.ManipuCmdGet(ctx, &mpc2); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		if !ok {
// 			t.Fatal("Should HAVE been found ManipuCmd after inserting")
// 		}
// 	}
// 	if e := db.ManipuCmdInsert(ctx, mplyrID, &mpc2); e != nil {
// 		t.Fatal(e)
// 	}
// 	// ManipuCmdsGetByManipulayerID
// 	if mpcs, err := db.ManipuCmdsGetByManipulayerID(ctx, mplyrID); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		if len(mpcs) != 2 {
// 			t.Fatalf("Expected 2 ManipuCmd, but got %d", len(mpcs))
// 		}
// 	}
// }
