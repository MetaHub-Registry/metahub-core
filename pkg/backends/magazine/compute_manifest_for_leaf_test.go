package magazine

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// reshapeFn abstracts the two push-reshape entrypoints so the characterization
// helper can drive both over identical fixtures:
//   - wrapper-typed ComputeManifest(ctx, types.ManifestWrapper)
//   - leaf-based     ComputeManifestForLeaf(ctx, types.Manifest)
type reshapeFn func(ctx context.Context, br *utils.BootstrapResult) error

// ociResult captures the observable OCI v1 output of a reshape: the content
// digest of the produced OCI Manifest plus its ordered layer digests. The OCI
// Manifest digest is the sha256 of the serialized OCI v1 JSON, so equal digests
// imply byte-identical OCI v1 output.
type ociResult struct {
	digest string
	layers []string
}

// wrapperReshape returns a reshapeFn that drives the wrapper-typed
// ComputeManifest. As of deprecate-manifest-wrapper Phase 4 no :ManifestWrapper
// node exists, so the shim is built from the content-leaf (manifest:<label>#id
// now aliases the content-leaf id) — exactly mirroring what
// ComputeManifestForLeaf does internally. This keeps the characterization test
// meaningful: both entrypoints must produce identical OCI v1 output.
func wrapperReshape(wrapperLabel string) reshapeFn {
	return func(ctx context.Context, br *utils.BootstrapResult) error {
		mID, e := br.GetInt64(fmt.Sprintf("manifest:%s#id", wrapperLabel))
		if e != nil {
			return e
		}
		md, found, e := mag.Database.ManifestGetByID(ctx, mID)
		if e != nil {
			return e
		}
		if !found {
			return fmt.Errorf("content-leaf not found for id %d", mID)
		}
		annos := md.Annotations
		if annos.Data == nil {
			annos = types.NewAnnotations()
		}
		if md.Type == "" {
			md.Type = "ORIGINAL"
		}
		mani := types.ManifestWrapper{
			ID:          md.ID,
			Upstream:    md.Upstream,
			Annotations: annos,
			Manifests:   []types.Manifest{md},
		}
		return mag.ComputeManifest(ctx, mani)
	}
}

// leafReshape returns a reshapeFn that drives the leaf-based
// ComputeManifestForLeaf for the content-leaf behind manifestdata:<leafLabel>#id.
func leafReshape(leafLabel string) reshapeFn {
	return func(ctx context.Context, br *utils.BootstrapResult) error {
		mdID, e := br.GetInt64(fmt.Sprintf("manifestdata:%s#id", leafLabel))
		if e != nil {
			return e
		}
		md, found, e := mag.Database.ManifestGetByID(ctx, mdID)
		if e != nil {
			return e
		}
		if !found {
			return fmt.Errorf("content-leaf not found for id %d", mdID)
		}
		return mag.ComputeManifestForLeaf(ctx, md)
	}
}

// captureOCIResult resolves the produced OCI Manifest for the content-leaf
// behind manifestdata:<leafLabel>#id and returns its digest + ordered layers.
func captureOCIResult(t *testing.T, ctx context.Context, br *utils.BootstrapResult, leafLabel string) ociResult {
	t.Helper()
	mdID, err := br.GetInt64(fmt.Sprintf("manifestdata:%s#id", leafLabel))
	require.NoError(t, err)
	q := fmt.Sprintf(
		`MATCH (oci:Manifest)-[]->(:Blob) WHERE oci.mediaType IN ["%s"] AND id(oci) = %d RETURN DISTINCT id(oci) AS ociID`,
		strings.Join([]string{MediaTypeDockerManifestV2, v1.MediaTypeImageManifest}, `", "`), mdID,
	)
	res, found, err := mag.Database.QueryOne(q)
	require.NoError(t, err)
	require.True(t, found, "reshape should produce an OCI Manifest for %s", leafLabel)
	ociID, err := res.GetInt64("ociID")
	require.NoError(t, err)
	md, found, err := mag.Database.ManifestGetByID(ctx, ociID)
	require.NoError(t, err)
	require.True(t, found)
	layers := make([]string, 0, len(md.Layers))
	for _, lyr := range md.Layers {
		layers = append(layers, lyr.Digest)
	}
	sort.Strings(layers)
	return ociResult{digest: md.Digest, layers: layers}
}

