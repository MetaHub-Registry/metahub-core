package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestFixturesAreSlotCentred asserts that the manifest test fixtures wire the
// Slot-centred graph model: setting up a repo/tag/profile fixture creates a
// :Slot and a content-leaf :Manifest connected by a :HAS_PUSHED edge, and that
// NO legacy :ManifestWrapper node is created.
//
// deprecate-manifest-wrapper Phase 4: the read-side Cypher has been migrated
// off :ManifestWrapper, so fixtures no longer create wrapper nodes and this
// test pins the "zero wrapper nodes" invariant.
func TestFixturesAreSlotCentred(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.AddBootstrap("config:ft_zen3")
	bs.AddBootstrap("upstream:ft_zen3")
	bs.AddBootstrap("manifest:ft_zen3")
	bs.AddBootstrap("layer:0:ft_zen3")
	bs.AddBootstrap("layer:1:ft_zen3")
	bs.AddBootstrap("manifestdata:ft_zen3")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// The content-leaf :Manifest node is keyed "manifestdata:...#id".
	mdID, err := br.GetInt64("manifestdata:ft_zen3#id")
	require.NoError(t, err, "content-leaf Manifest id should be in the BootstrapResult")

	// The fixture must have created a :Slot and recorded its id.
	slotID, err := br.GetInt64("slot:ft_zen3#id")
	require.NoError(t, err, "fixture should create a :Slot and record slot:ft_zen3#id")

	// The content-leaf :Manifest node must exist.
	maniQ := fmt.Sprintf(`MATCH (m:Manifest) WHERE id(m) = %d RETURN count(m) AS c`, mdID)
	res, found, err := db.QueryOne(maniQ)
	require.NoError(t, err)
	require.True(t, found)
	cMani, err := res.GetInt("c")
	require.NoError(t, err)
	assert.Equal(t, 1, cMani, "content-leaf :Manifest node should exist")

	// The :Slot node must exist.
	slotQ := fmt.Sprintf(`MATCH (s:Slot) WHERE id(s) = %d RETURN count(s) AS c`, slotID)
	res, found, err = db.QueryOne(slotQ)
	require.NoError(t, err)
	require.True(t, found)
	cSlot, err := res.GetInt("c")
	require.NoError(t, err)
	assert.Equal(t, 1, cSlot, ":Slot node should exist")

	// The Slot must be connected to the content-leaf via :HAS_PUSHED.
	edgeQ := fmt.Sprintf(
		`MATCH (s:Slot)-[:HAS_PUSHED]->(m:Manifest) WHERE id(s) = %d AND id(m) = %d RETURN count(*) AS c`,
		slotID, mdID,
	)
	res, found, err = db.QueryOne(edgeQ)
	require.NoError(t, err)
	require.True(t, found)
	cEdge, err := res.GetInt("c")
	require.NoError(t, err)
	assert.Equal(t, 1, cEdge, "Slot should be linked to content-leaf Manifest via :HAS_PUSHED")

	// deprecate-manifest-wrapper Phase 4: fixtures must NOT create any
	// :ManifestWrapper node — the content-leaf :Manifest is the canonical node.
	wrapperQ := `MATCH (m:ManifestWrapper) RETURN count(m) AS c`
	res, found, err = db.QueryOne(wrapperQ)
	require.NoError(t, err)
	require.True(t, found)
	cWrapper, err := res.GetInt("c")
	require.NoError(t, err)
	assert.Equal(t, 0, cWrapper, "no :ManifestWrapper node should exist after fixture setup")
}
