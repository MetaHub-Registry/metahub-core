package magazine

import (
	"context"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// Path Z task 8 tests for StoreIndexUpload (formerly StoreManifestListUpload).
// These tests verify the client-pushed dual-storage behaviour from design
// decision D8:
//
//  1. origin = "client-pushed" is set on the :Index node (first-writer-wins)
//  2. :REFERENCES edges carry an `order` property matching the descriptor
//     position in the pushed OCI manifest list
//  3. The pusher's :Slot gets a HAS_PUSHED_INDEX edge
//  4. First-writer-wins: if the same digest was previously computed
//     server-side (origin=computed), a subsequent client push does NOT
//     overwrite the origin
//
// All tests share the same fixture shape: bootstrap two ft_* Manifest
// nodes (arm64 and amd64) to act as the index members, then construct a
// synthetic types.Index pointing at their digests.

// bootstrapFtMultiArch seeds a fixture with both arm64 and amd64 Manifest
// nodes, ready to be referenced as member descriptors in a client-pushed
// :Index. Returns the bootstrap, the bootstrap result, and the two
// Manifest IDs (arm then amd) resolved from the bootstrap result.
func bootstrapFtMultiArch(t *testing.T) (utils.Bootstrap, utils.BootstrapResult, int64, int64) {
	t.Helper()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("empty:all")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.SetEmptyAll()

	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	armID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err, "manifestdata:ft_arm64#id must be in bootstrap result")
	amdID, err := br.GetInt64("manifestdata:ft_amd64#id")
	require.NoError(t, err, "manifestdata:ft_amd64#id must be in bootstrap result")
	return bs, br, armID, amdID
}

// buildTestIndex constructs a synthetic types.Index with two descriptor
// entries pointing at the ft_arm64 and ft_amd64 Manifest digests.
// The index digest is computed from ToOCIBytes so StoreIndexUpload can
// persist it via its normal dual-storage path.
func buildTestIndex(t *testing.T, armDigest, amdDigest string) types.Index {
	t.Helper()
	ml := types.Index{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.index.v1+json",
		Manifests: []types.Manifest{
			{
				Digest:    armDigest,
				MediaType: "application/vnd.docker.distribution.manifest.v2+json",
				Size:      735,
				Platform: types.Platform{
					Arch: "arm64",
					OS:   "linux",
				},
			},
			{
				Digest:    amdDigest,
				MediaType: "application/vnd.docker.distribution.manifest.v2+json",
				Size:      735,
				Platform: types.Platform{
					Arch: "amd64",
					OS:   "linux",
				},
			},
		},
	}
	// Compute the digest by serialising to OCI bytes. The helper already
	// handles digest calculation but we need to trigger it explicitly.
	ml.Digest = ""
	raw := ml.ToOCIBytes()
	ml.Size = len(raw)
	// Use the wrapper to populate the digest.
	reloaded, err := types.NewManifestListFromData(raw)
	require.NoError(t, err, "NewManifestListFromData must succeed")
	reloaded.Manifests = ml.Manifests
	return reloaded
}

// readIndexOrigin returns the `origin` property on the :Index with the
// given ID, or empty string if the property is not set.
func readIndexOrigin(t *testing.T, indexID int64) string {
	t.Helper()
	q := fmt.Sprintf(`MATCH (i:Index) WHERE id(i) = %d RETURN coalesce(i.origin, "") AS origin`, indexID)
	res, err := mag.Database.Execute(q)
	require.NoError(t, err)
	s, err := res.GetStr("origin")
	if err != nil {
		return ""
	}
	return s
}

// countSlotHasPushedIndexEdges counts :HAS_PUSHED_INDEX edges from the
// Slot matching the given triple. Used to verify that StoreIndexUpload
// appends the expected edge on push.
func countSlotHasPushedIndexEdges(t *testing.T, repo, tag, profile string) int {
	t.Helper()
	q := fmt.Sprintf(
		`MATCH (s:Slot {repo: "%s", tag: "%s", profile: "%s"})-[r:HAS_PUSHED_INDEX]->(:Index) RETURN count(r) AS c`,
		repo, tag, profile,
	)
	res, err := mag.Database.Execute(q)
	require.NoError(t, err)
	n, err := res.GetInt64("c")
	if err != nil {
		return 0
	}
	return int(n)
}

