package magazine

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/regclient/regclient/types/platform"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

// var ctx = context.Background()
/*
var cfgWithLabels = []byte(`{
	"schemaVersion": 2,
	"config": {
		"mediaType": "application/vnd.oci.image.config.v1+json",
		"digest": "sha256:8da2a54f4a6f0bf020f10f5d322b028b34f39f8056a5dfae595c767c9decf6ca",
		"size": 347,
		"data": "ewoJImF1dGhvciI6ICJkbkljNHRjODBPNUxwNTk4IiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfSwKICAgICAgICJjb25maWciOiB7CiAgICAgICAgICAgIkxhYmVscyI6IHsKICAgICAgICAgICAgICAib3JnLnN1cGVyY29udGFpbmVycy5oYXJkd2FyZS5jcHUub3B0aW1pemVkLm1vZGUiOiAiZ2VuZXJpYyIsCiAgICAgICAgICAgICAgIm9yZy5zdXBlcmNvbnRhaW5lcnMuaGFyZHdhcmUuY3B1Lm9wdGltaXplZC52ZXJzaW9uIjogIng4Nl82NF92NCIKICAgICAgICB9CiAgICAgIH0KCn0=",
		"newUnspecifiedField": "aGVsbG8gd29ybGQ="
	},
	"annotations": {
		"com.example.key1": "value1",
		"com.example.key2": "value2"
	},
	"layers": []
	}`)
*/

func TestCachManifest(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:arm64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	TeardownTest(bs, br)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Test
	//mani := types.FT_arm_Manifest()
	yMani := config.Manifest{
		Image: "registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm64-generic",
		Platform: platform.Platform{
			Architecture: "arm64",
			OS:           "linux",
			Variant:      "v8",
		},
	}
	mani, err := mag.CachManifest(ctx, yMani)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mani.Digest)
	// Check if layers are cached
	for _, l := range mani.Layers {
		psUrl, err := mag.Minio.BlobGetPreSignedUrl(ctx, l.Digest)
		if err != nil {
			t.Fatal(err)
		}
		assert.NotEmpty(t, psUrl)
	}
	mani, err = mag.CachManifest(ctx, yMani)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mani.Digest)
}

func TestCachManifestLocalCache(t *testing.T) {
	// Using the local test registry
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:arm64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	TeardownTest(bs, br)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Test
	//mani := types.FT_arm_Manifest()
	yMani := config.Manifest{
		Image: "registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm64-generic",
		Platform: platform.Platform{
			Architecture: "arm64",
			OS:           "linux",
			Variant:      "v8",
		},
	}
	mani, err := mag.CachManifest(ctx, yMani)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mani.Digest)
	// Check if layers are cached
	for _, l := range mani.Layers {
		psUrl, err := mag.Minio.BlobGetPreSignedUrl(ctx, l.Digest)
		if err != nil {
			t.Fatal(err)
		}
		assert.NotEmpty(t, psUrl)
	}
	mani, err = mag.CachManifest(ctx, yMani)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mani.Digest)
}

