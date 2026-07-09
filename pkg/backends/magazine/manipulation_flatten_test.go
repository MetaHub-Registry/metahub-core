package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// TestFlattenManipuLayers_Legacy confirms that the legacy flat
// `ManipuLayers` slice flattens as-is with rules preserved.
func TestFlattenManipuLayers_Legacy(t *testing.T) {
	mp := &types.Manipulation{
		ManipuLayers: []types.ManipuLayer{
			{Label: "a", Rule: "arch:arm64=true"},
			{Label: "b", Rule: "arch:amd64=true"},
		},
	}
	out := flattenManipuLayers(mp)
	assert.Len(t, out, 2)
	assert.Equal(t, "a", out[0].Label)
	assert.Equal(t, "arch:arm64=true", out[0].Rule)
	assert.Equal(t, "arch:amd64=true", out[1].Rule)
}

// TestManipuLayerMatchesPlatform_QuantRuleIsNotPlatform locks in the invariant
// behind the tryApplyModelManipulation fix: a multi-quant model variant layer
// carries a quant selector rule ("quant:<q>"), NOT a platform rule
// ("arch:.."/"os:.."). manipuLayerMatchesPlatform only understands platform
// rules, so it reports no-match for a quant rule. The model injection path
// therefore MUST NOT gate the resolved quant layer on manipuLayerMatchesPlatform
// — the quant is already resolved+matched (ResolveModelServe/layerImageMatchesQuant)
// and model weights are platform-agnostic. If the model path ever re-introduces
// this gate, the model layer silently stops injecting (base served unmanipulated),
// which is exactly the bug this test guards against.
func TestManipuLayerMatchesPlatform_QuantRuleIsNotPlatform(t *testing.T) {
	plat := types.Platform{OS: "linux", Arch: "amd64"}

	// A multi-quant variant layer's rule — must NOT read as a platform match.
	assert.False(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: "quant:q8-0"}, plat),
		"a quant selector rule is not a platform rule; the model path must bypass this gate")

	// Genuine platform rules still match; empty matches all (unchanged behaviour).
	assert.True(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: ""}, plat))
	assert.True(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: "os:linux=true"}, plat))
	assert.True(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: "arch:amd64=true"}, plat))
	assert.False(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: "arch:arm64=true"}, plat))
}

// TestFlattenManipuLayers_DefaultAndVariants verifies that default-block
// layers come first (with their rule unchanged) and then every variant's
// layers get the variant's rule propagated onto the layer entries.
func TestFlattenManipuLayers_DefaultAndVariants(t *testing.T) {
	mp := &types.Manipulation{
		Default: types.ManipuConfig{
			Layers: []types.ManipuLayer{{Label: "default-1"}},
		},
		Variants: []types.ManipuVariant{
			{
				Rule:   "arch:arm64=true",
				Config: types.ManipuConfig{Layers: []types.ManipuLayer{{Label: "arm64-1"}, {Label: "arm64-2"}}},
			},
			{
				Rule:   "arch:amd64=true",
				Config: types.ManipuConfig{Layers: []types.ManipuLayer{{Label: "amd64-1"}}},
			},
		},
	}
	out := flattenManipuLayers(mp)
	assert.Len(t, out, 4)
	// Default first, rule stays empty.
	assert.Equal(t, "default-1", out[0].Label)
	assert.Equal(t, "", out[0].Rule)
	// Variant rule propagated onto each variant's layers.
	assert.Equal(t, "arm64-1", out[1].Label)
	assert.Equal(t, "arch:arm64=true", out[1].Rule)
	assert.Equal(t, "arm64-2", out[2].Label)
	assert.Equal(t, "arch:arm64=true", out[2].Rule)
	assert.Equal(t, "amd64-1", out[3].Label)
	assert.Equal(t, "arch:amd64=true", out[3].Rule)
}

// TestFlattenManipuLayers_Empty ensures a manipulation with no layers
// anywhere returns an empty slice (never nil).
func TestFlattenManipuLayers_Empty(t *testing.T) {
	mp := &types.Manipulation{}
	out := flattenManipuLayers(mp)
	assert.NotNil(t, out)
	assert.Len(t, out, 0)
}

// TestFlattenManipuLayers_ExtendedSkipsLegacyMirror verifies that when
// the manipulation uses the new schema (Default or Variants populated)
// the legacy `ManipuLayers` slice is skipped. Otherwise the compat-wired
// `:HAS` edge would re-expose the same layers with empty rules, and the
// matcher would pick the first one for every platform regardless of the
// variant rule.
func TestFlattenManipuLayers_ExtendedSkipsLegacyMirror(t *testing.T) {
	mp := &types.Manipulation{
		// Legacy slice is a duplicate (no rule) of what's under Variants.
		// It exists because writeManipuConfig also writes :HAS for
		// backward-compat tooling (see metalib neo/manipuconfig.go).
		ManipuLayers: []types.ManipuLayer{
			{Label: "arm64-duplicate"},
			{Label: "amd64-duplicate"},
		},
		Variants: []types.ManipuVariant{
			{Rule: "arch:arm64=true", Config: types.ManipuConfig{Layers: []types.ManipuLayer{{Label: "arm64-variant"}}}},
			{Rule: "arch:amd64=true", Config: types.ManipuConfig{Layers: []types.ManipuLayer{{Label: "amd64-variant"}}}},
		},
	}
	out := flattenManipuLayers(mp)
	// Only the variant layers — legacy duplicates skipped.
	require.Len(t, out, 2)
	for _, lyr := range out {
		assert.NotContains(t, lyr.Label, "duplicate",
			"legacy mirror layers must be excluded when extended form is present")
		assert.NotEmpty(t, lyr.Rule, "variant layers must carry a rule")
	}
}

// TestManipuLayerMatchesPlatform verifies rule parsing for the various
// shapes the pull-through path produces.
func TestManipuLayerMatchesPlatform(t *testing.T) {
	plat := types.Platform{OS: "linux", Arch: "arm64"}

	// Empty rule matches anything (default-block layers).
	assert.True(t, manipuLayerMatchesPlatform(types.ManipuLayer{}, plat))

	// Arch match: arm64 hits, amd64 misses.
	assert.True(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: "arch:arm64=true"}, plat))
	assert.False(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: "arch:amd64=true"}, plat))

	// OS match: linux hits.
	assert.True(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: "os:linux=true"}, plat))
	assert.False(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: "os:darwin=true"}, plat))

	// Malformed rules don't match.
	assert.False(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: "notakey"}, plat))
	assert.False(t, manipuLayerMatchesPlatform(types.ManipuLayer{Rule: "unknown:foo=true"}, plat))
}
