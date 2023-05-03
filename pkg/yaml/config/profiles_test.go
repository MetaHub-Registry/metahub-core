package config

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	profileDir = "../../../misc/yaml/profiles"
)

var (
	ctx      = context.Background()
	yamlZen3 = []byte(`name: "zen3"
rule: cpu:zen3=true
`)
	yamlAwscli = []byte(`name: "awscli"
manipulations:
   - "layer/awscli:1.27.3"
`)
)

func TestFilterReadFilterConfig(t *testing.T) {
	f0, err := readProfileConfig(yamlZen3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "zen3", f0.Name)
	assert.Equal(t, "cpu:zen3=true", f0.Rule)
	f1, err := readProfileConfig(yamlAwscli)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "awscli", f1.Name)
	assert.Equal(t, f1.Rule, "")
	assert.Len(t, f1.Manipulations, 1)
}

func TestProfileLoadProfile(t *testing.T) {
	f0, err := LoadProfile(ctx, filepath.Join(profileDir, "/cpu/generic.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "cpu", f0.Name)
	assert.Equal(t, "generic", f0.Tag)
	f1, err := LoadProfile(ctx, filepath.Join(profileDir, "/cpu/skylake_avx512.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "cpu", f1.Name)
	assert.Equal(t, "skylake_avx512", f1.Tag)
	assert.Equal(t, "cpu:skylake_avx512=true", f1.Rule)
}
func TestProfileLoadProfileGeneric(t *testing.T) {
	f2, err := LoadProfile(ctx, filepath.Join(profileDir, "/cpu/generic.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "cpu", f2.Name)
	assert.Equal(t, "generic", f2.Tag)
	assert.Equal(t, "os:linux=true && cpu:noop=true", f2.Rule)

}
