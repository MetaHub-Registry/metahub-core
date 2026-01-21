package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestUpstreamCreate(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	up := types.Upstream{
		Reg:    "foo.bar",
		Repo:   "qnib/featuretest",
		Tag:    "2022.12.03",
		Digest: "sha256:1234567890123456789012345678901234567890123456789012345678901234",
	}

	upID, err := db.CreateUpstream(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, upID)
	r, err := db.Query("MATCH (u:Upstream) RETURN " + UpstreamReturn)
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, r, 1)
	r0 := r[0]
	reg, err := r0.GetStr("upstreamReg")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "foo.bar", reg)
	repo, err := r0.GetStr("upstreamRepo")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "qnib/featuretest", repo)
	tag, err := r0.GetStr("upstreamTag")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "2022.12.03", tag)
	gotUp, err := UpstreamFromResult(r0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, up.Reg, gotUp.Reg)
}

func TestUpstreamCRUD(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.PostTestSleep = 0
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	up := types.Upstream{
		Reg:    "foo.bar",
		Repo:   "qnib/featuretest",
		Tag:    "2022.12.03",
		Digest: "sha256:1234567890123456789012345678901234567890123456789012345678901234",
	}
	_, found, err := db.UpstreamGet(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	upID, err := db.CreateUpstream(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, upID)
	gotUpID, found, err := db.UpstreamGet(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, upID, gotUpID)
	gotUp, _, err := db.UpstreamGetByID(ctx, upID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, up.Reg, gotUp.Reg)
	// Insert the same
	reqFooBar := types.Request{
		Host:   "foo.bar",
		Repo:   "qnib/featuretest",
		Tag:    "2022.12.03",
		Digest: "sha256:1234567890123456789012345678901234567890123456789012345678901234",
	}
	gotUpFooBar, found, err := db.Upstream_GetInsert_ByRequest(ctx, reqFooBar)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, gotUp.ID, gotUpFooBar.ID)
	// Insert BarFoo
	req := types.Request{
		Host:   "bar.foo",
		Repo:   "qnib/test",
		Tag:    "2022.12.03",
		Digest: "sha256:5434567890123456789012345678901234567890123456789012345678901234",
	}
	gotUp2, found, err := db.Upstream_GetInsert_ByRequest(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	assert.Equal(t, "bar.foo", gotUp2.Reg)
	assert.Equal(t, req.Repo, gotUp2.Repo)
	assert.Equal(t, req.Tag, gotUp2.Tag)
	assert.NotEqual(t, 0, gotUp2.ID)
	assert.NotEqual(t, gotUp.ID, gotUp2.ID)
}

func TestUpstreamGetByLayerDigest(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	dig, err := br.Get("layer:0:ft_amd64#digest")
	if err != nil {
		t.Fatal(err)
	}
	expMT, err := br.Get("layer:0:ft_amd64#mediaType")
	if err != nil {
		t.Fatal(err)
	}
	if up, found, err := db.UpstreamGetByLayerDigest(ctx, dig); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Did not find upstream")
	} else {
		assert.Equal(t, expMT, up.MediaType)
		assert.Equal(t, "registry:5050", up.Reg)
	}
	md, err := db.getManiData(ctx, br, "manifestdata:ft_amd64")
	if err != nil {
		t.Fatal(err)
	}
	l0, _, err := db.LayerGetByDigest(ctx, dig)
	if err != nil {
		t.Fatal(err)
	}
	// Add more layers to trigger multiple results
	q := `MATCH (l:Blob), (md:ManifestData)`
	q += fmt.Sprintf(` WHERE id(l) = %d AND id(md) = %d AND l.type = "layer"`, l0.ID, md.ID)
	q += ` MERGE (l)<-[:LayerLink]-(md)`
	if _, e := db.Execute(q); e != nil {
		t.Fatal(e)
	}
	if up, found, err := db.UpstreamGetByLayerDigest(ctx, dig); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Did not find upstream")
	} else {
		assert.Equal(t, expMT, up.MediaType)
		assert.Equal(t, "registry:5050", up.Reg)
	}
	lrys, err := db.GetLayersForManifest(ctx, md)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(lrys))
	q = `MATCH (l:Blob), (md:ManifestData)`
	q += fmt.Sprintf(` WHERE id(l) = %d AND id(md) = %d`, l0.ID, md.ID)
	q += ` MERGE (l)<-[:LayerLink]-(md)`
	if _, e := db.Execute(q); e != nil {
		t.Fatal(e)
	}
	if up, found, err := db.UpstreamGetByLayerDigest(ctx, l0.Digest); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Did not find upstream")
	} else {
		assert.Equal(t, "application/vnd.docker.image.rootfs.diff.tar.gzip", up.MediaType)
		assert.Equal(t, "registry:5050", up.Reg)
	}
	up := upstream.UpstreamTriple{
		Reg:    "registry:5050",
		Repo:   "qnib/featuretest",
		Tag:    "amd64",
		Digest: digest.Digest(l0.Digest),
	}
	if found, e := db.LayerExists(ctx, up); e != nil {
		t.Fatal(e)
	} else {
		assert.True(t, found)
	}
	if up, found, err := db.UpstreamGetByLayerDigest(ctx, l0.Digest); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Should have found upstream")
	} else {
		assert.Equal(t, "registry:5050", up.Reg)
		assert.Equal(t, "qnib-pub-containers/qnib/plain-featuretest", up.Repo)
		assert.Equal(t, "20220715-amd64-generic", up.Tag)
		assert.Equal(t, "sha256:5bf8eee20fde14eb6e3e924c18fafa4d816e131f315810875136f0438ff7beed", up.Digest.String())
	}
}
