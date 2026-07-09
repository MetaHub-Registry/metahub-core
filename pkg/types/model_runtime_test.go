package types

import (
	"reflect"
	"testing"
)

func TestRuntimeCompatible(t *testing.T) {
	rt := Runtime{Name: "llama-server", Formats: []string{"gguf"}, ConfigContract: "qnib.model.config.v1"}
	cases := []struct {
		name    string
		format  string
		contract string
		want    bool
	}{
		{"format + contract match", "gguf", "qnib.model.config.v1", true},
		{"format match, case-insensitive", "GGUF", "qnib.model.config.v1", true},
		{"format mismatch", "safetensors", "qnib.model.config.v1", false},
		{"contract mismatch", "gguf", "other.contract", false},
		{"empty model format is permissive", "", "qnib.model.config.v1", true},
	}
	for _, c := range cases {
		if got := RuntimeCompatible(rt, c.format, c.contract); got != c.want {
			t.Fatalf("%s: RuntimeCompatible(%q,%q)=%v want %v", c.name, c.format, c.contract, got, c.want)
		}
	}

	// Empty runtime config-contract is a wildcard.
	wild := Runtime{Name: "any", Formats: []string{"gguf"}}
	if !RuntimeCompatible(wild, "gguf", "whatever") {
		t.Fatal("empty config-contract should match any config")
	}
}

func TestFormatsCSVRoundTrip(t *testing.T) {
	rt := Runtime{Formats: []string{"gguf", "safetensors"}}
	if rt.FormatsCSV() != "gguf,safetensors" {
		t.Fatalf("FormatsCSV = %q", rt.FormatsCSV())
	}
	got := FormatsFromCSV(" gguf , ,safetensors ")
	if !reflect.DeepEqual(got, []string{"gguf", "safetensors"}) {
		t.Fatalf("FormatsFromCSV = %#v", got)
	}
	if len(FormatsFromCSV("")) != 0 {
		t.Fatal("empty CSV should yield no formats")
	}
}
