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

func TestSplitImage(t *testing.T) {
	tests := []struct {
		image string
		err   bool
		reg   string
		repo  string
		ref   string
	}{
		{"qnib/featuretest:test", false, "docker.io", "qnib/featuretest", "test"},
		{"registry:5050/qnib/featuretest:test", false, "registry:5050", "qnib/featuretest", "test"},
		{"", true, "", "", ""},
		{"registry.gitlab.com/qnib/test:20220715-arm64-generic", false, "registry.gitlab.com", "qnib/test", "20220715-arm64-generic"},
		{"alpine:3.16", false, "docker.io", "library/alpine", "3.16"}, // we need a FQDN
		{"docker.io/library/alpine:3.16", false, "docker.io", "library/alpine", "3.16"},
		{"quay.io/library/alpine:3.16", false, "quay.io", "library/alpine", "3.16"},
		{"quay.io/library/alpine@sha256:91591e8f9968721c129f1d2113259a047f6f85af6491758296fda8f21ef81dd3", false, "quay.io", "library/alpine", "sha256:91591e8f9968721c129f1d2113259a047f6f85af6491758296fda8f21ef81dd3"},
		{"quay.io/cqnib/gromacs/gcc-7.3.1/2021.5/mpich:x86_64_v4", false, "quay.io", "cqnib/gromacs/gcc-7.3.1/2021.5/mpich", "x86_64_v4"},
		// Registry host with a hyphen must not be mis-split (regression: model-layer
		// push to metahub-dev stored reg="metahub.metahu" repo="b-dev.hvk/...").
		{"metahub.dev.example.com/models/e2e-gguf28-layer:q8-kxl", false, "metahub.dev.example.com", "models/e2e-gguf28-layer", "q8-kxl"},
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


func TestNormalizeRepoNameV0(t *testing.T) {
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
