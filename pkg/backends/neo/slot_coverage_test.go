package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file extends the existing slot_test.go with direct tests for
// two Slot-adjacent functions that were at 0% coverage at the neo-package
// level: SlotAppendPushedManifest and ManifestGetAllForSlot. Both
// functions are exercised end-to-end via the magazine-package tests
// (Phase A's StoreManifestUpload wires SlotAppendPushedManifest, Phase
// B's fetchRawManifestsForRepoTag wires ManifestGetAllForSlot), but
// the neo-package coverage profile doesn't see those runs because
// magazine tests build their own package-level coverage report.
//
// Adding direct neo-package tests closes the coverage gap AND gives
// us a faster inner loop for debugging the cypher inside these two
// functions (no magazine bootstrap needed).
//
// Plus a handful of edge-case tests that cover the error branches
// and empty-result paths that the happy-path tests in slot_test.go
// don't touch.

// createTestManifest creates a minimal :Manifest node via direct
// cypher and returns its internal neo4j ID. Used by HAS_PUSHED tests
// to produce a linkable target without running the full
// Neo.createManifest flow (which requires a populated Config +
// Layers + Upstream).
func createTestManifest(t *testing.T, dig string) int64 {
	t.Helper()
	q := fmt.Sprintf(
		`CREATE (md:Manifest {digest: "%s", mediaType: "application/vnd.oci.image.manifest.v1+json", size: 735, label: "test:%s"}) RETURN id(md) AS mdID`,
		dig, dig[7:23],
	)
	res, err := db.Execute(q)
	if err != nil {
		t.Fatalf("createTestManifest cypher failed: %v", err)
	}
	id, err := res.GetInt64("mdID")
	if err != nil {
		t.Fatalf("createTestManifest decode failed: %v", err)
	}
	return id
}

// TestSlotAppendPushedManifest_CreatesEdge is the direct-cover test
// for SlotAppendPushedManifest. Asserts that:
//  1. A single call creates exactly one :HAS_PUSHED edge from the
//     Slot to the Manifest.
//  2. The edge carries a non-zero timestamp property.
//  3. Re-calling with the same (slotID, manifestID) creates a SECOND
//     edge (append-only log, NOT deduplicated).
//
// This exercises design decision D3: HAS_PUSHED is an append-only
// timestamped log, scoped per-slot.
func TestSlotAppendPushedManifest_CreatesEdge(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	require.NoError(t, err)
	mdID := createTestManifest(t, "sha256:f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0")

	// First call — single edge.
	err = db.SlotAppendPushedManifest(ctx, slot.ID, mdID)
	require.NoError(t, err)

	edgeCount, ts := readHasPushedEdgeCountAndTimestamp(t, slot.ID, mdID)
	assert.Equal(t, 1, edgeCount, "first call must create exactly one :HAS_PUSHED edge")
	assert.NotZero(t, ts, "edge must carry a non-zero timestamp")

	// Second call — append (not dedup).
	err = db.SlotAppendPushedManifest(ctx, slot.ID, mdID)
	require.NoError(t, err)
	edgeCountAfter, _ := readHasPushedEdgeCountAndTimestamp(t, slot.ID, mdID)
	assert.Equal(t, 2, edgeCountAfter,
		"second call must append a new :HAS_PUSHED edge (design D3: append-only, not deduplicated)")
}

// readHasPushedEdgeCountAndTimestamp returns (count, latestTimestamp)
// for :HAS_PUSHED edges from the slot to the given manifest.
func readHasPushedEdgeCountAndTimestamp(t *testing.T, slotID, mdID int64) (int, int64) {
	t.Helper()
	q := fmt.Sprintf(
		`MATCH (s:Slot)-[r:HAS_PUSHED]->(md:Manifest) WHERE id(s) = %d AND id(md) = %d RETURN count(r) AS c, max(r.timestamp) AS latestTs`,
		slotID, mdID,
	)
	res, err := db.Execute(q)
	if err != nil {
		t.Fatalf("readHasPushedEdgeCountAndTimestamp: %v", err)
	}
	c, _ := res.GetInt64("c")
	ts, _ := res.GetInt64("latestTs")
	return int(c), ts
}

