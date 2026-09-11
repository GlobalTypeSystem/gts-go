/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"strings"
	"testing"
)

// ── validate.go coverage ────────────────────────────────────────────────────

func TestValidateInstance_NotFound_Extra(t *testing.T) {
	store := NewGtsStore(nil)
	r := store.ValidateInstance("gts.x.test.ns.type.v1~x.test.ns.inst.v1")
	if r.OK {
		t.Fatal("expected !ok for missing instance")
	}
	if !strings.Contains(r.Error, "not found") {
		t.Errorf("expected 'not found' in error: %s", r.Error)
	}
}

func TestValidateInstance_NoTypeID(t *testing.T) {
	store := NewGtsStore(nil)
	// Register an entity without a type_id (a plain object with no schema association)
	entity := &JsonEntity{
		Content: map[string]any{"name": "test"},
	}
	entity.GtsID = &GtsID{ID: "some-raw-id"}
	store.byID["some-raw-id"] = entity
	r := store.ValidateInstance("some-raw-id")
	if r.OK {
		t.Fatal("expected !ok for entity without type_id")
	}
}

func TestValidateInstance_SchemaNotFound(t *testing.T) {
	store := NewGtsStore(nil)
	entity := &JsonEntity{
		Content: map[string]any{"name": "test"},
		TypeID:  "gts.x.missing.schema.v1~",
	}
	entity.GtsID = &GtsID{ID: "gts.x.test.ns.type.v1~x.test.ns.inst.v1"}
	store.byID["gts.x.test.ns.type.v1~x.test.ns.inst.v1"] = entity
	r := store.ValidateInstance("gts.x.test.ns.type.v1~x.test.ns.inst.v1")
	if r.OK {
		t.Fatal("expected !ok for missing schema")
	}
	if !strings.Contains(r.Error, "not found") {
		t.Errorf("expected 'not found' in error: %s", r.Error)
	}
}

func TestValidateInstance_AbstractTypeRejected(t *testing.T) {
	store := NewGtsStore(nil)
	// Register an abstract schema
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.abstract.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "x-gts-abstract": true,
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
		"required":   []any{"name"},
	})
	// Register an instance referencing the abstract type
	inst := map[string]any{
		"id": "gts.x.test.ns.abstract.v1~x.test.ns.inst.v1", "type": "gts.x.test.ns.abstract.v1~",
		"name": "test",
	}
	instEntity := NewJsonEntity(inst, DefaultGtsConfig())
	_ = store.Register(instEntity)

	r := store.ValidateInstance("gts.x.test.ns.abstract.v1~x.test.ns.inst.v1")
	if r.OK {
		t.Fatal("expected !ok for abstract type instance")
	}
	if !strings.Contains(r.Error, "abstract") {
		t.Errorf("expected 'abstract' in error: %s", r.Error)
	}
}

func TestValidateInstance_Valid(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.type.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}},
		"required": []any{"name"},
	})
	inst := map[string]any{
		"id": "gts.x.test.ns.type.v1~x.test.ns.inst.v1", "type": "gts.x.test.ns.type.v1~",
		"name": "hello",
	}
	instEntity := NewJsonEntity(inst, DefaultGtsConfig())
	_ = store.Register(instEntity)

	r := store.ValidateInstance("gts.x.test.ns.type.v1~x.test.ns.inst.v1")
	if !r.OK {
		t.Fatalf("expected OK, got error: %s", r.Error)
	}
}

func TestValidateInstance_Invalid(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.type.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}},
		"required": []any{"name"},
	})
	inst := map[string]any{
		"id": "gts.x.test.ns.type.v1~x.test.ns.inst.v1", "type": "gts.x.test.ns.type.v1~",
		// missing required "name"
	}
	instEntity := NewJsonEntity(inst, DefaultGtsConfig())
	_ = store.Register(instEntity)

	r := store.ValidateInstance("gts.x.test.ns.type.v1~x.test.ns.inst.v1")
	if r.OK {
		t.Fatal("expected !ok for invalid instance")
	}
}

// ── parse.go coverage ───────────────────────────────────────────────────────

func TestParseID_ValidType(t *testing.T) {
	r := ParseID("gts.x.core.ns.type.v1~")
	if !r.OK {
		t.Fatalf("expected OK: %s", r.Error)
	}
	if !r.IsType {
		t.Error("expected is_type")
	}
	if len(r.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(r.Segments))
	}
	s := r.Segments[0]
	if s.Vendor != "x" || s.Package != "core" || s.Namespace != "ns" || s.Type != "type" {
		t.Errorf("segment fields: %+v", s)
	}
}

