package minio

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// func TestConfigGetter(t *testing.T) {
// 	bi := types.BlobInfo{
// 		Digest: digest.Digest(featureTestGenericCfgDigest),
// 	}
// 	cfg, err := min.configGetter(bi)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, featureTestGenericCfg, cfg)
// }

func TestGetConfig(t *testing.T) {
	ctx := context.Background()
	l, c, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestGetConfig",
	})
	ctx = c
	defer span.End()
	_ = l
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.PostTestSleep = 0
	defer min.TearDown(bs)
	// Tests
	err := min.PutBlob(ctx, []byte(featureTestGenericCfg), featureTestGenericCfgDigest)
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
	_, found, err = min.GetConfig(ctx, featureTestGenericCfgDigest)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	err = min.BlobRemoveObj(ctx, featureTestGenericCfgDigest)
	if err != nil {
		t.Fatal(err)
	}
	_, found, err = min.GetConfig(ctx, featureTestGenericCfgDigest)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
}

func TestBlobRemoveObj(t *testing.T) {
	ctx := context.Background()
	l, c, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestBlobRemoveObj",
	})
	ctx = c
	defer span.End()
	_ = l
	digest := "sha256:f70ce6a20a96d310e32c1f5a005b24dc8b225f949a69e1ec46dc5b7c12779dc9"
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.PostTestSleep = 0
	defer min.TearDown(bs)
	// Tests
	_, err := min.BlobGetBytes(ctx, digest)
	if err == nil {
		t.Fatal("expected error getting non existing blob")
	}
	err = min.PutBlob(ctx, []byte(featureTestGenericCfg), digest)
	if err != nil {
		t.Fatal(err)
	}
	b, err := min.BlobGetBytes(ctx, digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, b)
	err = min.BlobRemoveObj(ctx, digest)
	if err != nil {
		t.Fatal(err)
	}
	err = min.BlobRemoveObj(ctx, digest)
	assert.Error(t, err)
	_, err = min.BlobGetBytes(ctx, digest)
	assert.Error(t, err)
}

func TestGetTypesConfig(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("cfg:generic")
	_, err := min.SetupTest(ctx, bs)
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

func TestConfigGetBytes(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("cfg:generic")
	_, err := min.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer min.PreTearDown(bs)
	// Tests
	b, err := min.BlobGetBytes(ctx, BlobUp_FeaturetestGeneric.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, b)
	assert.Len(t, b, 1790)

}

func TestConfigFromCfgBlob(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("cfg:generic")
	_, err := min.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer min.PreTearDown(bs)
	// Tests
	b, err := min.BlobGetBytes(ctx, BlobUp_FeaturetestGeneric.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, b)
	assert.Len(t, b, 1790)
	ps, err := utils.PrettyString(b)
	if err != nil {
		t.Fatal(err)
	}
	_ = ps
	//t.Logf("Config: %s", ps)
	cfg, err := ConfigFromCfgBlob(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, cfg)
	assert.Equal(t, "amd64", cfg.Cfg.Architecture)
	exp := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"SKIP_ENTRYPOINTS=true",
		"PLATFORM_FEATURES=arch:cascadelake",
	}
	assert.Equal(t, exp, cfg.Cfg.Config.Env)
}

func TestConfigSetup(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.AddBootstrap("config:ft_amd64_83f")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := min.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		t.Logf("Error while SetupTest: %v", err)
		min.TearDown(bs)
		t.Fatal(err)

	}
	defer min.TearDown(bs)
	// Actual test
	dt := types.ConfigByteFtArm64x83f()
	cfg, err := min.ConfigGetObj(ctx, dt.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, dt.Digest.String(), cfg.Digest)
}
