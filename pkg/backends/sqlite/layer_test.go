package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var (
	testLayer = types.Layer{
		Digest:    "sha256:1234567890123",
		MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
		Size:      123456,
	}
)

func Test_Layer_InsertDescriptor(t *testing.T) {
	bs := utils.NewBootstrap()
	defer TeardownTest(bs)
	l0 := types.FT_arm_Layer0Reference()
	lID, err := sq.Layer.InsertDescriptor(l0)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), lID)

}
func Test_Layer_Insert(t *testing.T) {
	bs := utils.NewBootstrap()
	defer TeardownTest(bs)
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
	l, found, err := sq.Layer.LayerGetByDigest(l1.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, l1.Digest, l.Digest)
}
