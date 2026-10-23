package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResultAddTypes covers every branch of Result.Add. The type switch is
// part of the public payload plumbing, so it should not rely on incidental
// coverage from the database tests.
func TestResultAddTypes(t *testing.T) {
	r := NewResult()

	require.NoError(t, r.Add("str", "value"))
	assert.Equal(t, "value", r.Strings["str"])

	require.NoError(t, r.Add("int64", int64(42)))
	assert.Equal(t, int64(42), r.Int64s["int64"])

	require.NoError(t, r.Add("list", []interface{}{"first"}))
	assert.Equal(t, "first", r.Strings["list"])

	// Empty list is a no-op.
	require.NoError(t, r.Add("empty", []interface{}{}))

	// nil is explicitly ignored.
	require.NoError(t, r.Add("nil", nil))

	err := r.Add("bad", 1.5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown type")
}
