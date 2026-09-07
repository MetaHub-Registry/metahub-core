package neo

// Coverage for the collection-tag helpers introduced with the
// collection-scoped attribution + tag-REF pruning work:
//
//   - SlotProfilesForCollectionTag   (collection-scoped push attribution)
//   - UnLinkCollectionTagFromOtherIndexes (one-index-per-tag invariant)
//   - ManifestGetByCollectionTag slot-centred and direct-UPSTREAM fallbacks

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tagREFCount(t *testing.T, colTagID int64) int {
	t.Helper()
	q := fmt.Sprintf(`MATCH (ct:Tag)-[r:REF]->(ml:Index) WHERE id(ct) = %d RETURN count(r) AS c`, colTagID)
	res, found, err := db.QueryOne(q)
	require.NoError(t, err)
	if !found {
		return 0
	}
	c, err := res.GetInt64("c")
	require.NoError(t, err)
	return int(c)
}

func tagREFDigest(t *testing.T, colTagID int64) string {
	t.Helper()
	require.Equal(t, 1, tagREFCount(t, colTagID), "tag must hold exactly one REF")
	q := fmt.Sprintf(`MATCH (ct:Tag)-[:REF]->(ml:Index) WHERE id(ct) = %d RETURN ml.digest AS d`, colTagID)
	res, found, err := db.QueryOne(q)
	require.NoError(t, err)
	require.True(t, found)
	d, err := res.GetStr("d")
	require.NoError(t, err)
	return d
}

// colTagFixture creates the Repo/Tag skeleton for one collection and returns
// the internal node IDs.
func colTagFixture(t *testing.T, repo, tag string) (colID, colTagID int64) {
	t.Helper()
	q := fmt.Sprintf(`
		MERGE (r:Repo {repo: %q})
		MERGE (t:Tag {name: %q, repo: %q})
		MERGE (r)-[:HAS_TAG]->(t)
		RETURN id(r) AS colID, id(t) AS colTagID`, repo, tag, repo)
	res, err := db.Execute(q)
	require.NoError(t, err)
	colID, err = res.GetInt64("colID")
	require.NoError(t, err)
	colTagID, err = res.GetInt64("colTagID")
	require.NoError(t, err)
	return colID, colTagID
}

func TestSlotProfilesForCollectionTag(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	col, _, err := db.CollectionGetCreateByRepoTag(ctx, "qnib/slotattr", "v1")
	require.NoError(t, err)

	md1 := createTestManifest(t, "sha256:"+strings.Repeat("b1", 32))
	md2 := createTestManifest(t, "sha256:"+strings.Repeat("b2", 32))

	sA, err := db.SlotGetCreate(ctx, "qnib/slotattr", "v1", "alpha")
	require.NoError(t, err)
	sB, err := db.SlotGetCreate(ctx, "qnib/slotattr", "v1", "beta")
	require.NoError(t, err)
	require.NoError(t, db.SlotAppendPushedManifest(ctx, sA.ID, md1, "u1"))
	require.NoError(t, db.SlotAppendPushedManifest(ctx, sB.ID, md1, "u2"))
	require.NoError(t, db.SlotAppendPushedManifest(ctx, sA.ID, md2, "u1"))

	got, err := db.SlotProfilesForCollectionTag(ctx, col.ID, col.ColTag.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta"}, got[md1],
		"manifest pushed by two profiles must carry both, sorted")
	assert.Equal(t, []string{"alpha"}, got[md2])
	assert.NotContains(t, got, int64(-1))

	// A different collection must not see these pushes.
	other, _, err := db.CollectionGetCreateByRepoTag(ctx, "qnib/slotattr-other", "v1")
	require.NoError(t, err)
	gotOther, err := db.SlotProfilesForCollectionTag(ctx, other.ID, other.ColTag.ID)
	require.NoError(t, err)
	assert.Empty(t, gotOther)
}

