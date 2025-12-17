package magazine

// func TestComputeAllManifestsWithManipulation(t *testing.T) {
// 	// Replicate the following test:
// 	/// ./misc/bin/parse-glow.sh ./misc/yaml/collections/qnib/featuretest/arm64.yaml
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("upstream:glow150arm64")
// 	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/generic.yaml")
// 	bs.AddBootstrap("layer:0:glow150arm64_2660")
// 	bs.AddBootstrap("layer:1:glow150arm64_3571")
// 	bs.AddBootstrap("layer:2:glow150arm64_d54e")
// 	bs.AddBootstrap("config:glow150arm64")
// 	bs.AddBootstrap("manifestdata:glow150arm64")
// 	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
// 	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
// 	// ManiData Setup
// 	bs.AddBootstrap("platforms:arm64")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.AddBootstrap("platforms:amd64")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("manifest:ft_amd64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
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
// 		t.Logf("Error while SetupTest: %v", err)
// 		TeardownTest(bs)
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs)
// 	// Connect Collection/Tag
// 	maniID, err := br.GetInt64("manifest:ft_arm64#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get manifest:ft_arm64#id: %v", err)
// 	}
// 	colID, err := br.GetInt64("collection:ft#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get collection:ft#id: %v", err)
// 	}
// 	tagID, err := br.GetInt64("tag:generic#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get tag:generic#id: %v", err)
// 	}
// 	if e := mag.Database.LinkCollectionTagToManifest(ctx, tagID, maniID); e != nil {
// 		t.Fatalf("Failed to LinkCollectionTagToManifest: %v", e)
// 	}
// 	if e := mag.Database.LinkCollectionToManifest(ctx, colID, maniID); e != nil {
// 		t.Fatalf("Failed to LinkCollectionToManifest: %v", e)
// 	}
// 	// Eval Rules
// 	if e := mag.Database.Rule_EvalAll(ctx); e != nil {
// 		t.Fatalf("Failed to Rule_EvalAll: %v", e)
// 	}
// 	// Actual test
// 	if e := mag.ComputeAllManifests(ctx); e != nil {
// 		t.Fatalf("Failed to ComputeAllManifests: %v", e)
// 	}
// 	// Fetch ManifestData with manipulation
// 	q := `MATCH (oci:ManifestData)-[]->(:Blob)<-[]-(dkr:ManifestData)<-[:USES]-(mpl:ManipuLayer)`
// 	q += ` RETURN DISTINCT id(oci) AS ociID`
// 	t.Log(q)
// 	res, found, err := mag.Database.QueryOne(q)
// 	if err != nil {
// 		t.Fatalf("Failed to QueryOne: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to QueryOne: not found")
// 	}
// 	mdID, err := res.GetInt64("ociID")
// 	if err != nil {
// 		t.Fatalf("Failed to GetInt64: %v", err)
// 	}

// 	md, found, err := mag.Database.ManifestData_Get_ByID(ctx, mdID)
// 	if err != nil {
// 		t.Fatalf("Failed to ManifestData_Get_ByID: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to ManifestData_Get_ByID: not found")
// 	}
// 	// TODO: The OCI ManifestData has 5 OCI layes and the manipulation layers connected, hence 8 in total
// 	assert.Equal(t, 8, len(md.Layers))

// }

