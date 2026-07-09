package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestModelDetailGet seeds a model/<name> manipulation (profile USES it, a
// model-layer image with a tar-layer size, a subject-referenced source
// artifact) plus a sibling config/<name> manipulation (per-profile config image
// + QNIB_MODEL_CONFIG_DIR) and verifies ModelDetailGet assembles the card
// payload: quantizations with size, per-profile config, source artefact.
func TestModelDetailGet(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	_, err = db.Execute(`
		CREATE (mp:Manipulation {label: "model/testmodel"})-[:HAS_VARIANT {rule: "quant:q8-kxl"}]->(mvc:ManipuConfig)
		CREATE (mvc)-[:HAS_LAYER]->(mpl:ManipuLayer {label: "model/testmodel", rule: "quant:q8-kxl", select: ""})
		CREATE (mpl)-[:UPSTREAM]->(up:Upstream {reg: "reg", repo: "models/testmodel-layer", tag: "q8-kxl", digest: "sha256:abc123"})
		CREATE (prf:Profile {label: "vram#12gb"})-[:USES]->(mp)
		CREATE (mdModel:Manifest {digest: "sha256:abc123"})-[:ORIGINAL]->(up)
		CREATE (mdModel)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 262146560, type: "layer"})
		CREATE (mdModel)-[:SUBJECT_OF]->(mdSrc:Manifest {digest: "sha256:src456"})
		CREATE (srcRepo:Repo {repo: "e2e-models/testmodel"})<-[:OF_REPO]-(:Slot)-[:HAS_PUSHED]->(mdSrc)
		CREATE (cmp:Manipulation {label: "config/testmodel"})-[:HAS_DEFAULT]->(mc:ManipuConfig {env: "{\"QNIB_MODEL_CONFIG_DIR\":\"/model/testmodel/config\"}"})
		CREATE (mc)-[:HAS_LAYER]->(cml:ManipuLayer {label: "config/testmodel", rule: "", select: ""})-[:UPSTREAM]->(:Upstream {reg: "reg", repo: "models/testmodel-config", tag: "q8-kxl", digest: "sha256:cfg789"})
		CREATE (prf)-[:USES]->(cmp)
		CREATE (cfgMd:Manifest {digest: "sha256:cfgmani"})-[:SUBJECT_OF]->(mdModel)
		CREATE (cfgMd)-[:ORIGINAL]->(:Upstream {reg: "reg", repo: "models/testmodel-config", tag: "q8-kxl", digest: "sha256:cfgmani"})
		CREATE (cfgMd)-[:HAS_ANNOTATION]->(:Annotation {key: "org.qnib.model.profile", value: "vram#12gb"})
		CREATE (cfgMd)-[:HAS_ANNOTATION]->(:Annotation {key: "org.qnib.model.quant", value: "q8-kxl"})
		CREATE (cfgMd)-[:HAS_ANNOTATION]->(:Annotation {key: "org.qnib.model.runtime", value: "llama-server"})
		CREATE (:Runtime {name: "llama-server", image: "reg/llama-server:1", formats: "gguf", configContract: "qnib.model.config.v1"})
	`)
	require.NoError(t, err)

	d, found, err := db.ModelDetailGet(ctx, "testmodel")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "testmodel", d.Name)
	assert.Equal(t, "model/testmodel", d.Label)
	assert.Equal(t, []string{"vram#12gb"}, d.Profiles)

	// Quantizations grouped under the one model, each with its size.
	require.Len(t, d.Quantizations, 1)
	assert.Equal(t, "models/testmodel-layer", d.Quantizations[0].Repo)
	assert.Equal(t, "q8-kxl", d.Quantizations[0].Tag)
	assert.Equal(t, "sha256:abc123", d.Quantizations[0].Digest)
	assert.Equal(t, int64(262146560), d.Quantizations[0].Size)
	assert.Equal(t, int64(262146560), d.TotalSize)

	// Per-profile config (config/<name>) with image + config dir.
	require.Len(t, d.Configs, 1)
	assert.Equal(t, "vram#12gb", d.Configs[0].Profile)
	assert.Equal(t, "models/testmodel-config", d.Configs[0].Repo)
	assert.Equal(t, "q8-kxl", d.Configs[0].Tag)
	assert.Equal(t, "/model/testmodel/config", d.Configs[0].ConfigDir)

	// Source ORAS artifact via the OCI subject reference.
	require.NotNil(t, d.SourceArtifact)
	assert.Equal(t, "e2e-models/testmodel", d.SourceArtifact.Repo)
	assert.Equal(t, "sha256:src456", d.SourceArtifact.Digest)

	// Composition auto-derived from referrers(model): the config artefact
	// (subject→model-layer manifest) + its {profile,runtime,quant} annotations.
	require.Len(t, d.Compositions, 1)
	c := d.Compositions[0]
	assert.Equal(t, "vram#12gb", c.Profile)
	assert.Equal(t, "q8-kxl", c.Quant)
	assert.Equal(t, "llama-server", c.Runtime)
	assert.Equal(t, "reg/llama-server:1", c.RuntimeImage, "runtime name resolves to its registered image")
	assert.Equal(t, "models/testmodel-config", c.ConfigRepo)
	assert.Equal(t, "sha256:cfgmani", c.ConfigDigest)
}

