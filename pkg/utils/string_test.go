package utils

import (
	"fmt"
	"strings"
	"testing"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
)

func Test_IncludesString(t *testing.T) {
	assert.True(t, IncludesString("a", []string{"a", "b", "c"}))
	assert.False(t, IncludesString("d", []string{"a", "b", "c"}))
}

func Test_FindOverlap(t *testing.T) {
	exp := []string{"a"}
	got := FindOverlap([]string{"a", "d"}, []string{"a", "b", "c"})
	assert.Equal(t, exp, got)
}

func Test_FindMissingInSecond(t *testing.T) {
	exp := []string{"a"}
	got := FindMissingInSecond([]string{"a", "b", "c"}, []string{"b", "c"})
	assert.Equal(t, exp, got)
}

func Test_StringUnion(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"A", "B", "C"}
	exp := []string{"a", "b", "c", "A", "B", "C"}
	got := StringUnion(a, b)
	assert.Equal(t, exp, got)
}

func Test_shortenDigest(t *testing.T) {
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

func Test_shortenMediaType(t *testing.T) {
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

func Test_UpLabelKey(t *testing.T) {
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
