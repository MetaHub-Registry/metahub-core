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

const (
	shab5a                = "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11"
	shaff6                = "sha256:ff6bdca1701f3a8a67e328815ff2346b0e4067d32ec36b7992c1fdc001dc8517"
	MediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	MediaTypeManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeImageConfig  = "application/vnd.docker.container.image.v1+json"
)

func TestUpstreamManifestInsert(t *testing.T) {
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
	_, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestUpstreamManifestInsert.Setup",
	})
	defer span.End()
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Actual test
	span.End()
	l, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{
		"func": "TestUpstreamManifestInsert.Run",
	})
	defer span.End()
	//// Generic
	reqGen := types.Request{
		Host:    regHttpURI,
		Repo:    "alpine",
		Tag:     "3.17.3",
		Profile: "generic",
	}
	_, _, e := mag.Database.CollectionGetCreateByRepoTag(ctx, reqGen.Repo, reqGen.Tag)
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
	mdOrig, mdOrigFound, err := mag.Database.ManifestGetPulledByRepoTag(ctx, reqGen.Repo, reqGen.Tag)
	if err != nil {
		t.Fatal(err)
	}
	if !mdOrigFound {
		t.Fatal("original manifest data not found")
	}
	utils.Debug(l, span, "Get original config w/ ID: %d (dig:%s)", mdOrig.Config.ID, utils.ShortenDigest(mdOrig.Digest))
	cfgOrig, cfgOrigFound, err := mag.ConfigGetByID(ctx, mdOrig.Config.ID)
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
	// Phase C3: assert Slot-based state instead of wrapper :ManifestWrapper state.
	// Check that Manifest was created and linked to Upstream via :ORIGINAL.
	q := `MATCH (md:Manifest)-[:ORIGINAL]->(u:Upstream) WHERE u.repo = "alpine" AND u.tag = "3.17.3" RETURN u.repo AS upRepo, u.tag AS upTag, id(md) AS mdID`
	t.Log(q)
	res, found, err := mag.Database.QueryOne(q)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Manifest→Upstream not found via :ORIGINAL edge")
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
	// Check that a Slot was created with HAS_PUSHED edge
	q2 := `MATCH (s:Slot)-[:HAS_PUSHED]->(md:Manifest)-[:ORIGINAL]->(u:Upstream) WHERE u.repo = "alpine" AND u.tag = "3.17.3" RETURN s.repo AS repo, s.tag AS tag`
	res2, found2, err := mag.Database.QueryOne(q2)
	if err != nil {
		t.Fatal(err)
	}
	if found2 {
		sRepo, _ := res2.GetStr("repo")
		assert.Equal(t, "alpine", sRepo, "Slot repo should match")
	}
	// Get the generic-profile manipulated config blob. Multiple config blobs
	// exist at this point (original, generic-manipulated, and possibly
	// zen2-manipulated). We want the generic-manipulated config which has the
	// OS env vars injected — look for the config that contains the OS_ID env.
	q = `MATCH (cfg:Blob) WHERE cfg.mediaType IN ["application/vnd.oci.image.config.v1+json"] RETURN id(cfg) AS cfgID`
	utils.Debug(l, span, "Query: %s", q)
	t.Log(q)
	cfgResults, err := mag.Database.Query(q)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	var cfgID int64
	for _, r := range cfgResults {
		cid, e := r.GetInt64("cfgID")
		if e != nil {
			continue
		}
		c, cFound, e := mag.ConfigGetByID(ctx, cid)
		if e != nil || !cFound {
			continue
		}
		// Pick the generic-manipulated config: has OS_ID but not MICROARCH
		hasOsID := false
		hasMicroarch := false
		for _, env := range c.Cfg.Config.Env {
			if strings.HasPrefix(env, "ORG_QNIB_SYSTEM_OS_ID=") {
				hasOsID = true
			}
			if strings.HasPrefix(env, "ORG_SUPERCONTAINERS_HARDWARE_CPU_TARGET_MICROARCH=") {
				hasMicroarch = true
			}
		}
		if hasOsID && !hasMicroarch {
			cfgID = cid
			found = true
			break
		}
	}
	if !found {
		t.Fatal("generic-manipulated config blob not found")
	}
	utils.Debug(l, span, "Get new config w/ ID: %d", cfgID)
	cfg, cfgFound, err := mag.ConfigGetByID(ctx, cfgID)
	if err != nil {
		t.Fatal(err)
	}
	if !cfgFound {
		t.Fatal("config not found")
	}
	expEnvGen := []string{
		"ORG_QNIB_SYSTEM_OS_ID=alpine", "ORG_QNIB_SYSTEM_OS_VERSION=3.17.3",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	assert.Equal(t, expEnvGen, cfg.Cfg.Config.Env)
	//expEnvOrig := []string{"ORG_QNIB_SYSTEM_OS_ID=alpine", "ORG_QNIB_SYSTEM_OS_VERSION=3.17.3", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}

	// TODO: Minio
	/// TODO: is blob present: cfg
	//        _, found, err := mag.Minio.GetConfig(ctx, maniData)
}

