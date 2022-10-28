package minio

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
)

func Test_ManifestFromBlob(t *testing.T) {
	SetupTest("cfg:generic")
	defer TeardownTest("cfg:generic")
	mani, err := min.NewManifestFromBytes(featureTestGenericMani)
	assert.NoError(t, err)
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", mani.MediaType)
	assert.Equal(t, 2, len(mani.Layers))
	assert.Equal(t, "amd64", mani.Platform.Arch)
	assert.Equal(t, "linux", mani.Platform.OS)
}

func Test_Manifest_RemoveManifestAliases(t *testing.T) {
	ctx := context.Background()

	up := upstream.UpstreamTriple{
		Reg:    "docker.io",
		Repo:   "qnib/metahub-admin",
		Tag:    "latest",
		Digest: digest.Digest(featureTestGenericManiDigest),
	}
	err := min.PutManifest(ctx, []byte(featureTestGenericMani), up)
	assert.NoError(t, err)
	err = min.RemoveManifestAliases()
	assert.NoError(t, err)
	err = min.ManifestRemoveObj(ctx, featureTestGenericManiDigest)
	assert.NoError(t, err)
	_, err = min.ManifestsStatObj(ctx, featureTestGenericManiDigest)
	assert.Error(t, err)

}
func Test_Manifest_PutStatRemove(t *testing.T) {
	SetupTest("cfg:generic")
	defer TeardownTest("cfg:generic")
	// Test
	ctx := context.Background()
	err := min.ManifestsPutObj(ctx, featureTestGenericManiDigest, []byte(featureTestGenericMani))
	assert.NoError(t, err)
	obj, err := min.ManifestsStatObj(ctx, featureTestGenericManiDigest)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
	err = min.ManifestRemoveObj(ctx, featureTestGenericManiDigest)
	assert.NoError(t, err)
	_, err = min.ManifestsStatObj(ctx, featureTestGenericManiDigest)
	assert.Error(t, err)
	up := upstream.UpstreamTriple{
		Reg:    "docker.io",
		Repo:   "qnib/metahub-admin",
		Tag:    "latest",
		Digest: digest.Digest(featureTestGenericManiDigest),
	}
	err = min.PutManifest(ctx, []byte(featureTestGenericMani), up)
	assert.NoError(t, err)
	obj, err = min.ManifestsStatObj(ctx, featureTestGenericManiDigest)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
	mani, isCached, err := min.GetManifest(ctx, up)
	assert.NoError(t, err)
	assert.True(t, isCached)
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", mani.MediaType)
	err = min.ManifestRemoveObj(ctx, featureTestGenericManiDigest)
	assert.NoError(t, err)
	_, err = min.ManifestsStatObj(ctx, featureTestGenericManiDigest)
	assert.Error(t, err)
}
