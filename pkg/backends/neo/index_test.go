package neo

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManifestListCRUD(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
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
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestManifestListCRUD.Setup",
	})
	defer span.End()
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer db.TeardownTest(bs, br)
	span.End()
	// Actual test
	_, ctx, span = utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestManifestListCRUD.Run",
	})
	defer span.End()
	ml_arm64 := types.FTarmManifestlist()
	_, found, err := db.GetManifestList(digest.Digest(ml_arm64.Digest), ml_arm64.MediaType, int(ml_arm64.Size))
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	assert.False(t, found)
	// Add Index
	ml_arm64_id, err := db.CreateManifestList(digest.Digest(ml_arm64.Digest), ml_arm64.MediaType, int(ml_arm64.Size))
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	assert.NotNil(t, ml_arm64_id)
	ml_arm64_id_got, found, err := db.GetManifestList(digest.Digest(ml_arm64.Digest), ml_arm64.MediaType, int(ml_arm64.Size))
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, ml_arm64_id, ml_arm64_id_got)
	// LINKS
	//// Manifest
	arm_ml := types.FTarmManifest()
	arm_ml.ID, err = br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	amd_ml := types.FTarmManifest()
	amd_ml.ID, err = br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	//// GENERIC ML
	ml_generic := types.FTgenericManifestlist()
	ml_generic_id, err := db.CreateManifestList(digest.Digest(ml_generic.Digest), ml_generic.MediaType, int(ml_generic.Size))
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	assert.NotNil(t, ml_generic_id)
	err = db.LinkManifestListToManifests(ml_generic_id, []types.Manifest{arm_ml, amd_ml})
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	// Fetch Index by Digest
	mlDig := digest.Digest(ml_arm64.Digest)
	ml_got, err := db.ManifestListGetByDigest(ctx, mlDig.String())
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	assert.NotNil(t, ml_got)
	span.End()
	a := types.NewAnnotation("key", "val")
	if err := db.ManifestListAddAnnotation(ctx, ml_got.ID, &a); err != nil {
		span.End()
		t.Fatal(err)
	}
	ml_got2, err := db.ManifestListGetByDigest(ctx, mlDig.String())
	if err != nil {
		span.End()
		t.Fatal(err)
	}
	assert.NotNil(t, ml_got2)
	if val, found := ml_got2.Annotations["key"]; !found {
		span.End()
		t.Fatal("Annotation key not found")
	} else {
		assert.Equal(t, "val", val)
	}
}

// func TestManifestListByRequestSeedNeoSkylake(t *testing.T) {
// 	ctx := context.Background()
// 	_ = ctx
// 	bs := utils.NewBootstrap()
// 	bs.AddBootstrap("cypher:manifestlist:ft_generic")
// 	bs.AddBootstrap("cypher:manifestlist:ft_skylake")
// 	bs.SetEmptyAll()
// 	//bs.PostTestSleep = 300
// 	br, err := SetupTest(ctx, bs)
// 	_ = br
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer db.TeardownTest(bs, br)
// 	// Actual Test
// 	//// Generic
// 	req := types.Request{
// 		Host:    "metahub-registry.org",
// 		Repo:    "qnib/featuretest",
// 		Tag:     "tag1",
// 		Profile: "generic",
// 	}
// 	ml, found, err := db.IndexGetByRequest(ctx, req)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.True(t, found)
// 	assert.Equal(t, 2, len(ml.Manifests))
// 	//// Skylake
// 	req.Profile = "skylake"
// 	ml, found, err = db.IndexGetByRequest(ctx, req)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.True(t, found)
// 	assert.Equal(t, 1, len(ml.Manifests))
// }
