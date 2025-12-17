package magazine

// func TestLayerStoreBlobUpload(t *testing.T) {
// 	bu, b := types.FT_arm_BlobUpload_Layer0()
// 	err := mag.StoreBlobUpload(ctx, &bu, b)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	_, found, err := mag.Database.LayerGetByDigest(ctx, bu.Digest)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.True(t, found)
// 	req := types.Request{
// 		Repo:   bu.Repo,
// 		Tag:    "",
// 		Digest: bu.Digest,
// 	}
// 	if found, err := mag.BlobDelete(ctx, req); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		assert.True(t, found)
// 	}

// }

// func TestConfigStoreBlobUpload(t *testing.T) {
// 	ctx := context.Background()
// 	bs := utils.NewBootstrap()
// 	bs.AddBootstrap("users")
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.SetEmptyAll()
// 	_, err := mag.SetupTest(ctx, bs)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs)
// 	// Tests
// 	bu, b := types.FT_arm_BlobUpload_Config()
// 	_ = b
// 	_, found, err := mag.Database.Config_GetByDigest(ctx, bu.Digest)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.False(t, found)
// 	err = mag.StoreBlobUpload(ctx, &bu, b)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if err = mag.StoreBlobUpload(ctx, &bu, []byte(`{"foo":"bar"}`)); err != ErrJSONNOTCONFIG {
// 		if err == nil {
// 			t.Fatal("Should have failed")
// 		}
// 		t.Fatal(err)
// 	}
// 	req := types.Request{
// 		Repo:   bu.Repo,
// 		Tag:    "",
// 		Digest: bu.Digest,
// 	}

// 	// Blob Exists
// 	upt := upstream.UpstreamTriple{
// 		Repo:   bu.Repo,
// 		Digest: digest.Digest(bu.Digest),
// 	}
// 	if found, err := mag.BlobExists(ctx, upt); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		assert.True(t, found)
// 	}

// 	if found, err := mag.BlobDelete(ctx, req); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		assert.True(t, found)
// 	}
// }

// func TestBlobChunkCRU(t *testing.T) {
// 	ctx := context.Background()
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	br, err := mag.SetupTest(ctx, bs)
// 	if err != nil {
// 		TeardownTest(bs)
// 		t.Fatal(err)
// 	}
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	defer TeardownTest(bs)
// 	// Actual test
// 	//// CREATE
// 	ociBU := types.OCIBlobUpload{
// 		Repo:          "qnib/featuretest",
// 		UploadUser:    "test",
// 		UploadProfile: "generic",
// 		BlobType:      "layer",
// 		Digest:        "sha256:6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30",
// 	}
// 	err = mag.Database.BlobUpload_Create_ByOCI(ctx, &ociBU)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	bc, err := mag.Database.UploadChunk_Create(ctx, &ociBU, 0, 10)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.NotNil(t, bc.ID)
// 	// / READ
// 	bcs, err := mag.Database.UploadChunk_Get_ByUUID(ctx, ociBU.ID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 1, len(bcs))
// 	// LinkBlobUpdate
// 	if e := mag.Database.BlobUpload_Update_ByUUID(ctx, &ociBU); e != nil {
// 		t.Fatal(e)
// 	}
// 	// BlobUploadGetByBlobDigest
// 	upt := upstream.UpstreamTriple{
// 		Repo:   ociBU.Repo,
// 		Digest: digest.Digest(ociBU.Digest),
// 	}
// 	if found, err := mag.BlobExists(ctx, upt); err != nil {
// 		t.Fatal(err)
// 	} else if !found {
// 		t.Fatal("Did not find BlobUpload")
// 	}
// }
