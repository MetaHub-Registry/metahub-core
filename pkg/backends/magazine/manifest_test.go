package magazine

import (
	"context"
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
	_, err := mag.SetupTest(bs)
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mani.Digest)
	// Check if layers are cached
	for _, l := range mani.Layers {
		psUrl, err := mag.Minio.LayerGetPreSignedUrl(ctx, l.Digest)
		assert.NoError(t, err)
		assert.NotEmpty(t, psUrl)
	}
	mani, err = mag.CachManifest(ctx, yMani)
	assert.NoError(t, err)
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mani.Digest)

}

func Test_StoreManifestUpload_OCIManifest(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("config:ft_amd64")
	_, err := mag.SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Tests
	ctx := context.Background()
	inMani := types.FT_arm_Manifest()
	req := types.Request{
		Repo:    "qnib/plain-featuretest",
		Tag:     "20220715-arm64-generic",
		User:    "test",
		Profile: "generic",
		Digest:  inMani.Digest,
	}
	mani, ociMani, err := mag.storeManifestUpload_OCIManifest(ctx, req, inMani.Data)
	assert.NoError(t, err)
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mani.Digest)
	assert.Equal(t, "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", ociMani.Config.Digest.String())
	minMani, err := mag.Minio.ManifestGetObj(ctx, mani.Digest)
	assert.NoError(t, err)
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", minMani.Digest)
}

func Test_Manifest_StoreManifestUpload_EmbeddedCOnfig_NoLayer(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	//bs.PostTestSleep = 300
	_, err := mag.SetupTest(bs)
	assert.NoError(t, err)
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
		ContentType: "application/vnd.oci.image.manifest.v1+json",
	}
	// Upload Manifest with Embedded Config
	mani, err := mag.StoreManifestUpload(req, buf)
	assert.NoError(t, err)
	assert.Equal(t, bufDig.String(), mani.Digest)
	// Check if config is in DB
	cfg, found, err := mag.Database.ConfigGetByDigest("sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb")
	assert.Equal(t, "sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb", cfg.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "application/vnd.oci.image.config.v1+json", cfg.MediaType)
	// Check for config in Minio
	cfgObj, err := mag.Minio.ConfigGetObj(ctx, cfg.Digest)
	assert.NoError(t, err)
	assert.Equal(t, "sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb", cfgObj.Digest)
	// Check Collections
	cols, err := mag.Database.CollectionGetAll()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cols))
}

/*
func Test_Manifest_Annotations(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	//bs.PostTestSleep = 300
	_, err := mag.SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Tests
	//Manifest_LabelsToAnnotations
	// Tests
	buf := cfgWithLabels
	bufDig := digest.FromBytes(buf)
	req := types.Request{
		Repo:        "oci/conformance",
		Tag:         "test0",
		ContentType: "application/vnd.oci.image.manifest.v1+json",
	}
	// Upload Manifest with Embedded Config
	mani, err := mag.StoreManifestUpload(req, buf)
	assert.NoError(t, err)
	assert.Equal(t, bufDig.String(), mani.Digest)
	assert.Equal(t, 2, len(mani.Config.Labels.Map))
}
*/
