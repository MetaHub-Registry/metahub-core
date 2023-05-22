package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleNew(t *testing.T) {
	r0, err := NewRule("anno", "eq", "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "anno", r0.Type)
	assert.Equal(t, "eq", r0.Op)
	assert.Equal(t, "foo", r0.Key)
	assert.Equal(t, "bar", r0.Value)
	_, err = NewRule("fake", "eq", "foo", "bar")
	assert.Error(t, err, `type "fake" is not supported (anno)`)
}

func TestRuleToString(t *testing.T) {
	r0, err := NewRule("anno", "eq", "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	exp := fmt.Sprintf("%6s > %-45s | %-3s | %s", "anno", "foo", "eq", "bar")
	assert.Equal(t, exp, r0.ToString())
	r1, err := NewRule("plat", "eq", "arch", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	exp1 := fmt.Sprintf("%6s > %-45s | %-3s | %s", "plat", "arch", "eq", "amd64")
	assert.Equal(t, exp1, r1.ToString())

}

func TestRuleEvalRule(t *testing.T) {
	r0, err := NewRule("anno", "eq", "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, r0.evalRule("eq", "bar"))
	assert.False(t, r0.evalRule("eq", "baz"))
	assert.False(t, r0.evalRule("ne", "bar"))
	assert.True(t, r0.evalRule("ne", "baz"))
}
func TestRuleEvalRuleNull(t *testing.T) {
	r0, err := NewRule("anno", "null", "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, r0.evalRule("null", "bar"))
}
