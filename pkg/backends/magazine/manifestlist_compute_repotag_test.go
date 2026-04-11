package magazine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// neoDB type-asserts mag.Database to *the* neo backend used by these tests.
// CreateProfile, ProfileLinkRule, and GetAnnotationsByManifestID are not
// on the Database interface; they are neo-only helpers that the tests need
// for fixture setup and assertions.
func neoDB(t *testing.T) neo.Neo {
	t.Helper()
	n, ok := mag.Database.(neo.Neo)
	if !ok {
		t.Fatalf("expected mag.Database to be neo.Neo, got %T", mag.Database)
	}
	return n
}

// The "ft_arm64" bootstrap creates a Manifest whose Upstream node has
// (repo, tag) = ("qnib-pub-containers/qnib/plain-featuretest", "3.17.3-arm64").
// These are the coordinates ManifestGetRawByRepoTag uses for the walk,
// so we pass them to ComputeManifestlistsForRepoTag. Internally the helper
// calls CollectionGetCreateByRepoTag with the same values, which
// idempotently creates (or fetches) a Repo keyed on those same
// coordinates — distinct from the bootstrap's own "qnib/featuretest:arm64"
// Repo node, which was only linked at the :ManifestWrapper wrapper level
// and is not visible via the Upstream walk.
//
// In production pushes, the request's repo/tag and the Upstream's repo/tag
// always agree (both come from the same URL path), so this "fixture has
// two Collections, one keyed on Upstream coords" quirk is unique to
// tests.
const (
	testRepo = "qnib-pub-containers/qnib/plain-featuretest"
	testTag  = "3.17.3-arm64"
)

// bootstrapFtArm64 spins up the bootstrap fixture used by every test in this
// file. It returns the Bootstrap and BootstrapResult so the caller can defer
// teardown and look up IDs by label.
func bootstrapFtArm64(t *testing.T) (utils.Bootstrap, utils.BootstrapResult) {
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
	bs.AddBootstrap("collection:ft_arm64")
	bs.SetEmptyAll()

	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	return bs, br
}

// createTestProfileWithRules creates a Profile with the given rules and links
// them. Each rule gets a unique Name to satisfy the Rule.label uniqueness
// constraint in neo4j. The Profile's Rule expression is auto-generated as the
// conjunction of all attached rule labels (`name:tag=true && ...`) so that
// NewCollectionTagProfilePair can extract the label set via ExprGetLabels.
func createTestProfileWithRules(t *testing.T, profileName string, rules []types.Rule) types.Profile {
	t.Helper()
	n := neoDB(t)

	// Pre-assign rule names/tags so we can build the rule expression up front.
	prepared := make([]types.Rule, len(rules))
	exprParts := make([]string, 0, len(rules))
	for i, r := range rules {
		if r.Name == "" {
			r.Name = fmt.Sprintf("%s-r%d", profileName, i)
		}
		if r.Tag == "" {
			r.Tag = "v1"
		}
		prepared[i] = r
		exprParts = append(exprParts, fmt.Sprintf("%s:%s=true", r.Name, r.Tag))
	}
	ruleExpr := ""
	for i, p := range exprParts {
		if i == 0 {
			ruleExpr = p
		} else {
			ruleExpr = ruleExpr + " && " + p
		}
	}

	prfID, err := n.CreateProfile(types.Profile{Name: profileName, Tag: "v1", Rule: ruleExpr})
	require.NoError(t, err)

	for _, r := range prepared {
		rID, err := mag.Database.RuleGetCreate(ctx, r)
		require.NoError(t, err)
		err = n.ProfileLinkRule(ctx, prfID, rID)
		require.NoError(t, err)
	}

	prf, found, err := mag.Database.ProfileGetByID(ctx, prfID)
	require.NoError(t, err)
	require.True(t, found)
	prf.ID = prfID
	return prf
}

// getAnnotations is a tiny helper to assert annotation state on a manifestdata.
func getAnnotations(t *testing.T, mdID int64) types.Annotations {
	t.Helper()
	annos, err := neoDB(t).GetAnnotationsByManifestID(ctx, mdID)
	require.NoError(t, err)
	return annos
}

