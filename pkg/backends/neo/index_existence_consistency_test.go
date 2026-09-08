package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
)

// Regression tests for openspec align-manifest-existence-queries
// (OpenProject #42): the existence path (HEAD → ManifestListExists) and the
// serving path (GET → IndexGetByRequest) MUST agree for the same
// (repo, tag, profile) reference — no "exists but unservable" split — and
// superseded indexes (Slot HAS_HISTORY) must be hidden on BOTH paths.
//
// Decision (task 1.1, Option A): the Slot-[:POINTS_TO]->Index edge is wired
// synchronously at push, while the Profile/Repo/Tag :REF edges materialise
// asynchronously during the compute sweep. The Slot chain is therefore the
// read-after-write source of truth; IndexGetByRequest now mirrors
// ManifestListExists's Slot fallback (commit 19a0327), and
// ManifestListExists's primary query gains the HAS_HISTORY exclusion the
// serve path already had.

const (
	headGetRepo    = "qnib/llama-server"
	headGetTag     = "latest"
	headGetProfile = "frame"
)

// wireIndexREF connects an Index via the strict Profile/Repo/Tag :REF join
// (plus Repo-[:HAS_TAG]->Tag) so the settled post-compute state can be
// simulated.
func wireIndexREF(t *testing.T, idxID int64) {
	t.Helper()
	q := fmt.Sprintf(`MERGE (c:Repo {repo: "%s"}) MERGE (t:Tag {name: "%s"}) MERGE (p:Profile {label: "%s"}) MERGE (c)-[:HAS_TAG]->(t)`, headGetRepo, headGetTag, headGetProfile)
	q += fmt.Sprintf(` WITH c, t, p MATCH (i0:Index) WHERE id(i0) = %d MERGE (c)-[:REF]->(i0) MERGE (t)-[:REF]->(i0) MERGE (p)-[:REF]->(i0)`, idxID)
	if _, err := db.Execute(q); err != nil {
		t.Fatalf("wireIndexREF: %v", err)
	}
}

// TestHeadGetAgree_SlotWiredRefLessIndex pins the read-after-write window:
// a Slot wired via POINTS_TO but with NO REF edges (push landed, compute
// sweep still in flight — the exact state that produced HEAD 200 / GET 404
// during the gpu-workload outage) must be found by BOTH
// ManifestListExists (HEAD) and IndexGetByRequest (GET).
func TestHeadGetAgree_SlotWiredRefLessIndex(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	idx := createTestIndex(t, "sha256:1111111111111111111111111111111111111111111111111111111111111111")
	slot, err := db.SlotGetCreate(ctx, headGetRepo, headGetTag, headGetProfile)
	require.NoError(t, err)
	require.NoError(t, db.SlotLinkPointsTo(ctx, slot.ID, idx))

	up := upstream.UpstreamTriple{Repo: headGetRepo, Tag: headGetTag, Profile: headGetProfile}
	mtHead, foundHead, err := db.ManifestListExists(ctx, up)
	require.NoError(t, err)
	assert.True(t, foundHead, "HEAD must report the Slot-wired index as existing")
	assert.Equal(t, "application/vnd.oci.image.index.v1+json", mtHead)

	req := types.Request{Repo: headGetRepo, Tag: headGetTag, Profile: headGetProfile}
	mlGet, foundGet, err := db.IndexGetByRequest(ctx, req)
	require.NoError(t, err)
	assert.True(t, foundGet, "GET must serve the Slot-wired index (HEAD/GET agreement)")
	assert.Equal(t, idx, mlGet.ID, "GET must serve the Slot's current POINTS_TO target")
}

