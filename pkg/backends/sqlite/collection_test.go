package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func Test_Collection_Insert(t *testing.T) {
	err := sq.Collection.CreateViews()
	assert.NoError(t, err)
	c := types.Collection{
		Repo: "qnib/test",
		Tag:  "1.0",
	}
	err = sq.Collection.Insert(&c)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), c.SqlID)

}
