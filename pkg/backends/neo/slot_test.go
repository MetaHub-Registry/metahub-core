package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// slotTestSetup is a small helper that bootstraps an empty graph for a Slot
// test. It returns a cleanup function that tears the graph down again.
func slotTestSetup(ctx context.Context, t *testing.T) func() {
	t.Helper()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatalf("slotTestSetup: %v", err)
	}
	return func() {
		db.TeardownTest(bs, br)
	}
}

// countEdgesFrom returns the number of outgoing edges of the given type from
// the :Slot with id slotID. Used to assert that MERGE-based idempotency does
// not create duplicate join edges on repeated calls.
func countEdgesFrom(t *testing.T, slotID int64, edgeType string) int {
	t.Helper()
	q := fmt.Sprintf(`MATCH (s:Slot)-[r:%s]->() WHERE id(s) = %d RETURN count(r) AS c`, edgeType, slotID)
	res, found, err := db.QueryOne(q)
	if err != nil {
		t.Fatalf("countEdgesFrom: %v", err)
	}
	if !found {
		return 0
	}
	c, err := res.GetInt64("c")
	if err != nil {
		t.Fatalf("countEdgesFrom: decoding count: %v", err)
	}
	return int(c)
}

// countStringIdentityNodes returns the number of :Repo, :Tag, and :Profile
// nodes in the graph, for asserting that MERGE is idempotent.
func countStringIdentityNodes(t *testing.T, label string) int {
	t.Helper()
	q := fmt.Sprintf(`MATCH (n:%s) RETURN count(n) AS c`, label)
	res, found, err := db.QueryOne(q)
	if err != nil {
		t.Fatalf("countStringIdentityNodes(%s): %v", label, err)
	}
	if !found {
		return 0
	}
	c, err := res.GetInt64("c")
	if err != nil {
		t.Fatalf("countStringIdentityNodes(%s): decoding count: %v", label, err)
	}
	return int(c)
}

// createTestIndex creates an :Index node with the given digest and returns
// its internal ID. Used by POINTS_TO tests to produce a linkable target.
func createTestIndex(t *testing.T, dig string) int64 {
	t.Helper()
	id, err := db.CreateManifestList(digest.Digest(dig), "application/vnd.oci.image.index.v1+json", 100)
	if err != nil {
		t.Fatalf("createTestIndex: %v", err)
	}
	return id
}

func TestSlotGetCreate_Idempotent(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	s1, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	if err != nil {
		t.Fatalf("first SlotGetCreate: %v", err)
	}
	assert.GreaterOrEqual(t, s1.ID, int64(0), "Slot ID must be a valid neo4j ID (>= 0)")
	assert.Equal(t, "qnib/alpine", s1.Repo)
	assert.Equal(t, "latest", s1.Tag)
	assert.Equal(t, "generic", s1.Profile)

	s2, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	if err != nil {
		t.Fatalf("second SlotGetCreate: %v", err)
	}
	assert.Equal(t, s1.ID, s2.ID, "same triple must return same Slot ID")

	// Verify no duplicate join edges
	assert.Equal(t, 1, countEdgesFrom(t, s1.ID, "OF_REPO"))
	assert.Equal(t, 1, countEdgesFrom(t, s1.ID, "OF_TAG"))
	assert.Equal(t, 1, countEdgesFrom(t, s1.ID, "OF_PROFILE"))
}

func TestSlotGetCreate_DifferentTriplesDistinct(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	s1, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	if err != nil {
		t.Fatalf("first SlotGetCreate: %v", err)
	}
	// Same repo+tag, different profile -> distinct Slot
	s2, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "amd64")
	if err != nil {
		t.Fatalf("second SlotGetCreate: %v", err)
	}
	// Same repo+profile, different tag -> distinct Slot
	s3, err := db.SlotGetCreate(ctx, "qnib/alpine", "v1", "generic")
	if err != nil {
		t.Fatalf("third SlotGetCreate: %v", err)
	}
	assert.NotEqual(t, s1.ID, s2.ID)
	assert.NotEqual(t, s1.ID, s3.ID)
	assert.NotEqual(t, s2.ID, s3.ID)
}

