package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestModelQuantsGetByName seeds a model/<name> with one quant on the flat
// (HAS→ManipuLayer) path and another on the variant
// (HAS_VARIANT→ManipuConfig→HAS_LAYER) path, and verifies ModelQuantsGetByName
// gathers both (tag + model-layer size) and that types.ResolveModelServe picks
// the highest-precision quant that fits an aggregated profile.
func TestModelQuantsGetByName(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	const gb = int64(1) << 30
	_, err = db.Execute(`
		CREATE (mp:Manipulation {label: "model/multi"})
		CREATE (mp)-[:HAS]->(:ManipuLayer {label: "model/multi", rule: "", select: ""})-[:UPSTREAM]->(up4:Upstream {reg: "reg", repo: "models/multi-layer", tag: "q4", digest: "sha256:q4"})
		CREATE (md4:Manifest {digest: "sha256:q4"})-[:ORIGINAL]->(up4)
		CREATE (md4)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 4294967296, type: "layer"})
		CREATE (mp)-[:HAS_VARIANT {rule: "quant:q8"}]->(mc:ManipuConfig)
		CREATE (mc)-[:HAS_LAYER]->(:ManipuLayer {label: "model/multi", rule: "quant:q8", select: ""})-[:UPSTREAM]->(up8:Upstream {reg: "reg", repo: "models/multi-layer", tag: "q8", digest: "sha256:q8"})
		CREATE (md8:Manifest {digest: "sha256:q8"})-[:ORIGINAL]->(up8)
		CREATE (md8)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 8589934592, type: "layer"})
	`)
	require.NoError(t, err)

	quants, err := db.ModelQuantsGetByName(ctx, "multi")
	require.NoError(t, err)
	require.Len(t, quants, 2)

	bySize := map[string]int64{}
	for _, q := range quants {
		bySize[q.Tag] = q.Size
	}
	assert.Equal(t, 4*gb, bySize["q4"])
	assert.Equal(t, 8*gb, bySize["q8"])

	// Resolution: 12 GB fits both → highest precision q8; 6 GB → q4.
	served, err := types.ResolveModelServe(quants, "strixhalo", 12*gb, "", 1.0)
	require.NoError(t, err)
	assert.Equal(t, "q8", served.Tag)

	served, err = types.ResolveModelServe(quants, "strixhalo", 6*gb, "", 1.0)
	require.NoError(t, err)
	assert.Equal(t, "q4", served.Tag)

	// Named-infeasible: q8 under 6 GB → ErrQuantInfeasible.
	_, err = types.ResolveModelServe(quants, "strixhalo", 6*gb, "q8", 1.0)
	var infeasible *types.ErrQuantInfeasible
	require.ErrorAs(t, err, &infeasible)
}

// TestModelQuantVariantUpsert verifies repeatedly upserting quant variants
// accumulates one variant per quant under a single model/<name> (idempotently),
// links to the already-pushed model-layer manifests without re-fetching, and
// that the result feeds ModelQuantsGetByName + ResolveModelServe.
func TestModelQuantVariantUpsert(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	// Two pushed model-layer images (as repackage wires them), no manipulation yet.
	_, err = db.Execute(`
		CREATE (up4:Upstream {reg: "reg", repo: "models/up-layer", tag: "q4", digest: "sha256:up4"})
		CREATE (m4:Manifest {digest: "sha256:up4"})-[:ORIGINAL]->(up4)
		CREATE (m4)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 4294967296, type: "layer"})
		CREATE (up8:Upstream {reg: "reg", repo: "models/up-layer", tag: "q8", digest: "sha256:up8"})
		CREATE (m8:Manifest {digest: "sha256:up8"})-[:ORIGINAL]->(up8)
		CREATE (m8)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 8589934592, type: "layer"})
	`)
	require.NoError(t, err)

	id4, err := db.ModelQuantVariantUpsert(ctx, "up", "q4", "sha256:up4")
	require.NoError(t, err)
	require.Positive(t, id4)
	id8, err := db.ModelQuantVariantUpsert(ctx, "up", "q8", "sha256:up8")
	require.NoError(t, err)
	require.Equal(t, id4, id8, "both quants attach to the same model manipulation")
	// Idempotent: re-upsert q4.
	_, err = db.ModelQuantVariantUpsert(ctx, "up", "q4", "sha256:up4")
	require.NoError(t, err)

	quants, err := db.ModelQuantsGetByName(ctx, "up")
	require.NoError(t, err)
	require.Len(t, quants, 2, "one variant per quant, no duplicates")

	served, err := types.ResolveModelServe(quants, "strixhalo", int64(12)<<30, "", 1.0)
	require.NoError(t, err)
	require.Equal(t, "q8", served.Tag)

	// Missing pushed manifest → error (must push the image first).
	_, err = db.ModelQuantVariantUpsert(ctx, "up", "q2", "sha256:missing")
	require.Error(t, err)
}

