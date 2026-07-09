package types

// Hardware describes a profile's physical target. It is the source of truth for
// model-serving capacity (VRAM) and the `model-serving` capability: a profile is
// model-serving iff it carries a GPU block. The block is optional — CPU/cloud
// profiles omit it and are simply never model-serving candidates.
type Hardware struct {
	GPU    *HardwareGPU    `yaml:"gpu,omitempty" json:"gpu,omitempty"`
	System *HardwareSystem `yaml:"system,omitempty" json:"system,omitempty"`
}

// HardwareGPU is the GPU half of a hardware profile. VRAMGB is the capacity used
// for quant-fit; Vendor selects the runtime variant (rocm/cuda/mlx) via the runtime
// collection. The remaining fields are descriptive/scheduling metadata.
type HardwareGPU struct {
	Vendor      string `yaml:"vendor" json:"vendor"`
	Model       string `yaml:"model,omitempty" json:"model,omitempty"`
	ProductName string `yaml:"product_name,omitempty" json:"product_name,omitempty"`
	GfxFamily   string `yaml:"gfx_family,omitempty" json:"gfx_family,omitempty"`
	VRAMGB      int    `yaml:"vram_gb" json:"vram_gb"`
	CUCount     int    `yaml:"cu_count,omitempty" json:"cu_count,omitempty"`
	SIMDCount   int    `yaml:"simd_count,omitempty" json:"simd_count,omitempty"`
}

// HardwareSystem is the host (non-GPU) half of a hardware profile.
type HardwareSystem struct {
	RAMGB    int `yaml:"ram_gb,omitempty" json:"ram_gb,omitempty"`
	CPUCores int `yaml:"cpu_cores,omitempty" json:"cpu_cores,omitempty"`
}

// IsModelServing reports whether the profile can serve models: it needs a GPU
// block with a positive VRAM capacity and a vendor (to pick the runtime variant).
func (h *Hardware) IsModelServing() bool {
	return h != nil && h.GPU != nil && h.GPU.VRAMGB > 0 && h.GPU.Vendor != ""
}

// VRAMBytes returns the GPU VRAM capacity in bytes, or 0 when there is no GPU
// block. This is the capacity fed into ResolveModelServe (replacing the label parse).
func (h *Hardware) VRAMBytes() int64 {
	if h == nil || h.GPU == nil {
		return 0
	}
	return int64(h.GPU.VRAMGB) << 30
}

// FieldMap flattens the hardware block into dotted keys (e.g.
// "hardware.gpu.vram_gb", "hardware.gpu.vendor") matching the syntax used by
// override rules, so utils.EvalHardwareRule can evaluate a rule against a profile.
// Absent sub-blocks contribute no keys (a rule referencing a missing field is false).
func (h *Hardware) FieldMap() map[string]any {
	m := map[string]any{}
	if h == nil {
		return m
	}
	if h.GPU != nil {
		m["hardware.gpu.vendor"] = h.GPU.Vendor
		m["hardware.gpu.model"] = h.GPU.Model
		m["hardware.gpu.vram_gb"] = h.GPU.VRAMGB
		m["hardware.gpu.cu_count"] = h.GPU.CUCount
		m["hardware.gpu.simd_count"] = h.GPU.SIMDCount
		if h.GPU.ProductName != "" {
			m["hardware.gpu.product_name"] = h.GPU.ProductName
		}
		if h.GPU.GfxFamily != "" {
			m["hardware.gpu.gfx_family"] = h.GPU.GfxFamily
		}
	}
	if h.System != nil {
		m["hardware.system.ram_gb"] = h.System.RAMGB
		m["hardware.system.cpu_cores"] = h.System.CPUCores
	}
	return m
}