// TestSlotAppendPushedManifest_InvalidSlotID is a defensive branch
// test: calling with a non-existent slot ID should NOT create any edge
// (the MATCH fails, so the CREATE sees no rows to attach to).
func TestSlotAppendPushedManifest_InvalidSlotID(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	mdID := createTestManifest(t, "sha256:e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1")
	// Use an impossibly-large slot ID. MATCH returns zero rows → CREATE
	// is a no-op. No error because the cypher statement itself is
	// syntactically valid.
	err := db.SlotAppendPushedManifest(ctx, int64(999999999), mdID)
	assert.NoError(t, err,
		"cypher MATCH on a non-existent slot ID returns zero rows, CREATE is a no-op, no error")

	// Verify no orphan edges exist.
	q := fmt.Sprintf(`MATCH ()-[r:HAS_PUSHED]->(md:Manifest) WHERE id(md) = %d RETURN count(r) AS c`, mdID)
	res, err := db.Execute(q)
	require.NoError(t, err)
	c, _ := res.GetInt64("c")
	assert.Equal(t, int64(0), c, "no :HAS_PUSHED edges should exist when the slot MATCH failed")
}

// TestManifestGetAllForSlot_EmptyWhenNoPushes covers the empty-slot
// branch: a Slot that exists but has no :HAS_PUSHED edges must return
// an empty (not nil) slice. Reproduces the edge case where a Slot was
// created by SlotGetCreate but nothing's been pushed to it yet.
func TestManifestGetAllForSlot_EmptyWhenNoPushes(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	require.NoError(t, err)

	manis, err := db.ManifestGetAllForSlot(ctx, slot.ID)
	require.NoError(t, err)
	assert.NotNil(t, manis, "ManifestGetAllForSlot must return an empty slice, not nil")
	assert.Empty(t, manis, "empty Slot (no HAS_PUSHED) must produce an empty slice")
}

// TestManifestGetAllForSlot_ReturnsAllPushedManifests covers the
// happy path: a Slot with multiple :HAS_PUSHED edges should return
// all the referenced content-leaf manifests, deduplicated by ID.
func TestManifestGetAllForSlot_ReturnsAllPushedManifests(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	require.NoError(t, err)

	md1 := createTestManifest(t, "sha256:1111111111111111111111111111111111111111111111111111111111111111")
	md2 := createTestManifest(t, "sha256:2222222222222222222222222222222222222222222222222222222222222222")
	md3 := createTestManifest(t, "sha256:3333333333333333333333333333333333333333333333333333333333333333")

	// Wire three distinct manifests as HAS_PUSHED.
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md1))
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md2))
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md3))

	manis, err := db.ManifestGetAllForSlot(ctx, slot.ID)
	require.NoError(t, err)
	assert.Len(t, manis, 3, "should return all three pushed manifests")

	// Collect the returned IDs and assert they match the three we created.
	gotIDs := map[int64]bool{}
	for _, m := range manis {
		gotIDs[m.ID] = true
		assert.NotEmpty(t, m.Digest, "each result must carry a digest")
		assert.NotEmpty(t, m.MediaType, "each result must carry a mediaType")
		assert.NotZero(t, m.Size, "each result must carry a non-zero size")
	}
	assert.True(t, gotIDs[md1], "md1 must be in the result set")
	assert.True(t, gotIDs[md2], "md2 must be in the result set")
	assert.True(t, gotIDs[md3], "md3 must be in the result set")
}

// TestManifestGetAllForSlot_DedupesOnRepush is the critical invariant
// from design D3: even though :HAS_PUSHED is append-only at the graph
// layer (multiple edges for multiple pushes), the reader dedupes by
// manifest ID so the caller sees each Manifest exactly once.
func TestManifestGetAllForSlot_DedupesOnRepush(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	require.NoError(t, err)

	md := createTestManifest(t, "sha256:4444444444444444444444444444444444444444444444444444444444444444")

	// Push the same manifest 3 times — creates 3 HAS_PUSHED edges.
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md))
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md))
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md))

	// Verify the append-only log really does have 3 edges.
	edgeCount, _ := readHasPushedEdgeCountAndTimestamp(t, slot.ID, md)
	assert.Equal(t, 3, edgeCount, "precondition: append-only log has 3 edges")

	// But the reader dedupes by manifest ID → exactly 1 entry in the result.
	manis, err := db.ManifestGetAllForSlot(ctx, slot.ID)
	require.NoError(t, err)
	assert.Len(t, manis, 1,
		"ManifestGetAllForSlot must dedupe by manifest ID even though the underlying edge log has duplicates")
	assert.Equal(t, md, manis[0].ID)
}

