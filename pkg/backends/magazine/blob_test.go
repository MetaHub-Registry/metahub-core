package magazine

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
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
		t.Fatal(err)
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

// TestStoreBlobUploadReader exercises the streaming (constant-memory) blob store
// used for monolithic model-layer PUTs: the body is streamed to MinIO under the
// expected digest and verified — a wrong expected digest must fail without
// persisting the blob.
func TestStoreBlobUploadReader(t *testing.T) {
	ctx := context.Background()
	l, c, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestStoreBlobUploadReader",
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
	defer TeardownTest(bs, br)

	bu, buff := types.FTarmBlobUploadConfig()
	bu.Digest = digest.FromBytes(buff).String()
	utils.Debug(l, span, "Create BlobUpload + chunk")
	if err := mag.Database.BlobUploadCreateByOCI(ctx, &bu); err != nil {
		t.Fatal(err)
	}
	chunk, err := mag.Database.UploadChunkCreate(ctx, &bu, 0, int64(len(buff)))
	if err != nil {
		t.Fatal(err)
	}
	utils.Debug(l, span, "Streaming blob (constant memory)")
	if err := mag.StoreBlobUploadReader(ctx, &bu, bytes.NewReader(buff), int64(len(buff))); err != nil {
		t.Fatal(err)
	}
	if err := mag.ComposeFromChunks(ctx, &bu, []types.BlobChunk{chunk}); err != nil {
		t.Fatal(err)
	}
	dt := types.DataTupeFTConfigArm64()
	upt, err := upstream.UpstreamToUpstreamTriple(dt.Upstream, types.MediaTypeImageConfig)
	if err != nil {
		t.Fatal(err)
	}
	upt.Digest = digest.Digest(bu.Digest)
	if found, err := mag.BlobExists(ctx, upt); err != nil {
		t.Fatal(err)
	} else {
		assert.True(t, found)
	}
	// Dedup path: mirror the e2e --empty-graph=keep-upstream wipe by deleting the
	// :Blob graph node while the S3 object remains. Re-storing must skip the
	// upload (object already present) yet still recreate the node so the blob is
	// resolvable again — without paying the (slow remote-S3) re-upload.
	if err := mag.Database.BlobDeleteByDigest(ctx, bu.Digest); err != nil {
		t.Fatal(err)
	}
	bu2, _ := types.FTarmBlobUploadConfig()
	bu2.Digest = bu.Digest
	if err := mag.Database.BlobUploadCreateByOCI(ctx, &bu2); err != nil {
		t.Fatal(err)
	}
	chunk2, err := mag.Database.UploadChunkCreate(ctx, &bu2, 0, int64(len(buff)))
	if err != nil {
		t.Fatal(err)
	}
	if err := mag.StoreBlobUploadReader(ctx, &bu2, bytes.NewReader(buff), int64(len(buff))); err != nil {
		t.Fatalf("re-store of already-present blob must dedup and succeed: %v", err)
	}
	if err := mag.ComposeFromChunks(ctx, &bu2, []types.BlobChunk{chunk2}); err != nil {
		t.Fatal(err)
	}
	upt.Digest = digest.Digest(bu2.Digest)
	if found, err := mag.BlobExists(ctx, upt); err != nil {
		t.Fatal(err)
	} else {
		assert.True(t, found, "blob node must be recreated from the already-present object")
	}
	// A wrong expected digest must be rejected (and not persisted).
	bad := bu
	bad.Digest = "sha256:" + strings.Repeat("0", 64)
	if err := mag.StoreBlobUploadReader(ctx, &bad, bytes.NewReader(buff), int64(len(buff))); err == nil {
		t.Fatal("expected digest-mismatch error, got nil")
	}
}

// TestStoreBlobUpload_Dedup covers the buffered (PATCH-chunk) store path: once a
// content-addressed object is in the bucket, a second store of the same
// digest+size must skip the S3 PutObject rather than re-upload it. Traces showed
// this path (BlobUploadPatchHandler -> StoreBlobUpload -> PutBlob) re-writing
// every chunk unconditionally (~0.9s avg S3 write each) on a re-seed after the
// e2e --empty-graph wipe. The assertion is whitebox: re-storing the same digest
// with DIFFERENT bytes must leave the ORIGINAL payload in place — a skipped
// upload — because a re-upload would overwrite it with the decoy.
func TestStoreBlobUpload_Dedup(t *testing.T) {
	ctx := context.Background()
	l, c, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestStoreBlobUpload_Dedup",
	})
	ctx = c
	defer span.End()
	_ = l
	bs := utils.NewBootstrap()
	bs.AddSpan(span)
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	bu, buff := types.FTarmBlobUploadConfig()
	bu.Digest = digest.FromBytes(buff).String()
	if err := mag.Database.BlobUploadCreateByOCI(ctx, &bu); err != nil {
		t.Fatal(err)
	}
	if _, err := mag.Database.UploadChunkCreate(ctx, &bu, 0, int64(len(buff))); err != nil {
		t.Fatal(err)
	}
	// First store writes the object to MinIO.
	if err := mag.StoreBlobUpload(ctx, &bu, buff); err != nil {
		t.Fatal(err)
	}
	if _, err := mag.Minio.BlobStatObj(ctx, bu.Digest); err != nil {
		t.Fatalf("object must be present after first store: %v", err)
	}
	// Second store: same digest+size, DIFFERENT bytes. Must dedup (skip PutObject).
	decoy := make([]byte, len(buff))
	copy(decoy, buff)
	decoy[0] ^= 0xff // same length, different content
	bu2, _ := types.FTarmBlobUploadConfig()
	bu2.Digest = bu.Digest
	if err := mag.Database.BlobUploadCreateByOCI(ctx, &bu2); err != nil {
		t.Fatal(err)
	}
	if _, err := mag.Database.UploadChunkCreate(ctx, &bu2, 0, int64(len(decoy))); err != nil {
		t.Fatal(err)
	}
	if err := mag.StoreBlobUpload(ctx, &bu2, decoy); err != nil {
		t.Fatalf("re-store of already-present blob must dedup and succeed: %v", err)
	}
	got, err := mag.Minio.BlobGetBytes(ctx, bu.Digest)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, buff, got, "dedup must skip the S3 write — stored object stays the original payload, not the decoy")
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
