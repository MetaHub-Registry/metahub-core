package magazine

import (
	"context"
	"testing"

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
	ctx = types.WithRequestedQuant(ctx, "q8")
	base := types.Manifest{Digest: "sha256:base", Platform: types.Platform{OS: "linux", Arch: "amd64"}}
	_, err = mag.ApplyManipulationsToManifests(ctx, prf, []types.Manifest{base})

	var infeasible *types.ErrQuantInfeasible
	require.ErrorAs(t, err, &infeasible)
	require.Equal(t, "q8", infeasible.Quant, "requested quant must be threaded through the context")
}
