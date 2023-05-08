package magazine

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	shab5a                = "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11"
	shaff6                = "sha256:ff6bdca1701f3a8a67e328815ff2346b0e4067d32ec36b7992c1fdc001dc8517"
	MediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	MediaTypeManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeImageConfig  = "application/vnd.docker.container.image.v1+json"
)

func TestUpstreamManifestInsert(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen2.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen2")
	bs.AddBootstrap("profile:profiles/cpu/zen2.yaml")
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
		TeardownTest(bs)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs)
	// Actual test
	//// Generic
	reqGen := types.Request{
		Host:    regHttpURI,
		Repo:    "alpine",
		Tag:     "3.17.3",
		Profile: "generic",
	}
	_, _, e := mag.Database.Collection_GetCreate_ByRepoTag(ctx, reqGen.Repo, reqGen.Tag)
	if e != nil {
		t.Fatal(e)
	}

	blobGen := types.BlobMeta{
		MediaType: MediaTypeManifest,
		Digest:    shab5a,
	}
	err = mag.UpstreamBlobInsertManifest(ctx, reqGen, blobGen)
	if err != nil {
		t.Fatal(err)
	}
	// Original image should not have Zen3 manipulations
	mdOrig, mdOrigFound, err := mag.Database.ManifestData_GetPulled_ByRepoTag(ctx, reqGen.Repo, reqGen.Tag)
	if err != nil {
		t.Fatal(err)
	}
	if !mdOrigFound {
		t.Fatal("original manifest data not found")
	}
	cfgOrig, cfgOrigFound, err := mag.Config_GetByID(ctx, mdOrig.Config.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !cfgOrigFound {
		t.Fatal("config not found")
	}
	expEnvOrig := []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	assert.Equal(t, expEnvOrig, cfgOrig.Cfg.Config.Env)
	/// is blob present: manifest
	upManiGen := upstream.UpstreamTriple{
		Reg:       reqGen.Host,
		Repo:      reqGen.Repo,
		Tag:       reqGen.Tag,
		Digest:    blobGen.Digest,
		MediaType: blobGen.MediaType,
	}
	_, foundGen, err := mag.Minio.GetManifest(ctx, upManiGen)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, foundGen)
	maniLabelGen := fmt.Sprintf("%s/alpine:3.17.3", regHttpURI)
	maniGen, _, err := mag.Database.ManifestGetByLabel(ctx, maniLabelGen)
	if err != nil {
		t.Logf("maniLabel: %s", maniLabelGen)
		t.Fatal(err)
	}
	assert.Equal(t, "alpine", maniGen.Upstream.Repo)
	// TODO: check if upstream is in DB
	q := fmt.Sprintf(`MATCH (u:Upstream)<-[:UPSTREAM]-(m:Manifest) WHERE id(m)=%d RETURN u.repo AS upRepo, u.tag AS upTag`, maniGen.ID)
	t.Log(q)
	res, found, err := mag.Database.QueryOne(q)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("upstream not found")
	}
	var gotUpRepo, gotUpTag string
	if gotUpRepo, err = res.GetStr("upRepo"); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "alpine", gotUpRepo)
	if gotUpTag, err = res.GetStr("upTag"); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "3.17.3", gotUpTag)
	// TODO: check if manifest is linked to profile
	prfs, err := mag.Database.Profile_FromManifestID(ctx, maniGen.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(prfs) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(prfs))
	}
	prf := prfs[0]
	assert.Equal(t, "generic", prf.Label, prf)
	// TODO: Minio
	/// TODO: is blob present: cfg
	//        _, found, err := mag.Minio.GetConfig(ctx, maniData)
}

