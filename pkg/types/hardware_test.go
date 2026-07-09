package types

import "testing"

func TestHardwareIsModelServing(t *testing.T) {
	cases := []struct {
		name string
		hw   *Hardware
		want bool
	}{
		{"nil hardware", nil, false},
		{"no gpu block", &Hardware{System: &HardwareSystem{RAMGB: 128}}, false},
		{"gpu without vram", &Hardware{GPU: &HardwareGPU{Vendor: "amd"}}, false},
		{"gpu without vendor", &Hardware{GPU: &HardwareGPU{VRAMGB: 124}}, false},
		{"model-serving gpu", &Hardware{GPU: &HardwareGPU{Vendor: "amd", VRAMGB: 124}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.hw.IsModelServing(); got != c.want {
				t.Fatalf("IsModelServing() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestHardwareVRAMBytes(t *testing.T) {
	var nilHW *Hardware
	if got := nilHW.VRAMBytes(); got != 0 {
		t.Fatalf("nil VRAMBytes() = %d, want 0", got)
	}
	if got := (&Hardware{System: &HardwareSystem{RAMGB: 128}}).VRAMBytes(); got != 0 {
		t.Fatalf("no-gpu VRAMBytes() = %d, want 0", got)
	}
	hw := &Hardware{GPU: &HardwareGPU{Vendor: "amd", VRAMGB: 124}}
	if got, want := hw.VRAMBytes(), int64(124)<<30; got != want {
		t.Fatalf("VRAMBytes() = %d, want %d", got, want)
	}
}
