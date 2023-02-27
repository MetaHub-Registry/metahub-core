package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Annotations_CRU(t *testing.T) {
	a := NewAnnotations()
	fb := Annotation{
		Key:   "foo",
		Value: "bar",
	}
	ff := Annotation{
		Key:   "foo",
		Value: "foo",
	}
	err := a.AddAnnotation(fb)
	assert.NoError(t, err)
	err = a.AddAnnotation(fb)
	assert.NoError(t, err)
	err = a.AddAnnotation(ff)
	assert.Error(t, err)
	v, err := a.GetAnnotationValue("foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", v)
	_, err = a.GetAnnotation("foobar")
	assert.Error(t, err)
	a1, err := NewAnnotationsFromMap(map[string]string{"foo": "bar"})
	assert.NoError(t, err)
	got1, err := a1.GetAnnotation("foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", got1.Value)

}
