/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import "testing"

// ── OP#1: ValidateGtsID ─────────────────────────────────────────────────────

func TestValidateGtsID_Valid(t *testing.T) {
	r := ValidateGtsID("gts.x.core.ns.type.v1~")
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
	r := ValidateGtsID("gts.x.core.ns.type.v1~x.vendor.ns.inst.v1")
	if !r.Valid {
		t.Fatalf("expected valid, got error: %s", r.Error)
	}
	if r.IsType {
		t.Error("expected is_type=false for instance ID")
	}
}

func TestValidateGtsID_Invalid(t *testing.T) {
	r := ValidateGtsID("not-a-gts-id")
	if r.Valid {
		t.Fatal("expected invalid")
	}
	if r.Error == "" {
		t.Error("expected error message")
	}
}

func TestValidateGtsID_Wildcard_Valid(t *testing.T) {
	r := ValidateGtsID("gts.x.core.*")
	if !r.Valid {
		t.Fatalf("expected valid wildcard, got: %s", r.Error)
	}
	if !r.IsWildcard {
		t.Error("expected is_wildcard=true")
	}
}

func TestValidateGtsID_Wildcard_Invalid(t *testing.T) {
	r := ValidateGtsID("gts.*invalid*")
	if r.Valid {
		t.Fatal("expected invalid wildcard")
	}
	if r.Error == "" {
		t.Error("expected error message")
	}
}

// ── OP#2: ExtractGtsID ──────────────────────────────────────────────────────

func TestExtractGtsID_Schema(t *testing.T) {
	content := map[string]any{
		"$id":     "gts://gts.x.core.ns.type.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
	}
	r := ExtractGtsID(content, DefaultGtsConfig())
	if r.ID != "gts.x.core.ns.type.v1~" {
		t.Errorf("expected gts.x.core.ns.type.v1~, got %s", r.ID)
	}
	if !r.IsTypeSchema {
		t.Error("expected IsTypeSchema=true")
	}
}

func TestExtractGtsID_Instance(t *testing.T) {
	content := map[string]any{
		"id":      "gts.x.core.ns.type.v1~x.vendor.ns.inst.v1",
		"type_id": "gts.x.core.ns.type.v1~",
	}
	r := ExtractGtsID(content, DefaultGtsConfig())
	if r.ID != "gts.x.core.ns.type.v1~x.vendor.ns.inst.v1" {
		t.Errorf("expected instance ID, got %s", r.ID)
	}
	if r.IsTypeSchema {
		t.Error("expected IsTypeSchema=false for instance")
	}
}

func TestExtractGtsID_NoID(t *testing.T) {
	content := map[string]any{"foo": "bar"}
	r := ExtractGtsID(content, DefaultGtsConfig())
	if r.ID != "" {
		t.Errorf("expected empty ID for content without GTS ID, got %s", r.ID)
	}
}

// ── OP#3: ParseGtsID ────────────────────────────────────────────────────────

func TestParseGtsID_Valid(t *testing.T) {
	r := ParseGtsID("gts.x.core.ns.type.v1.0~")
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

func TestParseGtsID_Invalid(t *testing.T) {
	r := ParseGtsID("not-valid")
	if r.OK {
		t.Fatal("expected not OK")
	}
	if r.Error == "" {
		t.Error("expected error")
	}
}

func TestParseGtsID_Wildcard(t *testing.T) {
	r := ParseGtsID("gts.x.core.*")
	if !r.OK {
		t.Fatalf("expected OK, got error: %s", r.Error)
	}
	if !r.IsWildcard {
		t.Error("expected is_wildcard=true")
	}
}

func TestParseGtsID_WildcardInvalid(t *testing.T) {
	r := ParseGtsID("gts.*bad*")
	if r.OK {
		t.Fatal("expected not OK for invalid wildcard")
	}
}

func TestParseGtsID_Chained(t *testing.T) {
	r := ParseGtsID("gts.x.core.ns.base.v1~x.ext.ns.derived.v1~")
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
