package magazine

import (
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/regclient/regclient/types/platform"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

//var ctx = context.Background()

func Test_CachManifest(t *testing.T) {
	bs := utils.NewBootstrap()
	_, err := SetupTest(bs)
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

func Test_StoreManifestUpload(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("cfg:generic")
	bs.AddBootstrap("profiles")
	//bs.SetSqlite("Desktop/metahub.db")
	_, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	expCfg := types.FT_arm_Config()
	expUpstream := types.FT_arm_Upstream_Generic()
	inMani := types.FT_arm_Manifest()
	req := types.Request{
		Repo:    "qnib/plain-featuretest",
		Tag:     "20220715-arm64-generic",
		User:    "test",
		Profile: "generic",
		Digest:  inMani.Digest,
	}
	mani, err := mag.StoreManifestUpload(req, inMani.Data)
	assert.NoError(t, err)
	assert.Equal(t, inMani.Digest, mani.Digest)
	assert.Equal(t, expCfg.Digest, mani.Config.Digest)
	assert.Equal(t, expUpstream.Digest, mani.Upstream.Digest)
	// 2022-11-28: Had an issue where the tag_id was not set
	q := "SELECT collection_tag_id FROM collection_xtable"
	rows, err := mag.SQlite.Query(q)
	assert.NoError(t, err)
	defer rows.Close()
	assert.True(t, rows.Next())
	var colTagID int64
	err = rows.Scan(&colTagID)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(0), colTagID)
}

func Test_Manifest_StoreManifestUpload_EmbeddedCOnfig_NoLayer(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	//bs.SetSqlite("Desktop/metahub.db")
	_, err := SetupTest(bs)
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
		Repo: "oci/conformance",
		Tag:  "test0",
	}
	// Upload Manifest with Embedded Config
	mani, err := mag.StoreManifestUpload(req, buf)
	assert.NoError(t, err)
	assert.Equal(t, bufDig.String(), mani.Digest)
	// Check if config is in DB
	cfg, found, err := mag.SQlite.Config.ConfigGetByDigest("sha256:65b1de14a76de70a46ff0839ab79775aceb96ee12069fb6ed45bc35f958fdbcb")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "application/vnd.oci.image.config.v1+json", cfg.MediaType)

}
