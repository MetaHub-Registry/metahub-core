package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGatherCollectionPins_PullThroughAliasHasNoPins is the regression for the
// upstream pull-through pin-poisoning bug. A pull-through alias
// (`<registry-slug>/<rest>`) is not a parse-authored collection: an upstream
// pull under a manipulation profile (e.g. glow#1.5.0) wires the ORIGINAL base
// manifest into that profile's Slot. Before the fix, that edge was gathered as
// an exclusive pin, so the generic profile's computed index was empty and a
// plain <registry>/<repo>:<tag> pull found no servable Index (404).
//
// Pull-through aliases must yield no pins, so the shared base stays in the
// generic (multi-arch) index.
func TestGatherCollectionPins_PullThroughAliasHasNoPins(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	const aliasRepo = "harbor.example.com/library/alpine"
	const aliasTag = "3.24"

	// The manipulation-profile Slot edge that used to become a pin.
	glowSlot, err := mag.Database.SlotGetCreate(ctx, aliasRepo, aliasTag, "glow#1.5.0")
	require.NoError(t, err)
	require.NoError(t, mag.Database.SlotAppendPushedManifest(ctx, glowSlot.ID, mdID, ""))

	// Configure the alias as a pull-through registry so ResolveUpstreamRegistry
	// recognises it.
	prev := mag.Config.Registries
	mag.Config.Registries = []RegistryConfig{{
		Name:        "harbor.example.com",
		AllowAnon:   true,
		PullThrough: PullThroughConfig{Enabled: true},
	}}
	defer func() { mag.Config.Registries = prev }()

	l, span := testLogSpan()
	pins := mag.gatherCollectionPins(ctx, l, span, aliasRepo, aliasTag)
	assert.Empty(t, pins,
		"pull-through aliases carry no explicit collection pins (shared base stays generic-visible)")
}
