package minio

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManifestFromBlob(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	// TODO: Bootstrap config correctly
	bs.AddBootstrap("config:ft_amd64_b109")
	_, err := min.SetupTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	defer min.PreTearDown(bs)
	mani, err := min.NewManifestDataFromBytes(ctx, featureTestGenericMani)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", mani.MediaType)
	assert.Equal(t, 2, len(mani.Layers))
	assert.Equal(t, "amd64", mani.Platform.Arch)
	assert.Equal(t, "linux", mani.Platform.OS)
}

func TestManifestPutStatRemove(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	_, err := min.SetupTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	defer min.PreTearDown(bs)
	// Test
	ctx := context.Background()
	err = min.ManifestsPutObj(ctx, featureTestGenericManiDigest, []byte(featureTestGenericMani))
	if err != nil {
		t.Fatal(err)
	}
	obj, err := min.ManifestsStatObj(ctx, featureTestGenericManiDigest)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, obj)
	err = min.ManifestRemoveObj(ctx, featureTestGenericManiDigest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = min.ManifestsStatObj(ctx, featureTestGenericManiDigest)
	assert.Error(t, err)
	up := upstream.UpstreamTriple{
		Reg:    "docker.io",
		Repo:   "qnib/metahub-admin",
		Tag:    "latest",
		Digest: digest.Digest(featureTestGenericManiDigest),
	}
	err = min.PutManifest(ctx, []byte(featureTestGenericMani), up)
	if err != nil {
		t.Fatal(err)
	}
	obj, err = min.ManifestsStatObj(ctx, featureTestGenericManiDigest)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, obj)
	mani, isCached, err := min.GetManifest(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, isCached)
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", mani.MediaType)
	err = min.ManifestRemoveObj(ctx, featureTestGenericManiDigest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = min.ManifestsStatObj(ctx, featureTestGenericManiDigest)
	assert.Error(t, err)
}
