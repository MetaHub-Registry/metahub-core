package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnotationsCRU(t *testing.T) {
	a := NewAnnotations()
	assert.Equal(t, 0, a.Len())
	fb := Annotation{
		Key:   "foo",
		Value: "bar",
	}
	err := a.AddAnnotation(fb)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, a.Len())
	err = a.AddAnnotation(fb)
	if err != nil {
		t.Fatal(err)
	}
	ff := Annotation{
		Key:   "foo",
		Value: "foo",
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
	as := a.ToSlice()
	assert.Equal(t, 1, len(as))
	am := a.ToMap()
	assert.Equal(t, 1, len(am))
}

func TestAddNewAnnotation(t *testing.T) {
	a := NewAnnotations()
	err := a.AddNewAnnotation("org.test.key", "testvalue")
	assert.NoError(t, err)
	v, err := a.GetAnnotationValue("org.test.key")
	assert.NoError(t, err)
	assert.Equal(t, "testvalue", v)
}

func TestMergeAnnotations(t *testing.T) {
	a := NewAnnotations()
	_ = a.AddNewAnnotation("key1", "val1")
	b := NewAnnotations()
	_ = b.AddNewAnnotation("key2", "val2")
	err := a.MergeAnnotations(b)
	assert.NoError(t, err)
	assert.Equal(t, 2, a.Len())
	v1, _ := a.GetAnnotationValue("key1")
	assert.Equal(t, "val1", v1)
	v2, _ := a.GetAnnotationValue("key2")
	assert.Equal(t, "val2", v2)
}

func TestMergeAnnotations_Overlap(t *testing.T) {
	a := NewAnnotations()
	_ = a.AddNewAnnotation("key", "first")
	b := NewAnnotations()
	_ = b.AddNewAnnotation("key", "second")
	err := a.MergeAnnotations(b)
	// MergeAnnotations calls AddAnnotation which errors on conflicting values
	assert.Error(t, err, "merging annotations with conflicting values should error")
	// Original value preserved
	v, _ := a.GetAnnotationValue("key")
	assert.Equal(t, "first", v)
}

func TestAnnotationsJSON(t *testing.T) {
	a := NewAnnotations()
	_ = a.AddNewAnnotation("k", "v")
	b, err := a.MarshalJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(b), `"k":"v"`)
}