// TestModelQuantVariantUpsert_ReplacesStaleImage verifies that re-repackaging the
// same quant with a DIFFERENT manifest digest (non-deterministic assembly or a
// re-seed) replaces the quant variant's model-layer image link rather than
// accumulating a second one — so the model lists the quant exactly once (the
// current image), never twice.
func TestModelQuantVariantUpsert_ReplacesStaleImage(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	// Two pushed model-layer manifests for the SAME repo:tag, different digests.
	_, err = db.Execute(`
		CREATE (upA:Upstream {reg: "reg", repo: "models/rep-layer", tag: "q4", digest: "sha256:repA"})
		CREATE (mA:Manifest {digest: "sha256:repA"})-[:ORIGINAL]->(upA)
		CREATE (mA)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 4294967296, type: "layer"})
		CREATE (upB:Upstream {reg: "reg", repo: "models/rep-layer", tag: "q4", digest: "sha256:repB"})
		CREATE (mB:Manifest {digest: "sha256:repB"})-[:ORIGINAL]->(upB)
		CREATE (mB)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 4294967296, type: "layer"})
	`)
	require.NoError(t, err)

	idA, err := db.ModelQuantVariantUpsert(ctx, "rep", "q4", "sha256:repA")
	require.NoError(t, err)
	quants, err := db.ModelQuantsGetByName(ctx, "rep")
	require.NoError(t, err)
	require.Len(t, quants, 1)
	require.Equal(t, "sha256:repA", quants[0].Digest)

	// Re-repackage of the SAME quant → image B. Must replace, not accumulate.
	idB, err := db.ModelQuantVariantUpsert(ctx, "rep", "q4", "sha256:repB")
	require.NoError(t, err)
	require.Equal(t, idA, idB, "both upserts attach to the same model manipulation")
	quants, err = db.ModelQuantsGetByName(ctx, "rep")
	require.NoError(t, err)
	require.Len(t, quants, 1, "quant variant references exactly one (current) image")
	require.Equal(t, "sha256:repB", quants[0].Digest, "re-repackaged image replaces the stale one")
}

