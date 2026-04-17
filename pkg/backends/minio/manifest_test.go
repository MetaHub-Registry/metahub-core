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
	_, err := min.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer min.PreTearDown(bs)
	mani, err := min.NewManifestFromBytes(ctx, featureTestGenericMani)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", mani.MediaType)
	assert.Equal(t, 2, len(mani.Layers))
	assert.Equal(t, "amd64", mani.Platform.Arch)
	assert.Equal(t, "linux", mani.Platform.OS)
	// Regression guard: Manifest.Size MUST be the size of the manifest
	// bytes themselves (what an OCI descriptor points at), NOT the
	// size of the config blob it references. Before the fix in
	// NewManifestFromBytes, this was erroneously set to the config's
	// size (bInfo.Size) — which caused computed Indexes to emit
	// descriptors with the wrong size, breaking manifest-tool /
	// containerd pulls with 'failed to read expected number of bytes'.
	// The manifest bytes are the canonical source of truth here.
	assert.Equal(t, int64(len(featureTestGenericMani)), mani.Size,
		"Manifest.Size must equal len(manifest bytes), not Config.Size")
	// Sanity: Config.Size must NOT equal Manifest.Size for this fixture
	// (they are genuinely different — the manifest is ~hundreds of
	// bytes; the config is larger). If these ever become equal for a
	// fixture, pick a different fixture to preserve the test's value.
	assert.NotEqual(t, mani.Size, mani.Config.Size,
		"test fixture must have distinct Manifest.Size and Config.Size so the regression guard is meaningful")
}

func TestManifestPutStatRemove(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	ctx := context.Background()
	_, err := min.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer min.PreTearDown(bs)
	// Test
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
