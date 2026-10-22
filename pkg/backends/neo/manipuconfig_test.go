package neo

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManipuConfigRoundTrip verifies that a Manipulation with a Default
// block and two rule-keyed variants (no layers) round-trips through the
// graph. Layers are tested separately because they require an existing
// Upstream+Manifest.
func TestManipuConfigRoundTrip(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	mp := types.Manipulation{
		Label: "test-roundtrip#1",
		Default: types.ManipuConfig{
			Env:     map[string]string{"FOO": "bar", "PATH": "/opt/bin"},
			Workdir: "/opt/ws",
			User:    "1000:1000",
			Volumes: []string{"/data", "/var/log"},
			Entrypoint: &types.ManipuCommand{
				Op:    types.ManipuCmdOpReplace,
				Value: []string{"/bin/entry.sh"},
			},
			Cmd: &types.ManipuCommand{
				Op:    types.ManipuCmdOpAppend,
				Value: []string{"--verbose", "--log=/var/log/app.log"},
			},
		},
		Variants: []types.ManipuVariant{
			{
				Rule: "arch:arm64=true",
				Config: types.ManipuConfig{
					Env: map[string]string{"ARCH": "arm64"},
				},
			},
			{
				Rule: "arch:amd64=true",
				Config: types.ManipuConfig{
					Env: map[string]string{"ARCH": "amd64"},
					Cmd: &types.ManipuCommand{Op: types.ManipuCmdOpPrepend, Value: []string{"perf"}},
				},
			},
		},
	}

	err = db.ManipulationCreate(ctx, &mp)
	require.NoError(t, err)
	// Neo4j node IDs can be 0 for the first node created in a fresh graph.
	require.GreaterOrEqual(t, mp.ID, int64(0))

	got, found, err := db.ManipulationGetByID(ctx, mp.ID)
	require.NoError(t, err)
	require.True(t, found)

	assert.Equal(t, mp.Label, got.Label)
	assert.Equal(t, "/opt/ws", got.Default.Workdir)
	assert.Equal(t, "1000:1000", got.Default.User)
	assert.Equal(t, []string{"/data", "/var/log"}, got.Default.Volumes)
	assert.Equal(t, "bar", got.Default.Env["FOO"])
	assert.Equal(t, "/opt/bin", got.Default.Env["PATH"])
	require.NotNil(t, got.Default.Entrypoint)
	assert.Equal(t, types.ManipuCmdOpReplace, got.Default.Entrypoint.Op)
	assert.Equal(t, []string{"/bin/entry.sh"}, got.Default.Entrypoint.Value)
	require.NotNil(t, got.Default.Cmd)
	assert.Equal(t, types.ManipuCmdOpAppend, got.Default.Cmd.Op)

	require.Len(t, got.Variants, 2)
	// Order isn't guaranteed — index by rule.
	variantByRule := map[string]types.ManipuVariant{}
	for _, v := range got.Variants {
		variantByRule[v.Rule] = v
	}
	require.Contains(t, variantByRule, "arch:arm64=true")
	require.Contains(t, variantByRule, "arch:amd64=true")
	assert.Equal(t, "arm64", variantByRule["arch:arm64=true"].Config.Env["ARCH"])
	require.NotNil(t, variantByRule["arch:amd64=true"].Config.Cmd)
	assert.Equal(t, types.ManipuCmdOpPrepend, variantByRule["arch:amd64=true"].Config.Cmd.Op)
}

// TestManipuConfigEmptyDefaultOnlyVariants persists a manipulation with an
// empty Default and two variants, verifying only HAS_VARIANT edges are
// written.
func TestManipuConfigEmptyDefaultOnlyVariants(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	mp := types.Manipulation{
		Label: "test-varonly#1",
		Variants: []types.ManipuVariant{
			{Rule: "arch:arm64=true", Config: types.ManipuConfig{Workdir: "/opt/arm"}},
			{Rule: "arch:amd64=true", Config: types.ManipuConfig{Workdir: "/opt/amd"}},
		},
	}
	require.NoError(t, db.ManipulationCreate(ctx, &mp))
	got, found, err := db.ManipulationGetByID(ctx, mp.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, got.Default.IsZero())
	require.Len(t, got.Variants, 2)
}

// TestManipulationsWithConfigByProfileID builds a Profile->USES->Manipulation
// link and verifies the helper returns the hydrated tree.
func TestManipulationsWithConfigByProfileID(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// Create a Profile.
	prf := types.Profile{Name: "cfg-test", Tag: "v1", Rule: ""}
	prfID, err := db.CreateProfile(prf)
	require.NoError(t, err)

	// Create a Manipulation with default + variant.
	mp := types.Manipulation{
		Label: "test-linked#1",
		Default: types.ManipuConfig{
			Workdir: "/opt/default",
		},
		Variants: []types.ManipuVariant{
			{Rule: "arch:arm64=true", Config: types.ManipuConfig{Workdir: "/opt/arm"}},
		},
	}
	require.NoError(t, db.ManipulationCreate(ctx, &mp))

	// Link profile to manipulation.
	require.NoError(t, db.ProfileLinkToManipulation(ctx, prfID, mp.ID))

	got, err := db.ManipulationsWithConfigByProfileID(ctx, prfID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "test-linked#1", got[0].Label)
	assert.Equal(t, "/opt/default", got[0].Default.Workdir)
	require.Len(t, got[0].Variants, 1)
	assert.Equal(t, "arch:arm64=true", got[0].Variants[0].Rule)
	assert.Equal(t, "/opt/arm", got[0].Variants[0].Config.Workdir)

	// The natural-key lookup must return the same manipulation.
	byLabel, err := db.ManipulationsWithConfigByProfileLabel(ctx, "cfg-test#v1")
	require.NoError(t, err)
	require.Len(t, byLabel, 1)
	assert.Equal(t, "test-linked#1", byLabel[0].Label)
	assert.Equal(t, "/opt/default", byLabel[0].Default.Workdir)
}

// TestManipulationsWithConfigByProfileID_UnknownProfile verifies the helper
// returns an empty slice (no error) when the profile does not exist.
func TestManipulationsWithConfigByProfileID_UnknownProfile(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	got, err := db.ManipulationsWithConfigByProfileID(ctx, int64(-9999))
	require.NoError(t, err)
	assert.Empty(t, got)
}
