package magazine

import (
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"go.opentelemetry.io/otel/trace"
)

// nilSpan lets the helper's utils.Info/utils.ErrorHandler calls pass
// through safely — both guard `span != nil` so a literal untyped nil
// at the call site is fine, but the function signature is
// `trace.Span` (interface). A typed-nil interface value would be
// non-nil; we pass a noop span obtained via a nil provider instead.
func nilSpan() trace.Span {
	// The empty span returned by a nil provider's Tracer is
	// documented to be a no-op. Calling AddEvent etc. on it is
	// safe and produces nothing.
	return trace.SpanFromContext(nil) //nolint:staticcheck // intentional: no-op span for tests
}

// TestApplyManipuLayerSelect covers the three legs of the helper:
// identity (no select), valid slice, and invalid slice (out-of-range
// single index). The function is pure — no graph, no minio — so it
// runs without fixtures.
func TestApplyManipuLayerSelect(t *testing.T) {
	layers := []types.Layer{
		{Digest: "sha256:aaaa"},
		{Digest: "sha256:bbbb"},
		{Digest: "sha256:cccc"},
	}
	l := logrus.NewEntry(logrus.New())
	sp := nilSpan()

	t.Run("identity when select is empty", func(t *testing.T) {
		got, idx, err := applyManipuLayerSelect(l, sp, types.ManipuLayer{Label: "l"}, layers)
		require.NoError(t, err)
		assert.Nil(t, idx, "nil indices signal pass-through so DiffIDs stay untouched")
		assert.Equal(t, layers, got)
	})

	t.Run("identity when select is [:]", func(t *testing.T) {
		got, idx, err := applyManipuLayerSelect(l, sp, types.ManipuLayer{Label: "l", Select: "[:]"}, layers)
		require.NoError(t, err)
		assert.Nil(t, idx)
		assert.Equal(t, layers, got)
	})

	t.Run("skip first layer", func(t *testing.T) {
		got, idx, err := applyManipuLayerSelect(l, sp, types.ManipuLayer{Label: "l", Select: "[1:]"}, layers)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, idx)
		assert.Equal(t, layers[1:], got)
	})

	t.Run("last layer only via negative index", func(t *testing.T) {
		got, idx, err := applyManipuLayerSelect(l, sp, types.ManipuLayer{Label: "l", Select: "[-1]"}, layers)
		require.NoError(t, err)
		assert.Equal(t, []int{2}, idx)
		assert.Equal(t, layers[2:], got)
	})

	t.Run("out-of-range single index errors", func(t *testing.T) {
		_, _, err := applyManipuLayerSelect(l, sp, types.ManipuLayer{Label: "l", Select: "[5]"}, layers)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("malformed slice errors", func(t *testing.T) {
		_, _, err := applyManipuLayerSelect(l, sp, types.ManipuLayer{Label: "l", Select: "[::-1]"}, layers)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "step must be positive")
	})
}

// TestProjectDiffIDs guards the invariant that the diffID projection
// uses exactly the same indices as the layer projection. Mismatches
// produce a targeted error so the operator sees "image layers and
// config DiffIDs disagree" rather than a confusing downstream docker
// pull failure.
func TestProjectDiffIDs(t *testing.T) {
	in := []digest.Digest{
		digest.Digest("sha256:d1"),
		digest.Digest("sha256:d2"),
		digest.Digest("sha256:d3"),
	}

	t.Run("nil idx is identity", func(t *testing.T) {
		got, err := projectDiffIDs(in, nil)
		require.NoError(t, err)
		assert.Equal(t, in, got)
	})

	t.Run("subset projection", func(t *testing.T) {
		got, err := projectDiffIDs(in, []int{1, 2})
		require.NoError(t, err)
		assert.Equal(t, in[1:], got)
	})

	t.Run("single last", func(t *testing.T) {
		got, err := projectDiffIDs(in, []int{2})
		require.NoError(t, err)
		assert.Equal(t, in[2:], got)
	})

	t.Run("out-of-range errors with explanatory message", func(t *testing.T) {
		_, err := projectDiffIDs(in, []int{5})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
		assert.Contains(t, err.Error(), "diffIDs")
	})

	t.Run("shorter diffID list than layer list errors", func(t *testing.T) {
		shortIn := []digest.Digest{digest.Digest("sha256:d1")}
		_, err := projectDiffIDs(shortIn, []int{0, 1, 2})
		require.Error(t, err)
	})
}
