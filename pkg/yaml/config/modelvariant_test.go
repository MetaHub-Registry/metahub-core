package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUpsertModelVariant_Fresh(t *testing.T) {
	out, err := UpsertModelVariant(nil, "llama3.2", "12gb", "reg/models/llama3.2-layer:q4-km")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var m Manipulation
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if m.Label != "model/llama3.2" {
		t.Fatalf("label=%q", m.Label)
	}
	v, ok := m.Variants["vram:12gb=true"]
	if !ok {
		t.Fatalf("variant missing; got %v", m.Variants)
	}
	if len(v.Layers) != 1 || v.Layers[0].Image != "reg/models/llama3.2-layer:q4-km" {
		t.Fatalf("variant layers=%v", v.Layers)
	}
}

func TestUpsertModelVariant_AddsSecondTierKeepsFirst(t *testing.T) {
	first, err := UpsertModelVariant(nil, "qwen", "12gb", "reg/models/qwen-layer:q4-km")
	if err != nil {
		t.Fatal(err)
	}
	second, err := UpsertModelVariant(first, "qwen", "128gb", "reg/models/qwen-layer:q8-kxl")
	if err != nil {
		t.Fatal(err)
	}
	var m Manipulation
	if err := yaml.Unmarshal(second, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Variants) != 2 {
		t.Fatalf("want 2 variants, got %d: %v", len(m.Variants), m.Variants)
	}
	if m.Variants["vram:12gb=true"].Layers[0].Image != "reg/models/qwen-layer:q4-km" {
		t.Fatal("first tier lost")
	}
	if m.Variants["vram:128gb=true"].Layers[0].Image != "reg/models/qwen-layer:q8-kxl" {
		t.Fatal("second tier missing")
	}
}

func TestUpsertModelVariant_Idempotent(t *testing.T) {
	a, err := UpsertModelVariant(nil, "m", "8gb", "reg/models/m-layer:q4")
	if err != nil {
		t.Fatal(err)
	}
	b, err := UpsertModelVariant(a, "m", "8gb", "reg/models/m-layer:q4")
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("re-upsert not idempotent:\n%s\n---\n%s", a, b)
	}
}