// readReferencesOrder returns the ordered list of (mdID, order) tuples
// outgoing from the given Index via :REFERENCES, sorted by order.
func readReferencesOrder(t *testing.T, indexID int64) []struct {
	mdID  int64
	order int64
} {
	t.Helper()
	q := fmt.Sprintf(
		`MATCH (i:Index)-[r:REFERENCES]->(md:Manifest) WHERE id(i) = %d RETURN id(md) AS mdID, coalesce(r.order, -1) AS ord ORDER BY ord`,
		indexID,
	)
	results, err := mag.Database.Query(q)
	require.NoError(t, err)
	out := make([]struct {
		mdID  int64
		order int64
	}, 0, len(results))
	for _, r := range results {
		mdID, _ := r.GetInt64("mdID")
		order, _ := r.GetInt64("ord")
		out = append(out, struct {
			mdID  int64
			order int64
		}{mdID, order})
	}
	return out
}

// TestStoreIndexUpload_ClientPushedOrigin is the Path Z task 8.4 test
// for the client-pushed origin attribution. After a fresh
// StoreIndexUpload call with no prior compute, the resulting :Index
// node must carry origin="client-pushed".
func TestStoreIndexUpload_ClientPushedOrigin(t *testing.T) {
	bs, br, armID, amdID := bootstrapFtMultiArch(t)
	defer TeardownTest(bs, br)
	_ = armID
	_ = amdID

	armDig, err := br.Get("manifestdata:ft_arm64#digest")
	require.NoError(t, err)
	amdDig, err := br.Get("manifestdata:ft_amd64#digest")
	require.NoError(t, err)

	ml := buildTestIndex(t, armDig, amdDig)
	req := types.Request{
		Repo:    "qnib/featuretest",
		Tag:     "latest",
		Profile: "cpu#generic",
	}

	err = mag.StoreIndexUpload(context.Background(), req, &ml)
	require.NoError(t, err, "StoreIndexUpload must succeed for a valid multi-arch index")
	require.NotZero(t, ml.ID, "Index ID must be populated after StoreIndexUpload")

	origin := readIndexOrigin(t, ml.ID)
	assert.Equal(t, neo.IndexOriginClientPushed, origin,
		"fresh StoreIndexUpload must set origin=client-pushed on the :Index node")
}

// TestStoreIndexUpload_ReferencesCarryOrder verifies that the :REFERENCES
// edges created by StoreIndexUpload carry an `order` property matching
// the descriptor position in the pushed OCI manifest list.
func TestStoreIndexUpload_ReferencesCarryOrder(t *testing.T) {
	bs, br, armID, amdID := bootstrapFtMultiArch(t)
	defer TeardownTest(bs, br)

	armDig, err := br.Get("manifestdata:ft_arm64#digest")
	require.NoError(t, err)
	amdDig, err := br.Get("manifestdata:ft_amd64#digest")
	require.NoError(t, err)

	ml := buildTestIndex(t, armDig, amdDig)
	req := types.Request{
		Repo:    "qnib/featuretest",
		Tag:     "latest",
		Profile: "cpu#generic",
	}
	err = mag.StoreIndexUpload(context.Background(), req, &ml)
	require.NoError(t, err)

	refs := readReferencesOrder(t, ml.ID)
	assert.Len(t, refs, 2, "StoreIndexUpload must create exactly 2 :REFERENCES edges for a 2-descriptor index")
	if len(refs) < 2 {
		return
	}
	// First descriptor was arm64 (order 0), second was amd64 (order 1).
	assert.Equal(t, int64(0), refs[0].order, "first descriptor must have order=0")
	assert.Equal(t, int64(1), refs[1].order, "second descriptor must have order=1")
	assert.Equal(t, armID, refs[0].mdID, "first descriptor must point to ft_arm64 manifestdata")
	assert.Equal(t, amdID, refs[1].mdID, "second descriptor must point to ft_amd64 manifestdata")
}

