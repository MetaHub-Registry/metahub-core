package sqlite

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var (
	testFilterGeneric = types.Filter{
		Label:   "generic",
		Feature: "",
	}
)

func Test_Filter_Insert(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	defer TeardownTest(bs)
	err := sq.EmptyData(true)
	assert.NoError(t, err)
	err = sq.Filter.InsertFilter(&testFilterGeneric)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), testCfg.SqlID)
	prf := types.Profile{
		Name: "generic",
	}
	err = sq.Profile.Insert(&prf)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), prf.ID)
	pfp := types.ProfileFilterPlatform{
		Profile: prf,
		Filter:  testFilterGeneric,
	}
	err = sq.Filter.InsertProfileFilter(pfp)
	assert.NoError(t, err)
	q := fmt.Sprintf("SELECT id FROM profile_filter WHERE profile_id=%d AND filter_id = %d", prf.ID, testFilterGeneric.ID)
	rows, err := sq.Query(q)
	assert.NoError(t, err)
	defer rows.Close()
	assert.True(t, rows.Next())
}