func TestStoreManifestUploadOCIManifest(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	//bs.PostTestSleep = 300
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Tests
	ctx := context.Background()
	inMani := types.FTarmManifest()
	req := types.Request{
		Repo:    "qnib/plain-featuretest",
		Tag:     "20220715-arm64-generic",
		User:    "qnib",
		Profile: "cpu#generic",
		Digest:  inMani.Digest,
	}
	md, ociMani, err := mag.storeManifestUploadOCIManifest(ctx, req, inMani.Data)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", md.Digest)
	assert.Equal(t, "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", ociMani.Config.Digest.String())
	minMani, err := mag.Minio.ManifestGetObj(ctx, md.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", minMani.Digest)
}

func TestManifestStoreManifestUploadEmbeddedConfigNoLayer(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer TeardownTest(bs, br)
	// Tests
	buf := []byte(`{
			"schemaVersion": 2,
			"config": {
				"mediaType": "application/vnd.oci.image.config.v1+json",
				"digest": "sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb",
				"size": 129,
				"data": "ewoJImF1dGhvciI6ICJkbkljNHRjODBPNUxwNTk4IiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9",
				"newUnspecifiedField": "aGVsbG8gd29ybGQ="
			},
			"layers": []
			}`)
	bufDig := digest.FromBytes(buf)
	req := types.Request{
		Repo:        "oci/conformance",
		Tag:         "test0",
		Profile:     "cpu#generic",
		User:        "oci",
		ContentType: "application/vnd.oci.image.manifest.v1+json",
	}
	// Upload Manifest with Embedded Config
	mani, err := mag.StoreManifestUpload(ctx, req, buf)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, bufDig.String(), mani.Digest)
	// Check if config is in DB
	cfg, found, err := mag.Database.ConfigGetByDigest(ctx, "sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb", cfg.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, "application/vnd.oci.image.config.v1+json", cfg.MediaType)
	// Check for config in Minio
	cfgObj, err := mag.Minio.ConfigGetObj(ctx, cfg.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb", cfgObj.Digest)
	// Check Collections
	cols, err := mag.Database.CollectionGetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_ = cols
	//assert.Equal(t, 1, len(cols))
}

func TestManifestStoreManifestUploadEmbeddedConfigNoLayerwithLabels(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetEmptyAll()
	bs.AddBootstrap("users")
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("rules:rules/mpi/mpich.yaml")
	bs.AddBootstrap("profile:profiles/mpi/mpich/generic.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer TeardownTest(bs, br)
	// Tests
	// DO NOT CHANGE the data, it will change the digest
	data := `{
		"author": "dnIc4tc80O5Lp598",
		"architecture": "amd64",
		"os": "linux",
		"rootfs": {
			"type": "layers",
			"diff_ids": []
		},
		"config": {
			"Labels": {
				"org.supercontainers.hardware.cpu.target": "platform"
			}
		}
	}`
	sEnc := b64.StdEncoding.EncodeToString([]byte(data))
	cfgDigest := `sha256:828e1c75a150d8e82be821ed08a401c7b287b47a3966aa7030fd6667ce40cdcb`
	buf := []byte(fmt.Sprintf(`{
		"schemaVersion": 2,
		"config": {
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest": "%s",
			"size": 129,
			"data": "%s",
			"newUnspecifiedField": "aGVsbG8gd29ybGQ="
		},
		"layers": []
	}`, cfgDigest, sEnc))
	bufDig := digest.FromBytes(buf)
	req := types.Request{
		Host:        "localhost",
		Repo:        "oci/conformance",
		Tag:         "test0",
		Profile:     "cpu#generic",
		User:        "oci",
		ContentType: "application/vnd.oci.image.manifest.v1+json",
	}
	// Upload Manifest with Embedded Config
	mani, err := mag.StoreManifestUpload(ctx, req, buf)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, bufDig.String(), mani.Digest)
	// Check if config is in DB
	cfg, found, err := mag.Database.ConfigGetByDigest(ctx, cfgDigest)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, cfgDigest, cfg.Digest)
	assert.True(t, found)
	assert.Equal(t, "application/vnd.oci.image.config.v1+json", cfg.MediaType)
	// Check for config in Minio
	cfgObj, err := mag.Minio.ConfigGetObj(ctx, cfg.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, cfgDigest, cfgObj.Digest)
	// Path Z Phase C3: check content-leaf Manifest by digest instead of
	// wrapper ManifestWrapper. ManifestGetAll still walks via :ManifestWrapper wrapper
	// (pre-rename); use digest-based lookup instead.
	mdByDigest, mdFound, err := mag.Database.ManifestGetByDigest(ctx, mani.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, mdFound, "Manifest should exist after push")
	assert.Equal(t, mani.Digest, mdByDigest.Digest, "Manifest digest should match pushed manifest")
}

