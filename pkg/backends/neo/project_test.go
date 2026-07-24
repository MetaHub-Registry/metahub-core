package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test seeding helpers (raw Cypher on the shared package-level `db`) ---

func projRunCypher(t *testing.T, q string, params map[string]interface{}) {
	t.Helper()
	session := db.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	res, err := session.Run(q, params)
	require.NoError(t, err)
	_, err = res.Consume()
	require.NoError(t, err)
}

// projSeedManifest creates Repo->Tag->Index->Manifest for the given repo + digest.
func projSeedManifest(t *testing.T, repo, mani string) {
	t.Helper()
	projRunCypher(t, `
		MERGE (r:Repo {repo: $repo})
		MERGE (tg:Tag {name: "latest", repo: $repo})
		MERGE (r)-[:HAS_TAG]->(tg)
		MERGE (i:Index {digest: $idx})
		MERGE (tg)-[:REF]->(i)
		MERGE (m:Manifest {digest: $mani})
		MERGE (i)-[:REFERENCES]->(m)`,
		map[string]interface{}{"repo": repo, "idx": "idx-" + mani, "mani": mani})
}

// projLinkBlob MERGEs a blob (shared globally by digest) and links it to a manifest.
func projLinkBlob(t *testing.T, mani, digest string, size int64, typ, rel string) {
	t.Helper()
	projRunCypher(t, fmt.Sprintf(`
		MERGE (b:Blob {digest: $digest}) ON CREATE SET b.size = $size, b.type = $typ
		WITH b
		MATCH (m:Manifest {digest: $mani})
		MERGE (m)-[:%s]->(b)`, rel),
		map[string]interface{}{"digest": digest, "size": size, "typ": typ, "mani": mani})
}

func projUnlinkBlob(t *testing.T, mani, digest, rel string) {
	t.Helper()
	projRunCypher(t, fmt.Sprintf(`MATCH (m:Manifest {digest: $mani})-[r:%s]->(:Blob {digest: $digest}) DELETE r`, rel),
		map[string]interface{}{"mani": mani, "digest": digest})
}

func projBlobExists(t *testing.T, digest string) bool {
	t.Helper()
	session := db.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()
	res, err := session.Run(`MATCH (b:Blob {digest: $d}) RETURN count(b) AS c`, map[string]interface{}{"d": digest})
	require.NoError(t, err)
	if res.Next() {
		if v, ok := res.Record().Get("c"); ok {
			c, _ := v.(int64)
			return c > 0
		}
	}
	return false
}

// --- tests ---

func TestProjectCRUD(t *testing.T) {
	ctx := context.Background()
	team, project := "qteam-crud", "proj1"
	defer db.ProjectDelete(ctx, team, project)

	p, err := db.ProjectCreate(ctx, team, project, 1000)
	require.NoError(t, err)
	assert.Equal(t, team, p.TeamSlug)
	assert.Equal(t, project, p.ProjectSlug)
	assert.Equal(t, int64(1000), p.QuotaBytes)
	assert.Equal(t, "qteam-crud/proj1", p.Namespace())

	got, found, err := db.ProjectGet(ctx, team, project)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int64(1000), got.QuotaBytes)

	require.NoError(t, db.ProjectUpdateQuota(ctx, team, project, 2000))
	got, _, _ = db.ProjectGet(ctx, team, project)
	assert.Equal(t, int64(2000), got.QuotaBytes)

	found, err = db.ProjectDelete(ctx, team, project)
	require.NoError(t, err)
	assert.True(t, found)

	_, found, err = db.ProjectGet(ctx, team, project)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestProjectUsageDistinctWithinProject(t *testing.T) {
	ctx := context.Background()
	team, project := "qteam-distinct", "p"
	defer db.ProjectDelete(ctx, team, project)
	base := team + "/" + project
	// two manifests in the same project referencing the SAME blob → counted once
	projSeedManifest(t, base+"/img", "sha256:distinctm1")
	projSeedManifest(t, base+"/img2", "sha256:distinctm2")
	projLinkBlob(t, "sha256:distinctm1", "sha256:distinctblob", 500, "layer", "HAS_LAYER")
	projLinkBlob(t, "sha256:distinctm2", "sha256:distinctblob", 500, "layer", "HAS_LAYER")
	_, err := db.ProjectCreate(ctx, team, project, 10000)
	require.NoError(t, err)

	usage, err := db.ProjectUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, int64(500), usage)
}

