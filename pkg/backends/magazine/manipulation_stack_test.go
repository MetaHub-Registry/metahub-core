package magazine

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// The model-serving stack composes several manipulations (weights + config)
// onto one manifest. It must be strictly ADDITIVE: layers stack, env keys union,
// but if two manipulations set the same env key (or Entrypoint/Cmd/WorkingDir/
// User) to DIFFERENT values one would silently invalidate the other, so
// detectStackCollisions rejects it. These tests pin that contract.

func cmd(op string, v ...string) *types.ManipuCommand {
	return &types.ManipuCommand{Op: op, Value: v}
}

// TestDetectStackCollisions_AdditiveOK: the real model+config shape — the model
// (weights) step carries no config, the config step adds one env key — composes
// cleanly.
func TestDetectStackCollisions_AdditiveOK(t *testing.T) {
	steps := []stackStep{
		{label: "model/qnib/minicpm5", cfg: types.ManipuConfig{}},
		{label: "config/qnib/minicpm5", cfg: types.ManipuConfig{
			Env: map[string]string{"QNIB_MODEL_CONFIG_DIR": "/model/qnib/minicpm5/config"},
		}},
	}
	require.NoError(t, detectStackCollisions(steps))
}

// TestDetectStackCollisions_EnvOverwriteFails: two manipulations set the same env
// key to different values → the chain is rejected (naming both + the key).
func TestDetectStackCollisions_EnvOverwriteFails(t *testing.T) {
	steps := []stackStep{
		{label: "model/m", cfg: types.ManipuConfig{Env: map[string]string{"QNIB_MODEL_CONFIG_DIR": "/a"}}},
		{label: "config/m", cfg: types.ManipuConfig{Env: map[string]string{"QNIB_MODEL_CONFIG_DIR": "/b"}}},
	}
	err := detectStackCollisions(steps)
	require.Error(t, err)
	require.Contains(t, err.Error(), "QNIB_MODEL_CONFIG_DIR")
	require.Contains(t, err.Error(), "additive only")
	require.Contains(t, err.Error(), "model/m")
	require.Contains(t, err.Error(), "config/m")
}

// TestDetectStackCollisions_SameValueIdempotent: setting the same env key to the
// SAME value across steps is fine (union, not a conflict).
func TestDetectStackCollisions_SameValueIdempotent(t *testing.T) {
	steps := []stackStep{
		{label: "a", cfg: types.ManipuConfig{Env: map[string]string{"FOO": "bar"}}},
		{label: "b", cfg: types.ManipuConfig{Env: map[string]string{"FOO": "bar", "BAZ": "qux"}}},
	}
	require.NoError(t, detectStackCollisions(steps))
}

// TestDetectStackCollisions_EntrypointConflictFails: two manipulations both set
// Entrypoint → conflict (a manifest has exactly one entrypoint).
func TestDetectStackCollisions_EntrypointConflictFails(t *testing.T) {
	steps := []stackStep{
		{label: "a", cfg: types.ManipuConfig{Entrypoint: cmd(types.ManipuCmdOpReplace, "/bin/a")}},
		{label: "b", cfg: types.ManipuConfig{Entrypoint: cmd(types.ManipuCmdOpReplace, "/bin/b")}},
	}
	err := detectStackCollisions(steps)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Entrypoint")
}

// TestDetectStackCollisions_WorkdirConflictFails: conflicting WorkingDir → reject.
func TestDetectStackCollisions_WorkdirConflictFails(t *testing.T) {
	steps := []stackStep{
		{label: "a", cfg: types.ManipuConfig{Workdir: "/x"}},
		{label: "b", cfg: types.ManipuConfig{Workdir: "/y"}},
	}
	err := detectStackCollisions(steps)
	require.Error(t, err)
	require.Contains(t, err.Error(), "WorkingDir")
}

// TestDetectStackCollisions_UserConflictFails: conflicting User → reject.
func TestDetectStackCollisions_UserConflictFails(t *testing.T) {
	steps := []stackStep{
		{label: "a", cfg: types.ManipuConfig{User: "root"}},
		{label: "b", cfg: types.ManipuConfig{User: "nobody"}},
	}
	err := detectStackCollisions(steps)
	require.Error(t, err)
	require.Contains(t, err.Error(), "User")
}

// TestDetectStackCollisions_SingleStepSetsEverything: one manipulation setting
// every field is never a self-collision.
func TestDetectStackCollisions_SingleStepSetsEverything(t *testing.T) {
	steps := []stackStep{
		{label: "solo", cfg: types.ManipuConfig{
			Env:        map[string]string{"A": "1", "B": "2"},
			Workdir:    "/w",
			User:       "u",
			Entrypoint: cmd(types.ManipuCmdOpReplace, "/bin/x"),
			Cmd:        cmd(types.ManipuCmdOpAppend, "--flag"),
		}},
	}
	require.NoError(t, detectStackCollisions(steps))
}

// TestDetectStackCollisions_SameEntrypointIdempotent: identical Entrypoint set by
// two steps is allowed (same value, not a conflict).
func TestDetectStackCollisions_SameEntrypointIdempotent(t *testing.T) {
	steps := []stackStep{
		{label: "a", cfg: types.ManipuConfig{Entrypoint: cmd(types.ManipuCmdOpReplace, "/bin/x")}},
		{label: "b", cfg: types.ManipuConfig{Entrypoint: cmd(types.ManipuCmdOpReplace, "/bin/x")}},
	}
	require.NoError(t, detectStackCollisions(steps))
}
