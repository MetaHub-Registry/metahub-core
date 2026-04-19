// Package utils — LayerSlice carries a parsed, Python-style slice
// expression and applies it to a concrete layer (or diffID) list.
//
// Supported input shapes (bracket-optional, whitespace-tolerant):
//
//	""                 identity — returns the input slice unchanged
//	"[:]"              explicit identity
//	"[N]"              single index; negative wraps (N = -1 is the last)
//	"[N:]"             from N (inclusive) to the end
//	"[:M]"             start to M (exclusive)
//	"[N:M]"            N inclusive to M exclusive
//	"[N:M:S]"          step S (positive only; zero and negative rejected)
//	"[::S]"            full list stepped by S
//
// ParseLayerSlice validates syntax only. Out-of-range indices against
// the actual slice length are detected at ApplyTo*-time so callers get
// a clear error that mentions the observed length. This mirrors how
// Python raises IndexError only when a concrete single-index access
// cannot resolve — open-ended ranges silently clamp, while a single
// explicit index that falls outside the real list is a hard error
// (matches the "fail the pull" decision for the manipulation schema).
package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LayerSlice is a parsed slice expression. Callers use ApplyToLayers
// or ApplyToStrings; the struct itself is opaque.
type LayerSlice struct {
	// identity = true means "return the input unchanged". Set for
	// an empty input string and for "[:]".
	identity bool
	// single = true means the slice is a single-index access "[N]".
	// When true, idx is the (possibly negative) literal index.
	single bool
	idx    int
	// Closed / open / stepped range fields. Used only when single
	// is false and identity is false.
	hasStart bool
	start    int
	hasEnd   bool
	end      int
	step     int // defaults to 1 when the literal form omits the step
}

// layerSliceRe matches the recognised shapes after bracket / whitespace
// stripping. Using alternation (rather than a single group with lots of
// optional bits) keeps each sub-pattern readable.
//
//	single       ^-?\d+$
//	range        ^(-?\d+)?:(-?\d+)?$
//	rangeStepped ^(-?\d+)?:(-?\d+)?:(-?\d+)$
var (
	reSingle       = regexp.MustCompile(`^-?\d+$`)
	reRange        = regexp.MustCompile(`^(-?\d+)?:(-?\d+)?$`)
	reRangeStepped = regexp.MustCompile(`^(-?\d+)?:(-?\d+)?:(-?\d+)$`)
)

// ParseLayerSlice parses a slice expression string. An empty / pure
// whitespace input yields an identity slice.
func ParseLayerSlice(s string) (LayerSlice, error) {
	trim := strings.TrimSpace(s)
	if trim == "" {
		return LayerSlice{identity: true}, nil
	}
	// Tolerate bracket-wrapped forms and inner whitespace
	// ("[ 1 : 3 ]" → "1:3"). Brackets must be balanced when present.
	if strings.HasPrefix(trim, "[") || strings.HasSuffix(trim, "]") {
		if !strings.HasPrefix(trim, "[") || !strings.HasSuffix(trim, "]") {
			return LayerSlice{}, fmt.Errorf("select %q: unbalanced brackets", s)
		}
		trim = trim[1 : len(trim)-1]
	}
	trim = strings.ReplaceAll(trim, " ", "")
	if trim == "" || trim == ":" {
		return LayerSlice{identity: true}, nil
	}
	if reSingle.MatchString(trim) {
		n, err := strconv.Atoi(trim)
		if err != nil {
			return LayerSlice{}, fmt.Errorf("select %q: %v", s, err)
		}
		return LayerSlice{single: true, idx: n}, nil
	}
	if m := reRangeStepped.FindStringSubmatch(trim); m != nil {
		return parseRange(s, m[1], m[2], m[3])
	}
	if m := reRange.FindStringSubmatch(trim); m != nil {
		return parseRange(s, m[1], m[2], "1")
	}
	return LayerSlice{}, fmt.Errorf("select %q: not a recognised slice expression", s)
}

