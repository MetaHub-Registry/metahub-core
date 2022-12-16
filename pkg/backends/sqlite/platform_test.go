package sqlite

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Platform_Insert(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	defer TeardownTest(bs)
	// Test
	p1 := types.Example_Platform_amd64()
	err := sq.Platform.Insert(&p1)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), p1.ID)
}

func Test_Platform_Upsert(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("filter:generic")
	bs.SetEmptyAll()
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	p1 := types.Example_Platform_amd64()
	err = sq.Platform.Upsert(&p1)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), p1.ID)
	p2 := types.Example_Platform_amd64()
	err = sq.Platform.Upsert(&p2)
	assert.NoError(t, err)
	assert.Equal(t, p1.ID, p2.ID)
	fID, err := br.GetInt64("filter:generic#id")
	assert.NoError(t, err)
	f := types.Filter{
		ID: int64(fID),
	}
	pfp := types.ProfileFilterPlatform{
		Platform: types.Platform{ID: 0},
		Filter:   types.Filter{ID: 0},
	}
	err = sq.Platform.InsertPlatformFilter(pfp)
	assert.Error(t, err)
	pfp.Filter = f
	err = sq.Platform.InsertPlatformFilter(pfp)
	assert.Error(t, err)
	pfp.Filter = types.Filter{ID: 0}
	pfp.Platform = p1
	err = sq.Platform.InsertPlatformFilter(pfp)
	assert.Error(t, err)
	pfp.Filter = f
	err = sq.Platform.InsertPlatformFilter(pfp)
	assert.NoError(t, err)
	q := fmt.Sprintf("SELECT id FROM platform_filter WHERE platform_id=%d AND filter_id = %d", p1.ID, f.ID)
	rows, err := sq.Query(q)
	assert.NoError(t, err)
	defer rows.Close()
	assert.True(t, rows.Next())
}
