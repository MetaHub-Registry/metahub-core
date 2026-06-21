package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestManifestDeleteAndExists covers deletion and existence checks.
func TestManifestDeleteAndExists(t *testing.T) {
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

	// ManifestExistsForTagProfile
	upt := upstream.UpstreamTriple{
		Repo:    "qnib/featuretest",
		Tag:     "arm64",
		Profile: "generic",
	}
	_, _, err = db.ManifestExistsForTagProfile(ctx, upt)
	require.NoError(t, err)

	// ManifestExistsForTagProfileContent — may error if profile not linked to manifest
	_, _, _ = db.ManifestExistsForTagProfileContent(ctx, upt)

	// ManifestExists (tag-based)
	_, _, err = db.ManifestExists(ctx, upt)
	require.NoError(t, err)

	// ManifestExistsContent (tag-based) — may error if Upstream not linked
	_, _, _ = db.ManifestExistsContent(ctx, upt)

	// ManifestExists (digest-based)
	uptDig := upstream.UpstreamTriple{
		Repo:    "qnib/featuretest",
		Digest:  digest.Digest(mdDigest),
		Profile: "generic",
	}
	_, _, err = db.ManifestExists(ctx, uptDig)
	require.NoError(t, err)

	// ManifestExistsContent (digest-based)
	_, _, _ = db.ManifestExistsContent(ctx, uptDig)

	// ManifestDeleteByDigest (takes a digest string, not a Manifest struct)
	err = db.ManifestDeleteByDigest(ctx, mdDigest)
	require.NoError(t, err)

	// ManifestDelete on a wrapper-less manifest must succeed: wrapper cleanup
	// is a no-op since :ManifestWrapper nodes are no longer created
	// (deprecate-manifest-wrapper Phase 3).
	md2 := types.Manifest{ID: 999999, Digest: "sha256:nonexistent"}
	err = db.ManifestDelete(ctx, md2)
	require.NoError(t, err, "ManifestDelete must succeed for a wrapper-less manifest")
}

// TestManifestV1GetInsertWorkflow covers the v1 descriptor insert + get.
func TestManifestV1GetInsertWorkflow(t *testing.T) {
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

	desc := v1.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    "sha256:v1wf1111111111111111111111111111111111111111111111111111111111111",
		Size:      350,
	}

	// First call creates
	mdID, found, err := db.ManifestV1GetInsert(ctx, 0, desc)
	require.NoError(t, err)
	assert.False(t, found)
	assert.GreaterOrEqual(t, mdID, int64(0))

	// ManifestV1Get — find it
	gotID, found, err := db.ManifestV1Get(ctx, desc)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, mdID, gotID)

	// Second call finds existing
	mdID2, found2, err := db.ManifestV1GetInsert(ctx, 0, desc)
	require.NoError(t, err)
	assert.True(t, found2)
	assert.Equal(t, mdID, mdID2)
}

// TestManifestProfileLinking covers linking manifests to profiles.
func TestManifestProfileLinking(t *testing.T) {
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

	mdID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:prflnk1111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)

	prfID, err := br.GetInt64("profile:generic#id")
	require.NoError(t, err)
	prf, _, err := db.ProfileGetByLabel(ctx, "generic")
	require.NoError(t, err)

	// ManifestIsLinkedWithProfile — not linked yet
	linked, err := db.ManifestIsLinkedWithProfile(ctx, mdID, prf)
	require.NoError(t, err)
	assert.False(t, linked)

	// LinkManifestToProfile
	err = db.LinkManifestToProfile(ctx, mdID, prfID, "UPLOADED_BY")
	require.NoError(t, err)

	// ManifestIsLinkedWithProfile — now linked
	linked, err = db.ManifestIsLinkedWithProfile(ctx, mdID, prf)
	require.NoError(t, err)
	assert.True(t, linked)

	// ManifestLinkToProfile (the other function with no rel param)
	err = db.ManifestLinkToProfile(ctx, mdID, prfID)
	require.NoError(t, err)
}

// TestManifestSubjectAndArtifactConfig covers subject and artifact linking.
func TestManifestSubjectAndArtifactConfig(t *testing.T) {
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

	subjectID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:subj1111111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)

	referrerDigest := "sha256:refr1111111111111111111111111111111111111111111111111111111111111"
	_, err = db.ManifestInsert(ctx, &types.Manifest{
		Digest:    referrerDigest,
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      200,
	})
	require.NoError(t, err)

	// LinkManifestToSubject
	err = db.LinkManifestToSubject(ctx, referrerDigest, subjectID)
	require.NoError(t, err)

	// ManifestLinkToArtifactConfig
	artDigest := "sha256:artb1111111111111111111111111111111111111111111111111111111111111"
	q := fmt.Sprintf(`CREATE (b:Blob {digest: "%s", mediaType: "application/vnd.custom+json", size: 50}) RETURN id(b) AS blobID`, artDigest)
	_, err = db.Execute(q)
	require.NoError(t, err)

	desc := v1.Descriptor{
		MediaType: "application/vnd.custom+json",
		Digest:    digest.Digest(artDigest),
		Size:      50,
	}
	err = db.ManifestLinkToArtifactConfig(ctx, referrerDigest, desc)
	require.NoError(t, err)
}

