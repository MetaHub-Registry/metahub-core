package neo

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetLayersForManifestDigest proves the natural-key layer lookup returns
// the same ordered layers as the internal by-ID lookup. Public callers address
// a manifest by digest; the element ID never leaves the backend.
func TestGetLayersForManifestDigest(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	digest, err := br.Get("manifestdata:ft_amd64#digest")
	require.NoError(t, err)
	require.NotEmpty(t, digest)

	id, err := br.GetInt64("manifestdata:ft_amd64#id")
	require.NoError(t, err)

	byDigest, err := db.GetLayersForManifestDigest(ctx, digest)
	require.NoError(t, err)
	byID, err := db.GetLayersForManifestID(ctx, id)
	require.NoError(t, err)

	require.NotEmpty(t, byDigest, "fixture manifest must have layers")
	require.Len(t, byDigest, len(byID), "digest and ID lookups must return the same layer count")
	for i := range byID {
		assert.Equal(t, byID[i].Digest, byDigest[i].Digest, "layer order must match")
		assert.Equal(t, byID[i].MediaType, byDigest[i].MediaType)
		assert.Equal(t, byID[i].Size, byDigest[i].Size)
	}
}

// TestGetLayersForManifestDigest_Unknown returns an empty slice (no error) for
// a digest that is not in the graph.
func TestGetLayersForManifestDigest_Unknown(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	layers, err := db.GetLayersForManifestDigest(ctx, "sha256:0000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)
	assert.Empty(t, layers)
}
