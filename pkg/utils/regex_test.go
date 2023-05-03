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
	got := ReSubMatchMap(r, str)["repo"]
	assert.Equal(t, exp, got)
	exp = "3.16"
	got = ReSubMatchMap(r, str)["tag"]
	assert.Equal(t, exp, got)

}
