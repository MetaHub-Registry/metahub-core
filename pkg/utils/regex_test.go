package utils

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUtilsRegex(t *testing.T) {
	r := regexp.MustCompile(`^(?P<repo>[^:]+):(?P<tag>.*)$`)
	str := "alpine:3.16"
	exp := "alpine"
	got, err := ReSubMatchMap(r, str)
	assert.NoError(t, err)
	assert.Equal(t, exp, got["repo"])
	exp = "3.16"
	got, err = ReSubMatchMap(r, str)
	assert.NoError(t, err)
	assert.Equal(t, exp, got["tag"])

}
