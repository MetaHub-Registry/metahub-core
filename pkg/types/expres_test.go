package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	md1 = Manifest{ID: 1}
	md2 = Manifest{ID: 2}
)

func TestExpResSimple(t *testing.T) {
	er := NewExprRes()
	assert.True(t, er.IsEmpty())
	er.Add("foo", md1)
	mds := er.GetManifestdataForRules("foo")
	assert.Equal(t, 1, len(mds))
	er.Add("bar", md1)
	mds = er.GetManifestdataForRules("foo")
	assert.Equal(t, 1, len(mds))
	mds = er.GetManifestdataForRules("foo", "bar")
	assert.Equal(t, 1, len(mds))
	er.Add("foo", md2)
	ids := er.GetTrueIDs()
	assert.Equal(t, 1, len(ids))
	er.Add("bar", md2)
	ids = er.GetTrueIDs()
	assert.Equal(t, 2, len(ids))
	mds = er.GetTrueManifest()
	assert.Equal(t, 2, len(mds))
	mds1 := er.GetManifestdataForRules("foo")
	assert.Equal(t, 2, len(mds1))
	assert.False(t, er.IsEmpty())
}

func TestExpResMulti(t *testing.T) {
	er := NewExprRes()
	er.Add("foo", md1)
	mds := er.GetManifestdataForRules("foo")
	assert.Equal(t, 1, len(mds))
	er.Add("bar", md1)
	mds = er.GetManifestdataForRules("foo", "bar")
	assert.Equal(t, 1, len(mds))
}
