package magazine

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// func TestLayerStoreBlobUpload(t *testing.T) {
// 	bu, b := types.FT_arm_BlobUpload_Layer0()
// 	err := mag.StoreBlobUpload(ctx, &bu, b)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	_, found, err := mag.Database.LayerGetByDigest(ctx, bu.Digest)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.True(t, found)
// 	req := types.Request{
// 		Repo:   bu.Repo,
// 		Tag:    "",
// 		Digest: bu.Digest,
// 	}
// 	if found, err := mag.BlobDelete(ctx, req); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		assert.True(t, found)
// 	}
// }

func TestBlobGetByHTTPRequest(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	debug := 0
	bs.PostTestSleep = debug
	bs.AddBootstrap("users")
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.SetEmptyAll()
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestBlobGetByHTTPRequest",
	})
	defer span.End()
	// Setup
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	testProfile := "generic"
	testRepo := "qnib"
	testImage := "featuretest"
	// testTag := "v1"

	dig, err := br.Get("config:ft_arm64#digest")
	if err != nil {
		t.Fatal(err)
	}
	// HEAD Request
	utils.Debug(l, span, "Starting HEAD TestBlobGetByHTTPRequest")
	_, r := utils.CreateNewHTTPTest(http.MethodHead, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": dig,
	}, map[string]string{
		"accountName": token,
		"userName":    "qnib",
		"profileName": testProfile,
		"debug":       fmt.Sprintf("%d", debug),
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/blobs/{reference}", nil)

	blob, found, err := mag.BlobGetByHTTPRequest(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Should not have found a blob")
	}
	assert.Equal(t, blob.Digest, dig)
	if blob.Data != nil {
		t.Fatal("Blob data should be nil for HEAD request")
	}
	utils.Debug(l, span, "HEAD TestBlobGetByHTTPRequest OK")
	// GET Request
	utils.Debug(l, span, "Starting GET TestBlobGetByHTTPRequest")
	r.Method = http.MethodGet
	blob, found, err = mag.BlobGetByHTTPRequest(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Should not have found a blob")
	}
	assert.Equal(t, blob.Digest, dig)
	if blob.Data == nil {
		t.Fatal("Blob data should NOT be nil for GET request")
	}
	utils.Debug(l, span, "GET TestBlobGetByHTTPRequest OK")
}

// func TestConfigStoreBlobUpload(t *testing.T) {
// 	ctx := context.Background()
// 	bs := utils.NewBootstrap()
// 	bs.AddBootstrap("users")
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.SetEmptyAll()
// 	_, err := mag.SetupTest(ctx, bs)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs)
// 	// Tests
// 	bu, b := types.FT_arm_BlobUpload_Config()
// 	_ = b
// 	_, found, err := mag.Database.Config_GetByDigest(ctx, bu.Digest)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.False(t, found)
// 	err = mag.StoreBlobUpload(ctx, &bu, b)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if err = mag.StoreBlobUpload(ctx, &bu, []byte(`{"foo":"bar"}`)); err != ErrJSONNOTCONFIG {
// 		if err == nil {
// 			t.Fatal("Should have failed")
// 		}
// 		t.Fatal(err)
// 	}
// 	req := types.Request{
// 		Repo:   bu.Repo,
// 		Tag:    "",
// 		Digest: bu.Digest,
// 	}

// 	// Blob Exists
// 	upt := upstream.UpstreamTriple{
// 		Repo:   bu.Repo,
// 		Digest: digest.Digest(bu.Digest),
// 	}
// 	if found, err := mag.BlobExists(ctx, upt); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		assert.True(t, found)
// 	}

// 	if found, err := mag.BlobDelete(ctx, req); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		assert.True(t, found)
// 	}
// }

// func TestBlobChunkCRU(t *testing.T) {
// 	ctx := context.Background()
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	br, err := mag.SetupTest(ctx, bs)
// 	if err != nil {
// 		TeardownTest(bs)
// 		t.Fatal(err)
// 	}
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	defer TeardownTest(bs)
// 	// Actual test
// 	//// CREATE
// 	ociBU := types.OCIBlobUpload{
// 		Repo:          "qnib/featuretest",
// 		UploadUser:    "test",
// 		UploadProfile: "generic",
// 		BlobType:      "layer",
// 		Digest:        "sha256:6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30",
// 	}
// 	err = mag.Database.BlobUpload_Create_ByOCI(ctx, &ociBU)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	bc, err := mag.Database.UploadChunk_Create(ctx, &ociBU, 0, 10)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.NotNil(t, bc.ID)
// 	// / READ
// 	bcs, err := mag.Database.UploadChunk_Get_ByUUID(ctx, ociBU.ID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 1, len(bcs))
// 	// LinkBlobUpdate
// 	if e := mag.Database.BlobUpload_Update_ByUUID(ctx, &ociBU); e != nil {
// 		t.Fatal(e)
// 	}
// 	// BlobUploadGetByBlobDigest
// 	upt := upstream.UpstreamTriple{
// 		Repo:   ociBU.Repo,
// 		Digest: digest.Digest(ociBU.Digest),
// 	}
// 	if found, err := mag.BlobExists(ctx, upt); err != nil {
// 		t.Fatal(err)
// 	} else if !found {
// 		t.Fatal("Did not find BlobUpload")
// 	}
// }
