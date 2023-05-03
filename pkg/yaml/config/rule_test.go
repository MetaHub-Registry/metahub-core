package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	rulesDir = "../../../misc/yaml/rules"
)

var (
	yamlRuleZen3 = []byte(`name: "cpu" 
tag: "zen3"
type: "anno"
op: "eq"
key: "org.supercontainers.hardware.cpu.target.microarch"
value: "zen3"
`)
)

func TestRuleReadRuleConfig(t *testing.T) {
	r0, err := readRuleConfig(yamlRuleZen3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "cpu", r0.Name)
	assert.Equal(t, "zen3", r0.Tag)
	assert.Equal(t, "anno", r0.Type)
	assert.Equal(t, "eq", r0.Op)
	assert.Equal(t, "org.supercontainers.hardware.cpu.target.microarch", r0.Key)
}

func TestRuleLoadRule(t *testing.T) {
	f0, err := LoadRule(ctx, filepath.Join(rulesDir, "/cpu/zen3.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "cpu", f0.Name)
	assert.Equal(t, "zen3", f0.Tag)
}