func TestParseID_ValidInstance(t *testing.T) {
	r := ParseID("gts.x.core.ns.type.v1~x.ext.ns.inst.v1")
	if !r.OK {
		t.Fatalf("expected OK: %s", r.Error)
	}
	if r.IsType {
		t.Error("expected not is_type for instance")
	}
	if len(r.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(r.Segments))
	}
}

func TestParseID_Invalid(t *testing.T) {
	r := ParseID("garbage")
	if r.OK {
		t.Fatal("expected !ok")
	}
	if r.Error == "" {
		t.Error("expected error message")
	}
}

func TestParseID_WildcardValid(t *testing.T) {
	r := ParseID("gts.x.core.ns.*")
	if !r.OK {
		t.Fatalf("expected OK: %s", r.Error)
	}
	if !r.IsWildcard {
		t.Error("expected is_wildcard")
	}
}

func TestParseID_WildcardTypePattern(t *testing.T) {
	r := ParseID("gts.x.core.ns.type.v1~*")
	if !r.OK {
		t.Fatalf("expected OK: %s", r.Error)
	}
	if !r.IsType {
		t.Error("expected is_type for ~* pattern")
	}
}

func TestParseID_WildcardInvalid(t *testing.T) {
	r := ParseID("gts.x.*bad*")
	if r.OK {
		t.Fatal("expected !ok for invalid wildcard")
	}
}

// ── extract.go / EffectiveID coverage ───────────────────────────────────────

func TestEffectiveID_FromGtsID(t *testing.T) {
	e := &JsonEntity{GtsID: &GtsID{ID: "gts.x.test.v1~"}}
	if e.EffectiveID() != "gts.x.test.v1~" {
		t.Errorf("expected GTS ID: %s", e.EffectiveID())
	}
}

func TestEffectiveID_FromRawField(t *testing.T) {
	e := &JsonEntity{
		Content:             map[string]any{"id": "some-uuid"},
		SelectedEntityField: "id",
	}
	if e.EffectiveID() != "some-uuid" {
		t.Errorf("expected raw id: %s", e.EffectiveID())
	}
}

func TestEffectiveID_FromFile(t *testing.T) {
	e := &JsonEntity{
		File: &JsonFile{Path: "/some/path.json", Name: "path.json"},
	}
	if e.EffectiveID() != "/some/path.json" {
		t.Errorf("expected file path: %s", e.EffectiveID())
	}
}

func TestEffectiveID_FromFileWithSequence(t *testing.T) {
	seq := 3
	e := &JsonEntity{
		File:         &JsonFile{Path: "/some/path.json", Name: "path.json"},
		ListSequence: &seq,
	}
	if e.EffectiveID() != "/some/path.json#3" {
		t.Errorf("expected file#seq: %s", e.EffectiveID())
	}
}

func TestEffectiveID_Empty(t *testing.T) {
	e := &JsonEntity{}
	if e.EffectiveID() != "" {
		t.Errorf("expected empty: %s", e.EffectiveID())
	}
}

func TestEffectiveID_SkipTypeSchemaForRawField(t *testing.T) {
	e := &JsonEntity{
		IsTypeSchema:        true,
		Content:             map[string]any{"id": "some-value"},
		SelectedEntityField: "id",
	}
	// For type schemas, raw field is not used (IsTypeSchema && SelectedEntityField)
	if e.EffectiveID() != "" {
		t.Errorf("expected empty for type schema without GtsID: %s", e.EffectiveID())
	}
}

// ── schema_compat.go coverage ───────────────────────────────────────────────

func TestSchemaCompat_CheckMultipleOf(t *testing.T) {
	base := map[string]any{"multipleOf": float64(2)}
	derived := map[string]any{"multipleOf": float64(6)}
	errs := checkMultipleOf(base, derived, "prop")
	if len(errs) != 0 {
		t.Errorf("6 is a multiple of 2: %v", errs)
	}

	derived2 := map[string]any{"multipleOf": float64(3)}
	errs2 := checkMultipleOf(base, derived2, "prop")
	if len(errs2) == 0 {
		t.Error("3 is not a multiple of 2, expected error")
	}

	// Derived omits multipleOf
	errs3 := checkMultipleOf(base, map[string]any{}, "prop")
	if len(errs3) == 0 {
		t.Error("missing multipleOf should error")
	}

	// Base has no multipleOf → no check
	errs4 := checkMultipleOf(map[string]any{}, derived, "prop")
	if len(errs4) != 0 {
		t.Errorf("no base multipleOf → no error: %v", errs4)
	}
}

