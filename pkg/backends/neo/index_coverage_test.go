package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestIndexLinkingWorkflow exercises the full Index link/unlink lifecycle:
// Create Index → Link to Repo/Tag/Profile → ManifestListExists → Unlink → not found.
// Covers: LinkManifestListToCollection, LinkManifestListToCollectionTag,
// LinkManifestListToProfile, UnLinkManifestListToCollection,
// UnLinkManifestListToCollectionTag, ManifestListExists,
// ManifestListExistsForTagProfile, InsertManifestList.
func TestIndexLinkingWorkflow(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// Create a Repo + Tag via CollectionGetCreateByRepoTag
	col, _, err := db.CollectionGetCreateByRepoTag(ctx, "qnib/linktest", "v1")
	require.NoError(t, err)
	assert.NotZero(t, col.ID)
	assert.NotZero(t, col.ColTag.ID)

	// Get profile ID
	prfID, err := br.GetInt64("profile:generic#id")
	require.NoError(t, err)

	// Create an Index node via InsertManifestList
	mlDigest := "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	mlID, err := db.InsertManifestList(ctx,
		digest.Digest(mlDigest),
		"application/vnd.oci.image.index.v1+json",
		512,
	)
	require.NoError(t, err)
	assert.NotZero(t, mlID)

	// Link Index to Repo
	err = db.LinkManifestListToCollection(mlID, col.ID)
	require.NoError(t, err)

	// Link Index to Tag
	err = db.LinkManifestListToCollectionTag(mlID, col.ColTag.ID)
	require.NoError(t, err)

	// Link Index to Profile
	err = db.LinkManifestListToProfile(mlID, prfID)
	require.NoError(t, err)

	// ManifestListExistsForTagProfile should find it
	mt, found, err := db.ManifestListExistsForTagProfile(ctx, upstream.UpstreamTriple{
		Repo:    "qnib/linktest",
		Tag:     "v1",
		Profile: "generic",
	})
	require.NoError(t, err)
	assert.True(t, found, "ManifestListExistsForTagProfile should find linked index")
	assert.Equal(t, "application/vnd.oci.image.index.v1+json", mt)

	// ManifestListExists (tag-based) should find it
	mt, found, err = db.ManifestListExists(ctx, upstream.UpstreamTriple{
		Repo:    "qnib/linktest",
		Tag:     "v1",
		Profile: "generic",
	})
	require.NoError(t, err)
	assert.True(t, found, "ManifestListExists (tag) should find linked index")

	// ManifestListExists (digest-based) should find it
	mt, found, err = db.ManifestListExists(ctx, upstream.UpstreamTriple{
		Repo:    "qnib/linktest",
		Digest:  digest.Digest(mlDigest),
		Profile: "generic",
	})
	require.NoError(t, err)
	assert.True(t, found, "ManifestListExists (digest) should find linked index")

	// Unlink from Repo
	err = db.UnLinkManifestListToCollection(mlID, col.ID)
	require.NoError(t, err)

	// Unlink from Tag
	err = db.UnLinkManifestListToCollectionTag(mlID, col.ColTag.ID)
	require.NoError(t, err)

	// ManifestListExists should NOT find it anymore
	_, found, err = db.ManifestListExists(ctx, upstream.UpstreamTriple{
		Repo:    "qnib/linktest",
		Tag:     "v1",
		Profile: "generic",
	})
	require.NoError(t, err)
	assert.False(t, found, "after unlinking, ManifestListExists should not find it")
}