func TestManifestComputeManifestForProfileARM(t *testing.T) {
	ctx := context.Background()
	l, c, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestManifestComputeManifestForProfile",
	})
	defer span.End()
	ctx = c
	_ = l
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("empty:all")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}

	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Actual Test
	err = mag.Database.RuleEvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Path Z task 7.7: D5 invariant canary after every RuleEvalAll.
	assertRuleShortcutInvariantsOrFatal(t, ctx)
	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	prfID, err := br.GetInt64("profile:cpu:generic#id")
	if err != nil {
		t.Fatal(err)
	}
	prf, found, err := mag.Database.ProfileGetByID(ctx, prfID)
	if err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Profile not found")
	}
	t.Logf("Profile ID: %d // ManiID: %d", prfID, maniID)
	// Phase 4 (deprecate-manifest-wrapper): build a leaf-keyed shim instead of
	// fetching a :ManifestWrapper node.
	mani, err := mag.getMani(ctx, br, "manifest:ft_arm64")
	if err != nil {
		t.Fatal(err)
	}
	// Actual test
	err = mag.ComputeManifestForProfile(ctx, mani, prf)
	if err != nil {
		t.Fatal(err)
	}
	err = mag.Database.RuleEvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Path Z task 7.7: D5 invariant canary after every RuleEvalAll.
	assertRuleShortcutInvariantsOrFatal(t, ctx)
	// Phase 4 (deprecate-manifest-wrapper): verify a new OCI manifest was
	// created and linked to the profile via UPLOADED_BY (the computed manifest
	// is no longer attached to a :ManifestWrapper node, so walk Profile
	// directly). Replaces the dead wrapper-only ManifestGetIDByMediaType lookup.
	q := fmt.Sprintf(`MATCH (md:Manifest)-[:CREATED_BY]->(p:Profile) WHERE id(p) = %d AND md.mediaType IN ["application/vnd.docker.distribution.manifest.v2+json", "%s"] RETURN id(md) AS mdID`, prf.ID, v1.MediaTypeImageManifest)
	res, found, e := mag.Database.QueryOne(q)
	if e != nil {
		t.Fatal(e)
	}
	if !found {
		t.Fatal("OCI manifest for profile not found")
	}
	mdID, e := res.GetInt64("mdID")
	if e != nil {
		t.Fatal(e)
	}
	assert.NotEqual(t, mdID, maniID)
	span.End()
}

// func TestGetManifest(t *testing.T) {
// 	ctx := context.Background()
// 	_ = ctx
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("users")
// 	bs.AddBootstrap("platforms:amd64")
// 	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/generic.yaml")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("config:ft_amd64_83f")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("manifest:ft_amd64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
// 	bs.AddBootstrap("manifestdata:ft_amd64")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	br, err := mag.SetupTest(ctx, bs)
// 	if err != nil {
// 		t.Logf("Error while SetupTest: %v", err)
// 		TeardownTest(bs, br)
// 		t.Fatal(err)

// 	}
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}

// 	defer TeardownTest(bs, br)
// 	// Actual test
// 	if e := mag.Database.RuleEvalAll(ctx); e != nil {
// 		t.Fatal(e)
// 	}
// 	token, err := br.Get("user:test#bearer")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	testOrg := "qnib-pub-containers"
// 	testRepo := "qnib"
// 	testImage := "plain-featuretest"
// 	testTag := "20220715-amd64-generic"
// 	testProfile := "generic"
// 	/// via HTTPRequest
// 	_, rMDGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
// 		"repo":      testOrg,
// 		"repo2":     testRepo,
// 		"image":     testImage,
// 		"reference": testTag,
// 	}, map[string]string{
// 		"accountName": token,
// 		"userName":    testRepo,
// 		"profileName": testProfile,
// 	}, map[string]string{
// 		"Authorization": fmt.Sprintf("Bearer %s", token),
// 	}, "/{repo}/{repo2}/{image}/manifests/{reference}", nil)

// 	md, found, err := mag.ManifestGetByHTTPRequest(ctx, rMDGet)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !found {
// 		t.Fatal("Manifest not found")
// 	}
// 	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", md.MediaType)
// }

