package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// createMinimalManifest spins up just enough of a Manifest node to
// exercise ProfileInjectDeterministicAnnotations. It bypasses the full
// createManifest helper to avoid the Config/Platform prerequisites — this
// isolated test only cares about the (Manifest)-[:HAS_ANNOTATION]->()
// edges the helper under test creates.
func createMinimalManifest(t *testing.T, digest string) int64 {
	t.Helper()
	q := fmt.Sprintf(
		`CREATE (m:Manifest {digest: "%s", mediaType: "application/vnd.oci.image.manifest.v1+json", size: 0, label: "%s"}) RETURN id(m) AS manifestID`,
		digest, digest,
	)
	res, err := db.Execute(q)
	require.NoError(t, err)
	mdID, err := res.GetInt64("manifestID")
	require.NoError(t, err)
	return mdID
}

// createProfileWithRules creates a profile and links a set of rules to it.
// The rules are created via db.RuleCreate and linked via db.ProfileLinkRule
// so they go through the same code path as any production rule. Each rule
// is given a per-index unique Name/Tag so the derived `label` property
// respects the neo4j UNIQUE constraint on Rule.label (see setupConstraints
// in pkg/backends/neo/main.go).
func createProfileWithRules(t *testing.T, profileName string, rules []types.Rule) int64 {
	t.Helper()
	ctx := context.Background()
	prfID, err := db.CreateProfile(types.Profile{Name: profileName, Tag: "v1"})
	require.NoError(t, err)
	for i, r := range rules {
		if r.Name == "" {
			r.Name = fmt.Sprintf("%s-r%d", profileName, i)
		}
		if r.Tag == "" {
			r.Tag = "v1"
		}
		rID, err := db.RuleCreate(ctx, r)
		require.NoError(t, err)
		err = db.ProfileLinkRule(ctx, prfID, rID)
		require.NoError(t, err)
	}
	return prfID
}

func TestProfileInjectDeterministicAnnotations_OnlyAnnoEq(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	prfID := createProfileWithRules(t, "prf-onlyeq", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.metahub.release", Value: "stable"},
		{Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.metahub.team", Value: "platform"},
	})
	mdID := createMinimalManifest(t, "sha256:onlyeq")

	injected, skipped, err := db.ProfileInjectDeterministicAnnotations(ctx, prfID, mdID)
	require.NoError(t, err)
	assert.Equal(t, 2, injected)
	assert.Equal(t, 0, skipped)

	as, err := db.GetAnnotationsByManifestID(ctx, mdID)
	require.NoError(t, err)
	assert.Len(t, as.Data, 2)
	assert.Equal(t, "stable", as.Data["org.metahub.release"].Value)
	assert.Equal(t, "platform", as.Data["org.metahub.team"].Value)
}

func TestProfileInjectDeterministicAnnotations_MixedRuleTypesAndOps(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	prfID := createProfileWithRules(t, "prf-mixed", []types.Rule{
		// deterministic
		{Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.metahub.release", Value: "stable"},
		// non-deterministic — regex
		{Type: types.RuleTypeAnnotation, Op: "re", Key: "org.metahub.version", Value: "^v.*$"},
		// non-deterministic — platform rule
		{Type: types.RuleTypePlatform, Op: "eq", Key: "arch", Value: "amd64"},
		// non-deterministic — anno ne
		{Type: types.RuleTypeAnnotation, Op: "ne", Key: "org.metahub.test", Value: "true"},
	})
	mdID := createMinimalManifest(t, "sha256:mixed")

	injected, skipped, err := db.ProfileInjectDeterministicAnnotations(ctx, prfID, mdID)
	require.NoError(t, err)
	assert.Equal(t, 1, injected, "only the single anno eq rule should be injected")
	assert.Equal(t, 3, skipped, "the three non-deterministic rules should be counted as skipped")

	as, err := db.GetAnnotationsByManifestID(ctx, mdID)
	require.NoError(t, err)
	assert.Len(t, as.Data, 1, "only one annotation should exist on the manifestdata")
	assert.Equal(t, "stable", as.Data["org.metahub.release"].Value)
}

func TestProfileInjectDeterministicAnnotations_NoDeterministicRules(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	prfID := createProfileWithRules(t, "prf-nonedet", []types.Rule{
		{Type: types.RuleTypePlatform, Op: "eq", Key: "arch", Value: "amd64"},
		{Type: types.RuleTypeAnnotation, Op: "re", Key: "org.metahub.version", Value: "^v.*$"},
	})
	mdID := createMinimalManifest(t, "sha256:nonedet")

	injected, skipped, err := db.ProfileInjectDeterministicAnnotations(ctx, prfID, mdID)
	require.NoError(t, err, "no deterministic rules is not an error")
	assert.Equal(t, 0, injected)
	assert.Equal(t, 2, skipped)

	as, err := db.GetAnnotationsByManifestID(ctx, mdID)
	require.NoError(t, err)
	assert.Len(t, as.Data, 0, "no annotations should have been linked")
}

func TestProfileInjectDeterministicAnnotations_Idempotent(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	prfID := createProfileWithRules(t, "prf-idem", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.metahub.release", Value: "stable"},
	})
	mdID := createMinimalManifest(t, "sha256:idem")

	// First call
	injected, skipped, err := db.ProfileInjectDeterministicAnnotations(ctx, prfID, mdID)
	require.NoError(t, err)
	assert.Equal(t, 1, injected)
	assert.Equal(t, 0, skipped)

	// Second call — should not create duplicate annotations or edges.
	injected2, skipped2, err := db.ProfileInjectDeterministicAnnotations(ctx, prfID, mdID)
	require.NoError(t, err)
	assert.Equal(t, 1, injected2, "counter reflects rules processed, not edges created")
	assert.Equal(t, 0, skipped2)

	// Verify the underlying graph: there should still be exactly one
	// annotation linked to the manifestdata.
	as, err := db.GetAnnotationsByManifestID(ctx, mdID)
	require.NoError(t, err)
	assert.Len(t, as.Data, 1, "no duplicate annotations after re-injection")
	assert.Equal(t, "stable", as.Data["org.metahub.release"].Value)
}

func TestProfileInjectDeterministicAnnotations_EmptyProfile(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	prfID := createProfileWithRules(t, "prf-empty", nil)
	mdID := createMinimalManifest(t, "sha256:empty")

	injected, skipped, err := db.ProfileInjectDeterministicAnnotations(ctx, prfID, mdID)
	require.NoError(t, err)
	assert.Equal(t, 0, injected)
	assert.Equal(t, 0, skipped)
}
