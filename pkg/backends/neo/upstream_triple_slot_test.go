package neo

import (
	"context"
	"testing"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestCollectionGetCreateUpstreamTriple_NoWrapper asserts that the
// descriptor-ingest resolver no longer creates a legacy :ManifestWrapper node
// (deprecate-manifest-wrapper Phase 3, §5.1). It still resolves Upstream +
// Profile; the content-leaf :Manifest and its Slot are wired at ingest time via
// wireSlotOnPush, not by this resolver.
func TestCollectionGetCreateUpstreamTriple_NoWrapper(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// Baseline: no wrapper nodes exist after a clean bootstrap.
	wrapperBefore := countWrapperNodes(t)

	ups := types.Upstream{
		Reg:     "localhost:5000",
		Repo:    "qnib/test",
		Tag:     "latest",
		Digest:  "sha256:1234567890",
		Profile: "generic",
	}
	maniIn := manifestlist.ManifestDescriptor{
		Platform: manifestlist.PlatformSpec{
			Architecture: "amd64",
			OS:           "linux",
		},
	}
	upID, _, prf, err := db.CollectionGetCreateUpstreamTriple(ctx, ups, maniIn)
	require.NoError(t, err)
	assert.NotZero(t, prf.ID, "profile should resolve")
	assert.NotZero(t, upID, "upstream should resolve/create")

	// Upstream must have been created/resolved.
	_, upFound, err := db.UpstreamGet(ctx, ups)
	require.NoError(t, err)
	assert.True(t, upFound, "upstream should exist after CollectionGetCreateUpstreamTriple")

	// No :ManifestWrapper node may have been created.
	wrapperAfter := countWrapperNodes(t)
	assert.Equal(t, wrapperBefore, wrapperAfter,
		"CollectionGetCreateUpstreamTriple must NOT create a :ManifestWrapper node")
	assert.Equal(t, 0, wrapperAfter, "no :ManifestWrapper nodes should exist")
}

// countWrapperNodes returns the number of :ManifestWrapper nodes in the graph.
func countWrapperNodes(t *testing.T) int {
	t.Helper()
	res, found, err := db.QueryOne(`MATCH (m:ManifestWrapper) RETURN count(m) AS c`)
	require.NoError(t, err)
	require.True(t, found)
	c, err := res.GetInt("c")
	require.NoError(t, err)
	return c
}
