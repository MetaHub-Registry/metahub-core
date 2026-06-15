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
// a clear error that mentions the observed length.
//
// Resolution uses STRICT semantics (it does not mirror Python's silent
// clamping): any explicitly written bound — single index, range start,
// or range end — that falls outside [0, n] after negative-wrapping is a
// hard error, and a range that resolves to an empty layer set is also
// an error. This implements the manipulation schema's "out-of-range
// select fails the pull" decision so a misconfigured select surfaces
// loudly instead of silently producing a truncated or empty image.
// Only an omitted side of a range keeps its implicit default (0 / n).
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
// an input of length n. Explicit negative indices wrap relative to n;
// an omitted range side defaults to 0 (start) / n (end). It returns an
// error when any explicit bound falls outside [0, n] after wrapping,
// when a single-index access falls outside [0, n), or when a range
// resolves to an empty set.
func (ls LayerSlice) resolve(n int) (start, end, step int, err error) {
	if ls.identity {
		return 0, n, 1, nil
	}
	if ls.single {
		return ls.resolveSingle(n)
	}
	// Strict semantics: an explicitly written bound that falls
	// outside [0, n] after negative-wrapping is a hard error so a
	// misconfigured select fails the pull loudly instead of silently
	// resolving to a truncated (or empty) layer set. Only the
	// implicit defaults for an omitted side are left untouched.
	start, err = resolveBound("start", ls.hasStart, ls.start, 0, n)
	if err != nil {
		return 0, 0, 1, err
	}
	end, err = resolveBound("end", ls.hasEnd, ls.end, n, n)
	if err != nil {
		return 0, 0, 1, err
	}
	// An empty resolved range (e.g. "[3:1]" or "[0:0]") almost
	// certainly means a mistyped select — appending zero layers is a
	// silent footgun, so reject it.
	if end <= start {
		return 0, 0, 1, fmt.Errorf("select [%s]: resolves to an empty layer set for input of length %d", ls.rangeString(), n)
	}
	return start, end, ls.step, nil
}

// resolveSingle resolves a single-index access against length n,
// wrapping a negative index and erroring when it lands outside [0, n).
func (ls LayerSlice) resolveSingle(n int) (start, end, step int, err error) {
	idx := ls.idx
	if idx < 0 {
		idx += n
	}
	if idx < 0 || idx >= n {
		return 0, 0, 1, fmt.Errorf("select [%d]: out of range for input of length %d", ls.idx, n)
	}
	return idx, idx + 1, 1, nil
}

// resolveBound resolves one range bound. When the side is omitted it
// takes def; otherwise the literal value is negative-wrapped against n
// and validated to lie within [0, n]. name is used in the error.
func resolveBound(name string, has bool, lit, def, n int) (int, error) {
	if !has {
		return def, nil
	}
	v := lit
	if v < 0 {
		v += n
	}
	if v < 0 || v > n {
		return 0, fmt.Errorf("select %s %d: out of range for input of length %d", name, lit, n)
	}
	return v, nil
}

// rangeString renders the parsed range back to a readable "N:M" /
// "N:M:S" form for error messages.
func (ls LayerSlice) rangeString() string {
	startStr := ""
	if ls.hasStart {
		startStr = strconv.Itoa(ls.start)
	}
	endStr := ""
	if ls.hasEnd {
		endStr = strconv.Itoa(ls.end)
	}
	if ls.step != 1 {
		return fmt.Sprintf("%s:%s:%d", startStr, endStr, ls.step)
	}
	return fmt.Sprintf("%s:%s", startStr, endStr)
}

// ApplyToLayers returns the subset of layers selected by ls. The
// input is never mutated. Out-of-range selection (single index or
// range bound) and ranges that resolve to an empty set are errors.
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