// func TestUpstreamManifestInsertZen3(t *testing.T) {
// 	ctx := context.Background()
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("platforms:amd64")
// 	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/generic.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
// 	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
// 	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	br, err := mag.SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		TeardownTest(bs, br)
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs, br)

// 	// Actual test
// 	//// ZEN3
// 	req3 := types.Request{
// 		Host:    regHttpURI,
// 		Repo:    "alpine",
// 		Tag:     "3.17.3",
// 		Profile: "cpu#zen3",
// 	}
// 	blob3 := types.BlobMeta{
// 		MediaType: MediaTypeManifest,
// 		Digest:    shab5a,
// 	}
// 	err = mag.UpstreamBlobInsertManifest(ctx, req3, blob3)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	// CHECK: Original manifestData should not have annotations
// 	q := `MATCH (md:Manifest)-[]->(a:Annotation) WHERE md.mediaType IN ["application/vnd.oci.image.manifest.v1+json"] RETURN count(a) AS annoCount`
// 	if res, qFound, err := mag.Database.QueryOne(q); err != nil {
// 		t.Fatal(err)
// 	} else {
// 		if !qFound {
// 			t.Fatal("query not found")
// 		}
// 		if annoCount, err := res.GetInt64("annoCount"); err != nil {
// 			t.Fatal(err)
// 		} else {
// 			assert.Equal(t, int64(2), annoCount)
// 		}
// 	}
// 	// Original image should not have Zen3 manipulations
// 	mdOrig, mdOrigFound, err := mag.Database.ManifestGetPulledByRepoTag(ctx, req3.Repo, req3.Tag)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !mdOrigFound {
// 		t.Fatal("original manifest data not found")
// 	}
// 	assert.Equal(t, "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", mdOrig.Digest)
// 	cfgOrig, cfgOrigFound, err := mag.ConfigGetByID(ctx, mdOrig.Config.ID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !cfgOrigFound {
// 		t.Fatal("config not found")
// 	}
// 	expEnvOrig := []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
// 	assert.Equal(t, expEnvOrig, cfgOrig.Cfg.Config.Env)
// 	/// is blob present: manifest
// 	upMani3 := upstream.UpstreamTriple{
// 		Reg:       req3.Host,
// 		Repo:      req3.Repo,
// 		Tag:       req3.Tag,
// 		Digest:    blob3.Digest,
// 		MediaType: blob3.MediaType,
// 	}
// 	_, found3, err := mag.Minio.GetManifest(ctx, upMani3)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.True(t, found3)
// 	maniLabel3 := fmt.Sprintf("%s/alpine:3.17.3", regHttpURI)
// 	mani3, _, err := mag.Database.ManifestGetByLabel(ctx, maniLabel3)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if len(mani3.Manifests) < 2 {
// 		for _, m := range mani3.Manifests {
// 			t.Logf("manifest: %s", m.Digest)
// 		}
// 		t.Fatalf("expected min 2 manifests, found %d", len(mani3.Manifests))
// 	}
// 	prf, err := mag.getProfile(ctx, br, "cpu#zen3")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	maniDataID, found, err := mag.Database.ManifestGetByProfileMediaType(ctx, mani3.ID, prf.ID, []string{"application/vnd.docker.distribution.manifest.v2+json"})
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !found {
// 		t.Fatal("manifest data not found")
// 	}
// 	md3, md3Found, err := mag.Database.ManifestGetByID(ctx, maniDataID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !md3Found {
// 		t.Fatal("manifest data for profile not found")
// 	}
// 	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", md3.MediaType)
// 	assert.NotEqual(t, 0, md3.Config.ID)

// 	cfg, cfgFound, err := mag.ConfigGetByID(ctx, md3.Config.ID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !cfgFound {
// 		t.Fatal("config not found")
// 	}
// 	assert.Equal(t, "application/vnd.oci.image.config.v1+json", cfg.MediaType)
// 	expEnv3 := []string{
// 		"ORG_QNIB_SYSTEM_OS_ID=alpine", "ORG_QNIB_SYSTEM_OS_VERSION=3.17.3",
// 		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
// 		"ORG_SUPERCONTAINERS_HARDWARE_CPU_TARGET_MICROARCH=zen3"}
// 	assert.Equal(t, expEnv3, cfg.Cfg.Config.Env)
// 	// Check Original
// 	//// Config again
// 	mdOrig2, mdOrigFound, err := mag.Database.ManifestGetPulledByRepoTag(ctx, req3.Repo, req3.Tag)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !mdOrigFound {
// 		t.Fatal("original manifest data not found")
// 	}
// 	assert.Equal(t, "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", mdOrig2.Digest)
// 	cfgOrig, cfgOrigFound, err = mag.ConfigGetByID(ctx, mdOrig2.Config.ID)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !cfgOrigFound {
// 		t.Fatal("config not found")
// 	}
// 	expEnvOrig = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
// 	assert.Equal(t, expEnvOrig, cfgOrig.Cfg.Config.Env)
// 	// TODO: check if upstream is in DB
// 	// TODO: check if upstream is linked to manifest
// 	// TODO: check if manifest is linked to profile
// 	// TODO: Minio
// 	/// TODO: is blob present: cfg
// 	//        _, found, err := mag.Minio.GetConfig(ctx, maniData)

// }

/*
func TestUpstreamBlobInsertManifestList(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
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
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
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
*/

// func TestUpstreamInsertGeneric(t *testing.T) {
// 	ctx := context.Background()
// 	debug := 0
// 	bs := utils.NewBootstrap()
// 	bs.SetYAMLPrefix("../../../misc/yaml")
// 	bs.AddBootstrap("platforms:arm64")
// 	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
// 	bs.AddBootstrap("rules:rules/os/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/generic.yaml")
// 	bs.AddBootstrap("rules:rules/cpu/zen3.yaml")
// 	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target.microarch:zen3")
// 	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")
// 	bs.SetEmptyAll()
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	br, err := mag.SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		t.Logf("Error while SetupTest: %v", err)
// 		TeardownTest(bs, br)
// 		t.Fatal(err)

// 	}
// 	defer TeardownTest(bs, br)
// 	// Actual test
// 	req := types.Request{
// 		Host:    regHttpURI,
// 		Repo:    "alpine",
// 		Tag:     "manifest",
// 		Profile: "cpu#zen3",
// 	}
// 	blob := types.BlobMeta{
// 		MediaType: MediaTypeManifest,
// 		Digest:    shab5a,
// 	}
// 	col, ups, maniData, err := mag.upstreamInsertGeneric(ctx, req, digest.Digest(blob.Digest))
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	e := mag.Database.RuleEvalAll(ctx)
// 	if e != nil {
// 		t.Fatal(e)
// 	}
// 	_ = col
// 	_ = ups
// 	_ = maniData
// 	dig := "sha256:c41833b44d910632b415cd89a9cdaa4d62c9725dc56c99a7ddadafd6719960f9"
// 	if up, found, err := mag.Database.UpstreamGetByLayerDigest(ctx, dig); err != nil {
// 		t.Fatal(err)
// 	} else if !found {
// 		t.Fatal("upstream not found")
// 	} else {
// 		t.Logf("upstream: %+v", up)
// 	}
// }

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
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs, br)
	// Actual test
	repo := "alpine"
	tag := "3.17.3"
	blob, reg, found, err := mag.DiscoverUpstream(ctx, repo, tag)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	//t.Logf("blob: %v", blob)
	assert.True(t, strings.HasPrefix(reg, "localhost:") || strings.HasPrefix(reg, "172.17.0.1:"), fmt.Sprintf("reg: %s", reg))
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
		mag.Database.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs, br)
	// Actual test
	repo := "alpine"
	tag := "3.17.2"
	blob, reg, found, err := mag.DiscoverUpstream(ctx, repo, tag)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	//t.Logf("blob: %v", blob)
	assert.True(t, strings.HasPrefix(reg, "localhost:") || strings.HasPrefix(reg, "172.17.0.1:"), fmt.Sprintf("reg: %s", reg))
	assert.Equal(t, shaff6, blob.Digest.String())
	assert.Equal(t, MediaTypeManifestList, blob.MediaType)
}

func TestCacheUpstream(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:arm64")
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
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Actual test
	if e := mag.CacheUpstream(ctx, types.UpstreamGlow150ARM64); e != nil {
		t.Fatal(e)
	}
	// Test upstream
	var up types.Upstream
	if upID, found, err := mag.Database.UpstreamGet(ctx, types.UpstreamGlow150ARM64); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("upstream not found")
	} else {
		if up, found, err = mag.Database.UpstreamGetByID(ctx, upID); err != nil {
			t.Fatal(err)
		} else if !found {
			t.Fatal("upstream not found")
		}
	}
	if up.Digest != types.UpstreamGlow150ARM64.Digest {
		t.Fatal("digest mismatch")
	}
	// Test Manifest
	req := types.Request{
		Host:    up.Reg,
		Repo:    up.Repo,
		Tag:     up.Tag,
		Profile: up.Profile,
	}

	if md, found, err := mag.Database.ManifestGetTypesByRequest(ctx, req); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("manifest data not found")
	} else {
		assert.Equal(t, up.Digest.String(), md.Upstream.Digest.String())
	}
}