func TestSlotGetCreate_MergesStringNodes(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	// First call creates fresh Repo, Tag, Profile nodes.
	_, err := db.SlotGetCreate(ctx, "brand/new-repo", "v1", "generic")
	if err != nil {
		t.Fatalf("first SlotGetCreate: %v", err)
	}
	assert.Equal(t, 1, countStringIdentityNodes(t, "Repo"))
	assert.Equal(t, 1, countStringIdentityNodes(t, "Tag"))
	// Profile may include an auto-seeded "generic" from some other bootstrap,
	// but with bs.SetEmptyAll() the graph starts empty, so it should be 1.
	assert.Equal(t, 1, countStringIdentityNodes(t, "Profile"))

	// Second call with the same repo + new tag -> Repo reused, new Tag, Profile reused.
	_, err = db.SlotGetCreate(ctx, "brand/new-repo", "v2", "generic")
	if err != nil {
		t.Fatalf("second SlotGetCreate: %v", err)
	}
	assert.Equal(t, 1, countStringIdentityNodes(t, "Repo"), "Repo node should not be duplicated")
	assert.Equal(t, 2, countStringIdentityNodes(t, "Tag"))
	assert.Equal(t, 1, countStringIdentityNodes(t, "Profile"))
}

func TestSlotGetAllForRepoTag_UnionsProfiles(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	// Two Slots for same (repo, tag) with different profiles.
	_, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	if err != nil {
		t.Fatalf("first SlotGetCreate: %v", err)
	}
	_, err = db.SlotGetCreate(ctx, "qnib/alpine", "latest", "amd64")
	if err != nil {
		t.Fatalf("second SlotGetCreate: %v", err)
	}
	// A third Slot with a different tag (should NOT be returned).
	_, err = db.SlotGetCreate(ctx, "qnib/alpine", "v1", "generic")
	if err != nil {
		t.Fatalf("third SlotGetCreate: %v", err)
	}

	slots, err := db.SlotGetAllForRepoTag(ctx, "qnib/alpine", "latest")
	if err != nil {
		t.Fatalf("SlotGetAllForRepoTag: %v", err)
	}
	assert.Len(t, slots, 2, "should return both profiles for (qnib/alpine, latest)")
	profiles := map[string]bool{}
	for _, s := range slots {
		profiles[s.Profile] = true
	}
	assert.True(t, profiles["generic"])
	assert.True(t, profiles["amd64"])
}

