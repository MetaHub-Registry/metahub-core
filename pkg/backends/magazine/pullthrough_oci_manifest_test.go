package magazine

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestUpstreamBlobInsertOCIManifestType verifies that pull-through correctly
// handles an upstream that advertises application/vnd.oci.image.manifest.v1+json
// as its media type. Before the fix, OCI manifests fell through to the default
// case in UpstreamBlobInsertManifest's switch and were silently ignored; the
// upstream node was never inserted, leaving every profile pull returning 404.
func TestUpstreamBlobInsertOCIManifestType(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.SetEmptyAll()

	ctx := context.Background()
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	// Present the alpine:3.17.3 manifest as OCI v1. UpstreamBlobInsertManifest
	// routes both Docker v2 and OCI v1 to the same upstreamBlobInsert path;
	// this test guards that the OCI case is not missing from the switch.
	const repo = "alpine"
	const tag = "3.17.3"
	req := types.Request{
		Host:    regHttpURI,
		Repo:    repo,
		Tag:     tag,
		Profile: "generic",
	}
	blob := types.BlobMeta{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    shab5a,
	}
	require.NoError(t, mag.UpstreamBlobInsertManifest(ctx, req, blob),
		"OCI manifest media type must be handled — not rejected or silently dropped")

	// The upstream node must exist in the graph after ingestion.
	upQ := fmt.Sprintf(
		`MATCH (u:Upstream) WHERE u.repo = "%s" AND u.tag = "%s" RETURN count(u) AS n`,
		repo, tag)
	upRes, upFound, err := mag.Database.QueryOne(upQ)
	require.NoError(t, err)
	require.True(t, upFound)
	n, err := upRes.GetInt64("n")
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, int64(1),
		"Upstream node must be created for OCI manifest pull-through")

	// The manifest must be linked to the upstream via an ORIGINAL edge,
	// proving the full ingest path (not just a no-op routing) ran.
	origQ := fmt.Sprintf(
		`MATCH (md:Manifest)-[:ORIGINAL]->(u:Upstream) WHERE u.repo = "%s" AND u.tag = "%s" RETURN id(md) AS mdID`,
		repo, tag)
	origRes, origFound, err := mag.Database.QueryOne(origQ)
	require.NoError(t, err)
	require.True(t, origFound,
		"manifest must be wired to upstream via :ORIGINAL edge")
	mdID, err := origRes.GetInt64("mdID")
	require.NoError(t, err)
	require.NotZero(t, mdID)
}
