package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestLayerStoreBlobUpload(t *testing.T) {
	bu, b := types.FT_arm_BlobUpload_Layer0()
	err := mag.StoreBlobUpload(ctx, &bu, b)
	if err != nil {
		t.Fatal(err)
	}
	_, found, err := mag.Database.LayerGetByDigest(bu.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
}

/*
func TestConfigStoreBlobUpload(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.SetEmptyAll()
	_, err := mag.SetupTest(bs)
	if err != nil {
		t.Fatal(err)
}
	defer TeardownTest(bs)
	// Tests
	bu, b := types.FT_arm_BlobUpload_Config()
	_ = b
	_, found, err := mag.Database.Config_GetByDigestAndType(bu.Digest, bu.MediaType)
	if err != nil {
		t.Fatal(err)
}
	assert.False(t, found)
	err = mag.StoreBlobUpload(ctx, &bu, b)
	if err != nil {
		t.Fatal(err)
}
	err = mag.StoreBlobUpload(ctx, &bu, []byte(`{"foo":"bar"}`))
	assert.Error(t, err)
}
*/
