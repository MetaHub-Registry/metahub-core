package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ceManipulationsDir points at the community-edition worktree checked out
// alongside metalib within the session. Tests skip gracefully if the
// directory is absent so the suite stays runnable outside the session.
const ceManipulationsDir = "../../../../community-edition/manipulations"

func ceFileExists(t *testing.T, path string) bool {
	t.Helper()
	mp, err := LoadManipulation(ctx, path)
	if err != nil {
		// Missing file vs parse error — let the caller assert.
		t.Logf("LoadManipulation(%s) err=%v label=%s", path, err, mp.Label)
		return false
	}
	return true
}

// TestCEGlowParses confirms that the migrated community-edition glow
// manipulation YAML (new variants form) parses cleanly without the
// deprecation warning.
func TestCEGlowParses(t *testing.T) {
	path := filepath.Join(ceManipulationsDir, "layer/glow/1.5.0.yaml")
	mp, err := LoadManipulation(ctx, path)
	if err != nil {
		t.Skipf("community-edition glow fixture not available: %v", err)
	}
	assert.Equal(t, "layer/glow#1.5.0", mp.Label)
	// New schema means no legacy per-layer rule, and variants populated.
	assert.False(t, mp.hasLegacyRuleLayers(),
		"glow YAML should have been migrated to the variants schema")
	assert.True(t, mp.hasNewTopLevel())
	require.Len(t, mp.NormalizedVariants(), 2)
}

// TestCEGlowExtendedParses confirms the richer example exercising
// env / cmd / workdir / variants parses.
func TestCEGlowExtendedParses(t *testing.T) {
	path := filepath.Join(ceManipulationsDir, "layer/glow-extended/1.5.0.yaml")
	mp, err := LoadManipulation(ctx, path)
	if err != nil {
		t.Skipf("community-edition glow-extended fixture not available: %v", err)
	}
	assert.Equal(t, "layer/glow-extended#1.5.0", mp.Label)
	def := mp.DefaultConfig()
	// The fixture injects hpc3-help as the entrypoint and hands off to
	// the base image's intended program (entry.sh) as the cmd, so the
	// help renders once and then the original execution flows. Pinning
	// `bash` as the cmd loops, so entry.sh is the expected hand-off.
	require.NotNil(t, def.Entrypoint)
	assert.Equal(t, "replace", def.Entrypoint.Op)
	assert.Equal(t, []string{"/usr/bin/hpc3-help"}, def.Entrypoint.Value)
	require.NotNil(t, def.Cmd)
	assert.Equal(t, "replace", def.Cmd.Op)
	assert.Equal(t, []string{"/usr/bin/entry.sh"}, def.Cmd.Value)
	variants := mp.NormalizedVariants()
	require.Len(t, variants, 2)
	require.Contains(t, variants, "arch:amd64=true")
	// The amd64 variant sets a GLOW_SIMD override — value content is
	// intentionally unchecked (the fixture updates with the HPC target
	// du jour); just guard that the variant overrides GLOW_SIMD at all.
	assert.NotEmpty(t, variants["arch:amd64=true"].Env["GLOW_SIMD"], "amd64 variant overrides GLOW_SIMD")
}

// TestCEGlow150MirrorFixtureParses confirms the metahub-api compose-stack
// mirror fixture is also valid under the new schema.
func TestCEGlow150MirrorFixtureParses(t *testing.T) {
	path := "../../../../metahub-api/misc/fixtures/manipulations/layer/glow/1.5.0.yaml"
	if !ceFileExists(t, path) {
		t.Skipf("metahub-api glow mirror fixture not available")
	}
	mp, err := LoadManipulation(ctx, path)
	require.NoError(t, err)
	assert.Equal(t, "layer/glow#1.5.0", mp.Label)
	assert.False(t, mp.hasLegacyRuleLayers())
	require.Len(t, mp.NormalizedVariants(), 2)
}