// TestSetIndexOrigin covers the first-writer-wins semantics.
func TestSetIndexOrigin(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	idxID := createTestIndex(t, "sha256:origin111111111111111111111111111111111111111111111111111111111111")

	// First set: should succeed
	err := db.SetIndexOrigin(ctx, idxID, IndexOriginComputed)
	require.NoError(t, err)

	// Verify origin was set
	q := fmt.Sprintf(`MATCH (i:Index) WHERE id(i) = %d RETURN i.origin AS origin`, idxID)
	res, found, err := db.QueryOne(q)
	require.NoError(t, err)
	require.True(t, found)
	origin, _ := res.GetStr("origin")
	assert.Equal(t, IndexOriginComputed, origin)

	// Second set with different origin: should be a no-op (first-writer wins)
	err = db.SetIndexOrigin(ctx, idxID, IndexOriginClientPushed)
	require.NoError(t, err)

	// Verify origin is STILL "computed" (not overwritten)
	q = fmt.Sprintf(`MATCH (i:Index) WHERE id(i) = %d RETURN i.origin AS origin`, idxID)
	res, found, err = db.QueryOne(q)
	require.NoError(t, err)
	require.True(t, found)
	origin, _ = res.GetStr("origin")
	assert.Equal(t, IndexOriginComputed, origin, "first-writer-wins: origin should not change")

	// Empty origin should return error
	err = db.SetIndexOrigin(ctx, idxID, "")
	assert.Error(t, err, "empty origin should be rejected")
}

// TestLinkIndexReferenceWithOrder covers ordered REFERENCES edges.
func TestLinkIndexReferenceWithOrder(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	idxID := createTestIndex(t, "sha256:order1111111111111111111111111111111111111111111111111111111111111")

	// Create two Manifest nodes
	md1ID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:ref1111111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)
	md2ID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:ref2222222222222222222222222222222222222222222222222222222222222",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      200,
	})
	require.NoError(t, err)

	// Link with order
	err = db.LinkIndexReferenceWithOrder(ctx, idxID, md1ID, 0)
	require.NoError(t, err)
	err = db.LinkIndexReferenceWithOrder(ctx, idxID, md2ID, 1)
	require.NoError(t, err)

	// Verify order property
	q := fmt.Sprintf(`MATCH (i:Index)-[r:REFERENCES]->(md:Manifest) WHERE id(i) = %d RETURN md.digest AS digest, r.order AS ord ORDER BY r.order`, idxID)
	rs, err := db.Query(q)
	require.NoError(t, err)
	require.Len(t, rs, 2)
	d0, _ := rs[0].GetStr("digest")
	d1, _ := rs[1].GetStr("digest")
	o0, _ := rs[0].GetInt64("ord")
	o1, _ := rs[1].GetInt64("ord")
	assert.Equal(t, "sha256:ref1111111111111111111111111111111111111111111111111111111111111", d0)
	assert.Equal(t, "sha256:ref2222222222222222222222222222222222222222222222222222222222222", d1)
	assert.Equal(t, int64(0), o0)
	assert.Equal(t, int64(1), o1)

	// Invalid IDs should error
	err = db.LinkIndexReferenceWithOrder(ctx, -1, md1ID, 0)
	assert.Error(t, err)
}

// TestLinkManifestListToManifest covers the REFERENCES edge
// creation and the negative-ID guard.
func TestLinkManifestListToManifest(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	idxID := createTestIndex(t, "sha256:lnkmd11111111111111111111111111111111111111111111111111111111111")
	mdID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:lnkmd22222222222222222222222222222222222222222222222222222222222",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)

	// Link
	err = db.LinkManifestListToManifest(ctx, idxID, mdID)
	require.NoError(t, err)

	// Verify edge exists
	q := fmt.Sprintf(`MATCH (i:Index)-[:REFERENCES]->(md:Manifest) WHERE id(i) = %d RETURN count(md) AS c`, idxID)
	res, found, err := db.QueryOne(q)
	require.NoError(t, err)
	require.True(t, found)
	cnt, _ := res.GetInt64("c")
	assert.Equal(t, int64(1), cnt)

	// Negative IDs should error
	err = db.LinkManifestListToManifest(ctx, -1, mdID)
	assert.Error(t, err)
}

