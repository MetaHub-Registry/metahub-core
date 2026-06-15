package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// TestBindManifestToProfile_InjectsAnnotation exercises the
// collection-profile-authority binding: when a collection entry names a
// profile, BindManifestToProfile injects that profile's deterministic
// ("anno eq") rules onto the manifest so it becomes matchable under the
// profile. This is the mechanism that makes the zen2 image servable
// under `cpu#zen2`.
func TestBindManifestToProfile_InjectsAnnotation(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	// A profile mirroring cpu#zen2: a single deterministic anno-eq rule.
	prf := createTestProfileWithRules(t, "cpu-zen2bind", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq",
			Key: "org.supercontainers.hardware.cpu.target.microarch", Value: "zen2"},
	})

	// Pre-condition: the manifest does not yet carry the microarch annotation.
	before := getAnnotations(t, mdID)
	_, had := before.Data["org.supercontainers.hardware.cpu.target.microarch"]
	require.False(t, had, "fixture manifest must start without the microarch annotation")

	err = mag.BindManifestToProfile(ctx, testRepo, testTag, prf.Label, mdID)
	require.NoError(t, err)

	// Post: the deterministic rule is injected as an annotation.
	after := getAnnotations(t, mdID)
	anno, ok := after.Data["org.supercontainers.hardware.cpu.target.microarch"]
	assert.True(t, ok, "BindManifestToProfile must inject the microarch annotation")
	if ok {
		assert.Equal(t, "zen2", anno.Value)
	}

	// And the profile is now discoverable for that manifest via the
	// annotation-matching path.
	prfs, err := neoDB(t).ProfileFromManifestIDViaAnnotations(ctx, mdID)
	require.NoError(t, err)
	found := false
	for _, p := range prfs {
		if p.Label == prf.Label {
			found = true
		}
	}
	assert.True(t, found, "manifest should match the bound profile via annotations")
}

// TestBindManifestToProfile_ScopedToBoundProfile is the regression
// guard for the zen2/zen3 cross-contamination bug: binding a manifest
// to profile A must inject ONLY A's annotation, never a sibling
// profile B's. The original bug came from the admin parse loop calling
// the broadcast ComputeManifestlistsForRepoTag once per declared
// profile (each broadcasting its annotation onto every manifest in the
// tag); BindManifestToProfile itself must stay strictly scoped.
func TestBindManifestToProfile_ScopedToBoundProfile(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	// Two sibling microarch profiles, like cpu#zen2 and cpu#zen3.
	prfA := createTestProfileWithRules(t, "cpu-zen2scope", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq",
			Key: "org.supercontainers.hardware.cpu.target.microarch", Value: "zen2"},
	})
	_ = createTestProfileWithRules(t, "cpu-zen3scope", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq",
			Key: "org.supercontainers.hardware.cpu.target.microarch", Value: "zen3"},
	})

	// Bind the manifest to ONLY profile A (zen2).
	err = mag.BindManifestToProfile(ctx, testRepo, testTag, prfA.Label, mdID)
	require.NoError(t, err)

	annos := getAnnotations(t, mdID)
	microarch, ok := annos.Data["org.supercontainers.hardware.cpu.target.microarch"]
	require.True(t, ok, "the bound profile's annotation must be present")
	assert.Equal(t, "zen2", microarch.Value,
		"only the bound profile's value (zen2) must be present — NOT zen3")
}

// TestBindManifestToProfile_EmptyIsNoop asserts an empty profile label
// binds nothing and is not an error (the caller falls back to the
// generic / image-annotation path).
func TestBindManifestToProfile_EmptyIsNoop(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	before := len(getAnnotations(t, mdID).Data)
	err = mag.BindManifestToProfile(ctx, testRepo, testTag, "", mdID)
	require.NoError(t, err)
	after := len(getAnnotations(t, mdID).Data)
	assert.Equal(t, before, after, "empty profile must not change annotations")
}

// TestBindManifestToProfile_UnknownProfileErrors asserts a mis-authored
// collection profile label fails loudly instead of silently binding
// nothing.
func TestBindManifestToProfile_UnknownProfileErrors(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	err = mag.BindManifestToProfile(ctx, testRepo, testTag, "cpu#zne2-typo", mdID)
	require.Error(t, err, "unknown profile label must error")
	assert.Contains(t, err.Error(), "does not resolve")
}
