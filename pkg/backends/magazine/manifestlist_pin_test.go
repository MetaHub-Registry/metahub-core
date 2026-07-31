package magazine

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"go.opentelemetry.io/otel/trace"
)

// testLogSpan returns a discard logger and the no-op span carried by the
// package-global test context, suitable for calling internal helpers that
// take (l, span) purely for tracing.
func testLogSpan() (*logrus.Entry, trace.Span) {
	lg := logrus.New()
	lg.SetOutput(logrus.StandardLogger().Out)
	return logrus.NewEntry(lg), trace.SpanFromContext(ctx)
}

// TestGatherCollectionPins_ExplicitPinRecorded is the DB-walk half of the
// explicit-pin-exclusivity fix: an explicit collection `profile:` binding
// (BindManifestToProfile, which creates a Slot{repo,tag,profile≠generic}
// -[:HAS_PUSHED]-> md) must be gathered as a collection pin so FilterByCTPP
// can keep the manifest out of every OTHER profile's computed index.
func TestGatherCollectionPins_ExplicitPinRecorded(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	// An explicit `profile:` binding, exactly as a collection entry with
	// `profile: frame` would produce on parse.
	prf := createTestProfileWithRules(t, "gpu-frame-pin", []types.Rule{
		{Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.metahub.pin-test", Value: "frame"},
	})
	require.NoError(t, mag.BindManifestToProfile(ctx, testRepo, testTag, prf.Label, mdID))

	l, span := testLogSpan()
	pins := mag.gatherCollectionPins(ctx, l, span, testRepo, testTag)

	found := false
	for _, p := range pins {
		assert.NotEqual(t, "generic", p.profile, "generic slots must never surface as pins")
		if p.mdID == mdID && p.profile == prf.Label {
			found = true
		}
	}
	assert.True(t, found, "explicit profile bind must be gathered as a collection pin (%s -> md %d)", prf.Label, mdID)
}

// TestGatherCollectionPins_GenericOnlyYieldsNoPins guards the no-regression
// half: a manifest pushed only under the default "generic" profile is the
// non-exclusive multi-arch case and must NOT be recorded as a pin, so it
// keeps the historical rule-union index membership across profiles.
func TestGatherCollectionPins_GenericOnlyYieldsNoPins(t *testing.T) {
	bs, br := bootstrapFtArm64(t)
	defer TeardownTest(bs, br)

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	// A generic Slot with the manifest pushed under it — the default,
	// non-exclusive path every plain push takes.
	slot, err := mag.Database.SlotGetCreate(ctx, testRepo, testTag, "generic")
	require.NoError(t, err)
	require.NoError(t, mag.Database.SlotAppendPushedManifest(ctx, slot.ID, mdID, ""))

	l, span := testLogSpan()
	pins := mag.gatherCollectionPins(ctx, l, span, testRepo, testTag)

	for _, p := range pins {
		assert.NotEqualf(t, mdID, p.mdID,
			"generic-pushed manifest %d must not be gathered as an exclusive pin (got profile %q)", mdID, p.profile)
	}
}
