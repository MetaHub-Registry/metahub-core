package neo

// Tests for the one-image-per-profile invariant within a collection:
// UnLinkProfileFromOtherRepoIndexes must detach a Profile from every other
// Index of the same Repo before LinkManifestListToProfile attaches it to the
// freshly pushed image. Without the detach, the profile-scoped serve
// resolution (Profile→Index←Repo-HAS_TAG→Tag-REF→Index) matches two rows and
// serves stale content nondeterministically.

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	digestProfileGuardOld = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestProfileGuardNew = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// profileREFCount returns how many :Index nodes the profile prfID is attached
// to via REF within the repo identified by repoID.
func profileREFCount(t *testing.T, prfID, repoID int64) int {
	t.Helper()
	q := fmt.Sprintf(
		`MATCH (p:Profile)-[r:REF]->(ml:Index)<-[:REF]-(c:Repo) WHERE id(p) = %d AND id(c) = %d RETURN count(r) AS c`,
		prfID, repoID,
	)
	res, found, err := db.QueryOne(q)
	require.NoError(t, err)
	if !found {
		return 0
	}
	c, err := res.GetInt64("c")
	require.NoError(t, err)
	return int(c)
}

// profileREFDigest returns the digest of the single Index the profile is
// attached to within repoID. Fails the test if the count is not exactly 1.
func profileREFDigest(t *testing.T, prfID, repoID int64) string {
	t.Helper()
	require.Equal(t, 1, profileREFCount(t, prfID, repoID), "profile must be attached to exactly one index in the repo")
	q := fmt.Sprintf(
		`MATCH (p:Profile)-[:REF]->(ml:Index)<-[:REF]-(c:Repo) WHERE id(p) = %d AND id(c) = %d RETURN ml.digest AS d`,
		prfID, repoID,
	)
	res, found, err := db.QueryOne(q)
	require.NoError(t, err)
	require.True(t, found)
	d, err := res.GetStr("d")
	require.NoError(t, err)
	return d
}

func TestUnLinkProfileFromOtherRepoIndexes(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	col, _, err := db.CollectionGetCreateByRepoTag(ctx, "qnib/profileguard", "v1")
	require.NoError(t, err)
	colOther, _, err := db.CollectionGetCreateByRepoTag(ctx, "qnib/profileguard-other", "v1")
	require.NoError(t, err)

	// A Profile node, created the same way SlotGetCreate would MERGE it.
	if _, err := db.Execute(`MERGE (p:Profile {label: "generic"})`); err != nil {
		t.Fatalf("merging Profile node: %v", err)
	}
	res, found, err := db.QueryOne(`MATCH (p:Profile {label: "generic"}) RETURN id(p) AS id`)
	require.NoError(t, err)
	require.True(t, found)
	prfID, err := res.GetInt64("id")
	require.NoError(t, err)

	// Push #1: index A is linked to Repo/Tag/Profile (the compute path).
	idxA := createTestIndex(t, digestProfileGuardOld)
	require.NoError(t, db.LinkManifestListToCollection(idxA, col.ID))
	require.NoError(t, db.LinkManifestListToCollectionTag(idxA, col.ColTag.ID))
	require.NoError(t, db.LinkManifestListToProfile(idxA, prfID))
	assert.Equal(t, 1, profileREFCount(t, prfID, col.ID))

	// The same profile also pushed a different image to a DIFFERENT repo:
	// index C there. Repo-scoped attachments are legitimate and must survive.
	idxC := createTestIndex(t, "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	require.NoError(t, db.LinkManifestListToCollection(idxC, colOther.ID))
	require.NoError(t, db.LinkManifestListToCollectionTag(idxC, colOther.ColTag.ID))
	require.NoError(t, db.LinkManifestListToProfile(idxC, prfID))
	assert.Equal(t, 1, profileREFCount(t, prfID, colOther.ID))

	// Push #2 (different content, same repo/tag): index B. Detach-before-attach:
	// without the UnLink call the profile would REF both indexes and the
	// serve resolution for (profile, repo, tag) would be ambiguous.
	idxB := createTestIndex(t, digestProfileGuardNew)
	require.NoError(t, db.LinkManifestListToCollection(idxB, col.ID))
	require.NoError(t, db.LinkManifestListToCollectionTag(idxB, col.ColTag.ID))
	require.NoError(t, db.UnLinkProfileFromOtherRepoIndexes(idxB, prfID, col.ID))
	require.NoError(t, db.LinkManifestListToProfile(idxB, prfID))

	// Exactly one attachment per collection, and it is the NEW index.
	assert.Equal(t, digestProfileGuardNew, profileREFDigest(t, prfID, col.ID))
	// The attachment in the other repo is untouched.
	assert.Equal(t, 1, profileREFCount(t, prfID, colOther.ID))

	// Idempotency: re-running the guard while attaching the same index must
	// not change anything.
	require.NoError(t, db.UnLinkProfileFromOtherRepoIndexes(idxB, prfID, col.ID))
	assert.Equal(t, digestProfileGuardNew, profileREFDigest(t, prfID, col.ID))
}
