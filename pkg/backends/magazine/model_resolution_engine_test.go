package magazine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// seedOversizedModel wires a profile → model/huge whose only quant (q8) is 32 GB.
func seedOversizedModel(ctx context.Context, t *testing.T, profileLabel, digest string) types.Profile {
	t.Helper()
	res, err := mag.Database.Execute(`
		CREATE (prf:Profile {label: "` + profileLabel + `", name: "n", tag: "t"})
		CREATE (mp:Manipulation {label: "model/huge"})
		CREATE (prf)-[:USES]->(mp)
		CREATE (mpl:ManipuLayer {label: "model/huge", rule: "", select: ""})
		CREATE (mp)-[:HAS]->(mpl)
		CREATE (mpl)-[:UPSTREAM]->(up:Upstream {reg: "reg", repo: "models/huge-layer", tag: "q8", digest: "` + digest + `"})
		CREATE (img:Manifest {digest: "` + digest + `", mediaType: "application/vnd.oci.image.manifest.v1+json", label: "reg/models/huge-layer:q8", size: 700})
		CREATE (mpl)-[:USES]->(img)
		CREATE (img)-[:ORIGINAL]->(up)
		CREATE (img)-[:HAS_LAYER {order: 0}]->(:Blob {digest: "sha256:lyr", type: "layer", mediaType: "application/vnd.oci.image.layer.v1.tar", size: 34359738368})
		CREATE (img)-[:USES {mediaType: "config"}]->(:Blob {digest: "sha256:cfg", type: "config", mediaType: "application/vnd.oci.image.config.v1+json", size: 100})
		RETURN id(prf) AS pid
	`)
	require.NoError(t, err)
	pid, err := res.GetInt64("pid")
	require.NoError(t, err)
	// Build the Profile directly (label + graph id + hardware) — the dynamic model
	// path now gates on a model-serving hardware block and reads capacity from
	// hardware.gpu.vram_gb (12 GB here), no longer parsing the label.
	// ProfileGetByLabel would require a hydrated rule this minimal fixture omits.
	return types.Profile{
		ID:       pid,
		Label:    profileLabel,
		Hardware: &types.Hardware{GPU: &types.HardwareGPU{Vendor: "amd", VRAMGB: 12}},
	}
}

// TestApplyManipulations_ModelRejectInfeasible: under a platform-bearing
// aggregated profile (12gb#strixhalo), a model whose only quant (32 GB) exceeds
// the 12 GB capacity makes ApplyManipulationsToManifests return a fatal
// *ErrQuantInfeasible — the pull-path reject — rather than serve an unrunnable
// image.
func TestApplyManipulations_ModelRejectInfeasible(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := mag.SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	prf := seedOversizedModel(ctx, t, "12gb#strixhalo", "sha256:huge")

	// Model-centric pull: thread RequestedModel so the model-serving stack fires.
	ctx = types.WithRequestedModel(ctx, "huge")
	base := types.Manifest{Digest: "sha256:base", Platform: types.Platform{OS: "linux", Arch: "amd64"}}
	_, err = mag.ApplyManipulationsToManifests(ctx, prf, []types.Manifest{base})

	var infeasible *types.ErrQuantInfeasible
	require.ErrorAs(t, err, &infeasible)
	require.Equal(t, "12gb#strixhalo", infeasible.Profile)
	// Best-fit path (no requested quant): the reject names no specific quant.
	require.Equal(t, "", infeasible.Quant)
}

// TestApplyManipulations_NamedQuantThreaded verifies the model-centric pull tag is
// threaded into resolution via the request context: with an explicit quant, the
// reject names THAT quant (vs "" for the best-fit path), proving
// types.WithRequestedQuant reaches tryApplyModelManipulation → ResolveModelServe.
func TestApplyManipulations_NamedQuantThreaded(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := mag.SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	prf := seedOversizedModel(ctx, t, "12gb#strixhalo", "sha256:huge")

	// Pull models/huge:q8 explicitly — q8 (32 GB) exceeds the 12 GB profile → reject,
	// and the reject must name the requested quant.
	ctx = types.WithRequestedModel(ctx, "huge")
	ctx = types.WithRequestedQuant(ctx, "q8")
	base := types.Manifest{Digest: "sha256:base", Platform: types.Platform{OS: "linux", Arch: "amd64"}}
	_, err = mag.ApplyManipulationsToManifests(ctx, prf, []types.Manifest{base})

	var infeasible *types.ErrQuantInfeasible
	require.ErrorAs(t, err, &infeasible)
	require.Equal(t, "q8", infeasible.Quant, "requested quant must be threaded through the context")
}

