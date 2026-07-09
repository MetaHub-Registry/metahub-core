package config

import "testing"

// TestReadProfileConfigHardware guards the A1 fix: the `hardware:` block must
// survive YAML unmarshal into config.Profile (it was silently dropped before the
// Hardware field existed).
func TestReadProfileConfigHardware(t *testing.T) {
	yamlStr := []byte(`
rule: os:linux=true && gpu:amd=true
hardware:
  gpu:
    vendor: amd
    model: gfx1151
    vram_gb: 124
  system:
    ram_gb: 128
    cpu_cores: 32
`)
	p, err := readProfileConfig(yamlStr)
	if err != nil {
		t.Fatalf("readProfileConfig: %v", err)
	}
	if p.Hardware == nil {
		t.Fatal("hardware block dropped on unmarshal (A1 regression)")
	}
	if p.Hardware.GPU == nil || p.Hardware.GPU.Vendor != "amd" || p.Hardware.GPU.VRAMGB != 124 {
		t.Fatalf("gpu block not parsed: %+v", p.Hardware.GPU)
	}
	if !p.Hardware.IsModelServing() {
		t.Fatal("expected model-serving profile")
	}
	if p.Hardware.System == nil || p.Hardware.System.RAMGB != 128 {
		t.Fatalf("system block not parsed: %+v", p.Hardware.System)
	}
}

// TestReadProfileConfigNoHardware confirms a CPU/cloud profile (no hardware block)
// parses fine and is simply not model-serving.
func TestReadProfileConfigNoHardware(t *testing.T) {
	p, err := readProfileConfig([]byte("rule: cpu:zen2=true\n"))
	if err != nil {
		t.Fatalf("readProfileConfig: %v", err)
	}
	if p.Hardware.IsModelServing() {
		t.Fatal("profile without hardware block must not be model-serving")
	}
}
