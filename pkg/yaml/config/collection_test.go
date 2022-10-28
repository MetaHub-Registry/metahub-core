package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func Test_FindManifestCollections(t *testing.T) {
	var err error
	var cols map[string]ManifestCollection
	cols, err = FindManifestCollections(ctx, "../../../misc/yaml/manifests/qnib/featuretest/generic.yaml")
	assert.NoError(t, err)
	assert.Len(t, cols, 1)
	_, err = FindManifestCollections(ctx, "../../../misc/yaml/manifests/qnib/featuretest/mismatch_tag.yaml")
	assert.Error(t, err)
	cols, err = FindManifestCollections(ctx, "../../../misc/yaml/manifests/library/perl/5.26.yaml")
	assert.NoError(t, err)
	assert.Len(t, cols, 1)
	col := cols["library/perl:5.26"]
	assert.Equal(t, "library/perl", col.Name)
	assert.Len(t, col.Manifests, 1)
	m := col.Manifests[0]
	assert.Equal(t, m.Image, "docker.io/library/perl:5.26.3-slim-threaded-stretch")
	assert.Equal(t, "arm64", m.Platform.Architecture)
}