// parseRange turns the regex capture groups into a LayerSlice. Empty
// start / end groups mean "open" on that side; an empty step is the
// caller's problem (reRange passes "1" as default).
func parseRange(orig, startStr, endStr, stepStr string) (LayerSlice, error) {
	out := LayerSlice{}
	if startStr != "" {
		n, err := strconv.Atoi(startStr)
		if err != nil {
			return out, fmt.Errorf("select %q: invalid start %q", orig, startStr)
		}
		out.hasStart = true
		out.start = n
	}
	if endStr != "" {
		n, err := strconv.Atoi(endStr)
		if err != nil {
			return out, fmt.Errorf("select %q: invalid end %q", orig, endStr)
		}
		out.hasEnd = true
		out.end = n
	}
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		return out, fmt.Errorf("select %q: invalid step %q", orig, stepStr)
	}
	if step <= 0 {
		return out, fmt.Errorf("select %q: step must be positive (got %d)", orig, step)
	}
	out.step = step
	return out, nil
}

// resolve translates the stored start/end into concrete indices for
// an input of length n. Open-ended sides clamp to [0, n]; explicit
// negative indices wrap relative to n. Returns an error only when a
// single-index access falls outside [-n, n).
func (ls LayerSlice) resolve(n int) (start, end, step int, err error) {
	if ls.identity {
		return 0, n, 1, nil
	}
	if ls.single {
		idx := ls.idx
		if idx < 0 {
			idx += n
		}
		if idx < 0 || idx >= n {
			return 0, 0, 1, fmt.Errorf("select [%d]: out of range for input of length %d", ls.idx, n)
		}
		return idx, idx + 1, 1, nil
	}
	start = 0
	end = n
	step = ls.step
	if ls.hasStart {
		start = ls.start
		if start < 0 {
			start += n
		}
	}
	if ls.hasEnd {
		end = ls.end
		if end < 0 {
			end += n
		}
	}
	// Clamp to [0, n]. Python treats out-of-range open slices as
	// silently empty; we mirror that.
	start = clamp(start, 0, n)
	end = clamp(end, 0, n)
	if end < start {
		end = start
	}
	return start, end, step, nil
}

// clamp bounds x into [lo, hi].
func clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// ApplyToLayers returns the subset of layers selected by ls. The
// input is never mutated. Out-of-range single-index selection is an
// error; clampable range selection produces the (possibly empty)
// subset without error.
func (ls LayerSlice) ApplyToLayers(in []LayerLike) ([]LayerLike, error) {
	start, end, step, err := ls.resolve(len(in))
	if err != nil {
		return nil, err
	}
	out := make([]LayerLike, 0, (end-start+step-1)/maxInt(step, 1))
	for i := start; i < end; i += step {
		out = append(out, in[i])
	}
	return out, nil
}

// LayerLike is a narrow interface satisfied by types.Layer (and
// anything else shaped like it) so the apply helper stays in the
// utils package without a cyclic import. Callers pass their concrete
// slice wrapped in a tiny adapter — see ApplyIndices for a more
// convenient zero-copy helper when the caller already has the slice
// at hand.
type LayerLike = interface{}

// ApplyIndices returns the concrete indices selected by ls against an
// input of length n. Callers pick the same indices from their own
// slice + from the parallel diffID slice — this is the primitive we
// use inside magazine to keep the two in lockstep without the
// interface gymnastics of ApplyToLayers.
func (ls LayerSlice) ApplyIndices(n int) ([]int, error) {
	start, end, step, err := ls.resolve(n)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, n)
	for i := start; i < end; i += step {
		out = append(out, i)
	}
	return out, nil
}

// IsIdentity reports whether the slice is a pass-through. Callers
// can short-circuit the apply step when true.
func (ls LayerSlice) IsIdentity() bool { return ls.identity }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
