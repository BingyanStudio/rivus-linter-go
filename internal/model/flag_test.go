package model

import "testing"

func TestFlagSetAddHas(t *testing.T) {
	var s FlagSet
	s = s.Add(FlagPanic)
	s = s.Add(FlagIO)

	if !s.Has(FlagPanic) {
		t.Error("expected FlagPanic")
	}
	if !s.Has(FlagIO) {
		t.Error("expected FlagIO")
	}
	if s.Has(FlagGoroutine) {
		t.Error("unexpected FlagGoroutine")
	}
}

func TestFlagSetUnion(t *testing.T) {
	a := FlagSet(0).Add(FlagPanic).Add(FlagIO)
	b := FlagSet(0).Add(FlagGoroutine).Add(FlagIO)
	c := a.Union(b)

	if !c.Has(FlagPanic) {
		t.Error("expected FlagPanic in union")
	}
	if !c.Has(FlagGoroutine) {
		t.Error("expected FlagGoroutine in union")
	}
	if !c.Has(FlagIO) {
		t.Error("expected FlagIO in union")
	}
}

func TestFlagSetString(t *testing.T) {
	s := FlagSet(0).Add(FlagPanic).Add(FlagIO)
	got := s.String()
	if got != "I, P" {
		t.Errorf("expected 'I, P', got %q", got)
	}
}

func TestFlagTypeString(t *testing.T) {
	tests := []struct {
		f    FlagType
		want string
	}{
		{FlagPanic, "P"},
		{FlagGoroutine, "G"},
		{FlagContext, "D"},
		{FlagIO, "I"},
		{FlagSideEffect, "S"},
		{FlagUnsafe, "U"},
		{FlagTime, "T"},
		{FlagExit, "X"},
		{FlagReflect, "R"},
		{FlagCGO, "C"},
	}
	for _, tt := range tests {
		if got := tt.f.String(); got != tt.want {
			t.Errorf("FlagType(%d).String() = %q, want %q", tt.f, got, tt.want)
		}
	}
}
