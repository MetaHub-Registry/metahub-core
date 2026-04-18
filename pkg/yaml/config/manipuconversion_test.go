package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManipuCommand_ToTypesNilPassthrough(t *testing.T) {
	var c *ManipuCommand
	assert.Nil(t, c.ToTypesManipuCommand())
	zero := &ManipuCommand{}
	assert.Nil(t, zero.ToTypesManipuCommand())
}

func TestManipuCommand_ToTypes(t *testing.T) {
	c := &ManipuCommand{Op: "append", Value: []string{"-v", "--loud"}}
	out := c.ToTypesManipuCommand()
	assert.NotNil(t, out)
	assert.Equal(t, "append", out.Op)
	assert.Equal(t, []string{"-v", "--loud"}, out.Value)
}

func TestManipuConfig_ToTypes(t *testing.T) {
	cfg := ManipuConfig{
		Env:        map[string]string{"A": "1"},
		Workdir:    "/w",
		User:       "u",
		Volumes:    []string{"/data"},
		Entrypoint: &ManipuCommand{Op: "replace", Value: []string{"x"}},
		Cmd:        &ManipuCommand{Op: "append", Value: []string{"y"}},
		Layers: []ManipuLayer{
			{Image: "i1", Rule: "r1"},
			{Image: "i2"},
		},
	}
	out := cfg.ToTypesManipuConfig()
	assert.Equal(t, "/w", out.Workdir)
	assert.Equal(t, "u", out.User)
	assert.Equal(t, []string{"/data"}, out.Volumes)
	assert.Equal(t, "1", out.Env["A"])
	assert.NotNil(t, out.Entrypoint)
	assert.Equal(t, "replace", out.Entrypoint.Op)
	assert.NotNil(t, out.Cmd)
	assert.Equal(t, "append", out.Cmd.Op)
	// Layers mirror order and rule but not image (resolved later).
	assert.Len(t, out.Layers, 2)
	assert.Equal(t, "r1", out.Layers[0].Rule)
}

func TestManipuConfig_ToTypesEmpty(t *testing.T) {
	out := ManipuConfig{}.ToTypesManipuConfig()
	assert.Empty(t, out.Env)
	assert.Empty(t, out.Volumes)
	assert.Empty(t, out.Layers)
	assert.Nil(t, out.Entrypoint)
	assert.Nil(t, out.Cmd)
}
