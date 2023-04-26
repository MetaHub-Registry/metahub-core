package utils

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	mirrorConfiguration = `unqualified-search-registries = ["docker.io"]`
)

func Test_SplitImage(t *testing.T) {
	tests := []struct {
		image string
		err   bool
		reg   string
		repo  string
		ref   string
	}{
		{"", true, "", "", ""},
		{"alpine:3.16", false, "docker.io", "library/alpine", "3.16"},
		{"docker.io/library/alpine:3.16", false, "docker.io", "library/alpine", "3.16"},
		{"quay.io/library/alpine:3.16", false, "quay.io", "library/alpine", "3.16"},
		{"qnib/featuretest:test", false, "docker.io", "qnib/featuretest", "test"},
		{"quay.io/library/alpine@sha256:91591e8f9968721c129f1d2113259a047f6f85af6491758296fda8f21ef81dd3", false, "quay.io", "library/alpine", "sha256:91591e8f9968721c129f1d2113259a047f6f85af6491758296fda8f21ef81dd3"},
	}
	registriesConf, err := os.CreateTemp("", "repo-splitname-v0")
	require.NoError(t, err)
	defer registriesConf.Close()
	defer os.Remove(registriesConf.Name())
	err = os.WriteFile(registriesConf.Name(), []byte(mirrorConfiguration), 0600)
	require.NoError(t, err)
	for _, test := range tests {
		reg, repo, ref, err := SplitImageName(test.image)
		if test.err && err == nil {
			t.Errorf("Expected error for %s", test.image)
		}
		res := []string{}
		if err != nil && !test.err {
			res = append(res, fmt.Sprintf("%s: %s", test.image, err))
		}

		if test.reg != reg {
			res = append(res, fmt.Sprintf("%-50s: Expected '%-20s, got %-20s", test.image, test.reg, reg))
		}
		if test.repo != repo {
			res = append(res, fmt.Sprintf("%-50s: Expected '%-20s, got %-20s", test.image, test.repo, repo))
		}
		if test.ref != ref {
			res = append(res, fmt.Sprintf("%-50s: Expected '%-20s, got %-20s", test.image, test.ref, ref))
		}
		if len(res) > 0 {
			t.Errorf("    \n%s", strings.Join(res, "   \n"))
		}
	}

}

func getMirrorConf(regs []string) []byte {
	return []byte(fmt.Sprintf(`unqualified-search-registries = ["%s"]`, strings.Join(regs, `", "`)))
}
func Test_NormalizeRepoName_v0(t *testing.T) {
	input := "alpine"
	exp := "docker.io/library/alpine:latest"
	got, err := NormalizeRepoName(input)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, exp, got)
	input = "alpine:3.16"
	exp = "docker.io/library/alpine:3.16"
	got, err = NormalizeRepoName(input)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, exp, got)
	input = "-s"
	_, err = NormalizeRepoName(input)
	assert.Error(t, err)
}

func Test_NormalizeRepoName_v1(t *testing.T) {
	input := "alpine"
	exp := "docker.io/library/alpine:latest"
	got, err := NormalizeRepoName(input)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, exp, got)
}
