package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

var (
	testLayer = types.Layer{
		Digest:    "sha256:1234567890123",
		MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
		Size:      123456,
	}
)

func Test_Layer_Insert(t *testing.T) {
	l1 := testLayer
	err := sq.Layer.Insert(&l1)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), l1.SqlID)
	l11 := testLayer
	err = sq.Layer.Upsert(&l11)
	assert.NoError(t, err)
	assert.Equal(t, l1.SqlID, l11.SqlID)
	id, err := sq.Layer.GetID(l1.Digest)
	assert.NoError(t, err)
	assert.Equal(t, l1.SqlID, id)
}
