package types

import (
	"testing"
)

func TestNewSlot(t *testing.T) {
	s := NewSlot("qnib/alpine", "latest", "generic")
	if s.ID != -1 {
		t.Errorf("NewSlot should produce ID=-1 before persistence, got %d", s.ID)
	}
	if s.Repo != "qnib/alpine" || s.Tag != "latest" || s.Profile != "generic" {
		t.Errorf("NewSlot did not propagate triple: got %+v", s)
	}
}

func TestSlotKey(t *testing.T) {
	s := NewSlot("qnib/alpine", "latest", "generic")
	want := "qnib/alpine:latest#generic"
	if got := s.Key(); got != want {
		t.Errorf("Slot.Key() = %q, want %q", got, want)
	}
}

func TestSlotString(t *testing.T) {
	s := Slot{ID: 42, Repo: "qnib/alpine", Tag: "latest", Profile: "generic"}
	want := "[42]qnib/alpine:latest#generic"
	if got := s.String(); got != want {
		t.Errorf("Slot.String() = %q, want %q", got, want)
	}
}
