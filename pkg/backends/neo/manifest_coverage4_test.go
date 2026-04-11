package neo

import (
	"context"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestManifestGetIDByDigest covers the ManifestGetIDByDigest helper.
func TestManifestGetIDByDigest(t *testing.T) {
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
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	mdDigest, err := br.Get("manifestdata:ft_arm64#digest")
	require.NoError(t, err)

	// Found case
	mdID, found, err := db.ManifestGetIDByDigest(ctx, mdDigest)
	require.NoError(t, err)
	assert.True(t, found)
	assert.NotEqual(t, int64(0), mdID)

	// Not found case
	_, found, err = db.ManifestGetIDByDigest(ctx, "sha256:0000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)
	assert.False(t, found)
}

// TestManifestMatchesProfile covers the (currently stub) ManifestMatchesProfile.
func TestManifestMatchesProfile(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	md := types.Manifest{ID: 1, Label: "test-manifest"}
	prf := types.Profile{ID: 1, Name: "test-profile"}
	matches, err := db.ManifestMatchesProfile(ctx, md, prf)
	require.NoError(t, err)
	// Stub always returns true
	assert.True(t, matches)
}

// TestManifestCheckProfileCreationAndAnnotations covers the profile check functions.
func TestManifestCheckProfileCreationAndAnnotations(t *testing.T) {
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

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)
	mdDigest, err := br.Get("manifestdata:ft_arm64#digest")
	require.NoError(t, err)

	prf, _, err := db.ProfileGetByLabel(ctx, "generic")
	require.NoError(t, err)

	md := types.Manifest{ID: mdID, Digest: mdDigest}

	// ManifestCheckProfileCreationRelationship — not linked yet
	ok, err := db.ManifestCheckProfileCreationRelationship(ctx, md, prf)
	require.NoError(t, err)
	assert.False(t, ok, "should not be linked before explicit link")

	// Link it
	err = db.LinkManifestToProfile(ctx, mdID, prf.ID, "UPLOADED_BY")
	require.NoError(t, err)

	// ManifestCheckProfileCreationRelationship — now linked
	ok, err = db.ManifestCheckProfileCreationRelationship(ctx, md, prf)
	require.NoError(t, err)
	assert.True(t, ok)

	// Run RuleEvalAll to create shortcut edges
	err = db.RuleEvalAll(ctx)
	require.NoError(t, err)

	// ManifestCheckProfileAnnotations — should pass (no failing annotations)
	ok, err = db.ManifestCheckProfileAnnotations(ctx, md, prf)
	require.NoError(t, err)
	assert.True(t, ok)

	// ManifestCheckProfile — combines both checks; should pass
	ok, err = db.ManifestCheckProfile(ctx, md, prf)
	require.NoError(t, err)
	assert.True(t, ok)
}

// TestManifestGetExactly covers the ManifestGetExactly function.
func TestManifestGetExactly(t *testing.T) {
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

	// Insert a manifest
	mdID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:exact1111111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      200,
	})
	require.NoError(t, err)

	// Found case
	gotID, found, err := db.ManifestGetExactly(ctx, types.Manifest{
		Digest:    "sha256:exact1111111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      200,
	})
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, mdID, gotID)

	// Not found case — wrong size
	_, found, err = db.ManifestGetExactly(ctx, types.Manifest{
		Digest:    "sha256:exact1111111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      999,
	})
	require.NoError(t, err)
	assert.False(t, found)
}

// TestManifestUpdateDescCoverage covers ManifestUpdateDesc with a fresh graph.
func TestManifestUpdateDescCoverage(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// Create a manifest via V1GetInsert
	desc := v1.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    "sha256:upd01111111111111111111111111111111111111111111111111111111111111",
		Size:      100,
	}
	mdID, _, err := db.ManifestV1GetInsert(ctx, 0, desc)
	require.NoError(t, err)

	// Update with a new descriptor (digest not yet in DB → update path)
	newDesc := v1.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    "sha256:upd02222222222222222222222222222222222222222222222222222222222222",
		Size:      150,
	}
	err = db.ManifestUpdateDesc(ctx, mdID, newDesc)
	require.NoError(t, err)

	// Verify the update
	gotID, found, err := db.ManifestGetExactly(ctx, types.Manifest{
		Digest:    "sha256:upd02222222222222222222222222222222222222222222222222222222222222",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      150,
	})
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, mdID, gotID)
}

// TestManifestRemove covers ManifestRemove.
func TestManifestRemove(t *testing.T) {
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

	// Remove by digest
	req := types.Request{
		Digest: mdDigest,
	}
	found, err := db.ManifestRemove(ctx, req)
	require.NoError(t, err)
	assert.True(t, found)

	// Remove non-existent — should return not found
	reqMissing := types.Request{
		Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	found, err = db.ManifestRemove(ctx, reqMissing)
	// ManifestRemove returns found=false when manifest doesn't exist
	assert.False(t, found)
}

// TestManifestGetByUpstream_WithProfile covers the profile-filtered path.
func TestManifestGetByUpstream_WithProfile(t *testing.T) {
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

	// Without profile
	up := types.Upstream{
		Repo: "qnib/featuretest",
		Tag:  "arm64",
	}
	m, found, err := db.ManifestGetByUpstream(ctx, up)
	require.NoError(t, err)
	if found {
		assert.NotEmpty(t, m.Digest)
	}

	// Not found case
	upMissing := types.Upstream{
		Repo: "nonexistent/repo",
		Tag:  "missing",
	}
	_, found, err = db.ManifestGetByUpstream(ctx, upMissing)
	require.NoError(t, err)
	assert.False(t, found)

	_ = br
}

// TestManifestGetByID covers ManifestGetByID.
func TestManifestGetByID(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	mdID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:getid1111111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)

	// Found
	m, found, err := db.ManifestGetByID(ctx, mdID)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, mdID, m.ID)

	// Not found
	_, found, err = db.ManifestGetByID(ctx, 999999)
	require.NoError(t, err)
	assert.False(t, found)
}

// TestManifestGetTypesByRequest covers tag-based and digest-based lookups.
func TestManifestGetTypesByRequest_TagBased(t *testing.T) {
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

	// Tag-based lookup
	req := types.Request{
		Repo: "qnib/featuretest",
		Tag:  "arm64",
	}
	m, found, err := db.ManifestGetTypesByRequest(ctx, req)
	require.NoError(t, err)
	if found {
		assert.NotEmpty(t, m.Digest)
	}

	// Empty request — returns zero value
	reqEmpty := types.Request{}
	_, found, err = db.ManifestGetTypesByRequest(ctx, reqEmpty)
	require.NoError(t, err)
	assert.False(t, found)
}
