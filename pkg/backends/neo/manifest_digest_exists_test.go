package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestManifestExists_UnlinkedDigestIsNotAnError pins the Path Z contract from
// openspec fix-digest-manifest-existence (OpenProject #41):
//
//   - HEAD/GET **by digest** of a manifest that exists in the graph but is NOT
//     linked to the requesting profile must resolve as existing
//     (content-addressable, profile scoping is a tag-based concern).
//   - The same unlinked state addressed **by tag** must report not-found —
//     never an error (the old error path surfaced as HTTP 500 and broke
//     `oras cp` destination pre-checks).
//
// Fixture mirrors the outage state: Manifest + Upstream (+ Repo/Tag) nodes
// exist, no (Profile)-[]->(Manifest) linkage.
func TestManifestExists_UnlinkedDigestIsNotAnError(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	mdDigest, err := br.Get("manifestdata:ft_arm64#digest")
	require.NoError(t, err)
	upRepo, err := br.Get("upstream:ft_arm64#repo")
	require.NoError(t, err)
	upTag, err := br.Get("upstream:ft_arm64#tag")
	require.NoError(t, err)

	// The digest-based lookup joins Repo/Tag on the UPSTREAM's repo/tag —
	// ensure matching nodes exist (the push path does this in production;
	// MERGE because the fixture may already have created them).
	q := fmt.Sprintf(`MATCH (u:Upstream {repo: "%s", tag: "%s"}) MERGE (c:Repo {repo: u.repo}) MERGE (c)-[:HAS_TAG]->(t:Tag {name: u.tag})`, upRepo, upTag)
	if _, err = db.Execute(q); err != nil {
		t.Fatal(err)
	}

	// Sanity: the fixture really is the unlinked state.
	prfID, err := br.GetInt64("profile:generic#id")
	require.NoError(t, err)
	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)
	linked, err := db.ManifestIsLinkedWithProfile(ctx, mdID, types.Profile{ID: prfID, Label: "generic"})
	require.NoError(t, err)
	assert.False(t, linked, "fixture must start unlinked")

	// --- Digest-based: content-addressable → found, regardless of profile.
	upDig := upstream.UpstreamTriple{
		Reg:     "registry.example.com",
		Repo:    upRepo,
		Digest:  digest.Digest(mdDigest),
		Profile: "generic",
	}
	mediaType, found, err := db.ManifestExistsContent(ctx, upDig)
	require.NoError(t, err, "digest-based existence of an unlinked manifest must NOT error (was the 500)")
	assert.True(t, found, "digest-addressed content exists regardless of profile linkage")
	assert.NotEmpty(t, mediaType)

	mediaType, found, err = db.ManifestExists(ctx, upDig)
	require.NoError(t, err)
	assert.True(t, found, "ManifestExists digest path must find unlinked content")
	assert.NotEmpty(t, mediaType)

	// --- Tag-based: profile-scoped → not-found, never an error.
	upTagged := upstream.UpstreamTriple{
		Reg:     "registry.example.com",
		Repo:    upRepo,
		Tag:     upTag,
		Profile: "generic",
	}
	_, found, err = db.ManifestExistsContent(ctx, upTagged)
	require.NoError(t, err, "tag-based existence of an unlinked manifest must NOT error (was the 500)")
	assert.False(t, found, "unlinked manifest is not part of the profile's tag-scoped view")

	_, found, err = db.ManifestExistsForTagProfileContent(ctx, upTagged)
	require.NoError(t, err)
	assert.False(t, found)

	_, found, err = db.ManifestExists(ctx, upTagged)
	require.NoError(t, err)
	assert.False(t, found)

	// --- Link the profile and confirm tag-based existence flips to found
	// (digest-based was already found).
	q = fmt.Sprintf(`MATCH (md:Manifest), (prf:Profile) WHERE id(md) = %d AND id(prf) = %d CREATE (md)-[:UPLOADED]->(prf)`, mdID, prfID)
	if _, err = db.Execute(q); err != nil {
		t.Fatal(err)
	}
	mediaType, found, err = db.ManifestExistsContent(ctx, upTagged)
	require.NoError(t, err)
	assert.True(t, found, "once linked, the tag-scoped lookup finds the manifest")
	assert.NotEmpty(t, mediaType)
}
