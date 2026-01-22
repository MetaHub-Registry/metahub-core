package magazine

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

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
	defer TeardownTest(bs, br)
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
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodHead, map[string]string{
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

func TestStoreBlobUpload(t *testing.T) {
	ctx := context.Background()
	l, c, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestBlobHandlerLayerTest",
	})
	ctx = c
	defer span.End()
	debug := 0
	bs := utils.NewBootstrap()
	bs.AddSpan(span)
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
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
	/// Vars
	testRepo := "qnib"
	br.Add("repo", testRepo)
	br.Add("user", "admin")
	testProfile := "generic"
	br.Add("profile", testProfile)
	testImage := "alpine"
	br.Add("image", testImage)
	// Upload a Config Blob
	bu, buff := types.FTarmBlobUploadConfig()
	utils.Debug(l, span, "Uploading Config Blob")
	if err := mag.Database.BlobUploadCreateByOCI(ctx, &bu); err != nil {
		t.Fatal(err)
	}
	utils.Debug(l, span, "Create Chunk for Blob Upload")
	chunk, err := mag.Database.UploadChunkCreate(ctx, &bu, 0, int64(len(buff)))
	if err != nil {
		t.Fatal(err)
	}
	utils.Debug(l, span, "Storing Config Blob")
	if err = mag.StoreBlobUpload(ctx, &bu, buff); err != nil {
		t.Fatal(err)
	}
	utils.Debug(l, span, "Compose blob from chunk")
	err = mag.ComposeFromChunks(ctx, &bu, []types.BlobChunk{chunk})
	if err != nil {

	}
	dt := types.DataTupeFTConfigArm64()
	upt, err := upstream.UpstreamToUpstreamTriple(dt.Upstream, types.MediaTypeImageConfig)
	if err != nil {
		t.Fatal(err)
	}
	upt.Digest = digest.Digest(bu.Digest)
	utils.Debug(l, span, "Checking if Blob Exists")
	if found, err := mag.BlobExists(ctx, upt); err != nil {
		t.Fatal(err)
	} else {
		assert.True(t, found)
	}
}

func TestBlobChunkCRU(t *testing.T) {
	ctx := context.Background()
	l, c, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestBlobChunkCRU",
	})
	defer span.End()
	ctx = c
	debug := 0
	bs := utils.NewBootstrap()
	bs.AddSpan(span)
	// bs.AddBootstrap("config:ft_arm64")
	// bs.AddBootstrap("layer:0:ft_arm64")
	bs.SetEmptyAll()
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
	// Actual test
	//// CREATE
	ociBU, buff := types.FTarmBlobUploadConfig()

	// ociBU := types.OCIBlobUpload{
	// 	Repo:          "qnib/featuretest",
	// 	UploadUser:    "test",
	// 	UploadProfile: "generic",
	// 	BlobType:      "layer",
	// 	Digest:        "sha256:6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30",
	// }
	utils.Debug(l, span, "> BlobUploadCreateByOCI")
	err = mag.Database.BlobUploadCreateByOCI(ctx, &ociBU)
	if err != nil {
		t.Fatal(err)
	}
	utils.Debug(l, span, "> UploadChunkCreate")
	bc, err := mag.Database.UploadChunkCreate(ctx, &ociBU, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, bc.ID)
	utils.Debug(l, span, "> Update Blob to Minio")
	if err := mag.StoreBlobUpload(ctx, &ociBU, buff); err != nil {
		t.Fatal(err)
	}
	// / READ
	utils.Debug(l, span, "> UploadChunkGetByUUID")
	bcs, err := mag.Database.UploadChunkGetByUUID(ctx, ociBU.ID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(bcs))
	// LinkBlobUpdate
	utils.Debug(l, span, "> BlobUploadUpdateByUUID")
	if e := mag.Database.BlobUploadUpdateByUUID(ctx, &ociBU); e != nil {
		t.Fatal(e)
	}
	utils.Debug(l, span, "ComposeFromChunks")
	err = mag.ComposeFromChunks(ctx, &ociBU, []types.BlobChunk{bc})
	if err != nil {
		t.Fatal(err)
	}
	// BlobUploadGetByBlobDigest
	upt := upstream.UpstreamTriple{
		Repo:   ociBU.Repo,
		Digest: digest.Digest(ociBU.Digest),
	}
	utils.Debug(l, span, "> BlobUploadGetByBlobDigest")
	if found, err := mag.BlobExists(ctx, upt); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Did not find BlobUpload")
	}
}
