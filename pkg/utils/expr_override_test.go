package utils

import "testing"

// TestRuleSpecificity_GrammarProbe discovers which comparison operators the lep
// parser accepts for the hardware.* override grammar. It asserts nothing beyond
// "some form parses"; the concrete grammar is locked from its output.
func TestRuleSpecificity_GrammarProbe(t *testing.T) {
	probes := []string{
		"hardware.gpu.vram_gb > 100",
		"hardware.gpu.vram_gb >= 100",
		"hardware.gpu.vram_gb = 100",
		"hardware.gpu.vram_gb == 100",
		`hardware.gpu.vendor = "amd"`,
		`hardware.gpu.vendor == "amd"`,
		"hardware.gpu.vram_gb > 100 && hardware.gpu.vendor = \"amd\"",
		"hardware.gpu.vram_gb > 100 || hardware.gpu.vram_gb > 50",
		"hardware.gpu.vram_gb < 16",
	}
	parsed := 0
	for _, p := range probes {
		got, err := RuleSpecificity(p)
		if err == nil {
			parsed++
		}
		t.Logf("rule=%-58s specificity=%d err=%v", p, got, err)
	}
	if parsed == 0 {
		t.Fatal("no probe grammar parsed — need to rethink the override rule syntax")
	}
}

func TestRuleSpecificity(t *testing.T) {
	cases := []struct {
		rule string
		want int
	}{
		{"hardware.gpu.vram_gb >= 100", 1},
		{`hardware.gpu.vram_gb >= 100 && hardware.gpu.vendor = "amd"`, 2},        // AND sums
		{"hardware.gpu.vram_gb > 100 || hardware.gpu.vram_gb > 50", 1},          // OR = min
		{`hardware.gpu.vram_gb >= 100 && hardware.gpu.vendor = "amd" && hardware.gpu.model = "gfx1151"`, 3},
	}
	for _, c := range cases {
		got, err := RuleSpecificity(c.rule)
		if err != nil {
			t.Fatalf("RuleSpecificity(%q) error: %v", c.rule, err)
		}
		if got != c.want {
			t.Fatalf("RuleSpecificity(%q) = %d, want %d", c.rule, got, c.want)
		}
	}
	// `==` is not valid lep grammar → parse error.
	if _, err := RuleSpecificity("hardware.gpu.vram_gb == 100"); err == nil {
		t.Fatal("expected parse error for `==`")
	}
}

func TestEvalHardwareRule(t *testing.T) {
	frame := map[string]any{
		"hardware.gpu.vram_gb": 124,
		"hardware.gpu.vendor":  "amd",
		"hardware.gpu.model":   "gfx1151",
	}
	small := map[string]any{
		"hardware.gpu.vram_gb": int64(12),
		"hardware.gpu.vendor":  "nvidia",
	}
	cases := []struct {
		rule   string
		fields map[string]any
		want   bool
	}{
		{"hardware.gpu.vram_gb >= 100", frame, true},
		{"hardware.gpu.vram_gb >= 100", small, false},
		{"hardware.gpu.vram_gb < 16", small, true},
		{`hardware.gpu.vendor = "amd"`, frame, true},
		{`hardware.gpu.vendor = "amd"`, small, false},
		{`hardware.gpu.vram_gb >= 100 && hardware.gpu.vendor = "amd"`, frame, true},
		{`hardware.gpu.vram_gb >= 100 && hardware.gpu.vendor = "amd"`, small, false},
		{"hardware.gpu.vram_gb < 16 || hardware.gpu.vram_gb > 100", frame, true}, // 124>100
		{"hardware.gpu.vram_gb < 16 || hardware.gpu.vram_gb > 100", small, true}, // 12<16
		{"hardware.gpu.cu_count >= 40", frame, false},                           // absent field → false
		{`hardware.gpu.vendor = "amd"`, map[string]any{"hardware.gpu.vram_gb": 124}, false}, // absent field
	}
	for _, c := range cases {
		got, err := EvalHardwareRule(c.rule, c.fields)
		if err != nil {
			t.Fatalf("EvalHardwareRule(%q) error: %v", c.rule, err)
		}
		if got != c.want {
			t.Fatalf("EvalHardwareRule(%q, %v) = %v, want %v", c.rule, c.fields, got, c.want)
		}
	}
	// Parse error surfaces.
	if _, err := EvalHardwareRule("hardware.gpu.vram_gb == 100", frame); err == nil {
		t.Fatal("expected parse error for `==`")
	}
}
