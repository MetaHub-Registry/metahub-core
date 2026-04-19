package config

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadManipulation_SelectFixture guards that the example-select
// fixture parses cleanly and that each variant's `select:` survives
// the round-trip. The fixture is the authoritative example for the
// add-manipulayer-select change; any YAML-syntax drift in the parser
// surfaces here first.
func TestLoadManipulation_SelectFixture(t *testing.T) {
	path := filepath.Join(manipulationDir, "layer/example-select/1.0.0.yaml")
	mp, err := LoadManipulation(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, "layer/example-select#1.0.0", mp.Label)

	// The fixture uses the variants form, so `ManipuLayers` (legacy
	// flat slice) stays empty and the per-variant layers carry the
	// select. Index into Variants directly so the assertion is
	// deterministic regardless of map ordering.
	require.Len(t, mp.Variants, 2)
	arm := mp.Variants["arch:arm64=true"]
	require.Len(t, arm.Layers, 1)
	assert.Equal(t, "[1:]", arm.Layers[0].Select, "arm64 variant uses [1:] to skip base")

	amd := mp.Variants["arch:amd64=true"]
	require.Len(t, amd.Layers, 1)
	assert.Equal(t, "[-1]", amd.Layers[0].Select, "amd64 variant picks the last layer only")

	// Conversion to the types.ManipuConfig equivalent must carry the
	// Select field onto each resolved ManipuLayer.
	typed := arm.ToTypesManipuConfig()
	require.Len(t, typed.Layers, 1)
	assert.Equal(t, "[1:]", typed.Layers[0].Select)
}

// TestLoadManipulation_AbsentSelectDefaultsToEmpty checks the
// backward-compatibility contract: an existing YAML that predates
// `select:` parses to an empty string and the apply path treats
// empty as identity. Uses the legacy-form glow fixture which has no
// `select:` anywhere.
func TestLoadManipulation_AbsentSelectDefaultsToEmpty(t *testing.T) {
	path := filepath.Join(manipulationDir, "layer/glow/1.5.0.yaml")
	mp, err := LoadManipulation(context.Background(), path)
	require.NoError(t, err)
	// Either flat (legacy) or variants form — in both cases every
	// layer must have an empty Select.
	for _, lyr := range mp.ManipuLayers {
		assert.Empty(t, lyr.Select, "legacy layer must have no select")
	}
	for rule, cfg := range mp.Variants {
		for i, lyr := range cfg.Layers {
			assert.Empty(t, lyr.Select, "variant %q layer %d must have no select", rule, i)
		}
	}
}