// buildSingleArchFixture sets up the single-arch ft_arm64 fixture (mirrors the
// existing TestComputeManifest), wires Repo/Tag, runs the supplied reshape, and
// returns the resulting OCI Manifest result for the ft_arm64 content-leaf. Each
// call uses a fresh, isolated graph so the wrapper and leaf paths never interfere.
func buildSingleArchFixture(t *testing.T, run reshapeFn) ociResult {
	t.Helper()
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("collection:ft_arm64")
	bs.SetEmptyAll()

	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatalf("SetupTest failed: %v", err)
	}
	defer TeardownTest(bs, br)

	maniID, err := br.GetInt64("manifest:ft_arm64#id")
	require.NoError(t, err)
	colID, err := br.GetInt64("collection:ft#id")
	require.NoError(t, err)
	tagID, err := br.GetInt64("tag:arm64#id")
	require.NoError(t, err)
	require.NoError(t, mag.Database.LinkCollectionTagToManifest(ctx, tagID, maniID))
	require.NoError(t, mag.Database.LinkCollectionToManifest(ctx, colID, maniID))

	require.NoError(t, run(ctx, &br))
	return captureOCIResult(t, ctx, &br, "ft_arm64")
}

// buildMultiArchFixture sets up a multi-arch fixture with both ft_arm64 and
// ft_amd64 content-leaves, wires Repo/Tag for both, runs the supplied reshape,
// and returns the resulting OCI Manifest result for the requested arch leaf.
func buildMultiArchFixture(t *testing.T, targetLeaf string, run reshapeFn) ociResult {
	t.Helper()
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/platform/arch/amd64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
	// arm64 leaf
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	// amd64 leaf
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.SetEmptyAll()

	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatalf("SetupTest failed: %v", err)
	}
	defer TeardownTest(bs, br)

	require.NoError(t, run(ctx, &br))
	return captureOCIResult(t, ctx, &br, targetLeaf)
}

// TestComputeManifestForLeaf_MatchesWrapper_SingleArch asserts that the
// leaf-based ComputeManifestForLeaf produces byte-identical OCI v1 output (same
// OCI Manifest digest + layers) as the wrapper-typed ComputeManifest for a
// single-arch push.
func TestComputeManifestForLeaf_MatchesWrapper_SingleArch(t *testing.T) {
	wrapperRes := buildSingleArchFixture(t, wrapperReshape("ft_arm64"))
	leafRes := buildSingleArchFixture(t, leafReshape("ft_arm64"))

	assert.Equal(t, wrapperRes.digest, leafRes.digest,
		"ComputeManifestForLeaf must produce a byte-identical OCI Manifest (same digest) as ComputeManifest")
	assert.Equal(t, wrapperRes.layers, leafRes.layers,
		"ComputeManifestForLeaf must produce identical OCI layers as ComputeManifest")
	assert.Len(t, leafRes.layers, 2, "single-arch reshape should yield 2 layers")
}

// TestComputeManifestForLeaf_MatchesWrapper_MultiArch asserts byte-identical OCI
// v1 output between the two entrypoints when the wrapper holds MULTIPLE arch
// content-leaves (ft_arm64 + ft_amd64 are both present in the graph). This is
// the meaningful delta from the single-arch case: ComputeManifest iterates all
// of the wrapper's child leaves, so the leaf-based entrypoint must produce the
// same reshape while several siblings coexist.
//
// We assert on the ft_arm64 leaf, the reshapeable target in this fixture: the
// ft_amd64 fixture links its leaf with type "ORIGINAL", which CreateOCIv1 does
// not reshape (only PARSED / UPLOADED_BY / MANIFEST_DERIVED_FROM) — that is a
// fixture property, identical for both entrypoints, not a behavioural diff.
func TestComputeManifestForLeaf_MatchesWrapper_MultiArch(t *testing.T) {
	const targetLeaf = "ft_arm64"
	wrapperRes := buildMultiArchFixture(t, targetLeaf, wrapperReshape(targetLeaf))
	leafRes := buildMultiArchFixture(t, targetLeaf, leafReshape(targetLeaf))
	assert.Equal(t, wrapperRes.digest, leafRes.digest,
		"multi-arch: ComputeManifestForLeaf must match ComputeManifest OCI digest")
	assert.Equal(t, wrapperRes.layers, leafRes.layers,
		"multi-arch: ComputeManifestForLeaf must match ComputeManifest OCI layers")
}

// TestComputeManifestForLeaf_SkipsWithoutWrapper asserts the entrypoint is a
// safe no-op when the content-leaf has no associated wrapper identity (matching
// the prior `found` guard at the metahub-api call site), rather than erroring.
func TestComputeManifestForLeaf_SkipsWithoutWrapper(t *testing.T) {
	ctx := context.Background()
	err := mag.ComputeManifestForLeaf(ctx, types.Manifest{ID: 999999})
	assert.NoError(t, err, "reshape should be a no-op for a leaf without a wrapper identity")
}
