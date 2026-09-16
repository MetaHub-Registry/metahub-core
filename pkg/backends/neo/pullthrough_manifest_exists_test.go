package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestManifestExists_PrefixPreservedPullthroughViaSlot is the regression for
// prefix-preserved upstream pull-through (decisions D5/D6). The :Upstream node
// stores the registry-STRIPPED repo (e.g. "library/alpine") so upstream HTTP
// refs are well-formed, while the :Repo, :Tag and :Slot are keyed by the full
// client-facing alias (e.g. "harbor.example.com/library/alpine"). The
// Upstream-based join in ManifestExistsContent cannot bridge that mismatch, so
// before the Slot fallback a freshly-ingested pull-through manifest was
// reported as not-found and the plain pull 404'd even though the manifest and
// its generic Slot existed.
func TestManifestExists_PrefixPreservedPullthroughViaSlot(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	const aliasRepo = "harbor.example.com/library/alpine"
	const aliasTag = "3.24"

	// Seed the full-alias Repo/Tag + generic Slot holding the manifest while
	// the fixture's :Upstream node keeps its own (stripped/different) repo —
	// exactly the prefix-preservation shape.
	require.NoError(t, db.RepoLinkTag(ctx, aliasRepo, aliasTag))
	slot, err := db.SlotGetCreate(ctx, aliasRepo, aliasTag, "generic")
	require.NoError(t, err)
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, mdID, ""))

	prf, found, err := db.ProfileGetByLabel(ctx, "generic")
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, db.LinkManifestToProfile(ctx, mdID, prf.ID, "PULLED_BY"))

	mt, found, err := db.ManifestExists(ctx, upstream.UpstreamTriple{
		Repo:    aliasRepo,
		Tag:     aliasTag,
		Profile: "generic",
	})
	require.NoError(t, err)
	assert.True(t, found,
		"prefix-preserved pull-through manifest must be found via the Slot fallback")
	assert.NotEmpty(t, mt)
}
