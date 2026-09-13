/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gtsid

import "testing"

// ── OP#1: Validate ─────────────────────────────────────────────────────

func TestValidateGtsID_Valid(t *testing.T) {
	r := Validate("gts.x.core.ns.type.v1~")
	if !r.Valid {
		t.Fatalf("expected valid, got error: %s", r.Error)
	}
	if !r.IsType {
		t.Error("expected is_type=true for type ID")
	}
	if r.IsWildcard {
		t.Error("expected is_wildcard=false")
	}
	if r.ID != "gts.x.core.ns.type.v1~" {
		t.Errorf("ID echo: %s", r.ID)
	}
}

func TestValidateGtsID_Instance(t *testing.T) {
	r := Validate("gts.x.core.ns.type.v1~x.vendor.ns.inst.v1")
	if !r.Valid {
		t.Fatalf("expected valid, got error: %s", r.Error)
	}
	if r.IsType {
		t.Error("expected is_type=false for instance ID")
	}
}

func TestValidateGtsID_Invalid(t *testing.T) {
	r := Validate("not-a-gts-id")
	if r.Valid {
		t.Fatal("expected invalid")
	}
	if r.Error == "" {
		t.Error("expected error message")
	}
}

func TestValidateGtsID_Wildcard_Valid(t *testing.T) {
	r := Validate("gts.x.core.*")
	if !r.Valid {
		t.Fatalf("expected valid wildcard, got: %s", r.Error)
	}
	if !r.IsWildcard {
		t.Error("expected is_wildcard=true")
	}
}

func TestValidateGtsID_Wildcard_Invalid(t *testing.T) {
	r := Validate("gts.*invalid*")
	if r.Valid {
		t.Fatal("expected invalid wildcard")
	}
	if r.Error == "" {
		t.Error("expected error message")
	}
}

// ── OP#3: Parse ────────────────────────────────────────────────────────

func TestParse_Valid(t *testing.T) {
	r := Parse("gts.x.core.ns.type.v1.0~")
	if !r.OK {
		t.Fatalf("expected OK, got error: %s", r.Error)
	}
	if len(r.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(r.Segments))
	}
	seg := r.Segments[0]
	if seg.Vendor != "x" || seg.Package != "core" || seg.Namespace != "ns" || seg.Type != "type" {
		t.Errorf("unexpected segment: %+v", seg)
	}
	if seg.VerMajor != 1 || seg.VerMinor == nil || *seg.VerMinor != 0 {
		t.Errorf("unexpected version: major=%d minor=%v", seg.VerMajor, seg.VerMinor)
	}
}

func TestParse_Invalid(t *testing.T) {
	r := Parse("not-valid")
	if r.OK {
		t.Fatal("expected not OK")
	}
	if r.Error == "" {
		t.Error("expected error")
	}
}

func TestParse_Wildcard(t *testing.T) {
	r := Parse("gts.x.core.*")
	if !r.OK {
		t.Fatalf("expected OK, got error: %s", r.Error)
	}
	if !r.IsWildcard {
		t.Error("expected is_wildcard=true")
	}
}

func TestParse_WildcardInvalid(t *testing.T) {
	r := Parse("gts.*bad*")
	if r.OK {
		t.Fatal("expected not OK for invalid wildcard")
	}
}

func TestParse_Chained(t *testing.T) {
	r := Parse("gts.x.core.ns.base.v1~x.ext.ns.derived.v1~")
	if !r.OK {
		t.Fatalf("expected OK, got error: %s", r.Error)
	}
	if len(r.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(r.Segments))
	}
	if !r.IsType {
		t.Error("expected is_type=true for chained type ID")
	}
}

// ── OP#5: IDToUUID ──────────────────────────────────────────────────────────

func TestIDToUUID_Valid(t *testing.T) {
	r := IDToUUID("gts.x.core.ns.type.v1~")
	if r.Error != "" {
		t.Fatalf("expected no error, got: %s", r.Error)
	}
	if r.UUID == "" {
		t.Fatal("expected UUID")
	}
	if r.ID != "gts.x.core.ns.type.v1~" {
		t.Errorf("ID echo: %s", r.ID)
	}
}

func TestIDToUUID_Invalid(t *testing.T) {
	r := IDToUUID("not-valid")
	if r.Error == "" {
		t.Fatal("expected error")
	}
	if r.UUID != "" {
		t.Error("expected empty UUID on error")
	}
}

func TestIDToUUID_Deterministic(t *testing.T) {
	r1 := IDToUUID("gts.x.core.ns.type.v1~")
	r2 := IDToUUID("gts.x.core.ns.type.v1~")
	if r1.UUID != r2.UUID {
		t.Errorf("UUID not deterministic: %s != %s", r1.UUID, r2.UUID)
	}
}