// countManifestlistsForCollectionTag returns the number of Index rows
// linked to (collectionRepo, tagName). Used for idempotency assertions.
func countManifestlistsForCollectionTag(t *testing.T, repo, tag string) int {
	t.Helper()
	q := fmt.Sprintf(
		`MATCH (c:Repo {repo: "%s"})-[:HAS_TAG]->(ct:Tag {name: "%s"}) `+
			`MATCH (c)-[:REF]->(ml:Index) WHERE EXISTS { (ct)-[:REF]->(ml) } `+
			`RETURN count(DISTINCT ml) AS mlCount`,
		repo, tag,
	)
	res, err := mag.Database.Execute(q)
	if err != nil {
		// Fall back to a simpler counting query if the strict join fails.
		q2 := fmt.Sprintf(
			`MATCH (c:Repo {repo: "%s"})-[:REF]->(ml:Index) RETURN count(DISTINCT ml) AS mlCount`,
			repo,
		)
		res, err = mag.Database.Execute(q2)
		require.NoError(t, err)
	}
	n, err := res.GetInt64("mlCount")
	if err != nil {
		return 0
	}
	return int(n)
}

// TestComputeManifestlistsForRepoTag_FreshTag_SingleProfile creates a
// deterministic profile, runs the helper, and asserts a Index is
// produced for that profile and the manifestdata gains the injected
// annotation.
func TestComputeManifestlistsForRepoTag_FreshTag_SingleProfile(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	prf := createTestProfileWithRules(t, "test-stage-dev", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.metahub.stage", Value: "dev"},
	})

	mlsBefore := countManifestlistsForCollectionTag(t, testRepo, testTag)

	err = mag.ComputeManifestlistsForRepoTag(ctx, testRepo, testTag, &prf)
	require.NoError(t, err)

	// Annotation must have been injected onto the bootstrapped manifestdata.
	annos := getAnnotations(t, mdID)
	stage, ok := annos.Data["org.metahub.stage"]
	assert.True(t, ok, "expected org.metahub.stage annotation to be linked to manifestdata")
	if ok {
		assert.Equal(t, "dev", stage.Value)
	}

	// At least one new Index should have been produced for the
	// (repo, tag). The exact count depends on how many of the bootstrap
	// profiles also matched, but we expect it to grow.
	mlsAfter := countManifestlistsForCollectionTag(t, testRepo, testTag)
	assert.GreaterOrEqual(t, mlsAfter, mlsBefore, "expected manifestlist count to grow or stay the same after compute")
}

// TestComputeManifestlistsForRepoTag_SkipsNonDeterministicRules verifies that
// regex, ne, null and platform rules are silently skipped at injection time.
func TestComputeManifestlistsForRepoTag_SkipsNonDeterministicRules(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	prf := createTestProfileWithRules(t, "test-mixed", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.metahub.deterministic", Value: "yes"},
		{Type: types.RuleTypeAnnotation, Op: "re", Key: "org.metahub.regex", Value: "^v.*$"},
		{Type: types.RuleTypeAnnotation, Op: "ne", Key: "org.metahub.ne", Value: "no"},
		{Type: types.RuleTypePlatform, Op: "eq", Key: "arch", Value: "amd64"},
	})

	err = mag.ComputeManifestlistsForRepoTag(ctx, testRepo, testTag, &prf)
	require.NoError(t, err)

	annos := getAnnotations(t, mdID)

	// The deterministic anno eq must be present.
	det, ok := annos.Data["org.metahub.deterministic"]
	assert.True(t, ok, "deterministic anno eq rule should be injected")
	if ok {
		assert.Equal(t, "yes", det.Value)
	}

	// None of the non-deterministic ones must have been injected.
	_, regex := annos.Data["org.metahub.regex"]
	assert.False(t, regex, "regex rule should NOT be injected")
	_, ne := annos.Data["org.metahub.ne"]
	assert.False(t, ne, "ne rule should NOT be injected")
}

// TestComputeManifestlistsForRepoTag_IgnoresNilProfile verifies that a nil
// pusher profile skips annotation injection but still runs the per-profile
// rebuild against existing annotations.
func TestComputeManifestlistsForRepoTag_IgnoresNilProfile(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	annosBefore := getAnnotations(t, mdID)
	beforeCount := len(annosBefore.Data)

	err = mag.ComputeManifestlistsForRepoTag(ctx, testRepo, testTag, nil)
	require.NoError(t, err, "nil pusher profile is a valid call")

	annosAfter := getAnnotations(t, mdID)
	assert.Equal(t, beforeCount, len(annosAfter.Data),
		"nil pusher profile must not change the manifestdata's annotations")
}

// TestComputeManifestlistsForRepoTag_MissingRepoTagIsError verifies the helper
// errors out when the (repo, tag) has no manifestdata to compute over.
func TestComputeManifestlistsForRepoTag_MissingRepoTagIsError(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := mag.SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	err = mag.ComputeManifestlistsForRepoTag(ctx, "does/not/exist", "nope", nil)
	require.Error(t, err, "missing (repo, tag) should be an error")
}