// TestIndexGetByRequest covers the full pull-side query.
func TestIndexGetByRequest(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	prfID, err := br.GetInt64("profile:generic#id")
	require.NoError(t, err)

	// Create Repo + Tag
	col, _, err := db.CollectionGetCreateByRepoTag(ctx, "qnib/idxreq", "latest")
	require.NoError(t, err)

	// Create Manifest with platform
	mdID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:idxreq1111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      300,
	})
	require.NoError(t, err)
	platID, err := br.GetInt64("platform:linux/amd64#id")
	require.NoError(t, err)
	err = db.LinkManifestToPlatform(ctx, mdID, platID)
	require.NoError(t, err)

	// Create Index and wire everything
	mlDigest := "sha256:idxreqml11111111111111111111111111111111111111111111111111111111"
	mlID, err := db.CreateManifestList(digest.Digest(mlDigest), "application/vnd.oci.image.index.v1+json", 512)
	require.NoError(t, err)
	err = db.LinkManifestListToCollection(mlID, col.ID)
	require.NoError(t, err)
	err = db.LinkManifestListToCollectionTag(mlID, col.ColTag.ID)
	require.NoError(t, err)
	err = db.LinkManifestListToProfile(mlID, prfID)
	require.NoError(t, err)
	err = db.LinkManifestListToManifest(ctx, mlID, mdID)
	require.NoError(t, err)

	// IndexGetByRequest by tag
	req := types.Request{
		Repo:    "qnib/idxreq",
		Tag:     "latest",
		Profile: "generic",
	}
	idx, found, err := db.IndexGetByRequest(ctx, req)
	require.NoError(t, err)
	assert.True(t, found, "IndexGetByRequest should find the index")
	assert.Equal(t, mlDigest, idx.Digest)
	require.Len(t, idx.Manifests, 1, "index should have 1 manifest")
	assert.Equal(t, "amd64", idx.Manifests[0].Platform.Arch)
	assert.Equal(t, "linux", idx.Manifests[0].Platform.OS)

	// IndexGetByRequest by digest
	req2 := types.Request{
		Repo:    "qnib/idxreq",
		Digest:  mlDigest,
		Profile: "generic",
	}
	idx2, found2, err := db.IndexGetByRequest(ctx, req2)
	require.NoError(t, err)
	assert.True(t, found2)
	assert.Equal(t, mlDigest, idx2.Digest)

	// Not found
	req3 := types.Request{
		Repo:    "nonexistent/repo",
		Tag:     "missing",
		Profile: "generic",
	}
	_, found3, err := db.IndexGetByRequest(ctx, req3)
	require.NoError(t, err)
	assert.False(t, found3)
}

// TestManifestListViaSlot_Guards exercises the early-return branches in
// manifestListViaSlot: when profile, repo, or tag is empty the helper must
// skip the DB query. The digest-set guard is already covered by the
// digest-based lookup in TestIndexLinkingWorkflow.
func TestManifestListViaSlot_Guards(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	cases := []struct {
		name string
		up   upstream.UpstreamTriple
	}{
		{"empty profile", upstream.UpstreamTriple{Repo: "r", Tag: "t"}},
		{"empty repo", upstream.UpstreamTriple{Tag: "t", Profile: "p"}},
		{"empty tag", upstream.UpstreamTriple{Repo: "r", Profile: "p"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, found, err := db.ManifestListExists(ctx, tc.up)
			require.NoError(t, err)
			assert.False(t, found)
		})
	}
}

// TestManifestListExists_SlotFallback verifies that ManifestListExists finds
// an Index reachable only via Slot-[:POINTS_TO]->Index (no REF edges),
// exercising the success return path in manifestListViaSlot.
func TestManifestListExists_SlotFallback(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	idxDigest := "sha256:slotfb1111111111111111111111111111111111111111111111111111111111"
	idxID := createTestIndex(t, idxDigest)

	// Wire a Slot to the Index (no REF edges — the wrapper-era query misses it).
	_, err := db.Execute(fmt.Sprintf(
		`MATCH (i:Index) WHERE id(i) = %d
		 MERGE (s:Slot {repo: "qnib/slotfb", tag: "v1", profile: "generic"})
		 WITH s, i MERGE (s)-[:POINTS_TO]->(i)`, idxID))
	require.NoError(t, err)

	mt, found, err := db.ManifestListExists(ctx, upstream.UpstreamTriple{
		Repo:    "qnib/slotfb",
		Tag:     "v1",
		Profile: "generic",
	})
	require.NoError(t, err)
	assert.True(t, found, "should find Index via Slot POINTS_TO fallback")
	assert.Equal(t, "application/vnd.oci.image.index.v1+json", mt)
}
