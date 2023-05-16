package magazine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestLayerStoreBlobUpload(t *testing.T) {
	bu, b := types.FT_arm_BlobUpload_Layer0()
	err := mag.StoreBlobUpload(ctx, &bu, b)
	if err != nil {
		t.Fatal(err)
	}
	_, found, err := mag.Database.LayerGetByDigest(ctx, bu.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	req := types.Request{
		Repo:   bu.Repo,
		Tag:    "",
		Digest: bu.Digest,
	}
	if found, err := mag.BlobDelete(ctx, req); err != nil {
		t.Fatal(err)
	} else {
		assert.True(t, found)
	}

}

func TestConfigStoreBlobUpload(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.SetEmptyAll()
	_, err := mag.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Tests
	bu, b := types.FT_arm_BlobUpload_Config()
	_ = b
	_, found, err := mag.Database.Config_GetByDigest(ctx, bu.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	err = mag.StoreBlobUpload(ctx, &bu, b)
	if err != nil {
		t.Fatal(err)
	}
	if err = mag.StoreBlobUpload(ctx, &bu, []byte(`{"foo":"bar"}`)); err != ERRJSONNOTCONFIG {
		if err == nil {
			t.Fatal("Should have failed")
		}
		t.Fatal(err)
	}
	req := types.Request{
		Repo:   bu.Repo,
		Tag:    "",
		Digest: bu.Digest,
	}
	if found, err := mag.BlobDelete(ctx, req); err != nil {
		t.Fatal(err)
	} else {
		assert.True(t, found)
	}
}
