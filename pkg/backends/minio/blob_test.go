package minio

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
)

var layerBlob = []byte(`{
	"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
	"size": 2818413,
	"digest": "sha256:59bf1c3509f33515622619af21ed55bbe26d24913cedbca106468a5fb37a50c3"
 }`)

func TestPutStatRemove(t *testing.T) {
	ctx := context.Background()
	layerDig := "sha256:112233445566"
	err := min.BlobPutObj(ctx, layerDig, layerBlob)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := min.BlobStatObj(ctx, layerDig)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, obj)
	err = min.BlobRemoveObj(ctx, layerDig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = min.BlobStatObj(ctx, layerDig)
	assert.Error(t, err)
	err = min.PutBlob(ctx, layerBlob, layerDig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = min.BlobStatObj(ctx, layerDig)
	if err != nil {
		t.Fatal(err)
	}
	up := upstream.UpstreamTriple{
		Digest: digest.Digest(layerDig),
	}
	cached, err := min.GetBlob(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, cached)
	err = min.BlobRemoveObj(ctx, layerDig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = min.BlobStatObj(ctx, layerDig)
	assert.Error(t, err)
}

func TestRemoveObj(t *testing.T) {
	ctx := context.Background()
	layerDig := "sha256:112233445566"
	err := min.BlobRemoveObj(ctx, layerDig)
	if err == nil {
		t.Fatal("expected error on removing non existing object")
	}
}
func TestEnsureBlobExists(t *testing.T) {
	ctx := context.Background()
	layerDig := "sha256:112233445566"
	layerBlob := []byte(`testdata`)
	err := min.EnsureBlobExists(ctx, layerDig, layerBlob)
	if err != nil {
		t.Fatal(err)
	}
	err = min.EnsureBlobExists(ctx, layerDig, layerBlob)
	if err != nil {
		t.Fatal(err)
	}
}
