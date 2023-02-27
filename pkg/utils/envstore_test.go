package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_EnvStore(t *testing.T) {
	es := NewEnvStore()
	err := es.Add("FOO", "bar")
	assert.NoError(t, err)
	err = es.Add("FOO", "bar")
	assert.NoError(t, err)
	err = es.Add("FOO", "foo")
	assert.Error(t, err)
}