// func TestComputeAllManifestsWithManipulationArm64(t *testing.T) {
// 	//This test check whether a manipulation is not done, due to a rule violation.
// 	// an AMD64 manifest should not apply a manipulation for ARM64
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("empty:all")
// 	bs.AddBootstrap("upstream:glow150arm64")
// 	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("layer:0:glow150arm64_2660")
// 	bs.AddBootstrap("layer:1:glow150arm64_3571")
// 	bs.AddBootstrap("layer:2:glow150arm64_d54e")
// 	bs.AddBootstrap("config:glow150arm64")
// 	bs.AddBootstrap("manifestdata:glow150arm64")
// 	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
// 	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
// 	// ManiData Setup
// 	bs.AddBootstrap("platforms:amd64")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("manifest:ft_amd64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
// 	bs.AddBootstrap("manifestdata:ft_amd64")
// 	bs.AddBootstrap("collection:ft_amd64")
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
// 		t.Logf("Error while SetupTest: %v", err)
// 		TeardownTest(bs)
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs)
// 	// Connect Collection/Tag
// 	maniID, err := br.GetInt64("manifest:ft_amd64#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get manifest:ft_amd64#id: %v", err)
// 	}
// 	colID, err := br.GetInt64("collection:ft#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get collection:ft#id: %v", err)
// 	}
// 	tagID, err := br.GetInt64("tag:amd64#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get tag:amd64#id: %v", err)
// 	}
// 	if e := mag.Database.LinkCollectionTagToManifest(ctx, tagID, maniID); e != nil {
// 		t.Fatalf("Failed to LinkCollectionTagToManifest: %v", e)
// 	}
// 	if e := mag.Database.LinkCollectionToManifest(ctx, colID, maniID); e != nil {
// 		t.Fatalf("Failed to LinkCollectionToManifest: %v", e)
// 	}
// 	// Actual test
// 	if e := mag.ComputeAllManifests(ctx); e != nil {
// 		t.Fatalf("Failed to ComputeAllManifests: %v", e)
// 	}
// 	// Fetch ManifestData with manipulation
// 	q := `MATCH (m:Manifest)-[:CREATED]->(oci:ManifestData)`
// 	q += fmt.Sprintf(` WHERE id(m) = %d`, maniID)
// 	q += ` RETURN DISTINCT id(oci) AS ociID`
// 	t.Log(q)
// 	res, found, err := mag.Database.QueryOne(q)
// 	if err != nil {
// 		t.Fatalf("Failed to QueryOne: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to QueryOne: not found")
// 	}
// 	mdID, err := res.GetInt64("ociID")
// 	if err != nil {
// 		t.Fatalf("Failed to GetInt64: %v", err)
// 	}

// 	md, found, err := mag.Database.ManifestData_Get_ByID(ctx, mdID)
// 	if err != nil {
// 		t.Fatalf("Failed to ManifestData_Get_ByID: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to ManifestData_Get_ByID: not found")
// 	}
// 	assert.Equal(t, 2, len(md.Layers))

// }

// func TestComputeManifest(t *testing.T) {
// 	// Replicate the following test:
// 	/// ./misc/bin/parse-glow.sh ./misc/yaml/collections/qnib/featuretest/arm64.yaml
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("upstream:glow150arm64")
// 	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/generic.yaml")
// 	bs.AddBootstrap("layer:0:glow150arm64_2660")
// 	bs.AddBootstrap("layer:1:glow150arm64_3571")
// 	bs.AddBootstrap("layer:2:glow150arm64_d54e")
// 	bs.AddBootstrap("config:glow150arm64")
// 	bs.AddBootstrap("manifestdata:glow150arm64")
// 	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
// 	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
// 	// ManiData Setup
// 	bs.AddBootstrap("platforms:arm64")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.AddBootstrap("collection:ft_arm64")
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
// 		t.Logf("Error while SetupTest: %v", err)
// 		TeardownTest(bs)
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs)
// 	// Connect Collection/Tag
// 	maniID, err := br.GetInt64("manifest:ft_arm64#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get manifest:ft_arm64#id: %v", err)
// 	}
// 	colID, err := br.GetInt64("collection:ft#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get collection:ft#id: %v", err)
// 	}
// 	tagID, err := br.GetInt64("tag:arm64#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get tag:arm64#id: %v", err)
// 	}
// 	if e := mag.Database.LinkCollectionTagToManifest(ctx, tagID, maniID); e != nil {
// 		t.Fatalf("Failed to LinkCollectionTagToManifest: %v", e)
// 	}
// 	if e := mag.Database.LinkCollectionToManifest(ctx, colID, maniID); e != nil {
// 		t.Fatalf("Failed to LinkCollectionToManifest: %v", e)
// 	}
// 	// Actual test
// 	mdID, err := br.GetInt64("manifest:ft_arm64#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get manifest:ft_arm64#id: %v", err)
// 	}
// 	mani, found, err := mag.Database.Manifest_GetByID(ctx, mdID)
// 	if err != nil {
// 		t.Fatalf("Failed to Manifest_GetByID: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to Manifest_GetByID: not found")
// 	}
// 	if e := mag.ComputeManifest(ctx, mani); e != nil {
// 		t.Fatalf("Failed to ComputeAllManifests: %v", e)
// 	}
// 	// Fetch ManifestData with manipulation
// 	q := `MATCH (oci:ManifestData)-[]->(:Blob) WHERE oci.mediaType = "application/vnd.oci.image.manifest.v1+json"`
// 	q += ` RETURN DISTINCT id(oci) AS ociID`
// 	t.Log(q)
// 	res, found, err := mag.Database.QueryOne(q)
// 	if err != nil {
// 		t.Fatalf("Failed to QueryOne: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to QueryOne: not found")
// 	}
// 	mdID, err = res.GetInt64("ociID")
// 	if err != nil {
// 		t.Fatalf("Failed to GetInt64: %v", err)
// 	}
// 	md, found, err := mag.Database.ManifestData_Get_ByID(ctx, mdID)
// 	if err != nil {
// 		t.Fatalf("Failed to ManifestData_Get_ByID: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to ManifestData_Get_ByID: not found")
// 	}
// 	assert.Equal(t, 2, len(md.Layers))
// }