func TestSchemaCompat_JsonValueType(t *testing.T) {
	tests := []struct {
		val  any
		want string
	}{
		{"hello", "string"},
		{true, "boolean"},
		{float64(1), "number"},
		{[]any{}, "array"},
		{map[string]any{}, "object"},
		{nil, ""},
	}
	for _, tt := range tests {
		got := jsonValueType(tt.val)
		if got != tt.want {
			t.Errorf("jsonValueType(%v): want %q, got %q", tt.val, tt.want, got)
		}
	}
}

func TestSchemaCompat_ValueTypeCompatible(t *testing.T) {
	numSet := map[string]bool{"number": true}
	intSet := map[string]bool{"integer": true}
	if !valueTypeCompatible("integer", numSet) {
		t.Error("integer should be compatible with number set")
	}
	if !valueTypeCompatible("number", intSet) {
		t.Error("number should be compatible with integer set")
	}
	if valueTypeCompatible("string", numSet) {
		t.Error("string should not be compatible with number set")
	}
}

func TestSchemaCompat_CheckTypeCompatibility_NoBaseType(t *testing.T) {
	errs := checkTypeCompatibility(map[string]any{}, map[string]any{"type": "string"}, "prop")
	if len(errs) != 0 {
		t.Errorf("no base type → no error: %v", errs)
	}
}

func TestSchemaCompat_CheckTypeCompatibility_DerivedOmitsWithConst(t *testing.T) {
	base := map[string]any{"type": "string"}
	derived := map[string]any{"const": "hello"}
	errs := checkTypeCompatibility(base, derived, "prop")
	if len(errs) != 0 {
		t.Errorf("const string is compatible with base type string: %v", errs)
	}
}

func TestSchemaCompat_CheckTypeCompatibility_DerivedOmitsWithEnum(t *testing.T) {
	base := map[string]any{"type": "string"}
	derived := map[string]any{"enum": []any{"a", "b"}}
	errs := checkTypeCompatibility(base, derived, "prop")
	if len(errs) != 0 {
		t.Errorf("string enum is compatible with base type string: %v", errs)
	}
}

func TestSchemaCompat_CheckTypeCompatibility_DerivedOmitsNoConstOrEnum(t *testing.T) {
	base := map[string]any{"type": "string"}
	derived := map[string]any{"minLength": float64(1)}
	errs := checkTypeCompatibility(base, derived, "prop")
	if len(errs) == 0 {
		t.Error("derived omits type without const/enum → should error")
	}
}

// ── cast_compat.go coverage ─────────────────────────────────────────────────

func TestCheckStructuralCompatibility_EnumChanges(t *testing.T) {
	// Note: the structural diff checker uses consumer-model semantics:
	//   backward (checkBackward=true): flags NEW enum values (values in new not in old)
	//   forward  (checkBackward=false): flags REMOVED enum values (values in old not in new)

	old := map[string]any{
		"type": "object", "required": []any{"status"},
		"properties": map[string]any{
			"status": map[string]any{"type": "string", "enum": []any{"a", "b", "c"}},
		},
	}
	newSchema := map[string]any{
		"type": "object", "required": []any{"status"},
		"properties": map[string]any{
			"status": map[string]any{"type": "string", "enum": []any{"a", "b"}},
		},
	}

	// old=[a,b,c] → new=[a,b]: no new values added → backward OK
	ok, _ := checkBackwardCompatibility(old, newSchema)
	if !ok {
		t.Error("backward should be OK: no new enum values added in new")
	}

	// old=[a,b,c] → new=[a,b]: "c" removed → forward ERROR
	ok2, errs2 := checkForwardCompatibility(old, newSchema)
	if ok2 {
		t.Error("forward should be ERROR: old had 'c' which new doesn't")
	}
	if len(errs2) == 0 {
		t.Error("expected forward errors")
	}

	// Reverse: old=[a,b] → new=[a,b,c]: "c" added → backward ERROR
	ok3, errs3 := checkBackwardCompatibility(newSchema, old)
	if ok3 {
		t.Error("backward should be ERROR: new added 'c'")
	}
	if len(errs3) == 0 {
		t.Error("expected backward errors")
	}

	// Reverse: old=[a,b] → new=[a,b,c]: no values removed → forward OK
	ok4, _ := checkForwardCompatibility(newSchema, old)
	if !ok4 {
		t.Error("forward should be OK: no old enum values removed")
	}
}