// TestHeadGetAgree_HasHistoryHiddenOnBothPaths pins the superseded-index
// requirement: an Index archived into the Slot's HAS_HISTORY chain (whose
// stale Tag/Repo/Profile REF edges linger until pruned) must be excluded by
// BOTH the existence path and the serving path; both paths resolve the Slot's
// CURRENT target instead.
func TestHeadGetAgree_HasHistoryHiddenOnBothPaths(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	idxOld := createTestIndex(t, "sha256:2222222222222222222222222222222222222222222222222222222222222222")
	slot, err := db.SlotGetCreate(ctx, headGetRepo, headGetTag, headGetProfile)
	require.NoError(t, err)
	require.NoError(t, db.SlotLinkPointsTo(ctx, slot.ID, idxOld))
	// Settled state: the old index was REF-wired by the previous compute sweep.
	wireIndexREF(t, idxOld)

	// Repush: POINTS_TO moves to idx2, idxOld moves to HAS_HISTORY
	// (its REF edges are stale until the prune runs).
	idxNew := createTestIndex(t, "sha256:3333333333333333333333333333333333333333333333333333333333333333")
	require.NoError(t, db.SlotLinkPointsTo(ctx, slot.ID, idxNew))

	up := upstream.UpstreamTriple{Repo: headGetRepo, Tag: headGetTag, Profile: headGetProfile}
	mtHead, foundHead, err := db.ManifestListExists(ctx, up)
	require.NoError(t, err)
	assert.True(t, foundHead, "HEAD must find the Slot's current index")
	// The existence path must NOT surface the archived index's entry: with
	// only one media type in play the mediaType is identical, but the
	// QueryOne primary must have skipped idxOld (HAS_HISTORY exclusion) and
	// resolved via the Slot fallback. Assert via the digest below.
	assert.Equal(t, "application/vnd.oci.image.index.v1+json", mtHead)

	req := types.Request{Repo: headGetRepo, Tag: headGetTag, Profile: headGetProfile}
	mlGet, foundGet, err := db.IndexGetByRequest(ctx, req)
	require.NoError(t, err)
	assert.True(t, foundGet)
	assert.Equal(t, idxNew, mlGet.ID, "GET must serve the current target, not the archived one")
	assert.Equal(t, "sha256:3333333333333333333333333333333333333333333333333333333333333333", mlGet.Digest)

	// The archived index is hidden on the strict-REF path: run the primary's
	// exact shape manually — the only REF-wired index here is the archived
	// one, so the HAS_HISTORY exclusion must make it miss (before the fix
	// this query matched idxOld's stale REF edges).
	q := fmt.Sprintf(`MATCH (prf:Profile)-[:REF]->(ml:Index)<-[:REF]-(c:Repo)-[:HAS_TAG]->(t:Tag)-[:REF]->(ml) WHERE prf.label = "%s" AND c.repo = "%s" AND t.name = "%s" AND NOT (:Slot {repo: "%s", tag: "%s"})-[:HAS_HISTORY]->(ml) RETURN ml.digest AS d`, headGetProfile, headGetRepo, headGetTag, headGetRepo, headGetTag)
	_, found, err := db.QueryOne(q)
	require.NoError(t, err)
	assert.False(t, found, "primary query with HAS_HISTORY exclusion must not surface the archived index")
}

// TestHeadGetAgree_SettledRefJoinUnchanged pins that the settled post-compute
// state (Slot wired AND REF edges present) still resolves through the strict
// join on both paths — Option A must not weaken the settled path.
func TestHeadGetAgree_SettledRefJoinUnchanged(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	idx := createTestIndex(t, "sha256:4444444444444444444444444444444444444444444444444444444444444444")
	slot, err := db.SlotGetCreate(ctx, headGetRepo, headGetTag, headGetProfile)
	require.NoError(t, err)
	require.NoError(t, db.SlotLinkPointsTo(ctx, slot.ID, idx))
	wireIndexREF(t, idx)

	up := upstream.UpstreamTriple{Repo: headGetRepo, Tag: headGetTag, Profile: headGetProfile}
	_, foundHead, err := db.ManifestListExists(ctx, up)
	require.NoError(t, err)
	assert.True(t, foundHead)

	req := types.Request{Repo: headGetRepo, Tag: headGetTag, Profile: headGetProfile}
	mlGet, foundGet, err := db.IndexGetByRequest(ctx, req)
	require.NoError(t, err)
	assert.True(t, foundGet)
	assert.Equal(t, idx, mlGet.ID)
}

// TestHeadGetAgree_BothMissWithoutSlotOrRefs pins the other agreement
// direction: with neither REF edges nor a Slot, both paths report not-found
// (HEAD 404 / GET 404).
func TestHeadGetAgree_BothMissWithoutSlotOrRefs(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	up := upstream.UpstreamTriple{Repo: headGetRepo, Tag: headGetTag, Profile: headGetProfile}
	_, foundHead, err := db.ManifestListExists(ctx, up)
	require.NoError(t, err)
	assert.False(t, foundHead)

	req := types.Request{Repo: headGetRepo, Tag: headGetTag, Profile: headGetProfile}
	_, foundGet, err := db.IndexGetByRequest(ctx, req)
	require.NoError(t, err)
	assert.False(t, foundGet)
}
