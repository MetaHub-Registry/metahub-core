package neo

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_NeoResult(t *testing.T) {
	//ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual Tests
	_, err = db.Query("MATCH (n) RETURN n")
	assert.NoError(t, err)
	//assert.Equal(t, 0, len(r1))
	_, err = db.Execute(fmt.Sprintf("CREATE (n:Test {name: 'test', age: %d})", 42))
	assert.NoError(t, err)
	r, err := db.Query("MATCH (n:Test) RETURN n.name AS name, n.age AS age")
	assert.NoError(t, err)
	assert.Len(t, r, 1)
	r0 := r[0]
	n, err := r0.GetStr("name")
	assert.NoError(t, err)
	assert.Equal(t, "test", n)
	a, err := r0.GetInt64("age")
	assert.NoError(t, err)
	assert.Equal(t, int64(42), a)
}