// TestModelQuantVariantUpsert_DuplicateUpstream guards the real repackage graph
// shape: metahub's push registers one Upstream (keyed on the API host) while
// repackage's own UpstreamGetInsert adds another (keyed on the client registry
// URL) — same repo:tag:digest, different reg — so the model-layer Manifest ends
// up with TWO ORIGINAL→Upstream edges. The upsert must still persist exactly one
// manipulation. Before the fan-out fix the ManipuLayer MERGE ran once per
// upstream row; the second insert hit the ManipuLayer (label, rule) UNIQUE
// constraint and rolled the whole upsert back — silently, because Execute does
// not surface the commit-time constraint error — so no manipulation persisted and
// GET /api/v1/models/<name> returned 404 in the model-repackage-test e2e.
func TestModelQuantVariantUpsert_DuplicateUpstream(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	// One pushed model-layer image, but TWO Upstreams (different reg, same
	// repo:tag:digest) both ORIGINAL of the one manifest — as push + repackage wire it.
	_, err = db.Execute(`
		CREATE (m:Manifest {digest: "sha256:dup"})
		CREATE (m)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 262146560, type: "layer"})
		CREATE (m)-[:ORIGINAL]->(:Upstream {reg: "api:80", repo: "models/dup-layer", tag: "q4", digest: "sha256:dup"})
		CREATE (m)-[:ORIGINAL]->(:Upstream {reg: "localhost:8080", repo: "models/dup-layer", tag: "q4", digest: "sha256:dup"})
	`)
	require.NoError(t, err)

	_, err = db.ModelQuantVariantUpsert(ctx, "dup", "q4", "sha256:dup")
	require.NoError(t, err)

	// The manipulation must actually persist (pre-fix: constraint rollback → gone).
	d, found, err := db.ModelDetailGet(ctx, "dup")
	require.NoError(t, err)
	require.True(t, found, "manipulation must persist despite the duplicate ORIGINAL→Upstream fan-out")
	require.Len(t, d.Quantizations, 1)
	assert.Equal(t, "q4", d.Quantizations[0].Tag)
	assert.Equal(t, int64(262146560), d.Quantizations[0].Size)
}

// TestModelQuantVariantUpsert_OrgQualifiedName guards the model-centric-pull
// invariant that model names keep the org (`models/<org>/<name>`), so the name
// carries a `/` (e.g. qnib/minicpm5). The neo safe-name guard must accept it —
// before the fix modelNameSafe rejected the slash and repackage fataled with
// "unsafe model name", aborting the seed after the model-layer push (leaving a
// partial graph with no model/<name> manipulation).
func TestModelQuantVariantUpsert_OrgQualifiedName(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	// A pushed org-qualified model-layer image, as repackage --name qnib/minicpm5 wires it.
	_, err = db.Execute(`
		CREATE (up:Upstream {reg: "metahub-api:8080", repo: "models/qnib/minicpm5-layer", tag: "q8-0", digest: "sha256:orgq8"})
		CREATE (m:Manifest {digest: "sha256:orgq8"})-[:ORIGINAL]->(up)
		CREATE (m)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 1153532416, type: "layer"})
	`)
	require.NoError(t, err)

	// id(mp) can legitimately be 0 (Neo4j internal ids start at 0), so a
	// positivity check is unsafe — assert the round-trip instead.
	_, err = db.ModelQuantVariantUpsert(ctx, "qnib/minicpm5", "q8-0", "sha256:orgq8")
	require.NoError(t, err, "org-qualified model name must be accepted (the / is safe)")

	quants, err := db.ModelQuantsGetByName(ctx, "qnib/minicpm5")
	require.NoError(t, err)
	require.Len(t, quants, 1)
	assert.Equal(t, "q8-0", quants[0].Tag)

	d, found, err := db.ModelDetailGet(ctx, "qnib/minicpm5")
	require.NoError(t, err)
	require.True(t, found, "model/<org>/<name> manipulation must resolve via ModelDetailGet")
	assert.Equal(t, "model/qnib/minicpm5", d.Label)
	require.Len(t, d.Quantizations, 1)
	assert.Equal(t, "q8-0", d.Quantizations[0].Tag)
}

// TestModelQuantsGetByName_Unknown returns an empty slice (not an error) for a
// missing or unsafe name.
func TestModelQuantsGetByName_Unknown(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	got, err := db.ModelQuantsGetByName(ctx, "does-not-exist")
	require.NoError(t, err)
	assert.Empty(t, got)

	got, err = db.ModelQuantsGetByName(ctx, "bad name!$(x)")
	require.NoError(t, err)
	assert.Empty(t, got)
}