// seedModelAndConfig wires a model-serving profile with a model/<name>
// manipulation (variant path) and a config/<name> manipulation (legacy flat
// HAS edge). The model's only quant (q8-0) is 32 GB — oversized for the 12 GB
// profile, so the model-serving stack rejects with ErrQuantInfeasible. This
// avoids needing MinIO data (the rejection happens at quant resolution,
// before layer/config loading). The config manipulation (empty rule,
// match-all) would match via the static path without the filter.
func seedModelAndConfig(ctx context.Context, t *testing.T, profileLabel, modelDigest string) types.Profile {
	t.Helper()
	res, err := mag.Database.Execute(`
		CREATE (prf:Profile {label: "` + profileLabel + `", name: "n", tag: "t"})
		// Model manipulation (variant path)
		CREATE (mp:Manipulation {label: "model/testmodel"})
		CREATE (prf)-[:USES]->(mp)
		CREATE (mp)-[:HAS_VARIANT {rule: "quant:q8-0"}]->(mc:ManipuConfig)
		CREATE (mc)-[:HAS_LAYER]->(mpl:ManipuLayer {label: "model/testmodel", rule: "quant:q8-0", select: ""})
		CREATE (mpl)-[:UPSTREAM]->(up:Upstream {reg: "reg", repo: "models/testmodel-layer", tag: "q8-0", digest: "` + modelDigest + `"})
		CREATE (img:Manifest {digest: "` + modelDigest + `", mediaType: "application/vnd.oci.image.manifest.v1+json", label: "reg/models/testmodel-layer:q8-0", size: 700})
		CREATE (mpl)-[:USES]->(img)
		CREATE (img)-[:ORIGINAL]->(up)
		CREATE (img)-[:HAS_LAYER {order: 0}]->(:Blob {digest: "sha256:modellyr", type: "layer", mediaType: "application/vnd.oci.image.layer.v1.tar", size: 34359738368})
		CREATE (img)-[:USES {mediaType: "config"}]->(:Blob {digest: "sha256:modelcfg", type: "config", mediaType: "application/vnd.oci.image.config.v1+json", size: 100})
		// Config manipulation (legacy flat HAS edge, empty rule — match-all)
		CREATE (cmp:Manipulation {label: "config/testmodel"})
		CREATE (prf)-[:USES]->(cmp)
		CREATE (cmpLyr:ManipuLayer {label: "config/testmodel", rule: "", select: ""})
		CREATE (cmp)-[:HAS]->(cmpLyr)
		CREATE (cmpImg:Manifest {digest: "sha256:cmpimg", mediaType: "application/vnd.oci.image.manifest.v1+json", label: "configimg", size: 300})
		CREATE (cmpLyr)-[:USES]->(cmpImg)
		CREATE (cmpLyr)-[:UPSTREAM]->(:Upstream {reg: "reg", repo: "models/testmodel-config", tag: "q8-0", digest: "sha256:cmpimg"})
		CREATE (cmpImg)-[:HAS_LAYER {order: 0}]->(:Blob {digest: "sha256:cfgplyr", type: "layer", mediaType: "application/vnd.oci.image.layer.v1.tar", size: 200})
		CREATE (cmpImg)-[:USES {mediaType: "config"}]->(:Blob {digest: "sha256:cfgpcfg", type: "config", mediaType: "application/vnd.oci.image.config.v1+json", size: 50})
		RETURN id(prf) AS pid
	`)
	require.NoError(t, err)
	pid, err := res.GetInt64("pid")
	require.NoError(t, err)
	return types.Profile{
		ID:       pid,
		Label:    profileLabel,
		Hardware: &types.Hardware{GPU: &types.HardwareGPU{Vendor: "amd", VRAMGB: 12}},
	}
}

