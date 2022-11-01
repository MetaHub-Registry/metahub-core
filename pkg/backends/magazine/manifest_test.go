package magazine

import (
	"testing"

	"github.com/regclient/regclient/types/platform"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

//var ctx = context.Background()

func Test_CachManifest(t *testing.T) {
	bs := utils.NewBootstrap()
	err := SetupTest(bs)
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
	//bs.SetSqlite("Desktop/magazine2.db")
	err := SetupTest(bs)
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
}
