package neo

import (
	"fmt"
	"strings"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

// Profile hardware persistence: the GPU/System block is stored as flat hw_* props
// on the :Profile node (single-node write, no new label). These helpers are pure so
// they can be unit-tested without a live Neo.

// hardwareReturnFields projects the flat hw_* props back for ProfileFromResult. It
// is appended to returnStatementProfile. Missing props project as null → the typed
// getters return zero, and hardwareFromResult treats "no vendor & no vram" as nil.
const hardwareReturnFields = `, prf.hw_gpu_vendor AS hwGpuVendor, prf.hw_gpu_model AS hwGpuModel, ` +
	`prf.hw_gpu_product_name AS hwGpuProductName, prf.hw_gpu_gfx_family AS hwGpuGfxFamily, ` +
	`prf.hw_gpu_vram_gb AS hwGpuVramGb, prf.hw_gpu_cu_count AS hwGpuCuCount, ` +
	`prf.hw_gpu_simd_count AS hwGpuSimdCount, prf.hw_system_ram_gb AS hwSystemRamGb, ` +
	`prf.hw_system_cpu_cores AS hwSystemCpuCores`

// hardwareCreateProps returns a Cypher property fragment (leading ", ") encoding a
// profile's hardware block as flat hw_* props, or "" when there is no GPU block.
// Values originate from trusted CE YAML.
func hardwareCreateProps(hw *types.Hardware) string {
	if hw == nil || hw.GPU == nil {
		return ""
	}
	var b strings.Builder
	g := hw.GPU
	fmt.Fprintf(&b, `, hw_gpu_vendor: "%s"`, g.Vendor)
	if g.Model != "" {
		fmt.Fprintf(&b, `, hw_gpu_model: "%s"`, g.Model)
	}
	if g.ProductName != "" {
		fmt.Fprintf(&b, `, hw_gpu_product_name: "%s"`, g.ProductName)
	}
	if g.GfxFamily != "" {
		fmt.Fprintf(&b, `, hw_gpu_gfx_family: "%s"`, g.GfxFamily)
	}
	fmt.Fprintf(&b, `, hw_gpu_vram_gb: %d`, g.VRAMGB)
	if g.CUCount != 0 {
		fmt.Fprintf(&b, `, hw_gpu_cu_count: %d`, g.CUCount)
	}
	if g.SIMDCount != 0 {
		fmt.Fprintf(&b, `, hw_gpu_simd_count: %d`, g.SIMDCount)
	}
	if hw.System != nil {
		if hw.System.RAMGB != 0 {
			fmt.Fprintf(&b, `, hw_system_ram_gb: %d`, hw.System.RAMGB)
		}
		if hw.System.CPUCores != 0 {
			fmt.Fprintf(&b, `, hw_system_cpu_cores: %d`, hw.System.CPUCores)
		}
	}
	return b.String()
}

// hardwareFromResult reconstructs a *types.Hardware from a projected row, or nil
// when the profile has no GPU block (no vendor and no vram → not model-serving).
func hardwareFromResult(res types.Result) *types.Hardware {
	vendor, _ := res.GetStr("hwGpuVendor")
	vram, _ := res.GetInt64("hwGpuVramGb")
	if vendor == "" && vram == 0 {
		return nil
	}
	model, _ := res.GetStr("hwGpuModel")
	product, _ := res.GetStr("hwGpuProductName")
	gfx, _ := res.GetStr("hwGpuGfxFamily")
	cu, _ := res.GetInt64("hwGpuCuCount")
	simd, _ := res.GetInt64("hwGpuSimdCount")
	ram, _ := res.GetInt64("hwSystemRamGb")
	cores, _ := res.GetInt64("hwSystemCpuCores")
	hw := &types.Hardware{
		GPU: &types.HardwareGPU{
			Vendor:      vendor,
			Model:       model,
			ProductName: product,
			GfxFamily:   gfx,
			VRAMGB:      int(vram),
			CUCount:     int(cu),
			SIMDCount:   int(simd),
		},
	}
	if ram != 0 || cores != 0 {
		hw.System = &types.HardwareSystem{RAMGB: int(ram), CPUCores: int(cores)}
	}
	return hw
}
