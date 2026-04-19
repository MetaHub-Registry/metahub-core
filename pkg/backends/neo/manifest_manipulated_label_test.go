package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestManifestGetManipulatedByOriginalAndLabel_ScopedByMpLabel guards
// the dedup contract: two different manipulation labels pointing at
// the same original must be retrievable independently. Previously the
// query ignored manipulationLabel and returned whichever manipulated
// manifest ran first, causing `glow#1.5.0` pulls to reuse the
// `glow-extended#1.5.0` result (and vice-versa).
func TestManifestGetManipulatedByOriginalAndLabel_ScopedByMpLabel(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// Create the "original" manifest node directly — we don't need a
	// full manifest pipeline for this test, just three :Manifest nodes
	// and two MANIPULATED_FROM edges with different mpLabel values.
	origDigest := "sha256:orig0000000000000000000000000000000000000000000000000000000000"
	res, err := db.Execute(`CREATE (m:Manifest {digest: "` + origDigest + `", mediaType: "application/vnd.oci.image.manifest.v1+json", size: 0, label: "orig"}) RETURN id(m) AS id`)
	require.NoError(t, err)
	origID, err := res.GetInt64("id")
	require.NoError(t, err)

	res2, err := db.Execute(`CREATE (m:Manifest {digest: "sha256:glow0000000000000000000000000000000000000000000000000000000000", mediaType: "application/vnd.oci.image.manifest.v1+json", size: 0, label: "oci:glow"}) RETURN id(m) AS id`)
	require.NoError(t, err)
	glowID, err := res2.GetInt64("id")
	require.NoError(t, err)

	res3, err := db.Execute(`CREATE (m:Manifest {digest: "sha256:glowext00000000000000000000000000000000000000000000000000000000", mediaType: "application/vnd.oci.image.manifest.v1+json", size: 0, label: "oci:glowext"}) RETURN id(m) AS id`)
	require.NoError(t, err)
	glowExtID, err := res3.GetInt64("id")
	require.NoError(t, err)

	require.NoError(t, db.ManifestLinkManipulatedFromWithLabel(ctx, glowID, origID, "layer/glow#1.5.0"))
	require.NoError(t, db.ManifestLinkManipulatedFromWithLabel(ctx, glowExtID, origID, "layer/glow-extended#1.5.0"))

	// Lookup by the two different labels must return the two different
	// manipulated manifests.
	mdGlow, found, err := db.ManifestGetManipulatedByOriginalAndLabel(ctx, origDigest, "layer/glow#1.5.0")
	require.NoError(t, err)
	require.True(t, found, "glow manipulation must be found")
	assert.Equal(t, glowID, mdGlow.ID, "glow lookup must return the glow manifest, not glow-extended")

	mdExt, found, err := db.ManifestGetManipulatedByOriginalAndLabel(ctx, origDigest, "layer/glow-extended#1.5.0")
	require.NoError(t, err)
	require.True(t, found, "glow-extended manipulation must be found")
	assert.Equal(t, glowExtID, mdExt.ID, "glow-extended lookup must return the glow-extended manifest, not glow")

	// Lookup by an unknown label must return found=false.
	_, found, err = db.ManifestGetManipulatedByOriginalAndLabel(ctx, origDigest, "layer/unknown#9.9.9")
	require.NoError(t, err)
	assert.False(t, found, "unknown mpLabel must not match any edge")
}

// TestManifestLinkManipulatedFromLegacyHasEmptyLabel verifies the
// backward-compat shim: ManifestLinkManipulatedFrom (no-label variant)
// writes the edge with mpLabel="" so lookups with a non-empty label
// miss it — a legacy edge never pretends to be a named manipulation.
func TestManifestLinkManipulatedFromLegacyHasEmptyLabel(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	res, err := db.Execute(`CREATE (m:Manifest {digest: "sha256:origleg000000000000000000000000000000000000000000000000000000000", mediaType: "application/vnd.oci.image.manifest.v1+json", size: 0, label: "orig"}) RETURN id(m) AS id`)
	require.NoError(t, err)
	origID, err := res.GetInt64("id")
	require.NoError(t, err)
	res2, err := db.Execute(`CREATE (m:Manifest {digest: "sha256:manipleg0000000000000000000000000000000000000000000000000000000", mediaType: "application/vnd.oci.image.manifest.v1+json", size: 0, label: "oci:mani"}) RETURN id(m) AS id`)
	require.NoError(t, err)
	manipID, err := res2.GetInt64("id")
	require.NoError(t, err)

	require.NoError(t, db.ManifestLinkManipulatedFrom(ctx, manipID, origID))

	// A lookup with any non-empty label must NOT match this legacy edge.
	_, found, err := db.ManifestGetManipulatedByOriginalAndLabel(ctx, "sha256:origleg000000000000000000000000000000000000000000000000000000000", "layer/whatever#1.0")
	require.NoError(t, err)
	assert.False(t, found, "legacy edge (mpLabel='') must not match labelled lookup")
}
