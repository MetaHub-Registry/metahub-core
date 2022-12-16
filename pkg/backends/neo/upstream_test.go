package neo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Upstream_Create(t *testing.T) {
	//ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	up := types.Upstream{
		Reg:  "foo.bar",
		Repo: "qnib/featuretest",
		Tag:  "2022.12.03",
	}

	upID, err := db.CreateUpstream(up)
	assert.NoError(t, err)
	assert.NotNil(t, upID)
	r, err := db.Query("MATCH (n:Upstream) RETURN id(n) AS upstreamID, n.reg AS upstreamReg, n.repo AS upstreamRepo, n.tag AS upstreamTag")
	assert.NoError(t, err)
	assert.Len(t, r, 1)
	r0 := r[0]
	reg, err := r0.GetStr("upstreamReg")
	assert.NoError(t, err)
	assert.Equal(t, "foo.bar", reg)
	repo, err := r0.GetStr("upstreamRepo")
	assert.NoError(t, err)
	assert.Equal(t, "qnib/featuretest", repo)
	tag, err := r0.GetStr("upstreamTag")
	assert.NoError(t, err)
	assert.Equal(t, "2022.12.03", tag)
	gotUp, err := UpstreamFromResult(r0)
	assert.NoError(t, err)
	assert.Equal(t, up.Reg, gotUp.Reg)
}

func Test_Upstream_CRUD(t *testing.T) {
	//ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	up := types.Upstream{
		Reg:  "foo.bar",
		Repo: "qnib/featuretest",
		Tag:  "2022.12.03",
	}
	_, found, err := db.GetUpstream(up)
	assert.NoError(t, err)
	assert.False(t, found)
	upID, err := db.CreateUpstream(up)
	assert.NoError(t, err)
	assert.NotNil(t, upID)
	gotUpID, found, err := db.Upstream_Get(up)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, upID, gotUpID)
	gotUp, err := db.Upstream_Get_ByID(upID)
	assert.NoError(t, err)
	assert.Equal(t, up.Reg, gotUp.Reg)
	// Insert the same
	reqFooBar := types.Request{
		Host: "foo.bar",
		Repo: "qnib/featuretest",
		Tag:  "2022.12.03",
	}
	gotUpFooBar, found, err := db.Upstream_GetInsert_ByRequest(reqFooBar)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, gotUp.ID, gotUpFooBar.ID)
	// Insert BarFoo
	req := types.Request{
		Host: "bar.foo",
		Repo: "qnib/test",
		Tag:  "2022.12.03",
	}
	gotUp2, found, err := db.Upstream_GetInsert_ByRequest(req)
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, "bar.foo", gotUp2.Reg)
	assert.Equal(t, req.Repo, gotUp2.Repo)
	assert.Equal(t, req.Tag, gotUp2.Tag)
	assert.NotEqual(t, 0, gotUp2.ID)
	assert.NotEqual(t, gotUp.ID, gotUp2.ID)
}
