package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
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
	defer db.TeardownTest(bs)
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
	//bs.PostTestSleep = 60
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs)
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
