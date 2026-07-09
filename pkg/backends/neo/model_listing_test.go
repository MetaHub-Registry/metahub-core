package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestModelListingGetAll seeds a `model/<name>` manipulation (a profile USES it,
// its ManipuLayer UPSTREAMs the repackaged model-layer image) and verifies
// ModelListingGetAll returns the catalog entry with name, profile, and image.
func TestModelListingGetAll(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	_, err = db.Execute(`
		CREATE (mp:Manipulation {label: "model/testmodel"})-[:HAS]->(mpl:ManipuLayer {label: "model/testmodel", rule: "", select: ""})
		CREATE (mpl)-[:UPSTREAM]->(up:Upstream {reg: "reg", repo: "models/testmodel-layer", tag: "q8-kxl", digest: "sha256:abc123"})
		CREATE (prf:Profile {label: "vram#12gb"})-[:USES]->(mp)
		CREATE (mdModel:Manifest {digest: "sha256:abc123"})-[:ORIGINAL]->(up)
		CREATE (mdModel)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 262146560, type: "layer"})
		CREATE (mdModel)-[:SUBJECT_OF]->(mdSrc:Manifest {digest: "sha256:src456"})
		CREATE (srcRepo:Repo {repo: "e2e-models/testmodel"})<-[:OF_REPO]-(:Slot)-[:HAS_PUSHED]->(mdSrc)
	`)
	require.NoError(t, err)

	models, err := db.ModelListingGetAll(ctx)
	require.NoError(t, err)
	require.Len(t, models, 1)
	m := models[0]
	assert.Equal(t, "testmodel", m.Name)
	assert.Equal(t, "model/testmodel", m.Label)
	assert.Equal(t, []string{"vram#12gb"}, m.Profiles)
	require.Len(t, m.Images, 1)
	assert.Equal(t, "models/testmodel-layer", m.Images[0].Repo)
	assert.Equal(t, "q8-kxl", m.Images[0].Tag)
	assert.Equal(t, "sha256:abc123", m.Images[0].Digest)
	// Model-layer size flows through to the image and the catalog total.
	assert.Equal(t, int64(262146560), m.Images[0].Size)
	assert.Equal(t, int64(262146560), m.TotalSize)
	// Source ORAS artifact via the OCI subject reference (SUBJECT_OF).
	require.NotNil(t, m.SourceArtifact)
	assert.Equal(t, "e2e-models/testmodel", m.SourceArtifact.Repo)
	assert.Equal(t, "sha256:src456", m.SourceArtifact.Digest)
}

// TestCollectionListingExcludesModelImages verifies a repackaged model-layer
// image repo (the UPSTREAM of a `model/<name>` manipulation) is filtered out of
// the collection listing, while a normal image repo still appears.
func TestCollectionListingExcludesModelImages(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	// Control: a normal image repo (must appear). Subject: a model-layer image
	// repo that also satisfies the container-image config pattern but is wired as
	// a model manipulation's upstream (must be excluded).
	_, err = db.Execute(`
		CREATE (ca:Repo {repo: "apps/web"})-[:HAS_TAG]->(:Tag {name: "latest"})
		CREATE (ca)-[:UPSTREAM]->(:Manifest {digest: "sha256:aaa"})-[:USES]->(:Blob {type: "config", mediaType: "application/vnd.oci.image.config.v1+json"})
		CREATE (cm:Repo {repo: "models/testmodel-layer"})-[:HAS_TAG]->(:Tag {name: "q8-kxl"})
		CREATE (cm)-[:UPSTREAM]->(:Manifest {digest: "sha256:bbb"})-[:USES]->(:Blob {type: "config", mediaType: "application/vnd.oci.image.config.v1+json"})
		CREATE (mp:Manipulation {label: "model/testmodel"})-[:HAS]->(mpl:ManipuLayer {label: "model/testmodel", rule: "", select: ""})
		CREATE (mpl)-[:UPSTREAM]->(:Upstream {reg: "reg", repo: "models/testmodel-layer", tag: "q8-kxl", digest: "sha256:bbb"})
	`)
	require.NoError(t, err)

	cols, err := db.CollectionListingGetByRepoLike(ctx, "")
	require.NoError(t, err)
	repos := make(map[string]bool)
	for _, c := range cols {
		repos[c.Repo] = true
	}
	assert.True(t, repos["apps/web"], "normal image repo should appear in the collection listing")
	assert.False(t, repos["models/testmodel-layer"], "model-layer image repo should be excluded from the collection listing")
}

// TestCollectionListingExcludesVariantModelAndSource covers the post-B1 shape: a
// multi-quant model whose layer image is reached via the HAS_VARIANT path must be
// excluded, and the model's SOURCE artefact repo (subject of the model-layer's
// OCI subject ref) must also be excluded (deferred task 4.3). A normal repo still
// appears.
func TestCollectionListingExcludesVariantModelAndSource(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	_, err = db.Execute(`
		CREATE (ca:Repo {repo: "apps/web"})-[:HAS_TAG]->(:Tag {name: "latest"})
		CREATE (ca)-[:UPSTREAM]->(:Manifest {digest: "sha256:web"})-[:USES]->(:Blob {type: "config", mediaType: "application/vnd.oci.image.config.v1+json"})
		CREATE (mp:Manipulation {label: "model/vmodel"})-[:HAS_VARIANT {rule: "quant:q8"}]->(mc:ManipuConfig)
		CREATE (mc)-[:HAS_LAYER]->(:ManipuLayer {label: "model/vmodel", rule: "quant:q8", select: ""})-[:UPSTREAM]->(up:Upstream {reg: "reg", repo: "models/vmodel-layer", tag: "q8", digest: "sha256:vlayer"})
		CREATE (cm:Repo {repo: "models/vmodel-layer"})-[:HAS_TAG]->(:Tag {name: "q8"})
		CREATE (mlMani:Manifest {digest: "sha256:vlayer"})
		CREATE (cm)-[:UPSTREAM]->(mlMani)
		CREATE (mlMani)-[:USES]->(:Blob {type: "config", mediaType: "application/vnd.oci.image.config.v1+json"})
		CREATE (mlMani)-[:ORIGINAL]->(up)
		CREATE (mlMani)-[:SUBJECT_OF]->(src:Manifest {digest: "sha256:vsrc"})
		CREATE (srcRepo:Repo {repo: "e2e-models/vmodel"})-[:HAS_TAG]->(:Tag {name: "q8"})
		CREATE (srcRepo)-[:UPSTREAM]->(:Manifest {digest: "sha256:vsrc2"})-[:USES]->(:Blob {type: "config", mediaType: "application/vnd.oci.image.config.v1+json"})
		CREATE (srcRepo)<-[:OF_REPO]-(:Slot)-[:HAS_PUSHED]->(src)
	`)
	require.NoError(t, err)

	cols, err := db.CollectionListingGetByRepoLike(ctx, "")
	require.NoError(t, err)
	repos := make(map[string]bool)
	for _, c := range cols {
		repos[c.Repo] = true
	}
	assert.True(t, repos["apps/web"], "normal image repo should appear")
	assert.False(t, repos["models/vmodel-layer"], "variant-form model-layer image should be excluded")
	assert.False(t, repos["e2e-models/vmodel"], "model source artefact repo should be excluded")
}
