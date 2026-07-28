package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// setupProjectNS resets the graph and returns a teardown func:
//
//	defer setupProjectNS(t)()
func setupProjectNS(t *testing.T) func() {
	t.Helper()
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	return func() { db.TeardownTest(bs, br) }
}

func TestProjectVisibilityRoundTrip(t *testing.T) {
	defer setupProjectNS(t)()
	ctx := context.Background()

	_, err := db.ProjectCreate(ctx, "acme", "web", 1000)
	require.NoError(t, err)

	// defaults: private / project / no owner
	p, found, err := db.ProjectGet(ctx, "acme", "web")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, types.VisibilityPrivate, p.Visibility)
	assert.Equal(t, types.KindProject, p.Kind)
	assert.Equal(t, "", p.Owner)

	// set public → round-trips
	found, err = db.ProjectSetVisibility(ctx, "acme", "web", types.VisibilityPublic)
	require.NoError(t, err)
	assert.True(t, found)
	p, _, _ = db.ProjectGet(ctx, "acme", "web")
	assert.Equal(t, types.VisibilityPublic, p.Visibility)

	// invalid value rejected
	_, err = db.ProjectSetVisibility(ctx, "acme", "web", "bogus")
	assert.Error(t, err)

	// missing project → found=false, no error
	found, err = db.ProjectSetVisibility(ctx, "acme", "none", types.VisibilityPublic)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestPersonalNamespaceGetOrCreate(t *testing.T) {
	defer setupProjectNS(t)()
	ctx := context.Background()
	const fiveGi = int64(5) << 30

	p, err := db.PersonalNamespaceGetOrCreate(ctx, "alice", fiveGi)
	require.NoError(t, err)
	assert.Equal(t, types.PersonalTeamSlug, p.TeamSlug)
	assert.Equal(t, "alice", p.ProjectSlug)
	assert.Equal(t, "user/alice", p.Namespace())
	assert.Equal(t, types.KindPersonal, p.Kind)
	assert.Equal(t, "alice", p.Owner)
	assert.Equal(t, types.VisibilityPrivate, p.Visibility)
	assert.Equal(t, fiveGi, p.QuotaBytes)

	// idempotent: a repeat call with a different quota returns the same node and
	// MUST NOT reset the quota.
	p2, err := db.PersonalNamespaceGetOrCreate(ctx, "alice", 123)
	require.NoError(t, err)
	assert.Equal(t, p.ID, p2.ID)
	assert.Equal(t, fiveGi, p2.QuotaBytes)
}

func TestProjectVisibilityMigration(t *testing.T) {
	defer setupProjectNS(t)()
	ctx := context.Background()
	// a pre-authz :Project node has no visibility/kind/owner properties.
	projRunCypher(t, `CREATE (p:Project {team_slug: $t, project_slug: $p, quota_bytes: 10, created_at: 1})`,
		map[string]interface{}{"t": "legacy", "p": "proj"})
	p, found, err := db.ProjectGet(ctx, "legacy", "proj")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, types.VisibilityPrivate, p.Visibility, "legacy node reads as private")
	assert.Equal(t, types.KindProject, p.Kind, "legacy node reads as project")
	assert.Equal(t, "", p.Owner)
}
