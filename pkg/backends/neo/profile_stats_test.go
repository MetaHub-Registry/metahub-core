package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestProfileGetByLabelWithStats_UnknownLabel(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	ps, found, err := db.ProfileGetByLabelWithStats(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, int64(0), ps.CollectionCount)
	assert.Equal(t, int64(0), ps.ManifestCount)
}

func TestProfileGetByLabelWithStats_NoSlots(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	// Create a profile with no slots
	p := types.Profile{Name: "cpu", Tag: "test", Rule: ""}
	_, err = db.CreateProfile(p)
	require.NoError(t, err)

	ps, found, err := db.ProfileGetByLabelWithStats(ctx, "cpu#test")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "cpu", ps.Name)
	assert.Equal(t, "test", ps.Tag)
	assert.Equal(t, int64(0), ps.CollectionCount)
	assert.Equal(t, int64(0), ps.ManifestCount)
}

func TestProfileGetByLabelWithStats_WithRules(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	// Create profile and link rules
	p := types.Profile{Name: "cpu", Tag: "zen3"}
	prfID, err := db.CreateProfile(p)
	require.NoError(t, err)

	r := types.Rule{Name: "target", Tag: "microarch", Type: types.RuleTypeAnnotation, Op: "eq", Key: "org.supercontainers.hardware.cpu.target.microarch", Value: "zen3"}
	ruleID, err := db.RuleCreate(ctx, r)
	require.NoError(t, err)

	err = db.ProfileLinkRule(ctx, prfID, ruleID)
	require.NoError(t, err)

	ps, found, err := db.ProfileGetByLabelWithStats(ctx, "cpu#zen3")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "cpu#zen3", ps.Label)
	require.Len(t, ps.Rules, 1)
	assert.Equal(t, "eq", ps.Rules[0].Op)
	assert.Equal(t, "org.supercontainers.hardware.cpu.target.microarch", ps.Rules[0].Key)
	assert.Equal(t, "zen3", ps.Rules[0].Value)
}

func TestProfileGetByLabelWithStats_GenericLabel(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	// Create a "generic" profile (empty name → label is just the tag)
	p := types.Profile{Name: "", Tag: "generic"}
	_, err = db.CreateProfile(p)
	require.NoError(t, err)

	ps, found, err := db.ProfileGetByLabelWithStats(ctx, "generic")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "generic", ps.Label)
	assert.Equal(t, "", ps.Name)
	assert.Equal(t, "generic", ps.Tag)
}