// TestApplyManipulations_CollectionPullNoModelInjection verifies that a
// collection pull under a model-serving profile does NOT fire the
// model-serving stack. Without the gate, the oversized model would trigger
// ErrQuantInfeasible even on a collection pull. With the gate, the static
// path runs (skipping model/config), and the original is returned unchanged.
func TestApplyManipulations_CollectionPullNoModelInjection(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := mag.SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	prf := seedModelAndConfig(ctx, t, "12gb#frame", "sha256:testmodel")

	// NO WithRequestedModel — this is a collection pull, not model-centric.
	base := types.Manifest{Digest: "sha256:base", Platform: types.Platform{OS: "linux", Arch: "amd64"}}
	result, err := mag.ApplyManipulationsToManifests(ctx, prf, []types.Manifest{base})
	require.NoError(t, err, "collection pull must NOT fire model-serving stack (no ErrQuantInfeasible)")
	require.Len(t, result, 1, "Should have 1 manifest in result")
	assert.Equal(t, base.ID, result[0].ID, "Collection pull should return the original unchanged (model/config skipped)")
	t.Logf("Collection pull result: ID=%d, Digest=%s (model/config NOT injected)", result[0].ID, result[0].Digest)
}

// TestApplyManipulations_ModelCentricPullFiresStack verifies that a
// model-centric pull (RequestedModel set) under a model-serving profile
// DOES fire the model-serving stack. The oversized model (32 GB > 12 GB VRAM)
// makes the stack reject with ErrQuantInfeasible — proving the stack fired.
// Without the gate, collection pulls would also get this error.
func TestApplyManipulations_ModelCentricPullFiresStack(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := mag.SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	prf := seedModelAndConfig(ctx, t, "12gb#frame", "sha256:testmodel")

	// Model-centric pull: RequestedModel set → model-serving stack fires.
	ctx = types.WithRequestedModel(ctx, "testmodel")
	ctx = types.WithRequestedQuant(ctx, "q8-0")
	base := types.Manifest{Digest: "sha256:base", Platform: types.Platform{OS: "linux", Arch: "amd64"}}
	_, err = mag.ApplyManipulationsToManifests(ctx, prf, []types.Manifest{base})

	var infeasible *types.ErrQuantInfeasible
	require.ErrorAs(t, err, &infeasible, "model-centric pull should fire the stack and reject with ErrQuantInfeasible")
	assert.Equal(t, "12gb#frame", infeasible.Profile, "reject should name the profile")
	assert.Equal(t, "q8-0", infeasible.Quant, "reject should name the requested quant")
}

// TestApplyManipulations_CollectionPullSkipsConfigManip verifies that the
// static-path filter skips config/<name> manipulations on collection pulls.
// The config manipulation has an empty rule (match-all) and would match via
// the static path without the filter, causing a non-fatal error (no MinIO
// data). With the filter, it's skipped entirely — no error, original returned.
func TestApplyManipulations_CollectionPullSkipsConfigManip(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := mag.SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	prf := seedModelAndConfig(ctx, t, "12gb#frame", "sha256:testmodel")

	// NO WithRequestedModel — collection pull. The config manipulation
	// (config/testmodel, empty rule, match-all) must be skipped by the
	// static-path filter. Without the filter, tryApplyManipulation would
	// match it and crash trying to load config from MinIO for a synthetic
	// manifest (ID=0).
	base := types.Manifest{Digest: "sha256:base", Platform: types.Platform{OS: "linux", Arch: "amd64"}}
	result, err := mag.ApplyManipulationsToManifests(ctx, prf, []types.Manifest{base})
	require.NoError(t, err, "config manipulation must be skipped (not matched) on collection pulls")
	require.Len(t, result, 1)
	assert.Equal(t, base.ID, result[0].ID, "original should be returned unchanged")
	t.Logf("Collection pull with config manip: ID=%d (config skipped, no crash)", result[0].ID)
}
