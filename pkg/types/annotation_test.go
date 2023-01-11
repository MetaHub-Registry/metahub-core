package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Annotations_CRU(t *testing.T) {
	a := NewAnnotations()
	err := a.AddAnnotation("foo", "bar")
	assert.NoError(t, err)
	err = a.AddAnnotation("foo", "bar")
	assert.NoError(t, err)
	err = a.AddAnnotation("foo", "foo")
	assert.Error(t, err)
	v, err := a.GetAnnotation("foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", v)
	_, err = a.GetAnnotation("foobar")
	assert.Error(t, err)

	a1, err := NewAnnotationsFromMap(map[string]string{"foo": "bar"})
	assert.NoError(t, err)
	k1, err := a1.GetAnnotation("foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", k1)

}
