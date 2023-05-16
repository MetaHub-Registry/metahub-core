package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvStore(t *testing.T) {
	es := NewEnvStore()
	err := es.Add("FOO", "bar")
	if err != nil {
		t.Fatal(err)
	}
	err = es.Add("FOO", "bar")
	if err != nil {
		t.Fatal(err)
	}
	err = es.Add("FOO", "foo")
	assert.Error(t, err)
	err = es.Add("FOO", "bar")
	assert.NoError(t, err)
	err = es.AddEnv("KEY=bar=boo baz=bo")
	assert.NoError(t, err)
	err = es.AddEnv("KEY")
	assert.Error(t, err)

}
