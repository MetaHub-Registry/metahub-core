package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Annotation_EvalRule(t *testing.T) {
	a0 := NewAnnotation("foo", "bar")
	r0 := a0.evalRule("eq", "bar")
	assert.True(t, r0)
	r1 := a0.evalRule("ne", "bar")
	assert.False(t, r1)
	r2 := a0.evalRule("eq", "foo")
	assert.False(t, r2)
	r3 := a0.evalRule("re", "^bar$")
	assert.True(t, r3)
	r4 := a0.evalRule("re", "^b$")
	assert.False(t, r4)
}
