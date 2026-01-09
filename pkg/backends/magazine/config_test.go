package magazine

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestConfigStoreConfigUpload(t *testing.T) {
	c := context.Background()
	l, ctx, span := utils.InitLogging(c, tracer, map[string]any{
		"func": "TestConfigStoreConfigUpload",
	})
	defer span.End()
	_ = l
	bs := utils.NewBootstrap()
	debug := 0
	bs.AddSpan(span)
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
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
		"author": "dnIc4tc80O5Lp598",
		"architecture": "amd64",
		"os": "linux",
		"rootfs": {
			"type": "layers",
			"diff_ids": []
		},
		"config": {
			"Labels": {
			   "org.supercontainers.hardware.cpu.optimized.mode": "generic",
			   "org.supercontainers.hardware.cpu.optimized.version": "x86_64_v4"
		 }
	   }
	}`)
	bufEnc := base64.StdEncoding.EncodeToString(buf)

	bufDig := digest.FromBytes(buf)
	bu := types.OCIBlobUpload{
		Digest:    bufDig.String(),
		Data:      []byte(bufEnc),
		MediaType: "application/vnd.oci.image.config.v1+json",
	}
	// Check if config is NOT in DB
	_, found, err := mag.Database.ConfigGetByDigest(ctx, bufDig.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	// Check if config is NOT in Minio
	_, found, err = mag.Minio.GetConfig(ctx, bufDig.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	// Upload Config
	err = mag.StoreConfigUpload(ctx, &bu, buf)
	if err != nil {
		t.Fatal(err)
	}
	// Check if config is in DB
	cfg, found, err := mag.Database.ConfigGetByDigest(ctx, bufDig.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	// TODO: check if unknown is correct
	assert.Equal(t, "application/vnd.oci.image.config.v1+json", cfg.MediaType)
	//assert.Equal(t, 2, len(cfg.Labels.Map))
	// Check if config is in Minio
	_, found, err = mag.Minio.GetConfig(ctx, bufDig.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)

}

func TestConfigBlobGetByHTTPRequest(t *testing.T) {
	bs := utils.NewBootstrap()
	ctx := context.Background()
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestBlobHandlerLayerTest",
	})
	defer span.End()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
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
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	// Prepare HTTP Request
	cfgDigest, err := br.Get("config:ft_arm64#digest")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodHead, "/{repo}/{repo2}/{repo3}/{image}/referrers/{reference}", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", "qnib")
	rctx.URLParams.Add("repo2", "repo")
	rctx.URLParams.Add("repo3", "sub")
	rctx.URLParams.Add("image", "featuretest")
	rctx.URLParams.Add("reference", cfgDigest)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	cfg, found, err := mag.ConfigBlobGetByHTTPRequest(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", cfg.MediaType)
}
