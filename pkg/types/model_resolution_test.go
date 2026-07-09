package types

import (
	"errors"
	"testing"
)

const gb = int64(1) << 30

func quants() []ModelImageRef {
	return []ModelImageRef{
		{Repo: "models/m-layer", Tag: "q4", Size: 4 * gb},
		{Repo: "models/m-layer", Tag: "q8", Size: 8 * gb},
	}
}

func TestResolveModelServe_BestFit(t *testing.T) {
	// 12 GB fits both → highest precision (largest) = q8.
	got, err := ResolveModelServe(quants(), "frame", 12*gb, "", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "q8" {
		t.Fatalf("want q8, got %q", got.Tag)
	}

	// 6 GB fits only q4 → q4.
	got, err = ResolveModelServe(quants(), "frame", 6*gb, "", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "q4" {
		t.Fatalf("want q4, got %q", got.Tag)
	}
}

func TestResolveModelServe_ExactFit(t *testing.T) {
	// 8 GB exactly fits q8.
	got, err := ResolveModelServe(quants(), "x", 8*gb, "", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "q8" {
		t.Fatalf("want q8 at exact fit, got %q", got.Tag)
	}
}

func TestResolveModelServe_NoneFit(t *testing.T) {
	_, err := ResolveModelServe(quants(), "x", 2*gb, "", 1.0)
	var infeasible *ErrQuantInfeasible
	if !errors.As(err, &infeasible) {
		t.Fatalf("want ErrQuantInfeasible, got %v", err)
	}
	if infeasible.Quant != "" || infeasible.Required != 4*gb {
		t.Fatalf("unexpected infeasible: %+v", infeasible)
	}
}

func TestResolveModelServe_NamedFit(t *testing.T) {
	got, err := ResolveModelServe(quants(), "x", 12*gb, "q8", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "q8" {
		t.Fatalf("want q8, got %q", got.Tag)
	}
}

func TestResolveModelServe_NamedInfeasible(t *testing.T) {
	_, err := ResolveModelServe(quants(), "bazzite", 6*gb, "q8", 1.0)
	var infeasible *ErrQuantInfeasible
	if !errors.As(err, &infeasible) {
		t.Fatalf("want ErrQuantInfeasible, got %v", err)
	}
	if infeasible.Quant != "q8" || infeasible.Profile != "bazzite" || infeasible.Required != 8*gb {
		t.Fatalf("unexpected infeasible: %+v", infeasible)
	}
}

func TestResolveModelServe_Headroom(t *testing.T) {
	// 8 GB with 0.9 headroom → effective 7.2 GB → q8 (8 GB) no longer fits → q4.
	got, err := ResolveModelServe(quants(), "x", 8*gb, "", 0.9)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "q4" {
		t.Fatalf("with headroom: want q4, got %q", got.Tag)
	}
}

func TestResolveModelServe_NamedNotFound(t *testing.T) {
	_, err := ResolveModelServe(quants(), "x", 12*gb, "q2", 1.0)
	if err == nil {
		t.Fatal("expected error for unknown quant")
	}
	var infeasible *ErrQuantInfeasible
	if errors.As(err, &infeasible) {
		t.Fatalf("unknown quant should not be ErrQuantInfeasible: %v", err)
	}
}

func TestResolveModelServe_NoCapacity(t *testing.T) {
	// A non-model-serving profile (no hardware.gpu → capacity 0) errors, and it is
	// a plain error, not an ErrQuantInfeasible (nothing was even evaluated).
	_, err := ResolveModelServe(quants(), "cpu-zen2", 0, "", 1.0)
	if err == nil {
		t.Fatal("expected error for zero capacity")
	}
	var infeasible *ErrQuantInfeasible
	if errors.As(err, &infeasible) {
		t.Fatalf("zero capacity should not be ErrQuantInfeasible: %v", err)
	}
}
