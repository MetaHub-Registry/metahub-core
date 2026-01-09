package neo

import (
	"context"
	"fmt"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestLayerCRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	arm_l0 := types.FT_arm_Layer0Reference()
	_, found, err := db.LayerGetByDigest(ctx, arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	arm_l0_id, err := db.LayerCreateByDescriptor(arm_l0)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, arm_l0_id)
	arm_l0_got, found, err := db.LayerGetByDigest(ctx, arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, arm_l0_id, arm_l0_got.ID)
	arm_l0_got_id, found, err := db.GetLayer(arm_l0_got)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, arm_l0_id, arm_l0_got_id)
	// Delete
	err = db.LayerDeleteByDigest(ctx, arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	_, found, err = db.LayerGetByDigest(ctx, arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	// Create Layer
	arm_l0_id, err = db.CreateLayer(arm_l0_got)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, arm_l0_id)
	l0_id_got, err := db.Layer_PlainInsert(ctx, arm_l0_got)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, arm_l0_id, l0_id_got)
	err = db.LayerDeleteByDigest(ctx, arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	l0_id_got, err = db.Layer_PlainInsert(ctx, arm_l0_got)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, arm_l0_id, l0_id_got)
}

func TestLayerGetUpstream(t *testing.T) {
	ctx := context.Background()
	_ = ctx
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
	lId, e := br.GetInt64("layer:0:ft_amd64#id")
	if e != nil {
		t.Fatal(e)
	}
	up, upFound, e := db.LayerGetUpstream(ctx, lId)
	if e != nil {
		t.Fatal(e)
	}
	if !upFound {
		t.Fatal("Did not find upstream")
	}
	assert.Equal(t, "registry:5050", up.Reg)

	/// Layer_GetMediaType_ByID
	if mt, err := db.Layer_GetMediaType_ByID(ctx, lId); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, "application/vnd.docker.image.rootfs.diff.tar.gzip", mt)
	}
	if mt, err := db.Layer_GetMediaType_ByID(ctx, -1); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, "application/vnd.unknownLayer", mt)
	}
	md, err := db.getManiData(ctx, br, "manifestdata:ft_amd64")
	if err != nil {
		t.Fatal(err)
	}
	// Add more layers to trigger multiple results
	q := `MATCH (l:Blob), (md:ManifestData)`
	q += fmt.Sprintf(` WHERE id(l) = %d AND id(md) = %d`, lId, md.ID)
	q += fmt.Sprintf(` MERGE (l)<-[:%s]-(md)`, utils.ShortenMediaType(v1.MediaTypeImageLayerGzip))
	if _, e := db.Execute(q); e != nil {
		t.Fatal(e)
	}
	if mt, err := db.Layer_GetMediaType_ByID(ctx, lId); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, v1.MediaTypeImageLayerGzip, mt)
	}
	//TODO: Create second relationship to trigger multiple results
	//q := `MATCH (l:Blob), (md:ManifestData) `

}

func TestLayerDescriptorGetInsert(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
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
	desc := v1.Descriptor{
		MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
		Digest:    "sha256:1234567890123456789012345678901234567890123456789012345678901234",
		Size:      1234567890,
	}
	lyrID0, err := db.LayerDescriptorGetInsert(ctx, desc)
	if err != nil {
		t.Fatal(err)
	}
	assert.Greater(t, lyrID0, int64(0))

	if lyrID, err := db.LayerDescriptorGetInsert(ctx, desc); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, lyrID0, lyrID)
	}
}
