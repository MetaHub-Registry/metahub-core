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
	if err != nil {
		t.Fatal(err)
	}
	err = a.AddAnnotation(fb)
	if err != nil {
		t.Fatal(err)
	}
	err = a.AddAnnotation(ff)
	assert.Error(t, err)
	v, err := a.GetAnnotationValue("foo")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "bar", v)
	_, err = a.GetAnnotation("foobar")
	assert.Error(t, err)
	a1, err := NewAnnotationsFromMap(map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatal(err)
	}
	got1, err := a1.GetAnnotation("foo")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "bar", got1.Value)

}
