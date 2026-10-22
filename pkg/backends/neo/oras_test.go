package neo

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrasListingSeparation verifies that image-only repos do not appear in OrasArtifactGetByRepoLike
// and that ORAS-only repos do appear.
func TestOrasListingSeparation(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	// Set up a container image manifest (qnib/featuretest with amd64 platform)
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)
	for _, l := range br.ToLines() {
		_ = l
	}

	t.Run("image_only_repo_absent_from_oras_listing", func(t *testing.T) {
		listings, err := db.OrasArtifactGetByRepoLike(ctx, "")
		require.NoError(t, err)
		assert.Empty(t, listings, "image-only repo should not appear in ORAS listing")
	})

	t.Run("image_only_repo_absent_with_prefix", func(t *testing.T) {
		listings, err := db.OrasArtifactGetByRepoLike(ctx, "qnib")
		require.NoError(t, err)
		assert.Empty(t, listings, "image-only repo should not appear in ORAS listing with qnib prefix")
	})
}

// TestOrasMixedContentListing verifies that a repo with both image and ORAS manifests
// has its ORAS artifact entries returned by OrasArtifactGetByRepoLike.
func TestOrasMixedContentListing(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	// ORAS artifact first
	bs.AddBootstrap("scenario:oras_cyclonedx")
	// Then add a container image to the same repo
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	t.Run("mixed_repo_oras_entries_appear", func(t *testing.T) {
		listings, err := db.OrasArtifactGetByRepoLike(ctx, "qnib")
		require.NoError(t, err)
		require.Len(t, listings, 1, "expected exactly one ORAS listing for mixed-content qnib repo")
		assert.Equal(t, "qnib/featuretest", listings[0].Repo)
		assert.Equal(t, "application/vnd.cyclonedx", listings[0].ArtifactType)
	})
}

// TestOrasAll exercises all three OrasArtifact methods in a single bootstrap/teardown cycle
// to reduce shared-container contention.
//
// The bootstrap uses a single "scenario:oras_cyclonedx" which sets up a minimal ORAS
// CycloneDX artifact (upstream, config, manifest, manifestdata, collection) without any
// platform-specific machinery — ORAS artifacts are not platform-specific.
func TestOrasAll(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("scenario:oras_cyclonedx")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}

	t.Run("GetByRepoLike_unknownPrefix_empty", func(t *testing.T) {
		listings, err := db.OrasArtifactGetByRepoLike(ctx, "doesnotexist")
		require.NoError(t, err)
		assert.Empty(t, listings, "expected no ORAS listings for unknown prefix")
	})

	t.Run("GetByRepoLike_qnib_returns_cyclonedx", func(t *testing.T) {
		listings, err := db.OrasArtifactGetByRepoLike(ctx, "qnib")
		require.NoError(t, err)
		require.Len(t, listings, 1, "expected exactly one ORAS listing for qnib prefix")
		assert.Equal(t, "qnib/featuretest", listings[0].Repo)
		assert.Equal(t, "application/vnd.cyclonedx", listings[0].ArtifactType)
		assert.Equal(t, int64(1), listings[0].ArtifactCount)
	})

	t.Run("GetByName_found_cyclonedx", func(t *testing.T) {
		artifact, err := db.OrasArtifactGetByName(ctx, "qnib/featuretest", "application/vnd.cyclonedx")
		require.NoError(t, err)
		assert.Equal(t, "qnib/featuretest", artifact.Repo)
		assert.Equal(t, "application/vnd.cyclonedx", artifact.ArtifactType)
		// Verify enrichment: manifest annotations
		assert.NotEmpty(t, artifact.Annotations, "expected non-empty manifest annotations")
		assert.Equal(t, "2026-04-14T09:00:00Z", artifact.Annotations["org.opencontainers.image.created"])
		// Verify enrichment: layer blobs
		require.Len(t, artifact.Blobs, 1, "expected one layer blob")
		assert.Equal(t, "sha256:a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447", artifact.Blobs[0].Digest)
		assert.Equal(t, "application/vnd.oci.image.layer.v1.tar", artifact.Blobs[0].MediaType)
		assert.Equal(t, int64(12), artifact.Blobs[0].Size)
		assert.Equal(t, "sbom.cdx.json", artifact.Blobs[0].Title)

		// The natural-key digest lookup must resolve the same artifact.
		byDigest, err := db.OrasArtifactGetByDigest(ctx, artifact.Repo, artifact.Digest)
		require.NoError(t, err)
		assert.Equal(t, artifact.Repo, byDigest.Repo)
		assert.Equal(t, artifact.Digest, byDigest.Digest)
		assert.Equal(t, artifact.ArtifactType, byDigest.ArtifactType)
		assert.Equal(t, "sbom.cdx.json", artifact.Blobs[0].Annotations["org.opencontainers.image.title"])
	})

	t.Run("GetByName_notFound", func(t *testing.T) {
		_, err := db.OrasArtifactGetByName(ctx, "qnib/featuretest", "application/vnd.spdx+json")
		assert.Error(t, err, "expected error for non-existent ORAS artifact type")
	})

	t.Run("GetByID_found", func(t *testing.T) {
		maniDataID, e := br.GetInt64("manifestdata:ft_oras_cyclonedx#id")
		require.NoError(t, e, "expected manifestdata:ft_oras_cyclonedx#id to be in bootstrap result")
		artifact, err := db.OrasArtifactGetByID(ctx, maniDataID)
		require.NoError(t, err)
		assert.Equal(t, maniDataID, artifact.ID)
		assert.Equal(t, "qnib/featuretest", artifact.Repo)
		assert.Equal(t, "application/vnd.cyclonedx", artifact.ArtifactType)
	})

	t.Run("GetByID_notFound", func(t *testing.T) {
		_, err := db.OrasArtifactGetByID(ctx, -999999)
		assert.Error(t, err, "expected error for non-existent manifest ID")
	})
}