// func TestComputeManifestForProfileGlow(t *testing.T) {
// 	// Replicate the following test:
// 	/// ./misc/bin/parse-glow.sh ./misc/yaml/collections/qnib/featuretest/arm64.yaml
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("upstream:glow150arm64")
// 	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/generic.yaml")
// 	bs.AddBootstrap("layer:0:glow150arm64_2660")
// 	bs.AddBootstrap("layer:1:glow150arm64_3571")
// 	bs.AddBootstrap("layer:2:glow150arm64_d54e")
// 	bs.AddBootstrap("config:glow150arm64")
// 	bs.AddBootstrap("manifestdata:glow150arm64")
// 	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
// 	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
// 	// ManiData Setup
// 	bs.AddBootstrap("platforms:arm64")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.AddBootstrap("collection:ft_arm64")
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
// 		t.Logf("Error while SetupTest: %v", err)
// 		TeardownTest(bs)
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs)
// 	// Connect Collection/Tag
// 	maniID, err := br.GetInt64("manifest:ft_arm64#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get manifest:ft_arm64#id: %v", err)
// 	}
// 	colID, err := br.GetInt64("collection:ft#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get collection:ft#id: %v", err)
// 	}
// 	tagID, err := br.GetInt64("tag:arm64#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get tag:arm64#id: %v", err)
// 	}
// 	if e := mag.Database.LinkCollectionTagToManifest(ctx, tagID, maniID); e != nil {
// 		t.Fatalf("Failed to LinkCollectionTagToManifest: %v", e)
// 	}
// 	if e := mag.Database.LinkCollectionToManifest(ctx, colID, maniID); e != nil {
// 		t.Fatalf("Failed to LinkCollectionToManifest: %v", e)
// 	}
// 	// Eval RUles
// 	if e := mag.Database.Rule_EvalAll(ctx); e != nil {
// 		t.Fatalf("Failed to Rule_EvalAll: %v", e)
// 	}
// 	// Actual test
// 	prfGlowID, err := br.GetInt64("profile:glow:1.5.0-arm64#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get profile:glow:1.5.0-arm64#id: %v", err)
// 	}
// 	prfGlow, found, err := mag.Database.Profile_Get_ByID(ctx, prfGlowID)
// 	if err != nil {
// 		t.Fatalf("Failed to Profile_Get_ByID: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to Profile_Get_ByID: not found")
// 	}
// 	mdID, err := br.GetInt64("manifest:ft_arm64#id")
// 	if err != nil {
// 		t.Fatalf("Failed to get manifest:ft_arm64#id: %v", err)
// 	}
// 	mani, found, err := mag.Database.Manifest_GetByID(ctx, mdID)
// 	if err != nil {
// 		t.Fatalf("Failed to Manifest_GetByID: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to Manifest_GetByID: not found")
// 	}
// 	if e := mag.ComputeManifestForProfile(ctx, mani, prfGlow); e != nil {
// 		t.Fatalf("Failed to ComputeAllManifests: %v", e)
// 	}
// 	// Fetch ManifestData with manipulation
// 	q := `MATCH (oci:ManifestData)-[]->(:Blob) WHERE oci.mediaType = "application/vnd.oci.image.manifest.v1+json"`
// 	q += ` RETURN DISTINCT id(oci) AS ociID`
// 	t.Log(q)
// 	res, found, err := mag.Database.QueryOne(q)
// 	if err != nil {
// 		t.Fatalf("Failed to QueryOne: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to QueryOne: not found")
// 	}
// 	mdID, err = res.GetInt64("ociID")
// 	if err != nil {
// 		t.Fatalf("Failed to GetInt64: %v", err)
// 	}
// 	md, found, err := mag.Database.ManifestData_Get_ByID(ctx, mdID)
// 	if err != nil {
// 		t.Fatalf("Failed to ManifestData_Get_ByID: %v", err)
// 	} else if !found {
// 		t.Fatalf("Failed to ManifestData_Get_ByID: not found")
// 	}
// 	// TODO: The OCI ManifestData has 5 OCI layes and the manipulation layers connected, hence 8 in total
// 	assert.Equal(t, 8, len(md.Layers))
// }
