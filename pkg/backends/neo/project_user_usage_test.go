package neo

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projSeedPush attaches a pusher to an existing :Manifest: it MERGEs the
// (repo, tag, profile) :Slot and appends an append-only :HAS_PUSHED edge
// carrying `user`. An empty user models a legacy/system push (no attributable
// pusher) — the same shape the production stamp writes for non-user bindings.
func projSeedPush(t *testing.T, repo, tag, profile, mani, user string) {
	t.Helper()
	projRunCypher(t, `
		MERGE (s:Slot {repo: $repo, tag: $tag, profile: $profile})
		WITH s
		MATCH (m:Manifest {digest: $mani})
		CREATE (s)-[:HAS_PUSHED {timestamp: timestamp(), user: $user}]->(m)`,
		map[string]interface{}{"repo": repo, "tag": tag, "profile": profile, "mani": mani, "user": user})
}

// projDeletePush removes a specific user's HAS_PUSHED edge(s) to a manifest —
// models a pusher retracting their push (e.g. Bob deletes his tag).
func projDeletePush(t *testing.T, mani, user string) {
	t.Helper()
	projRunCypher(t, `MATCH (:Slot)-[hp:HAS_PUSHED {user: $user}]->(:Manifest {digest: $mani}) DELETE hp`,
		map[string]interface{}{"mani": mani, "user": user})
}

// sumFloatMap sums all values — used to assert the reconciliation invariant
// Σ(per-user buckets, incl. unknown) == ProjectUsageBytes.
func sumFloatMap(m map[string]float64) float64 {
	var s float64
	for _, v := range m {
		s += v
	}
	return s
}

// TestProjectUserUsage_AliceOnly: a single pusher gets the whole blob.
func TestProjectUserUsage_AliceOnly(t *testing.T) {
	ctx := context.Background()
	team, project := "quser-alice", "p"
	defer db.ProjectDelete(ctx, team, project)
	base := team + "/" + project
	repo := base + "/img"
	projSeedManifest(t, repo, "sha256:uaonly")
	projLinkBlob(t, "sha256:uaonly", "sha256:uablob", 500, "layer", "HAS_LAYER")
	projSeedPush(t, repo, "latest", "generic", "sha256:uaonly", "alice")

	usage, err := db.ProjectUserUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"alice": 500}, usage)

	total, err := db.ProjectUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, float64(total), sumFloatMap(usage), "Σ per-user must equal project total")
}

// TestProjectUserUsage_AliceBobSplit: the same image pushed by two users
// (Bob's push is a content dedup, but still stamps a HAS_PUSHED edge) splits
// the blob 50/50 — the core scenario.
func TestProjectUserUsage_AliceBobSplit(t *testing.T) {
	ctx := context.Background()
	team, project := "quser-split", "p"
	defer db.ProjectDelete(ctx, team, project)
	base := team + "/" + project
	repo := base + "/img"
	projSeedManifest(t, repo, "sha256:ubsplit")
	projLinkBlob(t, "sha256:ubsplit", "sha256:ubblob", 500, "layer", "HAS_LAYER")
	projSeedPush(t, repo, "latest", "generic", "sha256:ubsplit", "alice")
	projSeedPush(t, repo, "latest", "generic", "sha256:ubsplit", "bob")

	usage, err := db.ProjectUserUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"alice": 250, "bob": 250}, usage)

	total, err := db.ProjectUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, float64(total), sumFloatMap(usage))
	assert.Equal(t, int64(500), total, "project total dedups to one blob")
}

// TestProjectUserUsage_BobDeleteDriftFree: after Bob retracts his push, the
// on-demand recompute reassigns the full blob to Alice with no residual drift.
func TestProjectUserUsage_BobDeleteDriftFree(t *testing.T) {
	ctx := context.Background()
	team, project := "quser-drift", "p"
	defer db.ProjectDelete(ctx, team, project)
	base := team + "/" + project
	repo := base + "/img"
	projSeedManifest(t, repo, "sha256:udrift")
	projLinkBlob(t, "sha256:udrift", "sha256:udblob", 500, "layer", "HAS_LAYER")
	projSeedPush(t, repo, "latest", "generic", "sha256:udrift", "alice")
	projSeedPush(t, repo, "latest", "generic", "sha256:udrift", "bob")

	usage, err := db.ProjectUserUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"alice": 250, "bob": 250}, usage)

	projDeletePush(t, "sha256:udrift", "bob")
	usage, err = db.ProjectUserUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"alice": 500}, usage, "recompute reassigns full blob to sole remaining pusher")

	total, err := db.ProjectUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, int64(500), total, "blob still referenced → project total unchanged")
	assert.Equal(t, float64(total), sumFloatMap(usage))
}