// TestStoreManifestUpload_WiresSlot exercises Path Z task 6 phase A: the
// additive Slot wiring in StoreManifestUpload. A successful manifest push
// with a non-empty req.Profile SHALL:
//   - create a :Slot node for the (repo, tag, profile) triple (idempotently),
//   - append a :HAS_PUSHED edge from the Slot to the content-leaf Manifest.
//
// The legacy wrapper path (CollectionGetCreateByRepoTag + Link* helpers)
// continues to run alongside the Slot wiring during the phase-A migration
// window. This test does not assert the legacy path state — that is
// covered by the existing push tests.
func TestStoreManifestUpload_WiresSlot(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Use the same minimal OCI manifest with embedded config and no
	// layers as TestManifestStoreManifestUploadEmbeddedConfigNoLayer —
	// it's the shortest path to a successful StoreManifestUpload.
	buf := []byte(`{
			"schemaVersion": 2,
			"config": {
				"mediaType": "application/vnd.oci.image.config.v1+json",
				"digest": "sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb",
				"size": 129,
				"data": "ewoJImF1dGhvciI6ICJkbkljNHRjODBPNUxwNTk4IiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9",
				"newUnspecifiedField": "aGVsbG8gd29ybGQ="
			},
			"layers": []
			}`)
	req := types.Request{
		Repo:        "qnib/slot-wiring-test",
		Tag:         "v1",
		Profile:     "cpu#generic",
		User:        "oci",
		ContentType: "application/vnd.oci.image.manifest.v1+json",
	}
	mani, err := mag.StoreManifestUpload(ctx, req, buf)
	if err != nil {
		t.Fatal(err)
	}
	// Verify the manifest was actually persisted. We deliberately do NOT assert
	// mani.ID != 0: neo4j internal node ids (id(n)) start at 0 and are reused
	// after a DETACH DELETE, so under bs.SetEmptyAll() a freshly created
	// :Manifest can legitimately be assigned id 0 (identical rationale to the
	// slot.ID note below). Asserting NotZero here made this test flake
	// intermittently (metalib_main_prod #27). Re-fetch the content leaf by
	// digest instead — a real id, zero or not, must resolve back to the node we
	// just wrote.
	maniProbe, maniProbeFound, err := mag.Database.QueryOne(fmt.Sprintf(
		`MATCH (m:Manifest) WHERE m.digest = "%s" RETURN id(m) AS mid`, mani.Digest))
	if err != nil {
		t.Fatalf("manifest digest probe: %v", err)
	}
	assert.True(t, maniProbeFound, "pushed manifest should be retrievable by digest %q", mani.Digest)
	if maniProbeFound {
		mid, _ := maniProbe.GetInt64("mid")
		assert.Equal(t, mani.ID, mid, "digest lookup must resolve to the pushed manifest node (id %d)", mani.ID)
	}

	// Assertion 1: Slot was created for the triple
	slot, found, err := mag.Database.SlotGetByTriple(ctx, req.Repo, req.Tag, req.Profile)
	if err != nil {
		t.Fatalf("SlotGetByTriple: %v", err)
	}
	assert.True(t, found, "Slot should exist for (%s, %s, %s) after StoreManifestUpload", req.Repo, req.Tag, req.Profile)
	if !found {
		return
	}
	// Note: we do NOT assert slot.ID != 0. Neo4j node IDs start at 0 and
	// under bs.SetEmptyAll() the graph is freshly wiped, so the Slot may
	// legitimately get ID 0 in isolated test runs. The fact that
	// `found == true` is sufficient confirmation that the Slot exists;
	// the triple-fields assertions below pin down which Slot it is.
	assert.Equal(t, req.Repo, slot.Repo)
	assert.Equal(t, req.Tag, slot.Tag)
	assert.Equal(t, req.Profile, slot.Profile)

	// Assertion 2: HAS_PUSHED edge exists from the Slot to the pushed Manifest
	hasPushedQ := fmt.Sprintf(
		`MATCH (s:Slot)-[r:HAS_PUSHED]->(m:Manifest) WHERE id(s) = %d AND id(m) = %d RETURN count(r) AS edgeCount, r.timestamp AS ts`,
		slot.ID, mani.ID,
	)
	res, resFound, err := mag.Database.QueryOne(hasPushedQ)
	if err != nil {
		t.Fatalf("HAS_PUSHED query: %v", err)
	}
	assert.True(t, resFound, "HAS_PUSHED edge query should return a row")
	if resFound {
		edgeCount, _ := res.GetInt64("edgeCount")
		assert.Equal(t, int64(1), edgeCount, "exactly one HAS_PUSHED edge should exist from slot %d to manifest %d", slot.ID, mani.ID)
		ts, _ := res.GetInt64("ts")
		assert.NotZero(t, ts, "HAS_PUSHED edge should carry a non-zero timestamp")
	}

	// Assertion 3: Slot is idempotent - a second push of the same
	// manifest creates a second HAS_PUSHED edge (append-only log) but
	// does not duplicate the Slot node itself.
	_, err = mag.StoreManifestUpload(ctx, req, buf)
	if err != nil {
		t.Fatalf("second StoreManifestUpload: %v", err)
	}
	slot2, found2, err := mag.Database.SlotGetByTriple(ctx, req.Repo, req.Tag, req.Profile)
	if err != nil {
		t.Fatalf("second SlotGetByTriple: %v", err)
	}
	assert.True(t, found2)
	assert.Equal(t, slot.ID, slot2.ID, "Slot ID should be stable across repushes (MERGE idempotency)")
	// Count HAS_PUSHED edges on the slot - should be 2 now because
	// HAS_PUSHED is an append-only log per design D3.
	countQ := fmt.Sprintf(
		`MATCH (s:Slot)-[r:HAS_PUSHED]->() WHERE id(s) = %d RETURN count(r) AS edgeCount`,
		slot.ID,
	)
	countRes, countFound, err := mag.Database.QueryOne(countQ)
	if err != nil {
		t.Fatalf("HAS_PUSHED count query: %v", err)
	}
	if countFound {
		totalEdges, _ := countRes.GetInt64("edgeCount")
		assert.Equal(t, int64(2), totalEdges, "re-push should append a second HAS_PUSHED edge (append-only log)")
	}
}