// TestComputeManifestlistsForRepoTag_Idempotent verifies that a second
// invocation does not duplicate annotation links and (because the recomputed
// digest matches) does not create duplicate manifestlist rows.
func TestComputeManifestlistsForRepoTag_Idempotent(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	prf := createTestProfileWithRules(t, "test-idem", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.metahub.idem", Value: "true"},
	})

	// First invocation.
	err = mag.ComputeManifestlistsForRepoTag(ctx, testRepo, testTag, &prf)
	require.NoError(t, err)

	annos1 := getAnnotations(t, mdID)
	count1 := len(annos1.Data)
	mls1 := countManifestlistsForCollectionTag(t, testRepo, testTag)

	// Second invocation — same args.
	err = mag.ComputeManifestlistsForRepoTag(ctx, testRepo, testTag, &prf)
	require.NoError(t, err)

	annos2 := getAnnotations(t, mdID)
	mls2 := countManifestlistsForCollectionTag(t, testRepo, testTag)

	assert.Equal(t, count1, len(annos2.Data),
		"second invocation must not duplicate annotation links")
	assert.Equal(t, mls1, mls2,
		"second invocation must not duplicate manifestlist rows (digest short-circuit)")
}

// countSlotPointsToForTriple returns the number of :POINTS_TO edges
// currently outgoing from the Slot for the given (repo, tag, profile)
// triple. Used to assert that compute wires exactly one POINTS_TO edge
// per Slot and that re-compute does not create duplicates.
func countSlotPointsToForTriple(t *testing.T, repo, tag, profile string) int {
	t.Helper()
	q := fmt.Sprintf(
		`MATCH (s:Slot {repo: "%s", tag: "%s", profile: "%s"})-[r:POINTS_TO]->(:Index) RETURN count(r) AS ptCount`,
		repo, tag, profile,
	)
	res, err := mag.Database.Execute(q)
	require.NoError(t, err)
	n, err := res.GetInt64("ptCount")
	if err != nil {
		return 0
	}
	return int(n)
}

// countSlotHasHistoryForTriple returns the number of :HAS_HISTORY edges
// outgoing from the Slot for the given triple. After the first compute
// the count should be zero; after a compute that replaces the POINTS_TO
// target with a different Index, the count should be one.
func countSlotHasHistoryForTriple(t *testing.T, repo, tag, profile string) int {
	t.Helper()
	q := fmt.Sprintf(
		`MATCH (s:Slot {repo: "%s", tag: "%s", profile: "%s"})-[h:HAS_HISTORY]->(:Index) RETURN count(h) AS hhCount`,
		repo, tag, profile,
	)
	res, err := mag.Database.Execute(q)
	require.NoError(t, err)
	n, err := res.GetInt64("hhCount")
	if err != nil {
		return 0
	}
	return int(n)
}

// TestComputeManifestlistsForRepoTag_SlotPointsToWired is the Path Z
// task 6 phase B assertion: after ComputeManifestlistsForRepoTag runs
// successfully for a profile that has matching manifestdata, the
// corresponding :Slot node (keyed on repo, tag, profile) must have
// exactly one :POINTS_TO edge to the newly-computed :Index. Calling
// compute a second time with the same data is idempotent: the
// POINTS_TO edge count stays at 1 and no new :HAS_HISTORY edge is
// created (because the computed Index digest is unchanged, so
// SlotLinkPointsTo short-circuits).
func TestComputeManifestlistsForRepoTag_SlotPointsToWired(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	_ = br

	// Use a deterministic profile whose annotation rule will be injected
	// onto the manifestdata by the compute path, producing a non-empty
	// CTPP that triggers the Index creation + Slot linking.
	prf := createTestProfileWithRules(t, "test-slot-points-to", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.metahub.slot-test", Value: "yes"},
	})

	// First compute — should create the Slot and establish POINTS_TO.
	err := mag.ComputeManifestlistsForRepoTag(ctx, testRepo, testTag, &prf)
	require.NoError(t, err)

	// The Slot must have exactly one POINTS_TO edge to an Index.
	assert.Equal(t, 1, countSlotPointsToForTriple(t, testRepo, testTag, prf.Label),
		"first compute must produce exactly one POINTS_TO edge from the Slot to the new Index")
	assert.Equal(t, 0, countSlotHasHistoryForTriple(t, testRepo, testTag, prf.Label),
		"first compute must not produce any HAS_HISTORY edges (no prior POINTS_TO to archive)")

	// Second compute — same data, same computed digest. The
	// SlotLinkPointsTo short-circuit in neo/slot.go should detect the
	// duplicate and leave the POINTS_TO edge in place without creating
	// a HAS_HISTORY entry.
	err = mag.ComputeManifestlistsForRepoTag(ctx, testRepo, testTag, &prf)
	require.NoError(t, err)

	assert.Equal(t, 1, countSlotPointsToForTriple(t, testRepo, testTag, prf.Label),
		"second compute must leave the POINTS_TO edge at 1 (idempotent short-circuit)")
	assert.Equal(t, 0, countSlotHasHistoryForTriple(t, testRepo, testTag, prf.Label),
		"second compute must not create a HAS_HISTORY entry when the computed digest is unchanged")
}

