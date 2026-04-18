package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManipulationExtended_FullForm parses an example file that exercises
// every extended field.
func TestManipulationExtended_FullForm(t *testing.T) {
	yml := []byte(`label: example-full#1.0.0
env:
  FOO: bar
workdir: /opt/ws
user: "1000:1000"
volumes:
  - /data
entrypoint:
  op: replace
  value: ["/bin/entry.sh"]
cmd:
  op: append
  value: ["--flag"]
layers:
  - image: registry:5050/cfg:1.0.0
variants:
  "arch:arm64=true":
    layers:
      - image: registry:5050/arm64:1.0.0
  "arch:amd64=true":
    env:
      EXTRA: enabled
    layers:
      - image: registry:5050/amd64:1.0.0
`)
	mp, err := readManipulationConfig(yml)
	require.NoError(t, err)
	assert.Equal(t, "example-full#1.0.0", mp.Label)
	assert.True(t, mp.hasNewTopLevel())
	assert.False(t, mp.hasLegacyRuleLayers())

	def := mp.DefaultConfig()
	assert.Equal(t, "/opt/ws", def.Workdir)
	assert.Equal(t, "1000:1000", def.User)
	assert.Equal(t, []string{"/data"}, def.Volumes)
	require.NotNil(t, def.Entrypoint)
	assert.Equal(t, "replace", def.Entrypoint.Op)
	require.NotNil(t, def.Cmd)
	assert.Equal(t, "append", def.Cmd.Op)
	require.Len(t, def.Layers, 1)
	assert.Equal(t, "registry:5050/cfg:1.0.0", def.Layers[0].Image)

	variants := mp.NormalizedVariants()
	require.Len(t, variants, 2)
	require.Contains(t, variants, "arch:arm64=true")
	require.Contains(t, variants, "arch:amd64=true")
	assert.Equal(t, "enabled", variants["arch:amd64=true"].Env["EXTRA"])
}

// TestManipulationExtended_VariantsOnly ensures a YAML with only `variants:`
// parses and produces no default.
func TestManipulationExtended_VariantsOnly(t *testing.T) {
	yml := []byte(`label: varonly#1
variants:
  "arch:arm64=true":
    layers:
      - image: registry:5050/arm64:1
`)
	mp, err := readManipulationConfig(yml)
	require.NoError(t, err)
	assert.True(t, mp.hasNewTopLevel())
	assert.True(t, mp.DefaultConfig().IsZero())
	require.Len(t, mp.NormalizedVariants(), 1)
}

// TestManipulationExtended_DefaultsOnly ensures a YAML with only top-level
// defaults parses and produces empty variants.
func TestManipulationExtended_DefaultsOnly(t *testing.T) {
	yml := []byte(`label: defonly#1
workdir: /opt/ws
layers:
  - image: registry:5050/cfg:1
`)
	mp, err := readManipulationConfig(yml)
	require.NoError(t, err)
	assert.True(t, mp.hasNewTopLevel())
	assert.False(t, mp.hasLegacyRuleLayers())
	require.Len(t, mp.NormalizedVariants(), 0)
	def := mp.DefaultConfig()
	assert.Equal(t, "/opt/ws", def.Workdir)
	require.Len(t, def.Layers, 1)
}

// TestManipulationExtended_LegacyRewritesToVariants verifies the legacy
// flat form produces variants keyed by each layer's rule string.
func TestManipulationExtended_LegacyRewritesToVariants(t *testing.T) {
	yml := []byte(`label: legacy#1
layers:
  - image: registry:5050/arm64:1
    rule: arch:arm64=true
  - image: registry:5050/amd64:1
    rule: arch:amd64=true
`)
	mp, err := readManipulationConfig(yml)
	require.NoError(t, err)
	assert.True(t, mp.hasLegacyRuleLayers())
	assert.False(t, mp.hasNewTopLevel())

	variants := mp.NormalizedVariants()
	require.Len(t, variants, 2)
	require.Contains(t, variants, "arch:arm64=true")
	require.Contains(t, variants, "arch:amd64=true")
	assert.Equal(t, "registry:5050/arm64:1", variants["arch:arm64=true"].Layers[0].Image)
}

// TestManipulationExtended_MixedFormRejected verifies that a file mixing
// per-layer `rule:` with a top-level `variants:` block is rejected by
// validate().
func TestManipulationExtended_MixedFormRejected(t *testing.T) {
	yml := []byte(`label: mixed#1
layers:
  - image: registry:5050/arm64:1
    rule: arch:arm64=true
variants:
  "arch:amd64=true":
    layers:
      - image: registry:5050/amd64:1
`)
	mp, err := readManipulationConfig(yml)
	require.NoError(t, err)
	err = mp.validate("mixed.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mixed form")
}

// TestManipulationExtended_InvalidOpRejected verifies that an unknown
// command op triggers validation failure.
func TestManipulationExtended_InvalidOpRejected(t *testing.T) {
	yml := []byte(`label: op#1
entrypoint:
  op: overwrite
  value: ["/bin/entry.sh"]
`)
	mp, err := readManipulationConfig(yml)
	require.NoError(t, err)
	err = mp.validate("op.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op")
}

// TestManipulationExtended_InvalidVariantRuleRejected verifies that a rule
// string that cannot be parsed by expr causes validation failure.
func TestManipulationExtended_InvalidVariantRuleRejected(t *testing.T) {
	yml := []byte(`label: rule#1
variants:
  "arch&amd64":
    layers:
      - image: registry:5050/amd64:1
`)
	mp, err := readManipulationConfig(yml)
	require.NoError(t, err)
	err = mp.validate("rule.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "arch&amd64")
}

// TestManipulationExtended_LoadExampleFullFixture confirms the example
// fixture shipped under misc/yaml is valid.
func TestManipulationExtended_LoadExampleFullFixture(t *testing.T) {
	mp, err := LoadManipulation(ctx, filepath.Join(manipulationDir, "/layer/example-full/1.0.0.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "layer/example-full#1.0.0", mp.Label)
	require.Len(t, mp.NormalizedVariants(), 2)
	def := mp.DefaultConfig()
	assert.Equal(t, "/opt/glow", def.Workdir)
}

// TestManipulationExtended_LoadExampleMinimalFixture confirms the minimal
// fixture shipped under misc/yaml is valid.
func TestManipulationExtended_LoadExampleMinimalFixture(t *testing.T) {
	mp, err := LoadManipulation(ctx, filepath.Join(manipulationDir, "/layer/example-minimal/1.0.0.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "layer/example-minimal#1.0.0", mp.Label)
	assert.True(t, mp.DefaultConfig().IsZero())
	require.Len(t, mp.NormalizedVariants(), 2)
}

// TestManipulationLegacyLoadEmitsDeprecationAndStillWorks confirms that
// loading the existing legacy glow fixture still works (the warning goes
// to the log).
func TestManipulationLegacyLoadStillWorks(t *testing.T) {
	mp, err := LoadManipulation(ctx, filepath.Join(manipulationDir, "/layer/glow/1.5.0.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "layer/glow#1.5.0", mp.Label)
	assert.True(t, mp.hasLegacyRuleLayers())
	variants := mp.NormalizedVariants()
	require.Len(t, variants, 2)
}