// TestStoreIndexUpload_AppendsHasPushedIndex verifies that StoreIndexUpload
// resolves the pusher's :Slot and appends a :HAS_PUSHED_INDEX edge. The
// edge is append-only per design D3, so re-pushing the same index
// appends a second edge.
func TestStoreIndexUpload_AppendsHasPushedIndex(t *testing.T) {
	bs, br, _, _ := bootstrapFtMultiArch(t)
	defer TeardownTest(bs, br)

	armDig, err := br.Get("manifestdata:ft_arm64#digest")
	require.NoError(t, err)
	amdDig, err := br.Get("manifestdata:ft_amd64#digest")
	require.NoError(t, err)

	ml := buildTestIndex(t, armDig, amdDig)
	req := types.Request{
		Repo:    "qnib/featuretest",
		Tag:     "latest",
		Profile: "cpu#generic",
	}

	// First push: Slot is created and HAS_PUSHED_INDEX count goes to 1.
	err = mag.StoreIndexUpload(context.Background(), req, &ml)
	require.NoError(t, err)
	assert.Equal(t, 1, countSlotHasPushedIndexEdges(t, req.Repo, req.Tag, req.Profile),
		"after first StoreIndexUpload the Slot must have exactly 1 HAS_PUSHED_INDEX edge")

	// Second push of the same index: Slot is reused (MERGE), HAS_PUSHED_INDEX
	// count goes to 2 (append-only).
	err = mag.StoreIndexUpload(context.Background(), req, &ml)
	require.NoError(t, err)
	assert.Equal(t, 2, countSlotHasPushedIndexEdges(t, req.Repo, req.Tag, req.Profile),
		"re-pushing the same index must append a second HAS_PUSHED_INDEX edge (append-only log)")
}

// TestStoreIndexUpload_OriginFirstWriterWins verifies that if the same
// Index digest was previously MERGEd by the compute path with
// origin=computed, a subsequent StoreIndexUpload call does NOT overwrite
// the origin. This is Path Z task 8.4's first-writer-wins requirement
// from design D8.
func TestStoreIndexUpload_OriginFirstWriterWins(t *testing.T) {
	bs, br, _, _ := bootstrapFtMultiArch(t)
	defer TeardownTest(bs, br)

	armDig, err := br.Get("manifestdata:ft_arm64#digest")
	require.NoError(t, err)
	amdDig, err := br.Get("manifestdata:ft_amd64#digest")
	require.NoError(t, err)

	ml := buildTestIndex(t, armDig, amdDig)

	// Simulate the compute path creating the Index first with
	// origin=computed. We do this by calling InsertManifestList + SetIndexOrigin
	// directly rather than going through the full compute machinery.
	mlDig, err := digest.Parse(ml.Digest)
	require.NoError(t, err)
	mlID, err := mag.Database.InsertManifestList(context.Background(), mlDig, ml.MediaType, ml.Size)
	require.NoError(t, err)
	err = mag.Database.SetIndexOrigin(context.Background(), mlID, neo.IndexOriginComputed)
	require.NoError(t, err)

	// Verify the origin is now "computed".
	assert.Equal(t, neo.IndexOriginComputed, readIndexOrigin(t, mlID),
		"precondition: Index must have origin=computed after SetIndexOrigin(computed)")

	// Now perform the client push for the same digest.
	req := types.Request{
		Repo:    "qnib/featuretest",
		Tag:     "latest",
		Profile: "cpu#generic",
	}
	err = mag.StoreIndexUpload(context.Background(), req, &ml)
	require.NoError(t, err)
	assert.Equal(t, mlID, ml.ID, "StoreIndexUpload must reuse the existing Index node (same digest)")

	// First-writer-wins: origin must STILL be "computed", not "client-pushed".
	origin := readIndexOrigin(t, ml.ID)
	assert.Equal(t, neo.IndexOriginComputed, origin,
		"first-writer-wins: origin must remain 'computed' after a client push of the same digest")
}

// TestStoreIndexUpload_MissingMemberReturnsError verifies that pushing an
// index whose member digests don't exist in the graph fails loudly so
// the handler can map the error to MANIFEST_BLOB_UNKNOWN.
func TestStoreIndexUpload_MissingMemberReturnsError(t *testing.T) {
	bs, br, _, _ := bootstrapFtMultiArch(t)
	defer TeardownTest(bs, br)
	_ = br

	// Build an index whose descriptor digests do NOT exist in the graph.
	ml := buildTestIndex(t,
		"sha256:0000000000000000000000000000000000000000000000000000000000000001",
		"sha256:0000000000000000000000000000000000000000000000000000000000000002",
	)
	req := types.Request{
		Repo:    "qnib/featuretest",
		Tag:     "latest",
		Profile: "cpu#generic",
	}
	err := mag.StoreIndexUpload(context.Background(), req, &ml)
	assert.Error(t, err, "StoreIndexUpload must fail when a member digest is not in the graph")
	if err != nil {
		assert.Contains(t, err.Error(), "MANIFEST_BLOB_UNKNOWN",
			"error must include the MANIFEST_BLOB_UNKNOWN marker so the handler can map it to the right OCI error code")
	}
}
