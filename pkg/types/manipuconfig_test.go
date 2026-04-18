package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManipuConfig_IsZero(t *testing.T) {
	// Empty config is zero.
	assert.True(t, ManipuConfig{}.IsZero())
	// Non-empty fields un-zero.
	assert.False(t, ManipuConfig{Workdir: "/x"}.IsZero())
	assert.False(t, ManipuConfig{User: "1000"}.IsZero())
	assert.False(t, ManipuConfig{Env: map[string]string{"a": "b"}}.IsZero())
	assert.False(t, ManipuConfig{Volumes: []string{"/v"}}.IsZero())
	assert.False(t, ManipuConfig{Layers: []ManipuLayer{{}}}.IsZero())
	assert.False(t, ManipuConfig{Entrypoint: &ManipuCommand{Op: "replace", Value: []string{"x"}}}.IsZero())
	assert.False(t, ManipuConfig{Cmd: &ManipuCommand{Op: "append", Value: []string{"y"}}}.IsZero())
	// Zero-valued Entrypoint/Cmd pointers count as zero.
	assert.True(t, ManipuConfig{Entrypoint: &ManipuCommand{}}.IsZero())
	assert.True(t, ManipuConfig{Cmd: &ManipuCommand{}}.IsZero())
}

func TestManipuCommand_IsValid(t *testing.T) {
	assert.True(t, ManipuCommand{Op: "replace"}.IsValid())
	assert.True(t, ManipuCommand{Op: "prepend"}.IsValid())
	assert.True(t, ManipuCommand{Op: "append"}.IsValid())
	assert.False(t, ManipuCommand{Op: ""}.IsValid())
	assert.False(t, ManipuCommand{Op: "overwrite"}.IsValid())
	// IsZero
	assert.True(t, ManipuCommand{}.IsZero())
	assert.False(t, ManipuCommand{Op: "replace", Value: []string{"x"}}.IsZero())
	// Op-only with no value is not zero (we still want to persist).
	assert.False(t, ManipuCommand{Op: "replace"}.IsZero())
	// Value-only is also non-zero.
	assert.False(t, ManipuCommand{Value: []string{"x"}}.IsZero())
}

func TestManipulation_FlattenLayers(t *testing.T) {
	mp := Manipulation{
		Default: ManipuConfig{Layers: []ManipuLayer{{Label: "a"}, {Label: "b"}}},
		Variants: []ManipuVariant{
			{Rule: "r1", Config: ManipuConfig{Layers: []ManipuLayer{{Label: "c"}}}},
			{Rule: "r2", Config: ManipuConfig{Layers: []ManipuLayer{{Label: "d"}, {Label: "e"}}}},
		},
	}
	flat := mp.FlattenLayers()
	labels := make([]string, len(flat))
	for i, l := range flat {
		labels[i] = l.Label
	}
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, labels)
}