// TestProjectUserUsage_UnattributedToUnknown: a reachable blob with no
// attributable pusher (compute-generated manifest with no push edge, plus an
// empty-user push) lands in the reserved "unknown" bucket.
func TestProjectUserUsage_UnattributedToUnknown(t *testing.T) {
	ctx := context.Background()
	team, project := "quser-unknown", "p"
	defer db.ProjectDelete(ctx, team, project)
	base := team + "/" + project
	repo := base + "/img"
	// compute-generated: manifest + blob, no HAS_PUSHED at all
	projSeedManifest(t, repo, "sha256:ucompute")
	projLinkBlob(t, "sha256:ucompute", "sha256:ucblob", 400, "layer", "HAS_LAYER")
	// legacy/system: an empty-user push edge
	projSeedManifest(t, base+"/img2", "sha256:ulegacy")
	projLinkBlob(t, "sha256:ulegacy", "sha256:ulblob", 100, "layer", "HAS_LAYER")
	projSeedPush(t, base+"/img2", "latest", "generic", "sha256:ulegacy", "")

	usage, err := db.ProjectUserUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{UnknownUsageBucket: 500}, usage)

	total, err := db.ProjectUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, float64(total), sumFloatMap(usage))
}

// TestProjectUserUsage_SumReconcilesWithTotal is the reconciliation invariant on
// a mixed graph: shared-split blob + solo blob + unattributed blob. Σ(all
// buckets incl. unknown) MUST equal ProjectUsageBytes exactly.
func TestProjectUserUsage_SumReconcilesWithTotal(t *testing.T) {
	ctx := context.Background()
	team, project := "quser-recon", "p"
	defer db.ProjectDelete(ctx, team, project)
	base := team + "/" + project

	// blobX 500: pushed by alice + bob → 250/250
	repoX := base + "/imgx"
	projSeedManifest(t, repoX, "sha256:urx")
	projLinkBlob(t, "sha256:urx", "sha256:ublobx", 500, "layer", "HAS_LAYER")
	projSeedPush(t, repoX, "latest", "generic", "sha256:urx", "alice")
	projSeedPush(t, repoX, "latest", "generic", "sha256:urx", "bob")

	// blobY 300: pushed by alice only → 300
	repoY := base + "/imgy"
	projSeedManifest(t, repoY, "sha256:ury")
	projLinkBlob(t, "sha256:ury", "sha256:ubloby", 300, "layer", "HAS_LAYER")
	projSeedPush(t, repoY, "latest", "generic", "sha256:ury", "alice")

	// blobZ 100: compute-generated, no pusher → unknown
	repoZ := base + "/imgz"
	projSeedManifest(t, repoZ, "sha256:urz")
	projLinkBlob(t, "sha256:urz", "sha256:ublobz", 100, "layer", "HAS_LAYER")

	usage, err := db.ProjectUserUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.InDelta(t, 550.0, usage["alice"], 1e-9)
	assert.InDelta(t, 250.0, usage["bob"], 1e-9)
	assert.InDelta(t, 100.0, usage[UnknownUsageBucket], 1e-9)

	total, err := db.ProjectUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, int64(900), total)
	assert.True(t, math.Abs(float64(total)-sumFloatMap(usage)) < 1e-9, "Σ buckets == total")
}

// TestProjectUserUsage_EmptyProject: unknown/empty namespace yields an empty map.
func TestProjectUserUsage_EmptyProject(t *testing.T) {
	ctx := context.Background()
	usage, err := db.ProjectUserUsageBytes(ctx, "quser-none", "x")
	require.NoError(t, err)
	assert.Empty(t, usage)
}