// TestModelDetailGetNotFound verifies a missing model and an unsafe name both
// return found=false without error (→ the handler answers 404).
func TestModelDetailGetNotFound(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	_, found, err := db.ModelDetailGet(ctx, "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)

	_, found, err = db.ModelDetailGet(ctx, "bad name!$(MATCH)")
	require.NoError(t, err)
	assert.False(t, found)
}

// TestModelDetailGet_RealUpsertPath drives the ACTUAL repackage neo writes —
// ModelQuantVariantUpsert for a model-only push (no config, no source subject)
// then ProfileLinkToManipulation — and asserts ModelDetailGet finds it
// (found=true → 200) with the quant + size populated. This mirrors the e2e
// model-repackage-test end to end: the write→ModelDetailGet seam that
// TestModelQuantVariantUpsert (read via ModelQuantsGetByName) does not cover.
func TestModelDetailGet_RealUpsertPath(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	// A pushed model-layer image (as repackage wires it) + a profile; no
	// manipulation yet, no config, no source subject (model-only push).
	res, err := db.Execute(`
		CREATE (up:Upstream {reg: "reg", repo: "models/testmodel-layer", tag: "q4-km", digest: "sha256:mlayer"})
		CREATE (m:Manifest {digest: "sha256:mlayer"})-[:ORIGINAL]->(up)
		CREATE (m)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 262146560, type: "layer"})
		CREATE (prf:Profile {label: "12gb#bazzite"})
		RETURN id(prf) AS prfID
	`)
	require.NoError(t, err)
	prfID, err := res.GetInt64("prfID")
	require.NoError(t, err)

	// The real repackage neo writes: variant upsert + profile link.
	mpID, err := db.ModelQuantVariantUpsert(ctx, "testmodel", "q4-km", "sha256:mlayer")
	require.NoError(t, err)
	require.NoError(t, db.ProfileLinkToManipulation(ctx, prfID, mpID))

	// The endpoint the e2e curls (GET /api/v1/models/testmodel) → must be found.
	d, found, err := db.ModelDetailGet(ctx, "testmodel")
	require.NoError(t, err)
	require.True(t, found, "model must be found after the real repackage write path")
	assert.Equal(t, []string{"12gb#bazzite"}, d.Profiles)
	require.Len(t, d.Quantizations, 1)
	assert.Equal(t, "models/testmodel-layer", d.Quantizations[0].Repo)
	assert.Equal(t, "q4-km", d.Quantizations[0].Tag)
	assert.Equal(t, int64(262146560), d.Quantizations[0].Size)
}