func TestProjectUsageSharedAcrossProjectsUnlinkAndDelete(t *testing.T) {
	ctx := context.Background()
	teamA, teamB, proj := "qteam-a", "qteam-b", "p"
	defer db.ProjectDelete(ctx, teamA, proj)
	defer db.ProjectDelete(ctx, teamB, proj)

	projSeedManifest(t, teamA+"/"+proj+"/img", "sha256:shareA")
	projSeedManifest(t, teamB+"/"+proj+"/img", "sha256:shareB")
	projLinkBlob(t, "sha256:shareA", "sha256:sharedblob", 800, "layer", "HAS_LAYER")
	projLinkBlob(t, "sha256:shareB", "sha256:sharedblob", 800, "layer", "HAS_LAYER")
	_, err := db.ProjectCreate(ctx, teamA, proj, 10000)
	require.NoError(t, err)
	_, err = db.ProjectCreate(ctx, teamB, proj, 10000)
	require.NoError(t, err)

	// shared blob counted fully in each project
	uA, err := db.ProjectUsageBytes(ctx, teamA, proj)
	require.NoError(t, err)
	uB, err := db.ProjectUsageBytes(ctx, teamB, proj)
	require.NoError(t, err)
	assert.Equal(t, int64(800), uA)
	assert.Equal(t, int64(800), uB)

	// unlink A's reference → A drops to 0, B unchanged
	projUnlinkBlob(t, "sha256:shareA", "sha256:sharedblob", "HAS_LAYER")
	uA, err = db.ProjectUsageBytes(ctx, teamA, proj)
	require.NoError(t, err)
	uB, err = db.ProjectUsageBytes(ctx, teamB, proj)
	require.NoError(t, err)
	assert.Equal(t, int64(0), uA)
	assert.Equal(t, int64(800), uB)

	// delete project A → the shared blob remains, B unaffected
	found, err := db.ProjectDelete(ctx, teamA, proj)
	require.NoError(t, err)
	assert.True(t, found)
	assert.True(t, projBlobExists(t, "sha256:sharedblob"))
	uB, err = db.ProjectUsageBytes(ctx, teamB, proj)
	require.NoError(t, err)
	assert.Equal(t, int64(800), uB)
}

func TestProjectUsageExcludesNegativeSize(t *testing.T) {
	ctx := context.Background()
	team, project := "qteam-neg", "p"
	defer db.ProjectDelete(ctx, team, project)
	projSeedManifest(t, team+"/"+project+"/img", "sha256:negm")
	projLinkBlob(t, "sha256:negm", "sha256:goodblob", 300, "layer", "HAS_LAYER")
	projLinkBlob(t, "sha256:negm", "sha256:unknownblob", -1, "config", "HAS_CONFIG")

	usage, err := db.ProjectUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, int64(300), usage)
}

func TestProjectQuotaSum(t *testing.T) {
	ctx := context.Background()
	before, err := db.ProjectQuotaSum(ctx)
	require.NoError(t, err)
	defer db.ProjectDelete(ctx, "qteam-sum", "p")
	_, err = db.ProjectCreate(ctx, "qteam-sum", "p", 1234)
	require.NoError(t, err)
	after, err := db.ProjectQuotaSum(ctx)
	require.NoError(t, err)
	assert.Equal(t, before+1234, after)
}

func TestBlobRefCount(t *testing.T) {
	ctx := context.Background()
	projSeedManifest(t, "qteam-ref/p/img", "sha256:refm1")
	projSeedManifest(t, "qteam-ref/p/img2", "sha256:refm2")
	projLinkBlob(t, "sha256:refm1", "sha256:refblob", 10, "layer", "HAS_LAYER")
	n, err := db.BlobRefCount(ctx, "sha256:refblob")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	projLinkBlob(t, "sha256:refm2", "sha256:refblob", 10, "layer", "HAS_LAYER")
	n, err = db.BlobRefCount(ctx, "sha256:refblob")
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}