func TestUnLinkCollectionTagFromOtherIndexes(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	col, _, err := db.CollectionGetCreateByRepoTag(ctx, "qnib/tagguard", "v1")
	require.NoError(t, err)
	colOther, _, err := db.CollectionGetCreateByRepoTag(ctx, "qnib/tagguard-other", "v1")
	require.NoError(t, err)

	// Push #1: index A linked to the collection tag (the compute path).
	idxA := createTestIndex(t, "sha256:"+strings.Repeat("a1", 32))
	require.NoError(t, db.LinkManifestListToCollectionTag(idxA, col.ColTag.ID))
	assert.Equal(t, 1, tagREFCount(t, col.ColTag.ID))

	// An unrelated collection's tag attachment must survive the prune.
	idxC := createTestIndex(t, "sha256:"+strings.Repeat("c1", 32))
	require.NoError(t, db.LinkManifestListToCollectionTag(idxC, colOther.ColTag.ID))
	assert.Equal(t, 1, tagREFCount(t, colOther.ColTag.ID))

	// Push #2 (recompute): prune-then-link must leave exactly one REF, and it
	// must be the NEW index.
	idxB := createTestIndex(t, "sha256:"+strings.Repeat("b1", 32))
	require.NoError(t, db.UnLinkCollectionTagFromOtherIndexes(idxB, col.ColTag.ID))
	require.NoError(t, db.LinkManifestListToCollectionTag(idxB, col.ColTag.ID))
	assert.Equal(t, "sha256:"+strings.Repeat("b1", 32), tagREFDigest(t, col.ColTag.ID))
	assert.Equal(t, 1, tagREFCount(t, colOther.ColTag.ID), "other collection's attachment is untouched")

	// Idempotency: pruning for the already-linked index changes nothing.
	require.NoError(t, db.UnLinkCollectionTagFromOtherIndexes(idxB, col.ColTag.ID))
	assert.Equal(t, "sha256:"+strings.Repeat("b1", 32), tagREFDigest(t, col.ColTag.ID))
}

