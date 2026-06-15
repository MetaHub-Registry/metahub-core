package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseLayerSlice covers every accepted + rejected input shape.
// Uses table-driven cases for readability; each entry names the
// contract being asserted so regressions pinpoint the surprising
// case.
func TestParseLayerSlice(t *testing.T) {
	n := 5 // common input length used for resolution checks

	cases := []struct {
		name        string
		input       string
		wantIndices []int // indices when applied against len=n; empty = {} match
		wantErrSub  string
	}{
		{"empty string is identity", "", []int{0, 1, 2, 3, 4}, ""},
		{"explicit [:]", "[:]", []int{0, 1, 2, 3, 4}, ""},
		{"bare : is identity", ":", []int{0, 1, 2, 3, 4}, ""},
		{"single 0", "[0]", []int{0}, ""},
		{"single last via negative", "[-1]", []int{4}, ""},
		{"single negative mid", "[-2]", []int{3}, ""},
		{"skip first", "[1:]", []int{1, 2, 3, 4}, ""},
		{"head only", "[:2]", []int{0, 1}, ""},
		{"closed range", "[1:3]", []int{1, 2}, ""},
		{"step 2", "[::2]", []int{0, 2, 4}, ""},
		{"closed range stepped", "[1:5:2]", []int{1, 3}, ""},
		{"no brackets ok", "1:", []int{1, 2, 3, 4}, ""},
		{"whitespace tolerated", "[ 1 : 3 ]", []int{1, 2}, ""},
		{"negative end truncates in range", "[:-1]", []int{0, 1, 2, 3}, ""},
		{"negative start out of range errors", "[-100:]", nil, "out of range"},
		{"open end past length errors", "[2:999]", nil, "out of range"},
		{"start at length is empty errors", "[5:]", nil, "empty"},
		{"start past length errors", "[6:]", nil, "out of range"},
		{"end before start is empty errors", "[3:1]", nil, "empty"},
		{"explicitly empty range errors", "[0:0]", nil, "empty"},
		{"single oob positive", "[5]", nil, "out of range"},
		{"single oob negative", "[-6]", nil, "out of range"},
		{"negative step rejected", "[::-1]", nil, "step must be positive"},
		{"zero step rejected", "[::0]", nil, "step must be positive"},
		{"too many colons rejected", "[1:2:3:4]", nil, "not a recognised"},
		{"non-integer start rejected", "[a:]", nil, "not a recognised"},
		{"unbalanced brackets rejected", "[1:2", nil, "unbalanced"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ls, err := ParseLayerSlice(tc.input)
			if tc.wantErrSub != "" && err == nil {
				// The parser might accept the syntax and only fail at
				// resolve-time (single oob goes down that path).
				_, err = ls.ApplyIndices(n)
			}
			if tc.wantErrSub != "" {
				require.Error(t, err, "expected error for %q", tc.input)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.wantErrSub))
				return
			}
			require.NoError(t, err)
			got, err := ls.ApplyIndices(n)
			require.NoError(t, err)
			assert.Equal(t, tc.wantIndices, got)
		})
	}
}

// TestLayerSlice_OutOfRangeOnShortSource checks strict semantics: an
// open-ended slice whose start bound exceeds the source length fails
// (rather than silently resolving to empty as Python would), and a
// single index past the end fails the same way. Both must surface an
// "out of range" error so a misconfigured select fails the pull.
func TestLayerSlice_OutOfRangeOnShortSource(t *testing.T) {
	ls, err := ParseLayerSlice("[5:]")
	require.NoError(t, err)
	_, err = ls.ApplyIndices(3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")

	ls, err = ParseLayerSlice("[5]")
	require.NoError(t, err)
	_, err = ls.ApplyIndices(3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// TestLayerSlice_IsIdentity lets callers short-circuit the apply path
// entirely; guard the predicate here so that shortcut stays safe.
func TestLayerSlice_IsIdentity(t *testing.T) {
	cases := map[string]bool{
		"":    true,
		"[:]": true,
		":":   true,
		"[0]": false,
		"[1:]": false,
	}
	for in, want := range cases {
		ls, err := ParseLayerSlice(in)
		require.NoError(t, err, "parse %q", in)
		assert.Equal(t, want, ls.IsIdentity(), "IsIdentity(%q)", in)
	}
}

// TestApplyToLayers_Interface checks the LayerLike variant — callers
// that want to hand in a concrete slice without indexing.
func TestApplyToLayers_Interface(t *testing.T) {
	in := []LayerLike{"a", "b", "c", "d"}
	ls, err := ParseLayerSlice("[1:3]")
	require.NoError(t, err)
	got, err := ls.ApplyToLayers(in)
	require.NoError(t, err)
	assert.Equal(t, []LayerLike{"b", "c"}, got)
}
