package neo

// Tests for SlotLatestPushTimestamps / RepoTagLatestPushTimestamps: the
// HAS_PUSHED log is append-only, so the helpers must return the LATEST
// timestamp per manifest — the input to latest-push-wins dedupe at compute
// time.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlotLatestPushTimestamps(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/tslog", "v1", "generic")
	require.NoError(t, err)

	md1 := createTestManifest(t, "sha256:1111111111111111111111111111111111111111111111111111111111111111")
	md2 := createTestManifest(t, "sha256:2222222222222222222222222222222222222222222222222222222222222222")

	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md1, "alice"))
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md2, "alice"))
	// Re-push md1: a second, later HAS_PUSHED edge. The map must carry the
	// LATEST timestamp for md1, not the first.
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md1, "alice"))

	// Reference maxima straight from the graph.
	q := `MATCH (s:Slot {repo: "qnib/tslog", tag: "v1", profile: "generic"})-[hp:HAS_PUSHED]->(m:Manifest)
	      RETURN id(m) AS mid, max(hp.timestamp) AS maxTs, min(hp.timestamp) AS minTs`
	rows, err := db.Query(q)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	ref := map[int64]struct{ max, min int64 }{}
	for _, r := range rows {
		mid, err := r.GetInt64("mid")
		require.NoError(t, err)
		maxTs, err := r.GetInt64("maxTs")
		require.NoError(t, err)
		minTs, err := r.GetInt64("minTs")
		require.NoError(t, err)
		ref[mid] = struct{ max, min int64 }{maxTs, minTs}
	}

	ts, err := db.SlotLatestPushTimestamps(ctx, "qnib/tslog", "v1", "generic")
	require.NoError(t, err)
	require.Len(t, ts, 2, "both pushed manifests must be in the map")
	for mid, want := range ref {
		assert.Equalf(t, want.max, ts[mid], "manifest %d must carry its LATEST push timestamp", mid)
		// NB: no strict-greater assertion against min — neo4j timestamp() has
		// millisecond resolution, so rapid re-pushes can share one.
	}

	// Unknown slot -> empty map, no error.
	empty, err := db.SlotLatestPushTimestamps(ctx, "qnib/tslog", "v1", "no-such-profile")
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestRepoTagLatestPushTimestamps(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slotA, err := db.SlotGetCreate(ctx, "qnib/tslog2", "v1", "profileA")
	require.NoError(t, err)
	slotB, err := db.SlotGetCreate(ctx, "qnib/tslog2", "v1", "profileB")
	require.NoError(t, err)

	md1 := createTestManifest(t, "sha256:3333333333333333333333333333333333333333333333333333333333333333")
	md2 := createTestManifest(t, "sha256:4444444444444444444444444444444444444444444444444444444444444444")

	require.NoError(t, db.SlotAppendPushedManifest(ctx, slotA.ID, md1, "alice"))
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slotB.ID, md2, "bob"))

	ts, err := db.RepoTagLatestPushTimestamps(ctx, "qnib/tslog2", "v1")
	require.NoError(t, err)
	require.Len(t, ts, 2, "pushes from both profiles' slots must be unioned")
	assert.NotZero(t, ts[md1])
	assert.NotZero(t, ts[md2])

	// Slot-scoped lookup for profileA must NOT contain profileB's manifest.
	tsA, err := db.SlotLatestPushTimestamps(ctx, "qnib/tslog2", "v1", "profileA")
	require.NoError(t, err)
	assert.Contains(t, tsA, md1)
	assert.NotContains(t, tsA, md2)
}