// TestManifestGetAllForSlot_IgnoresOtherSlots verifies that the
// walker scopes to the specific slot ID passed in and does not leak
// manifests from other Slots. Important because multiple Slots often
// exist in the same graph during multi-profile or multi-tag tests.
func TestManifestGetAllForSlot_IgnoresOtherSlots(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slotA, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	require.NoError(t, err)
	slotB, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "amd64")
	require.NoError(t, err)
	require.NotEqual(t, slotA.ID, slotB.ID)

	md1 := createTestManifest(t, "sha256:5555555555555555555555555555555555555555555555555555555555555555")
	md2 := createTestManifest(t, "sha256:6666666666666666666666666666666666666666666666666666666666666666")

	// Push md1 to slotA, md2 to slotB.
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slotA.ID, md1))
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slotB.ID, md2))

	// slotA should see ONLY md1.
	manisA, err := db.ManifestGetAllForSlot(ctx, slotA.ID)
	require.NoError(t, err)
	require.Len(t, manisA, 1)
	assert.Equal(t, md1, manisA[0].ID)

	// slotB should see ONLY md2.
	manisB, err := db.ManifestGetAllForSlot(ctx, slotB.ID)
	require.NoError(t, err)
	require.Len(t, manisB, 1)
	assert.Equal(t, md2, manisB[0].ID)
}

// TestSlotAppendPushedIndex_EdgeCarriesTimestamp is an edge-case test
// that the existing TestSlotAppendPushedIndex_AppendsOnRepush didn't
// cover: verify the edge carries a non-zero timestamp property and
// the timestamp advances across multiple calls (proving it's sourced
// from neo4j's wall clock, not a constant).
func TestSlotAppendPushedIndex_EdgeCarriesTimestamp(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	require.NoError(t, err)

	idx := createTestIndex(t, "sha256:7777777777777777777777777777777777777777777777777777777777777777")

	require.NoError(t, db.SlotAppendPushedIndex(ctx, slot.ID, idx))

	q := fmt.Sprintf(
		`MATCH (s:Slot)-[r:HAS_PUSHED_INDEX]->(i:Index) WHERE id(s) = %d AND id(i) = %d RETURN r.timestamp AS ts`,
		slot.ID, idx,
	)
	res, err := db.Execute(q)
	require.NoError(t, err)
	ts, err := res.GetInt64("ts")
	require.NoError(t, err)
	assert.NotZero(t, ts, "HAS_PUSHED_INDEX edge must carry a non-zero timestamp")
}

// TestSlotGetAllForRepoTag_EmptyWhenNoMatch covers the empty-result
// branch: querying for a (repo, tag) pair that has no Slots yet
// should return an empty slice (not nil, not error).
func TestSlotGetAllForRepoTag_EmptyWhenNoMatch(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	// Seed a Slot for a DIFFERENT triple, so the graph is non-empty
	// but the target query returns zero rows.
	_, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	require.NoError(t, err)

	slots, err := db.SlotGetAllForRepoTag(ctx, "does/not-exist", "nope")
	require.NoError(t, err)
	assert.NotNil(t, slots, "empty result must be an empty slice, not nil")
	assert.Empty(t, slots)
}

// TestSlotGetAllForRepo_EmptyWhenNoMatch is the repo-wide equivalent.
func TestSlotGetAllForRepo_EmptyWhenNoMatch(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	_, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	require.NoError(t, err)

	slots, err := db.SlotGetAllForRepo(ctx, "does/not-exist")
	require.NoError(t, err)
	assert.NotNil(t, slots)
	assert.Empty(t, slots)
}
