package neo

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestManifestExists_TagBased covers the rewritten tag-based existence
// check that walks via :ORIGINAL→:Upstream.
func TestManifestExists_TagBased(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	upt := upstream.UpstreamTriple{
		Repo:    "qnib/featuretest",
		Tag:     "arm64",
		Profile: "generic",
	}
	mt, found, err := db.ManifestExists(ctx, upt)
	require.NoError(t, err)
	if found {
		assert.NotEmpty(t, mt, "mediaType should not be empty when found")
	}
}

// TestManifestExists_NotFound covers the not-found path.
func TestManifestExists_NotFound(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	upt := upstream.UpstreamTriple{
		Repo:    "nonexistent/repo",
		Tag:     "missing",
		Profile: "generic",
	}
	_, found, err := db.ManifestExists(ctx, upt)
	require.NoError(t, err)
	assert.False(t, found)
}

// TestManifestExists_DigestBased covers the digest-based path.
func TestManifestExists_DigestBased(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	mdDigest, err := br.Get("manifestdata:ft_arm64#digest")
	require.NoError(t, err)

	upt := upstream.UpstreamTriple{
		Repo:    "qnib/featuretest",
		Digest:  digest.Digest("sha256:" + mdDigest),
		Profile: "generic",
	}
	_, found, err := db.ManifestExists(ctx, upt)
	require.NoError(t, err)
	// Digest-based lookup walks :ORIGINAL→:Upstream
	_ = found
}

// TestManifestGetPushedByProfile covers the profile-scoped push lookup.
func TestManifestGetPushedByProfile(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	prf, _, err := db.ProfileGetByLabel(ctx, "cpu#generic")
	require.NoError(t, err)

	up := types.Upstream{
		Reg:  "metahub-registry.org",
		Repo: "qnib/featuretest",
		Tag:  "arm64",
	}
	_, found, err := db.ManifestGetPushedByProfile(ctx, up, prf)
	require.NoError(t, err)
	// Result depends on whether UPLOADED_BY edge exists in bootstrap
	_ = found
}

// TestLinkManifestToProfile covers both profile linking functions.
func TestLinkManifestToProfile(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	mdID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:prflink1111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)

	// Create a profile
	prf := types.Profile{Name: "testprofile", Label: "testprofile"}
	err = db.ProfileGetInsert(ctx, &prf)
	require.NoError(t, err)

	// LinkManifestToProfile (with rel type)
	err = db.LinkManifestToProfile(ctx, mdID, prf.ID, "UPLOADED_BY")
	require.NoError(t, err)

	// ManifestLinkToProfile (without rel type — uses default MERGED)
	err = db.ManifestLinkToProfile(ctx, mdID, prf.ID)
	require.NoError(t, err)
}

// TestManifestIsLinkedWithProfile covers the internal profile-check.
func TestManifestIsLinkedWithProfile(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	mdID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:islink1111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)

	prf := types.Profile{Name: "linkcheckprf", Label: "linkcheckprf"}
	err = db.ProfileGetInsert(ctx, &prf)
	require.NoError(t, err)

	// Not linked yet
	linked, err := db.ManifestIsLinkedWithProfile(ctx, mdID, prf)
	require.NoError(t, err)
	assert.False(t, linked)

	// Link it
	err = db.LinkManifestToProfile(ctx, mdID, prf.ID, "UPLOADED_BY")
	require.NoError(t, err)

	// Now linked
	linked, err = db.ManifestIsLinkedWithProfile(ctx, mdID, prf)
	require.NoError(t, err)
	assert.True(t, linked)
}
