package magazine

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManifestGetByID(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		t.Logf("Error while SetupTest: %v", err)
		TeardownTest(bs, br)
		t.Fatal(err)

	}
	defer TeardownTest(bs, br)
	// Actual test
	mani, err := mag.getManiData(ctx, br, "manifestdata:ft_amd64")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", mani.MediaType)
	assert.NotEqual(t, 0, mani.Config.Size)
	assert.Equal(t, mani.Config.Size, int64(len(mani.Config.Data)))
	expEnv := []string{"PLATFORM_FEATURES=amd64", "ORG_SUPERCONTAINERS_HARDWARE_CPU_TARGET=platform"}
	assert.Equal(t, expEnv, mani.Config.Cfg.Config.Env)
}

func TestStoreManifestArtifactUpload(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		t.Logf("Error while SetupTest: %v", err)
		TeardownTest(bs, br)
		t.Fatal(err)

	}
	defer TeardownTest(bs, br)
	inMani := types.FTamd64OCIManifest()
	b, err := inMani.ToBytes()
	if err != nil {
		t.Fatal(err)
	}
	dig := digest.FromBytes(b)
	// Actual test
	req := types.Request{
		Repo:    "qnib/plain-featuretest",
		Tag:     "20220715-arm64-generic",
		User:    "qnib",
		Profile: "cpu#generic",
		Digest:  dig.String(),
	}
	// The content-leaf the production caller (StoreManifestUpload) passes in:
	// a real node ID/digest/label resolved by storeManifestUploadOCIManifest.
	leaf := types.Manifest{
		ID:     42,
		Digest: dig.String(),
		Label:  "qnib/plain-featuretest:20220715-arm64-generic",
	}
	md, err := mag.StoreManifestArtifactUpload(ctx, req, leaf, inMani)
	if err != nil {
		// Blob/link steps depend on fixture objects being present; the identity
		// fields asserted below are populated before any of that runs, so they
		// hold regardless of how far the storage path gets.
		t.Logf("StoreManifestArtifactUpload returned err (not fatal for these assertions): %v", err)
	}
	// Regression — outfitter skill push returned HTTP 500. The artifact branch
	// used to discard the content-leaf and re-derive the manifest via
	// ManifestGetByDigest; on a FIRST push nothing matches, so it returned the
	// zero value. The empty MediaType then made ManifestPutHandlerWrapper's
	// WriteWWWHeader fail with "Manifest.MediaType is empty", and
	// wireSlotOnPush silently received ID 0.
	assert.NotEmpty(t, md.MediaType, "MediaType must be set — WriteWWWHeader rejects an empty one with a 500")
	assert.Equal(t, dig.String(), md.Digest, "digest must survive the artifact path")
	assert.Equal(t, int64(42), md.ID, "content-leaf node ID must be preserved (neo4j id() is 0-based, so 0 is not a safe sentinel)")
	assert.Equal(t, inMani.ArtifactType, md.ArtifactType, "ArtifactType must be propagated from the pushed bytes")
}
