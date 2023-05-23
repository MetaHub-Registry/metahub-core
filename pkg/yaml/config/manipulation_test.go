package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	manipulationDir = "../../../misc/yaml/manipulations"
)

var (
	failYaml  = []byte(`foobar`)
	yamlCaddy = []byte(`label: caddy#admin
layers:
  - image: quay.io/cqnib/metahub-layers/caddy/admin:2023-05-08`)
	yamlGlow = []byte(`label: glow#1.5.0
layers:
  - image: registry:5050/cqnib/metahub-layers/hpc3/glow/1.5.0-arm64
    rule: arch:arm64=true`)
	yamlGlow151 = []byte(`label: glow#1.5.1
layers:
  - image: registry:5050/cqnib/metahub-layers/hpc3/glow/1.5.1-arm64
    rule: arch:arm64=true
    cmd: 
      - rule: prepend
        value: hpc3-help
    environment:
     - key: HPC3_HELP
       rule: ifNULL`)
)

func TestManipulationCaddy(t *testing.T) {
	m0, err := readManipulationConfig(yamlCaddy)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "caddy#admin", m0.Label)
	assert.Equal(t, 1, len(m0.ManipuLayers))
	_, err = readManifestConfig(failYaml)
	assert.Error(t, err)
}

func TestManipulationGlow(t *testing.T) {
	m0, err := readManipulationConfig(yamlGlow)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "glow#1.5.0", m0.Label)
	if len(m0.ManipuLayers) != 1 {
		t.Fatalf("Expected 1 layer, got %d", len(m0.ManipuLayers))
	}
	lyr := m0.ManipuLayers[0]
	assert.Equal(t, `arch:arm64=true`, lyr.Rule)
	//tm := m0.ToTypesManipulation()
	//assert.Equal(t, "glow#1.5.0", tm.Label)
}
func TestManipulationGlow151(t *testing.T) {
	m0, err := readManipulationConfig(yamlGlow151)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "glow#1.5.1", m0.Label)
	if len(m0.ManipuLayers) != 1 {
		t.Fatalf("Expected 1 layer, got %d", len(m0.ManipuLayers))
	}
	lyr := m0.ManipuLayers[0]
	assert.Equal(t, `arch:arm64=true`, lyr.Rule)
	assert.Equal(t, 1, len(lyr.Cmd))
	assert.Equal(t, 1, len(lyr.Environment))
	//tm := m0.ToTypesManipulation()
	//assert.Equal(t, "glow#1.5.0", tm.Label)
}

func TestManipulationLoadFiles(t *testing.T) {
	m0, err := LoadManipulation(ctx, filepath.Join(manipulationDir, "/layer/caddy/admin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "layer/caddy#admin", m0.Label)
	// wrong yaml
	_, err = LoadManipulation(ctx, filepath.Join(manipulationDir, "/fail/foobar.yaml"))
	assert.Error(t, err)
	// Mismatch
	_, err = LoadManipulation(ctx, filepath.Join(manipulationDir, "/fail/tag_mismatch.yaml"))
	assert.Error(t, err)
	m1, err := LoadManipulation(ctx, filepath.Join(manipulationDir, "/layer/glow/1.5.0.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "layer/glow#1.5.0", m1.Label)
	assert.Equal(t, 2, len(m1.ManipuLayers))
}

func TestFindManipulations(t *testing.T) {
	mps, err := FindManipulations(ctx, filepath.Join(manipulationDir, "/layer/caddy/admin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(mps))
	mps, err = FindManipulations(ctx, filepath.Join(manipulationDir, "/layer/glow/1.5.1-arm64.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(mps))
}
