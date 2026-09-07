package magazine

// Tests for latestPerPlatformForSlot: HAS_PUSHED is an append-only log, so a
// profile re-pushing different content to the same tag must not accumulate
// every historical same-platform manifest into the slot's computed index.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func mdWithPlatform(id int64, os, arch string) types.Manifest {
	return types.Manifest{
		ID:       id,
		Platform: types.Platform{OS: os, Arch: arch},
	}
}

func TestLatestPerPlatformForSlot(t *testing.T) {
	amd64Old := mdWithPlatform(1, "linux", "amd64")
	amd64New := mdWithPlatform(2, "linux", "amd64")
	arm64 := mdWithPlatform(3, "linux", "arm64")

	t.Run("slot re-push collapses to the latest same-platform manifest", func(t *testing.T) {
		// Two amd64 pushes (old + new), one arm64 push. ts covers both amd64
		// manifests and the arm64 manifest (all pushed via the slot).
		ts := map[int64]int64{1: 100, 2: 200, 3: 150}
		got := latestPerPlatformForSlot([]types.Manifest{amd64Old, arm64, amd64New}, ts)
		assert.Equal(t, []types.Manifest{arm64, amd64New}, got,
			"only the latest amd64 push must survive; arm64 is a distinct platform")
	})

	t.Run("union member loses to the slot's own push on the same platform", func(t *testing.T) {
		// arm64 was pushed by ANOTHER profile (not in this slot's ts) — it is
		// a union member. The slot itself pushed amd64 twice; the latest wins
		// and the same-platform union member would have lost anyway.
		unionArm64 := mdWithPlatform(9, "linux", "arm64")
		ts := map[int64]int64{1: 100, 2: 200}
		got := latestPerPlatformForSlot([]types.Manifest{amd64Old, unionArm64, amd64New}, ts)
		assert.Equal(t, []types.Manifest{unionArm64, amd64New}, got)
	})

	t.Run("union member is dropped when the slot pushed that platform", func(t *testing.T) {
		// The slot's latest amd64 push (id 2) must beat an older same-platform
		// union member (id 9) from another profile.
		unionAmd64Old := mdWithPlatform(9, "linux", "amd64")
		ts := map[int64]int64{2: 200}
		got := latestPerPlatformForSlot([]types.Manifest{unionAmd64Old, amd64New}, ts)
		assert.Equal(t, []types.Manifest{amd64New}, got)
	})

	t.Run("timestamp tie breaks by higher node ID", func(t *testing.T) {
		ts := map[int64]int64{1: 200, 2: 200}
		got := latestPerPlatformForSlot([]types.Manifest{amd64Old, amd64New}, ts)
		assert.Equal(t, []types.Manifest{amd64New}, got)
	})

	t.Run("empty push log disables the dedupe", func(t *testing.T) {
		in := []types.Manifest{amd64Old, amd64New}
		got := latestPerPlatformForSlot(in, map[int64]int64{})
		assert.Equal(t, in, got, "no push log (fixtures) must leave candidates untouched")
	})

	t.Run("single candidate short-circuits", func(t *testing.T) {
		in := []types.Manifest{amd64Old}
		got := latestPerPlatformForSlot(in, map[int64]int64{1: 100})
		assert.Equal(t, in, got)
	})

	t.Run("input order of survivors is preserved", func(t *testing.T) {
		ts := map[int64]int64{1: 100, 2: 200}
		got := latestPerPlatformForSlot([]types.Manifest{amd64New, arm64, amd64Old}, ts)
		assert.Equal(t, []types.Manifest{amd64New, arm64}, got)
	})
}
