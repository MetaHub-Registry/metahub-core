package neo

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestLinkSubject(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	// Actual test
	refID, err := br.GetInt64(`manifestdata:ft_arm64#id`)
	if err != nil {
		t.Fatal(err)
	}
	subDigest, err := br.Get(`manifestdata:ft_amd64#digest`)
	if err != nil {
		t.Fatal(err)
	}
	subMediaType, err := br.Get(`manifestdata:ft_amd64#mediaType`)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.LinkSubjectByDigestMediaType(ctx, refID, subDigest, subMediaType); err != nil {
		t.Fatal(err)
	}
	res, err := db.ReferrersGetBySubjectDigest(ctx, subDigest)
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, res.Manifests, 1)
}

// TestReferrersGetBySubjectDigestIncludesIndex verifies that an :Index node
// linked via SUBJECT_OF is included in the referrers response. Before the
// fix, only :Manifest nodes were returned because the switch statement in
// ReferrersGetBySubjectDigest did not recognise the :Index label.
func TestReferrersGetBySubjectDigestIncludesIndex(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// Get the subject :Manifest node (ft_amd64) to use as the Subject
	subDigest, err := br.Get(`manifestdata:ft_amd64#digest`)
	require.NoError(t, err)
	subMediaType, err := br.Get(`manifestdata:ft_amd64#mediaType`)
	require.NoError(t, err)

	// Create an :Index node (the referrer) with a known digest
	indexDigest := digest.FromString("test-index-referrer")
	indexMediaType := "application/vnd.oci.image.index.v1+json"
	indexID, err := db.InsertManifestList(ctx, indexDigest, indexMediaType, 42)
	require.NoError(t, err)

	// Add an annotation to the :Index so we can verify it round-trips
	anno := types.NewAnnotation("org.opencontainers.test", "index-referrer")
	err = db.ManifestListAddAnnotation(ctx, indexID, &anno)
	require.NoError(t, err)

	// Link the :Index → SUBJECT_OF → :Manifest (the subject)
	err = db.LinkSubjectByDigestMediaType(ctx, indexID, subDigest, subMediaType)
	require.NoError(t, err)

	// Also link a :Manifest referrer (ft_arm64) for comparison
	refID, err := br.GetInt64(`manifestdata:ft_arm64#id`)
	require.NoError(t, err)
	err = db.LinkSubjectByDigestMediaType(ctx, refID, subDigest, subMediaType)
	require.NoError(t, err)

	// Query referrers — should return both the :Manifest and :Index
	refs, err := db.ReferrersGetBySubjectDigest(ctx, subDigest)
	require.NoError(t, err)
	assert.Len(t, refs.Manifests, 2, "expected both Manifest and Index referrers")

	// Verify the Index descriptor is present with correct properties
	foundIndex := false
	for _, desc := range refs.Manifests {
		if desc.Digest.String() == indexDigest.String() {
			foundIndex = true
			assert.Equal(t, indexMediaType, desc.MediaType)
			assert.Equal(t, "index-referrer", desc.Annotations["org.opencontainers.test"])
		}
	}
	assert.True(t, foundIndex, "Index referrer must appear in referrers response")
}
