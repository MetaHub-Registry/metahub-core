package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Filter_CRD(t *testing.T) {
	f0, err := NewFilter("foo_filter", "anno", "eq", "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "foo_filter", f0.Name)
	as0 := NewAnnotations()
	a0 := NewAnnotation("foo", "bar")
	as0.AddAnnotation(a0)
	err = f0.EvalRules(as0)
	assert.NoError(t, err)
	/// False
	a1 := NewAnnotation("foo", "foo")
	as1 := NewAnnotations()
	as1.AddAnnotation(a1)
	err = f0.EvalRules(as1)
	assert.Error(t, err)
}
