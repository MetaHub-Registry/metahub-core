package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Rule_New(t *testing.T) {
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

func Test_Rule_ToString(t *testing.T) {
	r0, err := NewRule("anno", "eq", "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	exp := fmt.Sprintf("%6s > %-45s | %-3s | %s", "anno", "foo", "eq", "bar")
	assert.Equal(t, exp, r0.ToString())

}
