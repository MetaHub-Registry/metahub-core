package types

import (
	"fmt"
)

// ErrQuantInfeasible is returned when a requested (or the only) quantization does
// not fit the profile's VRAM capacity. The pull path maps it to an OCI
// `UNSUPPORTED` error (HTTP 400).
type ErrQuantInfeasible struct {
	Quant     string // the requested quant, or "" when no quant fits at all
	Profile   string // the profile label (identity only; capacity comes from hardware)
	Required  int64  // the (smallest, when Quant=="") weights size in bytes
	Available int64  // the effective capacity (after headroom) in bytes
}

func (e *ErrQuantInfeasible) Error() string {
	if e.Quant == "" {
		return fmt.Sprintf("no quantization fits profile %q: smallest requires %d bytes, %d available",
			e.Profile, e.Required, e.Available)
	}
	return fmt.Sprintf("quantization %q does not fit profile %q: requires %d bytes, %d available",
		e.Quant, e.Profile, e.Required, e.Available)
}

// ResolveModelServe picks the model-layer quantization to serve for a profile with
// VRAM capacity capacityBytes — supplied by the caller from the profile's
// hardware.gpu.vram_gb (see Hardware.VRAMBytes). profileLabel is used only for
// error identity, no longer parsed for capacity. quants are the model's quant
// images (Tag = quant, Size = weights). When namedQuant is empty, it returns the
// highest-precision quant that fits (the largest Size ≤ the effective capacity).
// When namedQuant is set, it returns that quant iff it fits, else an
// *ErrQuantInfeasible. headroomFactor (0<f≤1, default 1.0) reserves capacity for
// runtime/KV-cache overhead: a quant fits when Size ≤ capacity*headroomFactor.
// A non-positive capacity means the profile is not model-serving.
func ResolveModelServe(quants []ModelImageRef, profileLabel string, capacityBytes int64, namedQuant string, headroomFactor float64) (ModelImageRef, error) {
	if capacityBytes <= 0 {
		return ModelImageRef{}, fmt.Errorf("profile %q has no VRAM capacity (not model-serving)", profileLabel)
	}
	if headroomFactor <= 0 || headroomFactor > 1 {
		headroomFactor = 1.0
	}
	effective := int64(float64(capacityBytes) * headroomFactor)

	if namedQuant != "" {
		return resolveNamedQuant(quants, namedQuant, effective, profileLabel)
	}
	// Highest-precision that fits = the largest Size ≤ effective capacity.
	best := pickBestFitIndex(quants, effective)
	if best < 0 {
		return ModelImageRef{}, &ErrQuantInfeasible{
			Quant: "", Profile: profileLabel, Required: smallestSize(quants), Available: effective,
		}
	}
	return quants[best], nil
}

// resolveNamedQuant returns the named quant iff it fits the effective capacity,
// else an *ErrQuantInfeasible (present but too large) or a not-found error.
func resolveNamedQuant(quants []ModelImageRef, namedQuant string, effective int64, profileLabel string) (ModelImageRef, error) {
	for _, q := range quants {
		if q.Tag != namedQuant {
			continue
		}
		if q.Size <= effective {
			return q, nil
		}
		return ModelImageRef{}, &ErrQuantInfeasible{
			Quant: namedQuant, Profile: profileLabel, Required: q.Size, Available: effective,
		}
	}
	return ModelImageRef{}, fmt.Errorf("quantization %q not found for model", namedQuant)
}

// pickBestFitIndex returns the index of the largest quant whose Size ≤ effective,
// or -1 when none fit.
func pickBestFitIndex(quants []ModelImageRef, effective int64) int {
	best := -1
	for i := range quants {
		if quants[i].Size <= effective && (best < 0 || quants[i].Size > quants[best].Size) {
			best = i
		}
	}
	return best
}

// smallestSize returns the smallest positive quant size (for the no-fit error
// message); 0 when there are no quants.
func smallestSize(quants []ModelImageRef) int64 {
	min := int64(0)
	for _, q := range quants {
		if q.Size > 0 && (min == 0 || q.Size < min) {
			min = q.Size
		}
	}
	return min
}
