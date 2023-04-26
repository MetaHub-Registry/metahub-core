package magazine

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
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
func Test_CachManifest(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	_, err := mag.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Test
	//mani := types.FT_arm_Manifest()
	yMani := config.Manifest{
		Image:  "registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm64-generic",
		Digest: "",
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
		psUrl, err := mag.Minio.LayerGetPreSignedUrl(ctx, l.Digest)
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

func Test_storeManifestUpload_OCIManifest(t *testing.T) {
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
	_, err := mag.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Tests
	ctx := context.Background()
	inMani := types.FT_arm_ManifestData()
	req := types.Request{
		Repo:    "qnib/plain-featuretest",
		Tag:     "20220715-arm64-generic",
		User:    "qnib",
		Profile: "cpu#generic",
		Digest:  inMani.Digest,
	}
	m, ociMani, err := mag.storeManifestUpload_OCIManifest(ctx, req, inMani.Data)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Manifests) != 1 {
		t.Fatalf("Expected 1 manifest, got %d", len(m.Manifests))
	}
	md := m.Manifests[0]
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", md.Digest)
	assert.Equal(t, "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", ociMani.Config.Digest.String())
	minMani, err := mag.Minio.ManifestGetObj(ctx, md.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", minMani.Digest)
}

func Test_Manifest_StoreManifestUpload_EmbeddedCOnfig_NoLayer(t *testing.T) {
	bs := utils.NewBootstrap()
	//debug = 300
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
		TeardownTest(bs)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer TeardownTest(bs)
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
	cfg, found, err := mag.Database.Config_GetByDigest(ctx, "sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb")
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

func Test_Manifest_StoreManifestUpload_EmbeddedCOnfig_NoLayer_with_Labels(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("users")
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("rules:rules/mpi/mpich.yaml")
	bs.AddBootstrap("profile:profiles/mpi/mpich/generic.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	//bs.PostTestSleep = 300
	_, err := mag.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
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
	cfg, found, err := mag.Database.Config_GetByDigest(ctx, cfgDigest)
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
	// Check Collections
	manis, err := mag.Database.Manifest_GetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(manis) != 1 {
		t.Errorf("Expected 1 manifest, got %d", len(manis))
	}
	gotMani := manis[0]
	assert.Equal(t, "oci/conformance", gotMani.Upstream.Repo)
}

func Test_Manifest_ComputeManifestForProfile(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
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
		TeardownTest(bs)
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Actual Test
	err = mag.Database.Rule_EvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	maniID, err := br.GetInt64("manifest:ft_amd64#id")
	if err != nil {
		t.Fatal(err)
	}
	prfID, err := br.GetInt64("profile:cpu:generic#id")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Profile ID: %d // ManiID: %d", prfID, maniID)
	mani, found, err := mag.Database.Manifest_GetByID(ctx, maniID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Manifest not found")
	}
	// Actual test
	err = mag.ComputeManifestForProfile(ctx, mani, prfID)
	if err != nil {
		t.Fatal(err)
	}
	err = mag.Database.Rule_EvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Check if the manifest is in the collection
	mdID, found, e := mag.Database.ManifestData_GetID_ByMediaType(ctx, mani.ID, "application/vnd.oci.image.manifest.v1+json")
	if e != nil {
		t.Fatal(e)
	}
	if !found {
		t.Fatal("ManifestData not found")
	}
	assert.NotEqual(t, mdID, maniID)
}
