package minio

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

/*
	func Test_ConfigGetter(t *testing.T) {
		bi := types.BlobInfo{
			Digest: digest.Digest(featureTestGenericCfgDigest),
		}
		cfg, err := min.configGetter(bi)
		if err != nil {
		t.Fatal(err)
}
		assert.Equal(t, featureTestGenericCfg, cfg)
	}
*/

func Test_GetConfig(t *testing.T) {
	ctx := context.Background()
	err := min.PutConfig(ctx, []byte(featureTestGenericCfg), featureTestGenericCfgDigest)
	if err != nil {
		t.Fatal(err)
	}
	up := upstream.UpstreamTriple{
		Digest: digest.Digest(featureTestGenericCfgDigest),
	}
	cfg, err := min.ConfigGetObj(ctx, up.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, cfg)
	_, found, err := min.GetConfigByUpTriple(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	err = min.ConfigRemoveObj(ctx, featureTestGenericCfgDigest)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_ConfigRemoveObj(t *testing.T) {
	err := min.PutConfig(ctx, []byte(featureTestGenericCfg), "sha256:f70ce6a20a96d310e32c1f5a005b24dc8b225f949a69e1ec46dc5b7c12779dc9")
	if err != nil {
		t.Fatal(err)
	}
	b, err := min.ConfigGetBytes(ctx, "sha256:f70ce6a20a96d310e32c1f5a005b24dc8b225f949a69e1ec46dc5b7c12779dc9")
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, b)
	err = min.ConfigRemoveObj(ctx, "sha256:f70ce6a20a96d310e32c1f5a005b24dc8b225f949a69e1ec46dc5b7c12779dc9")
	if err != nil {
		t.Fatal(err)
	}
	err = min.ConfigRemoveObj(ctx, "sha256:f70ce6a20a96d310e32c1f5a005b24dc8b225f949a69e1ec46dc5b7c12779dc9")
	assert.Error(t, err)
	_, err = min.ConfigGetBytes(ctx, "sha256:f70ce6a20a96d310e32c1f5a005b24dc8b225f949a69e1ec46dc5b7c12779dc9")
	assert.Error(t, err)
}

func Test_GetTypesConfig(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("cfg:generic")
	_, err := min.SetupTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	defer min.PreTearDown(bs)
	cfg, found, err := min.GetTypesConfig(ctx, BlobUp_FeaturetestGeneric.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.NotNil(t, cfg)
	assert.Equal(t, BlobUp_FeaturetestGeneric.Digest, cfg.Digest)
}

func Test_ConfigGetBytes(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("cfg:generic")
	_, err := min.SetupTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	defer min.PreTearDown(bs)
	// Tests
	b, err := min.ConfigGetBytes(ctx, BlobUp_FeaturetestGeneric.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, b)
	assert.Len(t, b, 1790)

}
