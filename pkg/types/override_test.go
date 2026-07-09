package types

import "testing"

// frameHW mirrors the `frame` profile (Strix Halo: amd, 124 GB).
func frameHW() map[string]any {
	return (&Hardware{
		GPU:    &HardwareGPU{Vendor: "amd", Model: "gfx1151", VRAMGB: 124},
		System: &HardwareSystem{RAMGB: 128},
	}).FieldMap()
}

func TestResolveOverride_ProfilePinWins(t *testing.T) {
	// A profile-alias pin beats a matching rule regardless of the rule's specificity.
	ovs := []Override[string]{
		{Rule: `hardware.gpu.vram_gb >= 100 && hardware.gpu.vendor = "amd"`, Value: "rule"},
		{Profile: "frame", Value: "pin"},
	}
	got, ok := ResolveOverride(ovs, "frame", frameHW())
	if !ok || got != "pin" {
		t.Fatalf("profile pin must win: got=%q ok=%v", got, ok)
	}
}

func TestResolveOverride_MostSpecificRuleWins(t *testing.T) {
	// The 2-term AND (specificity 2) beats the 1-term rule (specificity 1).
	ovs := []Override[string]{
		{Rule: "hardware.gpu.vram_gb >= 100", Value: "one"},
		{Rule: `hardware.gpu.vram_gb >= 100 && hardware.gpu.vendor = "amd"`, Value: "two"},
	}
	got, ok := ResolveOverride(ovs, "frame", frameHW())
	if !ok || got != "two" {
		t.Fatalf("most-specific rule must win: got=%q ok=%v", got, ok)
	}
}

func TestResolveOverride_TieFirstDeclared(t *testing.T) {
	// Two matching 1-term rules → the first-declared wins.
	ovs := []Override[string]{
		{Rule: `hardware.gpu.vendor = "amd"`, Value: "first"},
		{Rule: "hardware.gpu.vram_gb >= 100", Value: "second"},
	}
	got, ok := ResolveOverride(ovs, "frame", frameHW())
	if !ok || got != "first" {
		t.Fatalf("tie must go to first declared: got=%q ok=%v", got, ok)
	}
}

func TestResolveOverride_NoMatch(t *testing.T) {
	// A pin for another profile and a rule that doesn't match → no override.
	ovs := []Override[string]{
		{Profile: "bazzite", Value: "pin"},
		{Rule: "hardware.gpu.vram_gb < 16", Value: "small"},
	}
	if _, ok := ResolveOverride(ovs, "frame", frameHW()); ok {
		t.Fatal("no override should match frame")
	}
}

func TestHardwareFieldMap(t *testing.T) {
	m := (&Hardware{GPU: &HardwareGPU{Vendor: "amd", VRAMGB: 124}}).FieldMap()
	if m["hardware.gpu.vendor"] != "amd" || m["hardware.gpu.vram_gb"] != 124 {
		t.Fatalf("unexpected field map: %+v", m)
	}
	var nilHW *Hardware
	if len(nilHW.FieldMap()) != 0 {
		t.Fatal("nil hardware should yield an empty map")
	}
}