// TestStoreManifestUpload_WiresSubject is the regression guard for the model
// composition bug: an *image* manifest pushed via StoreManifestUpload that
// carries an OCI `subject` (the shape `metahub-admin repackage` emits for a
// model-config artefact, whose subject is the model-layer manifest) MUST get a
// (manifest)-[:SUBJECT_OF]->(subject) edge so referrers(subject) — and thus
// model compositions — resolve. Previously only StoreManifestArtifactUpload
// wired the subject, so subject-bearing image manifests silently lost the edge
// and compositions came back empty.
func TestStoreManifestUpload_WiresSubject(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	// A minimal OCI image manifest (embedded config, no layers). %s is either
	// empty (the subject-target manifest) or a subject descriptor (the config
	// manifest). Both go through the image path — IsArtifact() is false because
	// the config is a real image config — which is exactly the path that lost
	// the subject edge before this fix.
	baseManifest := `{
			"schemaVersion": 2,
			"mediaType": "application/vnd.oci.image.manifest.v1+json",
			"config": {
				"mediaType": "application/vnd.oci.image.config.v1+json",
				"digest": "sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb",
				"size": 129,
				"data": "ewoJImF1dGhvciI6ICJkbkljNHRjODBPNUxwNTk4IiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9"
			},
			"layers": []%s
			}`

	// 1. Push the subject-target ("model-layer") manifest.
	targetReq := types.Request{
		Repo: "qnib/subject-target", Tag: "v1", Profile: "cpu#generic",
		User: "oci", ContentType: "application/vnd.oci.image.manifest.v1+json",
	}
	targetMani, err := mag.StoreManifestUpload(ctx, targetReq, []byte(fmt.Sprintf(baseManifest, "")))
	if err != nil {
		t.Fatalf("push subject-target: %v", err)
	}

	// 2. Push a "config" manifest whose OCI subject points at the target.
	subjectFragment := fmt.Sprintf(`,
			"subject": {
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"digest": "%s",
				"size": 100
			}`, targetMani.Digest)
	cfgReq := types.Request{
		Repo: "qnib/subject-config", Tag: "v1", Profile: "cpu#generic",
		User: "oci", ContentType: "application/vnd.oci.image.manifest.v1+json",
	}
	cfgMani, err := mag.StoreManifestUpload(ctx, cfgReq, []byte(fmt.Sprintf(baseManifest, subjectFragment)))
	if err != nil {
		t.Fatalf("push subject-config: %v", err)
	}
	assert.NotZero(t, cfgMani.ID, "pushed config manifest should have a non-zero ID")

	// The config manifest must now carry a SUBJECT_OF edge to the target.
	q := fmt.Sprintf(
		`MATCH (m:Manifest)-[:SUBJECT_OF]->(s:Manifest) WHERE id(m) = %d RETURN s.digest AS subjectDigest`,
		cfgMani.ID,
	)
	res, found, err := mag.Database.QueryOne(q)
	if err != nil {
		t.Fatalf("SUBJECT_OF query: %v", err)
	}
	assert.True(t, found, "StoreManifestUpload must wire a SUBJECT_OF edge for a subject-bearing image manifest")
	if found {
		got, _ := res.GetStr("subjectDigest")
		assert.Equal(t, targetMani.Digest, got, "SUBJECT_OF must point at the manifest's declared subject")
	}
}

// Note: an earlier draft of this test file had a
// `TestStoreManifestUpload_EmptyProfileSkipsSlot` test that asserted
// the Slot wiring was a no-op for empty req.Profile. It was removed
// because the upstream `storeManifestUploadOCIManifest` helper already
// rejects empty profiles with `profile '' not found` BEFORE reaching
// the Slot wiring at the bottom of StoreManifestUpload. The defensive
// `if req.Profile != ""` guard in StoreManifestUpload is therefore
// belt-and-braces; it stays in place to keep the Slot wiring resilient
// against any future caller that bypasses storeManifestUploadOCIManifest
// (e.g., a hypothetical lower-level helper that constructs the manifest
// directly without going through the profile lookup).