func TestFlattenSchemaPreservesAllOfConstraints(t *testing.T) {
	flat := flattenSchema(map[string]any{
		"allOf": []any{
			map[string]any{"type": "object", "properties": map[string]any{"status": map[string]any{"enum": []any{"active"}}}},
			map[string]any{"properties": map[string]any{"status": map[string]any{"minLength": float64(3)}, "items": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}}},
		},
	})
	status := getMap(getPropertiesMap(flat), "status")
	if status == nil || !anySliceContains(status["enum"].([]any), "active") || getNumber(status, "minLength") == nil {
		t.Errorf("allOf property constraints were not preserved: %v", flat)
	}
	items := getMap(getPropertiesMap(flat), "items")
	if items == nil || getMap(items, "items") == nil {
		t.Errorf("allOf array item constraints were not preserved: %v", flat)
	}
}

func TestStructuralEnumAdditionRemoval(t *testing.T) {
	withoutEnum := map[string]any{"properties": map[string]any{"status": map[string]any{"type": "string"}}}
	withEnum := map[string]any{"properties": map[string]any{"status": map[string]any{"type": "string", "enum": []any{float64(1), true}}}}
	if compatible, _ := checkBackwardCompatibility(withoutEnum, withEnum); compatible {
		t.Error("adding an enum must be backward-incompatible")
	}
	if compatible, _ := checkForwardCompatibility(withEnum, withoutEnum); compatible {
		t.Error("removing an enum must be forward-incompatible")
	}
}

func TestCheckInclusion_TypeLessObjectAndArray(t *testing.T) {
	objectOld := map[string]any{"required": []any{"id"}}
	objectNew := map[string]any{"required": []any{"id", "source"}}
	if result := checkInclusion(objectOld, objectNew); result == nil || *result {
		t.Error("type-less required property addition must reject old objects")
	}
	arrayOld := map[string]any{"items": map[string]any{"type": "string"}}
	arrayNew := map[string]any{"items": map[string]any{"type": "string", "maxLength": float64(10)}}
	if result := checkInclusion(arrayOld, arrayNew); result == nil || *result {
		t.Error("type-less item constraint must reject old arrays")
	}
}

func TestCheckMinMaxConstraint_AllBranches(t *testing.T) {
	// Backward: increase minimum → tighten
	old := map[string]any{"minimum": float64(0), "maximum": float64(100)}
	newS := map[string]any{"minimum": float64(10), "maximum": float64(100)}
	errs := checkMinMaxConstraint("prop", old, newS, "minimum", "maximum", true)
	if len(errs) == 0 {
		t.Error("increasing minimum should be an error for backward")
	}

	// Backward: add minimum where none existed
	errs2 := checkMinMaxConstraint(
		"prop",
		map[string]any{},
		map[string]any{"minimum": float64(10)},
		"minimum",
		"maximum",
		true,
	)
	if len(errs2) == 0 {
		t.Error("adding minimum constraint should be an error for backward")
	}

	// Forward: decrease minimum → relax
	errs3 := checkMinMaxConstraint("prop", newS, old, "minimum", "maximum", false)
	if len(errs3) == 0 {
		t.Error("decreasing minimum should be an error for forward")
	}

	// Forward: remove minimum
	errs4 := checkMinMaxConstraint("prop", old, map[string]any{"maximum": float64(100)}, "minimum", "maximum", false)
	if len(errs4) == 0 {
		t.Error("removing minimum should be an error for forward")
	}

	// Backward: decrease maximum → tighten
	oldMax := map[string]any{"maximum": float64(100)}
	newMax := map[string]any{"maximum": float64(50)}
	errs5 := checkMinMaxConstraint("prop", oldMax, newMax, "minimum", "maximum", true)
	if len(errs5) == 0 {
		t.Error("decreasing maximum should be an error for backward")
	}

	// Backward: add maximum where none existed
	errs6 := checkMinMaxConstraint("prop", map[string]any{}, newMax, "minimum", "maximum", true)
	if len(errs6) == 0 {
		t.Error("adding maximum constraint should be an error for backward")
	}

	// Forward: increase maximum → relax
	errs7 := checkMinMaxConstraint("prop", newMax, oldMax, "minimum", "maximum", false)
	if len(errs7) == 0 {
		t.Error("increasing maximum should be an error for forward")
	}

	// Forward: remove maximum
	errs8 := checkMinMaxConstraint("prop", oldMax, map[string]any{}, "minimum", "maximum", false)
	if len(errs8) == 0 {
		t.Error("removing maximum should be an error for forward")
	}
}

