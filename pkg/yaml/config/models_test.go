package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const sampleModelYAML = `name: qnib/minicpm5
runtime: llama-server
quants:
  - { tag: q8-0, source: registry.gitlab.com/x/minicpm5:q8-0-gguf }
  - { tag: q4-k-m, source: registry.gitlab.com/x/minicpm5:q4-k-m-gguf }
latest:
  default: best-fit
  overrides:
    - { profile: frame, quant: q8-0 }
    - { rule: "hardware.gpu.vram_gb >= 100", quant: q8-0 }
config:
  default: { ctx_size: 32768, kv_cache_type: q8_0 }
  overrides:
    - { profile: frame, config: { ctx_size: 65536 } }
    - { rule: "hardware.gpu.vram_gb < 16", config: { ctx_size: 8192, kv_cache_type: q4_0 } }
`

func writeModelFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "models", "qnib", "minicpm5", "latest.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFindModels(t *testing.T) {
	dir := writeModelFile(t, sampleModelYAML)
	models, err := FindModels(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := models["qnib/minicpm5:latest"]
	if !ok {
		t.Fatalf("model not keyed as qnib/minicpm5:latest; got keys %v", keys(models))
	}
	if m.Runtime != "llama-server" {
		t.Fatalf("runtime = %q", m.Runtime)
	}
	if len(m.Quants) != 2 || m.Quants[0].Tag != "q8-0" || m.Quants[0].Source == "" {
		t.Fatalf("quants mis-parsed: %+v", m.Quants)
	}
	if m.Latest.Default != "best-fit" || len(m.Latest.Overrides) != 2 {
		t.Fatalf("latest mis-parsed: %+v", m.Latest)
	}
	if m.Latest.Overrides[0].Profile != "frame" || m.Latest.Overrides[0].Quant != "q8-0" {
		t.Fatalf("latest override[0] mis-parsed: %+v", m.Latest.Overrides[0])
	}
	if m.Latest.Overrides[1].Rule == "" {
		t.Fatalf("latest override[1] rule missing: %+v", m.Latest.Overrides[1])
	}
	cfg, ok := m.Config.Default.(map[string]any)
	if !ok || cfg["ctx_size"] != 32768 || cfg["kv_cache_type"] != "q8_0" {
		t.Fatalf("config default mis-parsed: %+v", m.Config.Default)
	}
	if len(m.Config.Overrides) != 2 || m.Config.Overrides[0].Profile != "frame" {
		t.Fatalf("config overrides mis-parsed: %+v", m.Config.Overrides)
	}
	if m.Config.Overrides[0].Config["ctx_size"] != 65536 {
		t.Fatalf("config override patch mis-parsed: %+v", m.Config.Overrides[0].Config)
	}
}

func TestFindModelsSkipsNonMatchingFiles(t *testing.T) {
	dir := writeModelFile(t, sampleModelYAML)
	// A template that loads as a valid model but whose path is not
	// `<repo>/<tag>.yaml` used to abort the whole scan: FindModels returned the
	// normalize error and metahub-admin's parse models then skipped the
	// directory, so every model was silently dropped.
	extra := filepath.Join(dir, "models", "sources.example.yaml")
	if err := os.WriteFile(extra, []byte(sampleModelYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	models, err := FindModels(context.Background(), dir)
	if err != nil {
		t.Fatalf("a non-matching filename must not fail the scan: %v", err)
	}
	if _, ok := models["qnib/minicpm5:latest"]; !ok {
		t.Fatalf("valid model dropped because of a stray file; keys %v", keys(models))
	}
}

func TestModelValidate(t *testing.T) {
	base := func() Model {
		return Model{
			Name:    "qnib/m",
			Runtime: "llama-server",
			Quants:  []ModelQuant{{Tag: "q8", Source: "reg/m:q8"}},
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("valid model rejected: %v", err)
	}
	noRuntime := base()
	noRuntime.Runtime = ""
	if noRuntime.Validate() == nil {
		t.Fatal("missing runtime must fail")
	}
	noSource := base()
	noSource.Quants = []ModelQuant{{Tag: "q8"}}
	if noSource.Validate() == nil {
		t.Fatal("quant without source must fail")
	}
	bothSelectors := base()
	bothSelectors.Latest.Overrides = []ModelQuantOverride{{Profile: "frame", Rule: "hardware.gpu.vram_gb >= 100", Quant: "q8"}}
	if bothSelectors.Validate() == nil {
		t.Fatal("override with both profile and rule must fail")
	}
	noQuant := base()
	noQuant.Latest.Overrides = []ModelQuantOverride{{Profile: "frame"}}
	if noQuant.Validate() == nil {
		t.Fatal("latest override without quant must fail")
	}
}

func keys(m map[string]Model) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