func TestUpstreamManifestInsertZen3(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
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
		TeardownTest(bs)
		t.Fatal(err)
	}
	defer TeardownTest(bs)

	// Actual test
	//// ZEN3
	req3 := types.Request{
		Host:    regHttpURI,
		Repo:    "alpine",
		Tag:     "3.17.3",
		Profile: "cpu#zen3",
	}
	blob3 := types.BlobMeta{
		MediaType: MediaTypeManifest,
		Digest:    shab5a,
	}
	err = mag.UpstreamBlobInsertManifest(ctx, req3, blob3)
	if err != nil {
		t.Fatal(err)
	}
	// CHECK: Original manifestData should not have annotations
	q := `MATCH (md:ManifestData)-[]->(a:Annotation) WHERE md.mediaType = "application/vnd.docker.distribution.manifest.v2+json" RETURN count(a) AS annoCount`
	t.Log(q)
	if res, qFound, err := mag.Database.QueryOne(q); err != nil {
		t.Fatal(err)
	} else {
		if !qFound {
			t.Fatal("query not found")
		}
		if annoCount, err := res.GetInt64("annoCount"); err != nil {
			t.Fatal(err)
		} else {
			if annoCount > 1 {
				// FIXME: Where does the annotation come from when running the full test suite?
				t.Fatalf("expected only 1 annotations, got %d", annoCount)
			}
			t.Logf("annoCount: %d", annoCount)
		}
	}
	// Original image should not have Zen3 manipulations
	mdOrig, mdOrigFound, err := mag.Database.ManifestData_GetPulled_ByRepoTag(ctx, req3.Repo, req3.Tag)
	if err != nil {
		t.Fatal(err)
	}
	if !mdOrigFound {
		t.Fatal("original manifest data not found")
	}
	assert.Greater(t, mdOrig.ID, int64(0))
	cfgOrig, cfgOrigFound, err := mag.Config_GetByID(ctx, mdOrig.Config.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !cfgOrigFound {
		t.Fatal("config not found")
	}
	expEnvOrig := []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	assert.Equal(t, expEnvOrig, cfgOrig.Cfg.Config.Env)
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
	maniLabel3 := fmt.Sprintf("%s/alpine:3.17.3", regHttpURI)
	mani3, _, err := mag.Database.ManifestGetByLabel(ctx, maniLabel3)
	if err != nil {
		t.Fatal(err)
	}
	if len(mani3.Manifests) != 3 {
		for _, m := range mani3.Manifests {
			t.Logf("manifest: %s", m.Digest)
		}
		t.Fatal("expected 3 manifests")
	}
	prf, err := mag.getProfile(ctx, br, "cpu#zen3")
	if err != nil {
		t.Fatal(err)
	}
	maniDataID, found, err := mag.Database.ManifestData_Get_ByProfileMediaType(ctx, mani3.ID, prf.ID, "application/vnd.oci.image.manifest.v1+json")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("manifest data not found")
	}
	md3, md3Found, err := mag.Database.ManifestData_Get_ByID(ctx, maniDataID)
	if err != nil {
		t.Fatal(err)
	}
	if !md3Found {
		t.Fatal("manifest data for profile not found")
	}
	assert.Equal(t, "application/vnd.oci.image.manifest.v1+json", md3.MediaType)
	assert.NotEqual(t, 0, md3.Config.ID)

	cfg, cfgFound, err := mag.Config_GetByID(ctx, md3.Config.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !cfgFound {
		t.Fatal("config not found")
	}
	assert.Equal(t, "application/vnd.oci.image.config.v1+json", cfg.MediaType)
	expEnv3 := []string{
		"ORG_QNIB_SYSTEM_OS_ID=alpine", "ORG_QNIB_SYSTEM_OS_VERSION=3.17.3",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"ORG_SUPERCONTAINERS_HARDWARE_CPU_TARGET_MICROARCH=zen3"}
	assert.Equal(t, expEnv3, cfg.Cfg.Config.Env)
	// Check Original
	//// Config again
	mdOrig2, mdOrigFound, err := mag.Database.ManifestData_GetPulled_ByRepoTag(ctx, req3.Repo, req3.Tag)
	if err != nil {
		t.Fatal(err)
	}
	if !mdOrigFound {
		t.Fatal("original manifest data not found")
	}
	assert.Greater(t, mdOrig2.ID, int64(1))

	cfgOrig, cfgOrigFound, err = mag.Config_GetByID(ctx, mdOrig2.Config.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !cfgOrigFound {
		t.Fatal("config not found")
	}
	expEnvOrig = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	assert.Equal(t, expEnvOrig, cfgOrig.Cfg.Config.Env)
	// TODO: check if upstream is in DB
	// TODO: check if upstream is linked to manifest
	// TODO: check if manifest is linked to profile
	// TODO: Minio
	/// TODO: is blob present: cfg
	//        _, found, err := mag.Minio.GetConfig(ctx, maniData)

}

func TestUpstreamBlobInsertManifestList(t *testing.T) {
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
		MediaType: MediaTypeManifestList,
		Digest:    shaff6,
	}
	err = mag.UpstreamBlobInsertIndex(ctx, req, blob)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpstreamInsertGeneric(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
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
		TeardownTest(bs)
		t.Fatal(err)

	}
	defer TeardownTest(bs)
	// Actual test
	req := types.Request{
		Host:    regHttpURI,
		Repo:    "alpine",
		Tag:     "manifest",
		Profile: "cpu#zen3",
	}
	blob := types.BlobMeta{
		MediaType: MediaTypeManifest,
		Digest:    shab5a,
	}
	col, ups, maniData, err := mag.upstreamInsertGeneric(ctx, req, digest.Digest(blob.Digest))
	if err != nil {
		t.Fatal(err)
	}
	e := mag.Database.Rule_EvalAll(ctx)
	if e != nil {
		t.Fatal(e)
	}
	_ = col
	_ = ups
	_ = maniData
}

func TestUpstreamDiscoverUpstream(t *testing.T) {
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
	assert.Equal(t, shab5a, blob.Digest.String())
}

func TestUpstreamDiscoverUpstreamManifestList(t *testing.T) {
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
	assert.Equal(t, shaff6, blob.Digest.String())
	assert.Equal(t, MediaTypeManifestList, blob.MediaType)
}