func TestSlotLinkPointsTo_ArchivesPrevious(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	if err != nil {
		t.Fatalf("SlotGetCreate: %v", err)
	}
	idx1 := createTestIndex(t, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	idx2 := createTestIndex(t, "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	// First link: no previous target to archive.
	if err := db.SlotLinkPointsTo(ctx, slot.ID, idx1); err != nil {
		t.Fatalf("first SlotLinkPointsTo: %v", err)
	}
	assert.Equal(t, 1, countEdgesFrom(t, slot.ID, "POINTS_TO"))
	assert.Equal(t, 0, countEdgesFrom(t, slot.ID, "HAS_HISTORY"))

	// Second link: idx1 should move to HAS_HISTORY.
	if err := db.SlotLinkPointsTo(ctx, slot.ID, idx2); err != nil {
		t.Fatalf("second SlotLinkPointsTo: %v", err)
	}
	assert.Equal(t, 1, countEdgesFrom(t, slot.ID, "POINTS_TO"))
	assert.Equal(t, 1, countEdgesFrom(t, slot.ID, "HAS_HISTORY"))

	// Verify POINTS_TO now targets idx2.
	q := fmt.Sprintf(`MATCH (s:Slot)-[:POINTS_TO]->(i:Index) WHERE id(s) = %d RETURN id(i) AS idxID`, slot.ID)
	res, found, err := db.QueryOne(q)
	if err != nil || !found {
		t.Fatalf("POINTS_TO read: err=%v found=%v", err, found)
	}
	current, _ := res.GetInt64("idxID")
	assert.Equal(t, idx2, current)
}

func TestSlotLinkPointsTo_IdempotentOnSameIndex(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	if err != nil {
		t.Fatalf("SlotGetCreate: %v", err)
	}
	idx := createTestIndex(t, "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")

	// First link establishes POINTS_TO.
	if err := db.SlotLinkPointsTo(ctx, slot.ID, idx); err != nil {
		t.Fatalf("first SlotLinkPointsTo: %v", err)
	}
	assert.Equal(t, 1, countEdgesFrom(t, slot.ID, "POINTS_TO"))
	assert.Equal(t, 0, countEdgesFrom(t, slot.ID, "HAS_HISTORY"))

	// Re-linking the same Index should be a no-op: no new HAS_HISTORY edge.
	if err := db.SlotLinkPointsTo(ctx, slot.ID, idx); err != nil {
		t.Fatalf("second SlotLinkPointsTo: %v", err)
	}
	assert.Equal(t, 1, countEdgesFrom(t, slot.ID, "POINTS_TO"))
	assert.Equal(t, 0, countEdgesFrom(t, slot.ID, "HAS_HISTORY"), "re-linking the same Index should not create a HAS_HISTORY entry")
}

func TestSlotAppendPushedIndex_AppendsOnRepush(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	if err != nil {
		t.Fatalf("SlotGetCreate: %v", err)
	}
	idx := createTestIndex(t, "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")

	// Push the same index twice.
	if err := db.SlotAppendPushedIndex(ctx, slot.ID, idx); err != nil {
		t.Fatalf("first SlotAppendPushedIndex: %v", err)
	}
	if err := db.SlotAppendPushedIndex(ctx, slot.ID, idx); err != nil {
		t.Fatalf("second SlotAppendPushedIndex: %v", err)
	}
	// HAS_PUSHED_INDEX should be append-only -> 2 edges.
	assert.Equal(t, 2, countEdgesFrom(t, slot.ID, "HAS_PUSHED_INDEX"))
}

func TestSlotGetByTriple_NotFoundReturnsFalse(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	_, found, err := db.SlotGetByTriple(ctx, "never/created", "missing", "none")
	if err != nil {
		t.Fatalf("SlotGetByTriple: %v", err)
	}
	assert.False(t, found)
}

func TestSlotGetByTriple_FoundAfterCreate(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	created, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	if err != nil {
		t.Fatalf("SlotGetCreate: %v", err)
	}
	got, found, err := db.SlotGetByTriple(ctx, "qnib/alpine", "latest", "generic")
	if err != nil {
		t.Fatalf("SlotGetByTriple: %v", err)
	}
	assert.True(t, found)
	assert.Equal(t, created.ID, got.ID)
}

func TestSlotGetAllForRepo_ReturnsAllPairs(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	// Seed 3 Slots for the same repo across 3 different (tag, profile) pairs.
	_, _ = db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	_, _ = db.SlotGetCreate(ctx, "qnib/alpine", "v1", "generic")
	_, _ = db.SlotGetCreate(ctx, "qnib/alpine", "latest", "amd64")
	// A fourth Slot for a different repo (should NOT be returned).
	_, _ = db.SlotGetCreate(ctx, "other/repo", "latest", "generic")

	slots, err := db.SlotGetAllForRepo(ctx, "qnib/alpine")
	if err != nil {
		t.Fatalf("SlotGetAllForRepo: %v", err)
	}
	assert.Len(t, slots, 3)
	for _, s := range slots {
		assert.Equal(t, "qnib/alpine", s.Repo)
	}
}

func TestSlotAppendPushedManifest_AppendsMultiple(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")
	if err != nil {
		t.Fatalf("SlotGetCreate: %v", err)
	}
	// Create two Manifest nodes to push
	md1ID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)
	md2ID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:2222222222222222222222222222222222222222222222222222222222222222",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      200,
	})
	require.NoError(t, err)

	// Append both
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md1ID, ""))
	require.NoError(t, db.SlotAppendPushedManifest(ctx, slot.ID, md2ID, ""))
	assert.Equal(t, 2, countEdgesFrom(t, slot.ID, "HAS_PUSHED"))

	// Retrieve via ManifestGetAllForSlot
	mds, err := db.ManifestGetAllForSlot(ctx, slot.ID)
	require.NoError(t, err)
	assert.Len(t, mds, 2, "ManifestGetAllForSlot should return both pushed manifests")

	digests := map[string]bool{}
	for _, md := range mds {
		digests[md.Digest] = true
	}
	assert.True(t, digests["sha256:1111111111111111111111111111111111111111111111111111111111111111"])
	assert.True(t, digests["sha256:2222222222222222222222222222222222222222222222222222222222222222"])
}

func TestManifestGetAllForSlot_EmptyReturnsNil(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slot, err := db.SlotGetCreate(ctx, "qnib/empty", "latest", "generic")
	require.NoError(t, err)

	mds, err := db.ManifestGetAllForSlot(ctx, slot.ID)
	require.NoError(t, err)
	assert.Empty(t, mds, "Slot with no HAS_PUSHED edges should return empty slice")
}

func TestSlotGetAllForRepoTag_NoSlots(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slots, err := db.SlotGetAllForRepoTag(ctx, "nonexistent/repo", "missing")
	require.NoError(t, err)
	assert.Empty(t, slots, "should return empty for nonexistent repo/tag")
}

func TestSlotGetCreate_EmptyComponents(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	_, err := db.SlotGetCreate(ctx, "", "latest", "generic")
	assert.Error(t, err, "empty repo should be rejected")
	_, err = db.SlotGetCreate(ctx, "qnib/alpine", "", "generic")
	assert.Error(t, err, "empty tag should be rejected")

	// The empty profile is the generic pull; it normalises to the same slot
	// sentinel the generic compute uses rather than being rejected.
	slot, err := db.SlotGetCreate(ctx, "qnib/alpine", "latest", "")
	require.NoError(t, err, "empty profile should map to the generic slot")
	assert.Equal(t, "generic", slot.Profile, "empty profile maps to the generic sentinel")
}