// TestManifestGetByCollectionTag_FallbackPaths wires the slot-centred and
// direct-UPSTREAM graph shapes by hand and verifies both fallbacks of
// ManifestGetByCollectionTag, including collection-scoped profile attribution
// and content-size aggregation.
func TestManifestGetByCollectionTag_FallbackPaths(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	slotRepo, slotTag := "qnib/coltag-slot", "v1"
	upRepo, upTag := "qnib/coltag-up", "v1"
	colID, colTagID := colTagFixture(t, slotRepo, slotTag)
	upColID, upColTagID := colTagFixture(t, upRepo, upTag)

	// --- Scenario A: slot-centred shape (Repo→Tag→REF→Index→REFERENCES→Manifest) ---
	q := fmt.Sprintf(`
		MATCH (r:Repo {repo: %q})-[:HAS_TAG]->(t:Tag {name: %q})
		CREATE (i:Index {digest: "sha256:%s", mediaType: "application/vnd.oci.image.index.v1+json", size: 100, label: "slotidx"})
		CREATE (m:Manifest {digest: "sha256:%s", mediaType: "application/vnd.oci.image.manifest.v1+json", size: 300, label: "slotmd"})
		CREATE (cfg:Blob {digest: "sha256:%s", mediaType: "application/vnd.oci.image.config.v1+json", size: 120, type: "config", label: "slotcfg"})
		CREATE (p:Platform {os: "linux", arch: "amd64", variant: "", label: "linux/amd64"})
		CREATE (t)-[:REF]->(i)
		CREATE (i)-[:REFERENCES]->(m)
		CREATE (m)-[:USES]->(cfg)
		CREATE (m)-[:HAS_PLATFORM]->(p)
		CREATE (prf:Profile {label: "tester"})
		CREATE (s:Slot {repo: %q, tag: %q, profile: "tester", created_at: 1})
		CREATE (s)-[:OF_REPO]->(r)
		CREATE (s)-[:OF_TAG]->(t)
		CREATE (s)-[:OF_PROFILE]->(prf)
		CREATE (s)-[:HAS_PUSHED]->(m)
		CREATE (s)-[:POINTS_TO]->(i)`,
		slotRepo, slotTag,
		strings.Repeat("11", 32), strings.Repeat("22", 32), strings.Repeat("33", 32),
		slotRepo, slotTag)
	_, err := db.Execute(q)
	require.NoError(t, err)

	// --- Scenario B: direct UPSTREAM shape (Repo→UPSTREAM→Manifest, Tag→UPSTREAM) ---
	q = fmt.Sprintf(`
		MATCH (r:Repo {repo: %q})-[:HAS_TAG]->(t:Tag {name: %q})
		CREATE (m:Manifest {digest: "sha256:%s", mediaType: "application/vnd.oci.image.manifest.v1+json", size: 400, label: "upmd"})
		CREATE (cfg:Blob {digest: "sha256:%s", mediaType: "application/vnd.oci.image.config.v1+json", size: 150, type: "config", label: "upcfg"})
		CREATE (u:Upstream {reg: "reg.example", repo: "up/stream", tag: "v1", digest: "sha256:%s"})
		CREATE (p:Platform {os: "linux", arch: "arm64", variant: "", label: "linux/arm64"})
		CREATE (r)-[:UPSTREAM]->(m)
		CREATE (t)-[:UPSTREAM]->(m)
		CREATE (m)-[:USES]->(cfg)
		CREATE (m)-[:ORIGINAL]->(u)
		CREATE (m)-[:HAS_PLATFORM]->(p)`,
		upRepo, upTag,
		strings.Repeat("44", 32), strings.Repeat("55", 32), strings.Repeat("44", 32))
	_, err = db.Execute(q)
	require.NoError(t, err)

	t.Run("slot_centred_path", func(t *testing.T) {
		manis, err := db.ManifestGetByCollectionTag(ctx, colID, colTagID)
		require.NoError(t, err)
		require.Len(t, manis, 1)
		m := manis[0]
		assert.Equal(t, "sha256:"+strings.Repeat("22", 32), m.Digest)
		assert.Equal(t, "tester", m.Profile, "collection-scoped attribution must override the generic default")
		assert.Equal(t, "tester", m.Upstream.Profile)
		assert.Equal(t, slotRepo, m.Upstream.Repo, "upstream is synthesized from the Repo/Tag metadata")
		assert.Equal(t, slotTag, m.Upstream.Tag)
		assert.Equal(t, int64(120), m.ContentSize, "content size aggregates the config blob")
		assert.Equal(t, "linux/amd64", m.Platform.GetLabel())
	})

	t.Run("direct_upstream_fallback", func(t *testing.T) {
		manis, err := db.ManifestGetByCollectionTag(ctx, upColID, upColTagID)
		require.NoError(t, err)
		require.Len(t, manis, 1)
		m := manis[0]
		assert.Equal(t, "sha256:"+strings.Repeat("44", 32), m.Digest)
		assert.Equal(t, "generic", m.Profile, "no slot pushed here — generic attribution")
		assert.Equal(t, "up/stream", m.Upstream.Repo, "upstream comes from the ORIGINAL node")
		assert.Equal(t, "v1", m.Upstream.Tag)
		assert.Equal(t, int64(150), m.ContentSize)
		assert.Equal(t, "linux/arm64", m.Platform.GetLabel())
	})

	t.Run("empty_collection", func(t *testing.T) {
		emptyCol, emptyTag := colTagFixture(t, "qnib/coltag-empty", "v1")
		manis, err := db.ManifestGetByCollectionTag(ctx, emptyCol, emptyTag)
		require.NoError(t, err)
		assert.Empty(t, manis)
	})
}
