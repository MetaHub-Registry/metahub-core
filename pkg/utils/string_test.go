package utils

import (
	"fmt"
	"strings"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestIncludesString(t *testing.T) {
	assert.True(t, IncludesString("a", []string{"a", "b", "c"}))
	assert.False(t, IncludesString("d", []string{"a", "b", "c"}))
}

func TestFindOverlap(t *testing.T) {
	exp := []string{"a"}
	got := FindOverlap([]string{"a", "d"}, []string{"a", "b", "c"})
	assert.Equal(t, exp, got)
}

func TestFindMissingInSecond(t *testing.T) {
	exp := []string{"a"}
	got := FindMissingInSecond([]string{"a", "b", "c"}, []string{"b", "c"})
	assert.Equal(t, exp, got)
}

func TestStringUnion(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"A", "B", "C"}
	exp := []string{"a", "b", "c", "A", "B", "C"}
	got := StringUnion(a, b)
	assert.Equal(t, exp, got)
}

func TestShortenDigest(t *testing.T) {
	tests := []struct {
		input string
		exp   string
	}{
		{"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", "a3ed95cae"},
		{"latest1234", "latest123"},
		{"latest", "latest"},
	}
	for _, test := range tests {
		got := ShortenDigest(test.input)
		res := []string{}
		if got != test.exp {
			res = append(res, fmt.Sprintf("(input:'%-56s' EXP: '%s' // GOT: '%s' >> ", test.input, test.exp, got))
		}
		if len(res) > 0 {
			t.Error(t, strings.Join(res, "\n"))
		}
	}

}

func TestShortenMediaType(t *testing.T) {
	tests := []struct {
		input string
		exp   string
	}{
		{v1.MediaTypeImageManifest, "oci_image_manifest_v1JSON"},
		{v1.MediaTypeImageLayerGzip, "oci_image_layer_v1_tarGZIP"},
		{v1.MediaTypeImageLayer, "oci_image_layer_v1_tar"},
		{"application/vnd.docker.image.rootfs.diff.tar.gzip", "docker_image_rootfs_diff_tar_gzip"},
		{"application/vnd.docker.image.rootfs.diff.tar", "docker_image_rootfs_diff_tar"},
		{"application/vnd.oci.image.manifest.v1+json", "oci_image_manifest_v1JSON"},
		{"application/vnd.docker.distribution.manifest.v2+json", "docker_distribution_manifest_v2JSON"},
	}
	for _, test := range tests {
		got := ShortenMediaType(test.input)
		res := []string{}
		if got != test.exp {
			res = append(res, fmt.Sprintf("(input:'%-56s' EXP: '%s' // GOT: '%s' >> ", test.input, test.exp, got))
		}
		if len(res) > 0 {
			t.Error(t, strings.Join(res, "\n"))
		}
	}
}

func TestUpLabelKey(t *testing.T) {
	tests := []struct {
		input string
		exp   string
	}{
		{"foo", "FOO"},
		{"foo.bar", "FOO_BAR"},
	}

	for _, test := range tests {
		got := UpLabelKey(test.input)
		res := []string{}
		if got != test.exp {
			res = append(res, fmt.Sprintf("(input:'%-56s' EXP: '%s' // GOT: '%s' >> ", test.input, test.exp, got))
		}
		if len(res) > 0 {
			t.Error(t, strings.Join(res, "\n"))
		}
	}
}

func TestCreateLabelFromTypeAndDigest(t *testing.T) {
	tests := []struct {
		mt  string
		dig string
		exp string
	}{
		{"application/vnd.docker.distribution.manifest.v2+json", "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", "dkr:b5a5b7ce4"},
		{"application/vnd.oci.image.manifest.v1+json", "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", "oci:b5a5b7ce4"},
		{"application/vnd.oci.image.layer.v1.tar+gzip", "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", "ociGz:b5a5b7ce4"},
		{"application/vnd.oci.image.layer.v1.tar+zstd", "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", "ociZstd:b5a5b7ce4"},
		{"application/vnd.docker.image.rootfs.diff.tar", "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", "dkr:b5a5b7ce4"},
		{"application/vnd.docker.image.rootfs.diff.tar.gzip", "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", "dkrGz:b5a5b7ce4"},
	}
	for _, test := range tests {
		got := CreateLabelFromTypeAndDigest(test.mt, test.dig)
		res := []string{}
		if got != test.exp {
			res = append(res, fmt.Sprintf("(input:'%s / %s' EXP: '%s' // GOT: '%s' >> ", test.mt, test.dig, test.exp, got))
		}
		if len(res) > 0 {
			t.Error(t, strings.Join(res, "\n"))
		}
	}
}

func TestGenPasswordHash(t *testing.T) {
	passwd := "test"
	hash := GenPasswordHash(passwd)
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(passwd))
	assert.NoError(t, err)
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte("FAKE"))
	assert.Error(t, err)
}

func TestDecodeEncode(t *testing.T) {
	str := "string"
	en := Base64Encode(str)
	assert.Equal(t, "c3RyaW5n", en)
	de := Base64Decode(en)
	assert.Equal(t, str, de)
}

func TestSortTags(t *testing.T) {
	tags := []string{
		"tagtest0", "TEST0", "test0",
		"TEST1", "test1",
		"TEST2", "test2",
		"TEST3", "test3",
	}
	exp := []string{"tagtest0", "TEST0", "test0", "TEST1", "test1", "TEST2", "test2", "TEST3", "test3"}
	got := SortTags(tags)
	assert.Equal(t, exp, got)
	got2 := SortTags([]string{"generic", "arm64"})
	assert.Equal(t, []string{"arm64", "generic"}, got2)
}