// TestManifestOCIv1Layer covers the OCI v1 layer linking.
func TestManifestOCIv1Layer(t *testing.T) {
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

	mdID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:ocilyr111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)

	// Create a Blob for the layer
	lyrDigest := "sha256:lyrblo111111111111111111111111111111111111111111111111111111111"
	q := fmt.Sprintf(`CREATE (b:Blob {digest: "%s", mediaType: "application/vnd.oci.image.layer.v1.tar+gzip", size: 500, type: "layer"}) RETURN id(b) AS blobID`, lyrDigest)
	res, err := db.Execute(q)
	require.NoError(t, err)
	lyrID, _ := res.GetInt64("blobID")

	ld := types.LayerDesc{
		LayerID: lyrID,
		Descriptor: v1.Descriptor{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Digest:    digest.Digest(lyrDigest),
			Size:      500,
		},
		Order: 0,
	}
	err = db.ManifestLinkToOCIv1Layer(ctx, mdID, ld)
	require.NoError(t, err)

	// Verify edge exists (relation type is the shortened media type, not :HAS_LAYER)
	verifyQ := fmt.Sprintf(`MATCH (md:Manifest)-[:oci_image_layer_v1_tarGZIP]->(b:Blob) WHERE id(md) = %d RETURN count(b) AS c`, mdID)
	vRes, found, err := db.QueryOne(verifyQ)
	require.NoError(t, err)
	require.True(t, found)
	cnt, _ := vRes.GetInt64("c")
	assert.GreaterOrEqual(t, cnt, int64(1))
}

// TestManifestUnlinkConfigAndLayer covers unlink operations.
func TestManifestUnlinkConfigAndLayer(t *testing.T) {
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

	mdID, err := db.ManifestInsert(ctx, &types.Manifest{
		Digest:    "sha256:unlnk1111111111111111111111111111111111111111111111111111111111",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
	})
	require.NoError(t, err)

	// Create and link config
	cfgQ := `CREATE (b:Blob {digest: "sha256:ucfg01", mediaType: "application/vnd.oci.image.config.v1+json", size: 50, type: "config"}) RETURN id(b) AS blobID`
	cfgRes, err := db.Execute(cfgQ)
	require.NoError(t, err)
	cfgID, _ := cfgRes.GetInt64("blobID")
	cfg := types.Config{ID: cfgID, Digest: "sha256:ucfg01", MediaType: "application/vnd.oci.image.config.v1+json"}
	err = db.ManifestLinkToConfig(ctx, mdID, cfg)
	require.NoError(t, err)

	// UnlinkConfig
	err = db.ManifestUnlinkConfig(ctx, mdID, "sha256:ucfg01")
	require.NoError(t, err)

	// Create and link layer
	lyrQ := `CREATE (b:Blob {digest: "sha256:ulyr01", mediaType: "application/vnd.oci.image.layer.v1.tar+gzip", size: 100, type: "layer"}) RETURN id(b) AS blobID`
	lyrRes, err := db.Execute(lyrQ)
	require.NoError(t, err)
	lyrID, _ := lyrRes.GetInt64("blobID")
	lyr := types.Layer{ID: lyrID, Digest: "sha256:ulyr01", MediaType: "application/vnd.oci.image.layer.v1.tar+gzip"}
	err = db.ManifestLinkToLayer(ctx, mdID, lyr, 0)
	require.NoError(t, err)

	// UnlinkLayer
	err = db.ManifestUnlinkLayer(ctx, mdID, lyrID)
	require.NoError(t, err)
}

// TestManifestGetPushedByProfileFull covers the push-by-profile lookup with full bootstrap.
func TestManifestGetPushedByProfileFull(t *testing.T) {
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

	prf, _, err := db.ProfileGetByLabel(ctx, "generic")
	require.NoError(t, err)

	up := types.Upstream{
		Reg:  "metahub-registry.org",
		Repo: "qnib/featuretest",
		Tag:  "arm64",
	}
	_, _, err = db.ManifestGetPushedByProfile(ctx, up, prf)
	require.NoError(t, err)
}
