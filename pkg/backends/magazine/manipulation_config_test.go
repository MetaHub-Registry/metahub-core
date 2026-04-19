package magazine

import (
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// TestEffectiveConfigForLayer_DefaultOnly confirms that a manipulation
// with only a Default block produces that block as the effective config
// when the matched layer has no variant rule.
func TestEffectiveConfigForLayer_DefaultOnly(t *testing.T) {
	mp := &types.Manipulation{
		Default: types.ManipuConfig{
			Workdir: "/opt",
			Env:     map[string]string{"A": "1"},
		},
	}
	layer := types.ManipuLayer{Rule: ""}
	out := effectiveConfigForLayer(mp, layer)
	assert.Equal(t, "/opt", out.Workdir)
	assert.Equal(t, "1", out.Env["A"])
}

// TestEffectiveConfigForLayer_VariantOverride confirms that a variant
// block overrides the default's scalar fields and adds to the env map.
func TestEffectiveConfigForLayer_VariantOverride(t *testing.T) {
	mp := &types.Manipulation{
		Default: types.ManipuConfig{
			Workdir: "/opt/default",
			User:    "1000:1000",
			Env:     map[string]string{"A": "1", "B": "2"},
			Volumes: []string{"/data"},
			Entrypoint: &types.ManipuCommand{
				Op:    types.ManipuCmdOpReplace,
				Value: []string{"/bin/default"},
			},
		},
		Variants: []types.ManipuVariant{
			{
				Rule: "arch:arm64=true",
				Config: types.ManipuConfig{
					Workdir: "/opt/arm",
					Env:     map[string]string{"B": "overridden", "C": "3"},
					Volumes: []string{"/data", "/var/log"},
					Cmd: &types.ManipuCommand{
						Op:    types.ManipuCmdOpAppend,
						Value: []string{"--arm"},
					},
				},
			},
		},
	}
	layer := types.ManipuLayer{Rule: "arch:arm64=true"}
	out := effectiveConfigForLayer(mp, layer)
	assert.Equal(t, "/opt/arm", out.Workdir, "variant workdir overrides default")
	assert.Equal(t, "1000:1000", out.User, "default user persists when variant omits it")
	assert.Equal(t, "1", out.Env["A"], "default env keys persist")
	assert.Equal(t, "overridden", out.Env["B"], "variant env overrides default")
	assert.Equal(t, "3", out.Env["C"], "variant adds new env keys")
	// Volumes: default has /data; variant adds /var/log; /data is deduped.
	assert.ElementsMatch(t, []string{"/data", "/var/log"}, out.Volumes)
	require.NotNil(t, out.Entrypoint, "default entrypoint persists when variant omits it")
	assert.Equal(t, "replace", out.Entrypoint.Op)
	require.NotNil(t, out.Cmd, "variant cmd is adopted")
	assert.Equal(t, "append", out.Cmd.Op)
}

// TestEffectiveConfigForLayer_LegacyReturnsZero ensures that a legacy
// flat-layers manipulation produces a zero ManipuConfig so the caller
// skips config application.
func TestEffectiveConfigForLayer_LegacyReturnsZero(t *testing.T) {
	mp := &types.Manipulation{
		ManipuLayers: []types.ManipuLayer{{Label: "legacy"}},
	}
	out := effectiveConfigForLayer(mp, mp.ManipuLayers[0])
	assert.True(t, out.IsZero())
}

// TestApplyManipuConfigToV1_AllFields verifies that each supported
// field of a ManipuConfig propagates to the v1.Image config.
func TestApplyManipuConfigToV1_AllFields(t *testing.T) {
	base := &v1.Image{}
	base.Config.Env = []string{"PATH=/usr/bin", "KEEP=me"}
	base.Config.Cmd = []string{"/bin/sh"}
	cfg := types.ManipuConfig{
		Workdir: "/opt/tool",
		User:    "nonroot",
		Env:     map[string]string{"A": "1", "KEEP": "overridden"},
		Volumes: []string{"/data", "/logs"},
		Entrypoint: &types.ManipuCommand{
			Op:    types.ManipuCmdOpReplace,
			Value: []string{"/bin/entry.sh"},
		},
		Cmd: &types.ManipuCommand{
			Op:    types.ManipuCmdOpAppend,
			Value: []string{"--verbose"},
		},
	}
	applyManipuConfigToV1(cfg, base)
	assert.Equal(t, "/opt/tool", base.Config.WorkingDir)
	assert.Equal(t, "nonroot", base.Config.User)
	assert.Contains(t, base.Config.Env, "PATH=/usr/bin")
	assert.Contains(t, base.Config.Env, "A=1")
	assert.Contains(t, base.Config.Env, "KEEP=overridden")
	assert.NotContains(t, base.Config.Env, "KEEP=me", "duplicate key must be replaced, not appended")
	assert.Contains(t, base.Config.Volumes, "/data")
	assert.Contains(t, base.Config.Volumes, "/logs")
	assert.Equal(t, []string{"/bin/entry.sh"}, base.Config.Entrypoint)
	assert.Equal(t, []string{"/bin/sh", "--verbose"}, base.Config.Cmd)
}

// TestApplyManipuCommand_Ops exercises replace/prepend/append/unknown.
func TestApplyManipuCommand_Ops(t *testing.T) {
	existing := []string{"/bin/sh", "-c"}
	t.Run("replace", func(t *testing.T) {
		out := applyManipuCommand(existing, &types.ManipuCommand{Op: types.ManipuCmdOpReplace, Value: []string{"/x"}})
		assert.Equal(t, []string{"/x"}, out)
	})
	t.Run("prepend", func(t *testing.T) {
		out := applyManipuCommand(existing, &types.ManipuCommand{Op: types.ManipuCmdOpPrepend, Value: []string{"tini", "--"}})
		assert.Equal(t, []string{"tini", "--", "/bin/sh", "-c"}, out)
	})
	t.Run("append", func(t *testing.T) {
		out := applyManipuCommand(existing, &types.ManipuCommand{Op: types.ManipuCmdOpAppend, Value: []string{"--log"}})
		assert.Equal(t, []string{"/bin/sh", "-c", "--log"}, out)
	})
	t.Run("unknown op leaves existing", func(t *testing.T) {
		out := applyManipuCommand(existing, &types.ManipuCommand{Op: "overwrite", Value: []string{"nope"}})
		assert.Equal(t, existing, out)
	})
	t.Run("nil is identity", func(t *testing.T) {
		out := applyManipuCommand(existing, nil)
		assert.Equal(t, existing, out)
	})
	t.Run("zero cmd is identity", func(t *testing.T) {
		out := applyManipuCommand(existing, &types.ManipuCommand{})
		assert.Equal(t, existing, out)
	})
}

// TestUpsertEnv_OrderStability verifies the emitted env slice is
// deterministic across runs so the manipulated manifest digest doesn't
// flap.
func TestUpsertEnv_OrderStability(t *testing.T) {
	base := []string{"PATH=/usr/bin"}
	add := map[string]string{"Z": "last", "A": "first", "M": "middle"}
	out1 := upsertEnv(base, add)
	out2 := upsertEnv(base, add)
	assert.Equal(t, out1, out2)
	// Sorted order of added keys: A, M, Z
	assert.Equal(t, []string{"PATH=/usr/bin", "A=first", "M=middle", "Z=last"}, out1)
}

// TestApplyManipuConfigToV1_Nil is safe to call with nil v1.Image.
func TestApplyManipuConfigToV1_Nil(t *testing.T) {
	// Must not panic.
	applyManipuConfigToV1(types.ManipuConfig{Workdir: "/x"}, nil)
}

// TestApplyManipuConfigToV1_ZeroCfgNoOp ensures a zero config leaves
// the v1.Image untouched.
func TestApplyManipuConfigToV1_ZeroCfgNoOp(t *testing.T) {
	base := &v1.Image{}
	base.Config.WorkingDir = "/keep"
	base.Config.User = "keep-user"
	base.Config.Env = []string{"X=1"}
	applyManipuConfigToV1(types.ManipuConfig{}, base)
	assert.Equal(t, "/keep", base.Config.WorkingDir)
	assert.Equal(t, "keep-user", base.Config.User)
	assert.Equal(t, []string{"X=1"}, base.Config.Env)
}
