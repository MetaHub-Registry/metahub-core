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

func Test_PutStatRemove(t *testing.T) {
	ctx := context.Background()
	layerDig := "sha256:112233445566"
	err := min.LayerPutObj(ctx, layerDig, layerBlob)
	assert.NoError(t, err)
	obj, err := min.LayersStatObj(ctx, layerDig)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
	err = min.LayerRemoveObj(ctx, layerDig)
	assert.NoError(t, err)
	_, err = min.LayersStatObj(ctx, layerDig)
	assert.Error(t, err)
	err = min.PutLayer(ctx, layerBlob, layerDig)
	assert.NoError(t, err)
	_, err = min.LayersStatObj(ctx, layerDig)
	assert.NoError(t, err)
	up := upstream.UpstreamTriple{
		Digest: digest.Digest(layerDig),
	}
	cached, err := min.GetLayer(ctx, up)
	assert.NoError(t, err)
	assert.True(t, cached)
	err = min.LayerRemoveObj(ctx, layerDig)
	assert.NoError(t, err)
	_, err = min.LayersStatObj(ctx, layerDig)
	assert.Error(t, err)

}
