package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindManifestCollections(t *testing.T) {

	var err error
	var cols map[string]ManifestCollection
	cols, err = FindManifestCollections(ctx, "../../../misc/yaml/collections/qnib/featuretest/generic.yaml")
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, cols, 1)
	_, err = FindManifestCollections(ctx, "../../../misc/yaml/collections/qnib/featuretest/mismatch_tag.yaml")
	assert.Error(t, err)
	cols, err = FindManifestCollections(ctx, "../../../misc/yaml/collections/library/perl/5.26.yaml")
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, cols, 1)
	col := cols["library/perl:5.26"]
	assert.Equal(t, "library/perl", col.Name)
	assert.Len(t, col.Manifests, 1)
	m := col.Manifests[0]
	assert.Equal(t, m.Image, "docker.io/library/perl:5.26.3-slim-threaded-stretch")
	assert.Equal(t, "arm64", m.Platform.Architecture)
}

// TestManifestProfileRoundTrip guards that an explicit per-manifest
// `profile:` survives parsing into Manifest.Profile (it was silently
// dropped before this field existed), while an entry without it parses
// to empty. This is the authoritative binding signal consumed at
// materialization (see ProfileInjectDeterministicAnnotations).
func TestManifestProfileRoundTrip(t *testing.T) {
	yamlStr := []byte(`name: qnib/ftest
tag: latest
manifests:
  - image: registry.gitlab.com/qnib/plain-featuretest:3-17-10-amd64
  - image: registry.gitlab.com/qnib/plain-featuretest:3-17-10-amd64-cpu_zen2
    profile: cpu#zen2
`)
	col, err := readManifestConfig(yamlStr)
	assert.NoError(t, err)
	assert.Len(t, col.Manifests, 2)
	assert.Empty(t, col.Manifests[0].Profile, "entry without profile parses empty")
	assert.Equal(t, "cpu#zen2", col.Manifests[1].Profile, "explicit profile round-trips")
}
