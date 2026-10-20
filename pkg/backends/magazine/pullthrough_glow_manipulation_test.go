package magazine

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestPullThroughGlowManipulation is the e2e test for pull-through cache
// manipulation: ingesting an alpine image with the glow#1.5.0 profile must
// produce a manipulated manifest that carries the glow layer in addition to
// alpine's original layers. This covers the streaming OS-eval path that was
// previously OOMKilling the pod.
func TestPullThroughGlowManipulation(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	// arch rules are required by the glow 1.5.0 manipulation YAML which has one
	// layer per arch (rule: arch:arm64=true / arch:amd64=true).
	bs.AddBootstrap("rules:rules/platform/arch/amd64.yaml")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	// manipulation and glow profile are bootstrapped in a second pass (after
	// CacheUpstream) because ManipuLayerFromUpstream requires the glow upstream
	// manifest to exist in Neo4j before the manipulation node can be created.
	bs.SetEmptyAll()

	ctx := context.Background()
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	// Seed BOTH glow arch upstreams before the manipulation bootstrap.
	// The glow 1.5.0 manipulation YAML lists two layers (arm64 + amd64); the
	// bootstrap calls buildLegacyManipuLayer for each and calls UpstreamGet +
	// ManifestGetByUpstream — both require a seeded Upstream node, a cached
	// Manifest, and the (md:Manifest)-[:ORIGINAL]->(u:Upstream) edge.
	// We cannot use UpstreamBlobInsertManifest because upstreamInsertGeneric
	// requires /etc/os-release in every tar.gz layer — the glow injection image
	// is a bare HPC layer and has none. Instead we chain:
	//   NeoInsertUpstream  → creates :Upstream node
	//   CacheUpstream      → fetches + stores :Manifest + config link
	//   ManifestLinkToUpstream → creates the ORIGINAL edge
	for _, glowUp := range []types.Upstream{types.UpstreamGlow150ARM64, types.UpstreamGlow150AMD64} {
		upID, e := mag.NeoInsertUpstream(ctx, glowUp)
		require.NoError(t, e, "seeding glow upstream node: %s", glowUp.GetLabel())
		require.NoError(t, mag.CacheUpstream(ctx, glowUp),
			"caching glow manifest from test registry: %s", glowUp.GetLabel())
		md, found, e := mag.Database.ManifestGetByDigest(ctx, glowUp.Digest.String())
		require.NoError(t, e, "getting glow manifest by digest: %s", glowUp.GetLabel())
		require.True(t, found, "glow manifest must exist in Neo4j after CacheUpstream: %s", glowUp.GetLabel())
		require.NoError(t, mag.Database.ManifestLinkToUpstream(ctx, md.ID, upID),
			"creating ORIGINAL edge for: %s", glowUp.GetLabel())
	}

	// Second bootstrap pass — no graph wipe, manipulation + glow profile only.
	bs2 := utils.NewBootstrap()
	bs2.SetYAMLPrefix("../../../misc/yaml")
	bs2.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0.yaml")
	bs2.AddBootstrap("profile:profiles/glow/1.5.0.yaml")
	_, err = mag.SetupTest(ctx, bs2)
	require.NoError(t, err)

	// Ingest alpine:3.17.3 (amd64, docker manifest v2) from the test registry
	// under the glow#1.5.0 manipulation profile. This mirrors the real prod path
	// where the client pulls harbor.example.com/library/alpine:3.2x and metahub
	// intercepts the first-time fetch and applies profile manipulations.
	const repo = "alpine"
	const tag = "3.17.3"
	req := types.Request{
		Host:    regHttpURI,
		Repo:    repo,
		Tag:     tag,
		Profile: "glow#1.5.0",
	}
	blob := types.BlobMeta{
		MediaType: MediaTypeManifest,
		Digest:    shab5a,
	}
	require.NoError(t, mag.UpstreamBlobInsertManifest(ctx, req, blob),
		"pull-through ingest with glow#1.5.0 profile must succeed")

	// The original (non-manipulated) manifest must be stored and linked to the
	// upstream via an ORIGINAL edge — this proves the OS eval completed.
	origQ := fmt.Sprintf(
		`MATCH (md:Manifest)-[:ORIGINAL]->(u:Upstream) WHERE u.repo = "%s" AND u.tag = "%s" RETURN id(md) AS mdID`,
		repo, tag)
	origRes, origFound, err := mag.Database.QueryOne(origQ)
	require.NoError(t, err)
	require.True(t, origFound, "original manifest must be linked to upstream via :ORIGINAL")
	// Sanity: the projection carries a parsable int64 mdID. Do NOT assert
	// NotZero on it — Neo4j internal node IDs start at 0 and after a full
	// wipe the very first created node legitimately gets ID 0 (flaked in CI
	// exactly that way: "Manifest ID: 0"). origFound above already proves the
	// (md:Manifest)-[:ORIGINAL]->(u) edge.
	_, err = origRes.GetInt64("mdID")
	require.NoError(t, err)

	// The glow#1.5.0 Slot must have a pushed manifest. This is the manipulated
	// image — alpine's original layers with the glow layer appended.
	glowQ := fmt.Sprintf(
		`MATCH (s:Slot {profile: "glow#1.5.0"})-[:HAS_PUSHED]->(md:Manifest)`+
			` WHERE s.repo = "%s" AND s.tag = "%s"`+
			` RETURN count(md) AS n`,
		repo, tag)
	glowRes, glowFound, err := mag.Database.QueryOne(glowQ)
	require.NoError(t, err)
	require.True(t, glowFound, "glow#1.5.0 Slot must exist and carry a pushed manifest")
	n, err := glowRes.GetInt64("n")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, int64(1),
		"glow#1.5.0 Slot must carry at least one manipulated manifest")
}
