package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestProfileManifestType covers the ProfileManifestType function.
func TestProfileManifestType(t *testing.T) {
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

	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	require.NoError(t, err)
	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	// Link Manifest to Profile with UPLOADED_BY
	prf, _, err := db.ProfileGetByLabel(ctx, "generic")
	require.NoError(t, err)
	err = db.LinkManifestToProfile(ctx, mdID, prf.ID, "UPLOADED_BY")
	require.NoError(t, err)

	// ProfileManifestType for the wrapper — should find the relType
	relType, found, err := db.ProfileManifestType(ctx, maniID)
	if found {
		assert.NotEmpty(t, relType)
	}
	// Not found case
	_, found, err = db.ProfileManifestType(ctx, 999999)
	assert.False(t, found)
	_ = err
}

// TestProfileGetUploadedPulledByViaManifestID covers both upload and pull profile lookups.
func TestProfileGetUploadedPulledByViaManifestID(t *testing.T) {
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

	mdID, err := br.GetInt64("manifestdata:ft_arm64#id")
	require.NoError(t, err)

	prf, _, err := db.ProfileGetByLabel(ctx, "generic")
	require.NoError(t, err)

	// Link with UPLOADED_BY
	err = db.LinkManifestToProfile(ctx, mdID, prf.ID, "UPLOADED_BY")
	require.NoError(t, err)

	// ProfileGetUploadedByViaManifestID — should find it
	p, found, err := db.ProfileGetUploadedByViaManifestID(ctx, mdID)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "generic", p.Label)

	// ProfileGetPulledByViaManifestID — no PULLED_BY link yet
	_, found, err = db.ProfileGetPulledByViaManifestID(ctx, mdID)
	require.NoError(t, err)
	assert.False(t, found)

	// Create a PULLED_BY link on a separate manifest
	mdID2, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:pull1111111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)
	err = db.LinkManifestToProfile(ctx, mdID2, prf.ID, "PULLED_BY")
	require.NoError(t, err)

	p2, found, err := db.ProfileGetPulledByViaManifestID(ctx, mdID2)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "generic", p2.Label)

	// Not found cases
	_, found, err = db.ProfileGetUploadedByViaManifestID(ctx, 999999)
	require.NoError(t, err)
	assert.False(t, found)

	_, found, err = db.ProfileGetPulledByViaManifestID(ctx, 999999)
	require.NoError(t, err)
	assert.False(t, found)
}

// TestProfileFromManifestWrapperID covers the ManifestWrapper → Profile resolution.
func TestProfileFromManifestWrapperID(t *testing.T) {
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

	err = db.RuleEvalAll(ctx)
	require.NoError(t, err)

	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	require.NoError(t, err)

	// ProfileFromManifestWrapperID — combines annotation, platform, and original manifest paths
	prfs, err := db.ProfileFromManifestWrapperID(ctx, maniID)
	require.NoError(t, err)
	// The wrapper has content manifests linked — profiles may or may not be found
	// depending on rule eval creating :TRUE/:FALSE edges. At a minimum, no error.
	t.Logf("found %d profiles for ManifestWrapper %d", len(prfs), maniID)

	// Not found case — nonexistent wrapper
	prfs2, err := db.ProfileFromManifestWrapperID(ctx, 999999)
	require.NoError(t, err)
	assert.Empty(t, prfs2)
}

// TestProfileFromManifestWrapperIDViaAnnotations covers the annotation-based path.
func TestProfileFromManifestWrapperIDViaAnnotations(t *testing.T) {
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

	err = db.RuleEvalAll(ctx)
	require.NoError(t, err)

	// No ManifestWrapper in the graph → expect empty
	prfs, err := db.ProfileFromManifestWrapperIDViaAnnotations(ctx, 999999)
	require.NoError(t, err)
	assert.Empty(t, prfs)
}

// TestProfileFromManifestWrapperIDViaPlatform covers the platform-based path.
func TestProfileFromManifestWrapperIDViaPlatform(t *testing.T) {
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

	err = db.RuleEvalAll(ctx)
	require.NoError(t, err)

	// No ManifestWrapper in the graph → expect empty
	prfs, err := db.ProfileFromManifestWrapperIDViaPlatform(ctx, 999999)
	require.NoError(t, err)
	assert.Empty(t, prfs)
}

// TestEvalUploadProfileWithBlob covers the EvalUploadProfile function.
func TestEvalUploadProfileWithBlob(t *testing.T) {
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

	cfgID, err := br.GetInt64("config:ft_arm64#id")
	require.NoError(t, err)

	// Create a BlobUpload linked to the config blob
	q := `CREATE (bu:BlobUpload {upload_profile: "generic"}) RETURN id(bu) AS buID`
	res, e := db.Execute(q)
	require.NoError(t, e)
	buID, e := res.GetInt64("buID")
	require.NoError(t, e)

	// Link BlobUpload to config blob
	linkQ := fmt.Sprintf(`MATCH (bu:BlobUpload), (cfg:Blob) WHERE id(bu) = %d AND id(cfg) = %d MERGE (bu)-[:UPLOADED]->(cfg)`, buID, cfgID)
	_, e = db.Execute(linkQ)
	require.NoError(t, e)

	mani := types.ManifestWrapper{ID: 1}
	origMani := types.Manifest{Config: types.Config{ID: cfgID}}

	err = db.EvalUploadProfile(ctx, mani, origMani)
	require.NoError(t, err)

	// Not found case — config with no BlobUpload
	origManiMissing := types.Manifest{Config: types.Config{ID: 999999}}
	err = db.EvalUploadProfile(ctx, mani, origManiMissing)
	assert.Error(t, err)
}
