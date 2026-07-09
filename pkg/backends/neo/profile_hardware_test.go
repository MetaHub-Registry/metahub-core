package neo

import (
	"strings"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestHardwareCreateProps(t *testing.T) {
	if got := hardwareCreateProps(nil); got != "" {
		t.Fatalf("nil → %q, want empty", got)
	}
	if got := hardwareCreateProps(&types.Hardware{System: &types.HardwareSystem{RAMGB: 128}}); got != "" {
		t.Fatalf("no-gpu → %q, want empty", got)
	}
	hw := &types.Hardware{
		GPU:    &types.HardwareGPU{Vendor: "amd", Model: "gfx1151", VRAMGB: 124, CUCount: 40},
		System: &types.HardwareSystem{RAMGB: 128, CPUCores: 32},
	}
	got := hardwareCreateProps(hw)
	if !strings.HasPrefix(got, ", ") {
		t.Fatalf("fragment must start with ', ' to append inside a prop map: %q", got)
	}
	for _, want := range []string{
		`hw_gpu_vendor: "amd"`, `hw_gpu_model: "gfx1151"`, `hw_gpu_vram_gb: 124`,
		`hw_gpu_cu_count: 40`, `hw_system_ram_gb: 128`, `hw_system_cpu_cores: 32`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fragment %q missing %q", got, want)
		}
	}
	// omitted optionals must not appear
	if strings.Contains(got, "hw_gpu_simd_count") {
		t.Fatalf("zero-valued simd_count should be omitted: %q", got)
	}
}

func TestHardwareFromResult(t *testing.T) {
	if hw := hardwareFromResult(types.Result{}); hw != nil {
		t.Fatalf("empty result → %+v, want nil", hw)
	}
	res := types.Result{
		Strings: map[string]string{"hwGpuVendor": "amd", "hwGpuModel": "gfx1151"},
		Int64s:  map[string]int64{"hwGpuVramGb": 124, "hwSystemRamGb": 128},
	}
	hw := hardwareFromResult(res)
	if hw == nil || hw.GPU == nil {
		t.Fatal("expected hardware with gpu")
	}
	if hw.GPU.Vendor != "amd" || hw.GPU.VRAMGB != 124 {
		t.Fatalf("gpu mismatch: %+v", hw.GPU)
	}
	if !hw.IsModelServing() || hw.VRAMBytes() != int64(124)<<30 {
		t.Fatalf("capability/capacity wrong: serving=%v bytes=%d", hw.IsModelServing(), hw.VRAMBytes())
	}
	if hw.System == nil || hw.System.RAMGB != 128 {
		t.Fatalf("system: %+v", hw.System)
	}
}
