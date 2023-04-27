package magazine

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Upstream_ManifestInsert(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/zen2.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen2")
	bs.AddBootstrap("profile:profiles/cpu/zen2.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := mag.SetupTest(ctx, bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		mag.Database.TeardownTest(bs)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs)
	// Actual test
	//// ZEN2
	req2 := types.Request{
		Host:    regHttpURI,
		Repo:    "alpine",
		Tag:     "3.17.3",
		Profile: "cpu#zen2",
	}
	blob2 := types.BlobMeta{
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Digest:    "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11",
	}
	err = mag.UpstreamBlob_InsertManifest(ctx, req2, blob2)
	if err != nil {
		t.Fatal(err)
	}
	/// is blob present: manifest
	upMani2 := upstream.UpstreamTriple{
		Reg:       req2.Host,
		Repo:      req2.Repo,
		Tag:       req2.Tag,
		Digest:    blob2.Digest,
		MediaType: blob2.MediaType,
	}
	_, found2, err := mag.Minio.GetManifest(ctx, upMani2)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found2)
	maniLabel2 := fmt.Sprintf("%s/alpine:3.17.3@cpu#zen2", regHttpURI)
	mani2, _, err := mag.Database.Manifest_GetByLabel(ctx, maniLabel2)
	if err != nil {
		t.Logf("maniLabel: %s", maniLabel2)
		t.Fatal(err)
	}
	assert.Equal(t, "alpine", mani2.Upstream.Repo)
	// TODO: check if upstream is in DB
	// TODO: check if upstream is linked to manifest
	// TODO: check if manifest is linked to profile
	// TODO: Minio
	/// TODO: is blob present: cfg
	//        _, found, err := mag.Minio.GetConfig(ctx, maniData)
}

func Test_Upstream_ManifestInsert_Zen2_Zen3(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/zen2.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen2")
	bs.AddBootstrap("profile:profiles/cpu/zen2.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := mag.SetupTest(ctx, bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		TeardownTest(bs)
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Actual test
	//// ZEN2
	req2 := types.Request{
		Host:    regHttpURI,
		Repo:    "alpine",
		Tag:     "3.17.3",
		Profile: "cpu#zen2",
	}
	blob2 := types.BlobMeta{
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Digest:    "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11",
	}
	err = mag.UpstreamBlob_InsertManifest(ctx, req2, blob2)
	if err != nil {
		t.Fatal(err)
	}
	/// is blob present: manifest
	upMani2 := upstream.UpstreamTriple{
		Reg:       req2.Host,
		Repo:      req2.Repo,
		Tag:       req2.Tag,
		Digest:    blob2.Digest,
		MediaType: blob2.MediaType,
	}
	_, found2, err := mag.Minio.GetManifest(ctx, upMani2)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found2)
	maniLabel2 := fmt.Sprintf("%s/alpine:3.17.3@cpu#zen2", regHttpURI)
	mani2, _, err := mag.Database.Manifest_GetByLabel(ctx, maniLabel2)
	if err != nil {
		t.Logf("maniLabel: %s", maniLabel2)
		t.Fatal(err)
	}
	assert.Equal(t, "alpine", mani2.Upstream.Repo)
	// TODO: check if upstream is in DB
	// TODO: check if upstream is linked to manifest
	// TODO: check if manifest is linked to profile
	// TODO: Minio
	/// TODO: is blob present: cfg
	//        _, found, err := mag.Minio.GetConfig(ctx, maniData)
	//// ZEN3
	req3 := types.Request{
		Host:    regHttpURI,
		Repo:    "alpine",
		Tag:     "3.17.3",
		Profile: "cpu#zen3",
	}
	blob3 := types.BlobMeta{
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Digest:    "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11",
	}
	err = mag.UpstreamBlob_InsertManifest(ctx, req3, blob3)
	if err != nil {
		t.Fatal(err)
	}
	/// is blob present: manifest
	upMani3 := upstream.UpstreamTriple{
		Reg:       req3.Host,
		Repo:      req3.Repo,
		Tag:       req3.Tag,
		Digest:    blob3.Digest,
		MediaType: blob3.MediaType,
	}
	_, found3, err := mag.Minio.GetManifest(ctx, upMani3)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found3)
	maniLabel3 := fmt.Sprintf("%s/alpine:3.17.3@cpu#zen3", regHttpURI)
	mani3, _, err := mag.Database.Manifest_GetByLabel(ctx, maniLabel3)
	if err != nil {
		t.Logf("maniLabel: %s", maniLabel3)
	}
	assert.Equal(t, "alpine", mani3.Upstream.Repo)
	// TODO: check if upstream is in DB
	// TODO: check if upstream is linked to manifest
	// TODO: check if manifest is linked to profile
	// TODO: Minio
	/// TODO: is blob present: cfg
	//        _, found, err := mag.Minio.GetConfig(ctx, maniData)

}

func Test_Upstream_BlobInsert_ManifestList(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := mag.SetupTest(ctx, bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		mag.Database.TeardownTest(bs)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs)
	// Actual test
	req := types.Request{
		Host:    regHttpURI,
		Repo:    "alpine",
		Tag:     "3.17.2",
		Profile: "cpu#zen3",
	}
	blob := types.BlobMeta{
		MediaType: "application/vnd.docker.distribution.manifest.list.v2+json",
		Digest:    "sha256:ff6bdca1701f3a8a67e328815ff2346b0e4067d32ec36b7992c1fdc001dc8517",
	}
	err = mag.UpstreamBlob_InsertIndex(ctx, req, blob)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Upstream_DiscoverUpstream(t *testing.T) {
	if testing.Short() {
		t.Log("skipping test")
		return
	}
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := mag.SetupTest(ctx, bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		TeardownTest(bs)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs)
	// Actual test
	repo := "alpine"
	tag := "3.17.3"
	blob, reg, found, err := mag.DiscoverUpstream(ctx, repo, tag)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	//t.Logf("blob: %v", blob)
	assert.True(t, strings.HasPrefix(reg, "localhost:"))
	assert.Equal(t, "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", blob.Digest.String())
}

func Test_Upstream_DiscoverUpstream_ManifestList(t *testing.T) {
	if testing.Short() {
		t.Log("skipping test")
		return
	}
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := mag.SetupTest(ctx, bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		mag.Database.TeardownTest(bs)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs)
	// Actual test
	repo := "alpine"
	tag := "3.17.2"
	blob, reg, found, err := mag.DiscoverUpstream(ctx, repo, tag)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	//t.Logf("blob: %v", blob)
	assert.True(t, strings.HasPrefix(reg, "localhost:"))
	assert.Equal(t, "sha256:ff6bdca1701f3a8a67e328815ff2346b0e4067d32ec36b7992c1fdc001dc8517", blob.Digest.String())
	assert.Equal(t, "application/vnd.docker.distribution.manifest.list.v2+json", blob.MediaType)
}
