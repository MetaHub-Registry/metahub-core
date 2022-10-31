package magazine

import (
	"testing"

	"github.com/regclient/regclient/types/platform"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

//var ctx = context.Background()

func Test_CachManifest(t *testing.T) {
	err := SetupTest("")
	assert.NoError(t, err)
	defer TeardownTest("")
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