// ── schema_traits.go / ValidateEntity coverage ──────────────────────────────

func TestValidateEntity_SchemaType(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.type.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}},
	})

	r := store.ValidateEntity("gts.x.test.ns.type.v1~")
	if !r.OK {
		t.Fatalf("expected OK for schema: %s", r.Error)
	}
	if r.EntityType != "schema" {
		t.Errorf("expected entity_type='schema', got %q", r.EntityType)
	}
}

func TestValidateEntity_InstanceType(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.type.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}},
		"required": []any{"name"},
	})
	inst := map[string]any{
		"id": "gts.x.test.ns.type.v1~x.test.ns.inst.v1", "type": "gts.x.test.ns.type.v1~",
		"name": "hello",
	}
	instEntity := NewJsonEntity(inst, DefaultGtsConfig())
	_ = store.Register(instEntity)

	r := store.ValidateEntity("gts.x.test.ns.type.v1~x.test.ns.inst.v1")
	if !r.OK {
		t.Fatalf("expected OK for instance: %s", r.Error)
	}
	if r.EntityType != "instance" {
		t.Errorf("expected entity_type='instance', got %q", r.EntityType)
	}
}

func TestValidateEntity_NotFound_Extra(t *testing.T) {
	store := NewGtsStore(nil)
	r := store.ValidateEntity("gts.x.missing.v1~")
	if r.OK {
		t.Fatal("expected !ok")
	}
}

// ── GtsID / IsWildcard coverage ─────────────────────────────────────────────

func TestGtsID_IsWildcard(t *testing.T) {
	id, err := NewGtsID("gts.x.core.ns.type.v1~")
	if err != nil {
		t.Fatal(err)
	}
	if id.IsWildcard() {
		t.Error("regular ID should not be wildcard")
	}
}

// ── GTS ID Error types ──────────────────────────────────────────────────────

func TestInvalidGtsIDError(t *testing.T) {
	e := &InvalidGtsIDError{GtsID: "bad", Cause: "reason"}
	if !strings.Contains(e.Error(), "bad") || !strings.Contains(e.Error(), "reason") {
		t.Errorf("error: %s", e.Error())
	}
	e2 := &InvalidGtsIDError{GtsID: "bad"}
	if !strings.Contains(e2.Error(), "bad") {
		t.Errorf("error: %s", e2.Error())
	}
}

func TestInvalidSegmentError(t *testing.T) {
	e := &InvalidSegmentError{Num: 1, Offset: 4, Segment: "bad", Cause: "reason"}
	if !strings.Contains(e.Error(), "bad") || !strings.Contains(e.Error(), "reason") {
		t.Errorf("error: %s", e.Error())
	}
	e2 := &InvalidSegmentError{Num: 1, Offset: 4, Segment: "bad"}
	if !strings.Contains(e2.Error(), "bad") {
		t.Errorf("error: %s", e2.Error())
	}
}

// ── schema_traits.go mergeRFC7396Recursive coverage ─────────────────────────

func TestMergeRFC7396Recursive(t *testing.T) {
	base := map[string]any{
		"a": "base",
		"b": map[string]any{"nested": "original", "keep": "yes"},
		"c": "stays",
	}
	patch := map[string]any{
		"a": "patched",
		"b": map[string]any{"nested": "updated"},
		"d": "added",
	}
	// Apply merge
	mergeRFC7396Into(base, patch)
	if base["a"] != "patched" {
		t.Errorf("a: %v", base["a"])
	}
	nested := base["b"].(map[string]any)
	if nested["nested"] != "updated" {
		t.Errorf("b.nested: %v", nested["nested"])
	}
	if nested["keep"] != "yes" {
		t.Errorf("b.keep should be preserved: %v", nested["keep"])
	}
	if base["c"] != "stays" {
		t.Errorf("c: %v", base["c"])
	}
	if base["d"] != "added" {
		t.Errorf("d: %v", base["d"])
	}
}

func TestMergeRFC7396Recursive_NullRemoval(t *testing.T) {
	base := map[string]any{"a": "val", "b": "val2"}
	patch := map[string]any{"a": nil}
	mergeRFC7396Into(base, patch)
	if _, ok := base["a"]; ok {
		t.Error("a should be removed by null patch")
	}
	if base["b"] != "val2" {
		t.Error("b should be preserved")
	}
}