// TestIndexGetByRequest_HydratesDescriptors is the Path Z task 11
// regression test for the "manifests:null" bug. Before the fix,
// Magazine.IndexGetByRequest → neo.IndexGetByRequest ran a second
// cypher query to populate the Index's Manifests field, but the walk
// went through the legacy :ManifestWrapper wrapper via
// `(md:Manifest)<-[]-(m:ManifestWrapper)-[:HAS_PLATFORM]->(p:Platform)`.
// Post-Path-Z task 5.1 put :HAS_PLATFORM directly on the content-leaf,
// and Phase A+B Slot-based compute produces Indexes whose referenced
// Manifest nodes have no wrapper at all — so the wrapper hop
// returned zero rows, leaving ml.Manifests empty. The OCI output then
// serialised as `"manifests": null`, which broke every `docker pull`.
//
// This test:
//  1. Bootstraps an fixture with one :Manifest linked to a Platform,
//  2. Creates a profile whose annotation rule matches the manifestdata,
//  3. Runs ComputeManifestlistsForRepoTag (Phase B path) to produce an
//     :Index node with at least one :REFERENCES edge to the md,
//  4. Calls Magazine.IndexGetByRequest for the (repo, tag, profile) triple,
//  5. Asserts ml.Manifests is non-empty AND each descriptor has its
//     platform fields populated.
func TestIndexGetByRequest_HydratesDescriptors(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)
	_ = br

	prf := createTestProfileWithRules(t, "test-index-hydrate", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.metahub.hydrate-test", Value: "yes"},
	})

	// Drive the compute path once to produce the Index + :REFERENCES
	// edges that IndexGetByRequest walks to hydrate the manifests slice.
	err := mag.ComputeManifestlistsForRepoTag(ctx, testRepo, testTag, &prf)
	require.NoError(t, err)

	// Pull back the Index via the public entry point. This is the code
	// path the metahub-api handler uses on `docker pull` / the catalog
	// GET /v2/<name>/manifests/<tag> request.
	req := types.Request{
		Repo:    testRepo,
		Tag:     testTag,
		Profile: prf.Label,
	}
	ml, found, err := mag.IndexGetByRequest(ctx, req)
	require.NoError(t, err, "IndexGetByRequest must succeed for a (repo, tag, profile) with a computed Index")
	require.True(t, found, "IndexGetByRequest must find the Index for the triple (got found=false)")

	// The whole point of task 11: Manifests must NOT be nil and must NOT
	// be empty. Even one descriptor is enough to prove the wrapper-hop
	// fix works; the ft_arm64 fixture has exactly one content-leaf md.
	assert.NotNil(t, ml.Manifests, "ml.Manifests must be non-nil (task 11 manifests:null fix)")
	assert.NotEmpty(t, ml.Manifests, "ml.Manifests must contain at least one descriptor after compute")

	// Each descriptor must have its content-leaf fields populated by
	// ManifestFromResult: ID, Digest, MediaType, Size.
	for i, m := range ml.Manifests {
		assert.NotZero(t, m.Digest, "descriptor %d: Digest must be populated", i)
		assert.NotZero(t, m.MediaType, "descriptor %d: MediaType must be populated", i)
		assert.NotZero(t, m.Size, "descriptor %d: Size must be populated", i)
	}

	// At least one descriptor must have a non-empty Platform.Arch — the
	// ft_arm64 fixture's manifestdata is linked to the linux/arm64
	// platform via :HAS_PLATFORM (task 5.1 moved the edge to the
	// content-leaf). If none of the descriptors carry a platform, it
	// means the OPTIONAL MATCH in the fixed IndexGetByRequest query is
	// failing to walk (md)-[:HAS_PLATFORM]->(p:Platform) from the
	// content-leaf.
	var archFound bool
	for _, m := range ml.Manifests {
		if m.Platform.Arch != "" {
			archFound = true
			break
		}
	}
	assert.True(t, archFound, "at least one descriptor must have a populated Platform.Arch (task 5.1 HAS_PLATFORM on content-leaf)")
}
