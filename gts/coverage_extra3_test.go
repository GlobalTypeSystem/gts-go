/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"strings"
	"testing"
)

// ── attribute.go — resolveAttributePath branches ────────────────────────────

func TestGetAttribute_NotFound(t *testing.T) {
	store := NewGtsStore(nil)
	r := store.GetAttribute("gts.x.nonexistent.v1~x.inst.v1@name")
	if r.Resolved {
		t.Error("expected not resolved for missing entity")
	}
}

func TestGetAttribute_NoPath(t *testing.T) {
	store := NewGtsStore(nil)
	r := store.GetAttribute("gts.x.test.v1~x.inst.v1")
	// Should handle a missing @ gracefully; exact behavior depends on the
	// implementation, so we only assert it does not panic.
	_ = r
}

func TestGetAttribute_MissingField(t *testing.T) {
	store := NewGtsStore(nil)
	inst := map[string]any{
		"id":   "gts.x.test.ns.type.v1~x.test.ns.inst.v1",
		"type": "gts.x.test.ns.type.v1~",
		"name": "hello",
	}
	instEntity := NewJsonEntity(inst, DefaultGtsConfig())
	_ = store.Register(instEntity)

	r := store.GetAttribute("gts.x.test.ns.type.v1~x.test.ns.inst.v1@nonexistent")
	if r.Resolved {
		t.Error("expected not resolved for missing field")
	}
	if len(r.AvailableFields) == 0 {
		t.Error("expected available fields to be listed")
	}
}

// ── store.go — ValidateSchema with GTS ref validation ───────────────────────

func TestGtsStore_ValidateSchema_WithGtsRefValidation(t *testing.T) {
	config := &RegistryConfig{ValidateGtsReferences: true}
	store := NewGtsStoreWithConfig(nil, config)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.target.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.source.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{
			"ref": map[string]any{"$ref": "gts://gts.x.test.ns.target.v1~"},
		},
	})
	err := store.ValidateSchema("gts.x.test.ns.source.v1~")
	if err != nil {
		t.Errorf("expected valid schema: %v", err)
	}
}

// ── validate.go — ValidateInstance with schema modifiers ────────────────────

func TestValidateInstance_WithSchemaModifiersInContent(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.type.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}},
	})
	// Instance with x-gts-final (schema-only keyword in instance content)
	inst := map[string]any{
		"id": "gts.x.test.ns.type.v1~x.test.ns.inst.v1", "type": "gts.x.test.ns.type.v1~",
		"name": "hello", "x-gts-final": true,
	}
	instEntity := NewJsonEntity(inst, DefaultGtsConfig())
	_ = store.Register(instEntity)

	r := store.ValidateInstance("gts.x.test.ns.type.v1~x.test.ns.inst.v1")
	if r.OK {
		t.Fatal("expected !ok when instance has schema-only modifier")
	}
	if !strings.Contains(r.Error, "x-gts-final") {
		t.Errorf("expected error about x-gts-final: %s", r.Error)
	}
}

// ── query.go — matchesIDPattern error paths ─────────────────────────────────

func TestQuery_ExactMatch_Extra(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.type.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
	})
	r := store.Query("gts.x.test.ns.type.v1~", 100)
	if r.Count != 1 {
		t.Errorf("expected 1 result for exact match, got %d", r.Count)
	}
}

func TestQuery_WithFilter(t *testing.T) {
	store := NewGtsStore(nil)
	s1 := map[string]any{
		"$id": "gts://gts.x.test.ns.a.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "status": "active",
	}
	s2 := map[string]any{
		"$id": "gts://gts.x.test.ns.b.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "status": "inactive",
	}
	_ = store.Register(NewJsonEntity(s1, DefaultGtsConfig()))
	_ = store.Register(NewJsonEntity(s2, DefaultGtsConfig()))

	r := store.Query("gts.x.test.ns.*[status=active]", 100)
	if r.Count != 1 {
		t.Errorf("expected exactly 1 filtered result, got %d (error: %s)", r.Count, r.Error)
	}
}

// ── schema_compat.go — stringSliceContains ──────────────────────────────────

func TestStringSliceContains(t *testing.T) {
	if !stringSliceContains([]string{"a", "b", "c"}, "b") {
		t.Error("expected true for present element")
	}
	if stringSliceContains([]string{"a", "b", "c"}, "d") {
		t.Error("expected false for absent element")
	}
	if stringSliceContains([]string{}, "a") {
		t.Error("expected false for empty slice")
	}
}

// ── schema_compat.go — checkEnumeratedValuesAgainstBase multipleOf ──────────

func TestCheckEnumeratedValuesAgainstBase_MultipleOf(t *testing.T) {
	base := map[string]any{"multipleOf": float64(3)}
	errs := checkEnumeratedValuesAgainstBase(base, []any{float64(7)}, "prop")
	if len(errs) == 0 {
		t.Error("expected error: 7 is not a multiple of 3")
	}
	errs2 := checkEnumeratedValuesAgainstBase(base, []any{float64(9)}, "prop")
	if len(errs2) != 0 {
		t.Errorf("9 is a multiple of 3, unexpected errors: %v", errs2)
	}
}

func TestCheckEnumeratedValuesAgainstBase_Pattern(t *testing.T) {
	base := map[string]any{"pattern": "^[a-z]+$"}
	errs := checkEnumeratedValuesAgainstBase(base, []any{"ABC"}, "prop")
	if len(errs) == 0 {
		t.Error("expected error: 'ABC' doesn't match ^[a-z]+$")
	}
	errs2 := checkEnumeratedValuesAgainstBase(base, []any{"abc"}, "prop")
	if len(errs2) != 0 {
		t.Errorf("'abc' matches pattern, unexpected errors: %v", errs2)
	}
}

// ── file_reader.go — NewGtsFileReader branches ──────────────────────────────

func TestNewGtsFileReader_EmptyPaths(t *testing.T) {
	reader := NewGtsFileReader([]string{}, DefaultGtsConfig())
	if reader == nil {
		t.Fatal("expected non-nil reader")
	}
	if reader.Next() != nil {
		t.Error("expected nil from empty reader")
	}
}

func TestNewGtsFileReader_NonexistentPath(t *testing.T) {
	reader := NewGtsFileReader([]string{"/nonexistent/path"}, DefaultGtsConfig())
	if reader == nil {
		t.Fatal("expected non-nil reader")
	}
	// Should handle gracefully
	if reader.Next() != nil {
		t.Error("expected nil for nonexistent path")
	}
}
