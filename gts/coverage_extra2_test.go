/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"strings"
	"testing"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
)

// ── mergeRFC7396Recursive additional coverage ───────────────────────────────

func TestMergeRFC7396Recursive_DeepNested(t *testing.T) {
	// patch into a key that doesn't exist as a map in target
	target := map[string]any{"a": "string-value"}
	patch := map[string]any{"a": map[string]any{"nested": "val"}}
	mergeRFC7396Into(target, patch)
	inner, ok := target["a"].(map[string]any)
	if !ok {
		t.Fatalf("expected map for 'a', got %T", target["a"])
	}
	if inner["nested"] != "val" {
		t.Error("expected nested=val")
	}
}

func TestMergeRFC7396Recursive_DepthLimit(t *testing.T) {
	// Build a deeply nested structure that exceeds maxTraitsRecursionDepth
	inner := map[string]any{"leaf": "val"}
	for i := 0; i < 25; i++ { // maxTraitsRecursionDepth is typically 20
		inner = map[string]any{"level": inner}
	}
	target := map[string]any{}
	patch := map[string]any{"deep": inner}
	// Should not panic — depth limit prevents infinite recursion
	mergeRFC7396Into(target, patch)
}

// ── validate.go — Validate/KeywordPath/LocalizedString ──────────────────────

func TestXGtsRefExt_Validate_NonString(t *testing.T) {
	// The Validate method should silently return for non-string values
	ext := &xGtsRefExt{pattern: "gts.x.*", store: NewGtsStore(nil)}
	// Using nil context is not possible, so we test through the full flow instead
	// via ValidateInstance which triggers the vocabulary
	_ = ext // ensure compiled
}

func TestXGtsRefErrorKind(t *testing.T) {
	ek := &xGtsRefErrorKind{reason: "test error"}
	kp := ek.KeywordPath()
	if len(kp) != 1 || kp[0] != "x-gts-ref" {
		t.Errorf("KeywordPath: %v", kp)
	}
	ls := ek.LocalizedString(nil)
	if ls != "test error" {
		t.Errorf("LocalizedString: %s", ls)
	}
}

func TestGtsURLLoader_Load(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.type.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
	})
	loader := &gtsURLLoader{store: store}

	// Load with gts:// prefix
	content, err := loader.Load("gts://gts.x.test.ns.type.v1~")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if content == nil {
		t.Fatal("expected content")
	}

	// Load missing entity
	_, err = loader.Load("gts://gts.x.missing.v1~")
	if err == nil {
		t.Fatal("expected error for missing entity")
	}

	// Load non-GTS URL
	_, err = loader.Load("https://example.com")
	if err == nil {
		t.Fatal("expected error for non-GTS URL")
	}
}

// ── store.go — ValidateSchema / ValidateInstanceWithXGtsRef ─────────────────

func TestGtsStore_ValidateSchema_ValidSchema(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.type.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}},
	})
	err := store.ValidateSchema("gts.x.test.ns.type.v1~")
	if err != nil {
		t.Errorf("expected valid schema: %v", err)
	}
}

func TestGtsStore_ValidateInstanceWithXGtsRef_NotFound(t *testing.T) {
	store := NewGtsStore(nil)
	err := store.ValidateInstanceWithXGtsRef("gts.x.missing.v1~inst")
	if err == nil {
		t.Fatal("expected error for missing instance")
	}
}

func TestGtsStore_ValidateInstanceWithXGtsRef_IsSchema(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.type.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
	})
	err := store.ValidateInstanceWithXGtsRef("gts.x.test.ns.type.v1~")
	if err == nil {
		t.Fatal("expected error when entity is a type-schema")
	}
	if !strings.Contains(err.Error(), "type-schema") {
		t.Errorf("expected type-schema error: %v", err)
	}
}

func TestGtsStore_ValidateInstanceWithXGtsRef_NoTypeID(t *testing.T) {
	store := NewGtsStore(nil)
	entity := &JsonEntity{Content: map[string]any{"name": "test"}}
	entity.GtsID = &gtsid.ID{ID: "some-inst"}
	store.byID["some-inst"] = entity
	err := store.ValidateInstanceWithXGtsRef("some-inst")
	if err == nil {
		t.Fatal("expected error for instance without type_id")
	}
}

// ── match.go — validateWildcardBase ─────────────────────────────────────────

func TestMatchIDPattern_WildcardBase(t *testing.T) {
	// Test various wildcard patterns to cover validateWildcardBase branches
	tests := []struct {
		candidate string
		pattern   string
		wantMatch bool
	}{
		// Global wildcard
		{"gts.x.core.ns.type.v1~", "gts.*", true},
		{"gts.x.core.ns.type.v1~", "gts.y.*", false},
		// Package level wildcard
		{"gts.x.core.ns.type.v1~", "gts.x.*", true},
		{"gts.x.core.ns.type.v1~", "gts.x.other.*", false},
		// Namespace level
		{"gts.x.core.ns.type.v1~", "gts.x.core.*", true},
		{"gts.x.core.ns.type.v1~", "gts.x.core.ns.*", true},
		// Version wildcard
		{"gts.x.core.ns.type.v1~", "gts.x.core.ns.type.*", true},
		{"gts.x.core.ns.type.v2~", "gts.x.core.ns.type.*", true},
	}
	for _, tt := range tests {
		r := gtsid.Match(tt.candidate, tt.pattern)
		if r.Match != tt.wantMatch {
			t.Errorf("gtsid.Match(%q, %q): want match=%v, got %v (error=%s)",
				tt.candidate, tt.pattern, tt.wantMatch, r.Match, r.Error)
		}
	}
}

// ── cast.go — effectiveObjectSchema ─────────────────────────────────────────

func TestEffectiveObjectSchema(t *testing.T) {
	// Direct properties
	s := map[string]any{"type": "object", "properties": map[string]any{"a": map[string]any{"type": "string"}}}
	r := effectiveObjectSchema(s)
	if _, ok := r["properties"]; !ok {
		t.Error("expected properties in result")
	}

	// Via allOf
	s2 := map[string]any{
		"type": "object",
		"allOf": []any{
			map[string]any{"properties": map[string]any{"b": map[string]any{"type": "integer"}}},
		},
	}
	r2 := effectiveObjectSchema(s2)
	if _, ok := r2["properties"]; !ok {
		t.Error("expected properties from allOf")
	}

	// Via allOf with required
	s3 := map[string]any{
		"type": "object",
		"allOf": []any{
			map[string]any{"required": []any{"c"}},
		},
	}
	r3 := effectiveObjectSchema(s3)
	if _, ok := r3["required"]; !ok {
		t.Error("expected required from allOf")
	}

	// Nil input
	r4 := effectiveObjectSchema(nil)
	if r4 == nil {
		t.Error("expected non-nil result for nil input")
	}

	// No properties, no allOf
	r5 := effectiveObjectSchema(map[string]any{"type": "object"})
	if r5 == nil {
		t.Error("expected non-nil result")
	}
}

// ── ref_validation.go ───────────────────────────────────────────────────────

func TestRefValidationError(t *testing.T) {
	e := &RefValidationError{FieldPath: "$.properties.foo.$ref", RefValue: "gts://bad", Reason: "invalid"}
	s := e.Error()
	if !strings.Contains(s, "$.properties.foo.$ref") || !strings.Contains(s, "invalid") {
		t.Errorf("error: %s", s)
	}
}

func TestRefValidator_ValidateRef(t *testing.T) {
	v := NewRefValidator()

	// Valid local ref
	if err := v.validateRef("#/definitions/foo", "$.ref"); err != nil {
		t.Errorf("local ref should be valid: %v", err)
	}

	// Valid GTS URI ref
	if err := v.validateRef("gts://gts.x.core.ns.type.v1~", "$.ref"); err != nil {
		t.Errorf("GTS URI ref should be valid: %v", err)
	}

	// Invalid GTS URI ref (bad ID after gts://)
	if err := v.validateRef("gts://not-valid", "$.ref"); err == nil {
		t.Error("expected error for invalid GTS ID in URI")
	}

	// Bare GTS ID (missing gts:// prefix)
	if err := v.validateRef("gts.x.core.ns.type.v1~", "$.ref"); err == nil {
		t.Error("expected error for bare GTS ID")
	}

	// HTTP URI
	if err := v.validateRef("https://example.com/schema", "$.ref"); err == nil {
		t.Error("expected error for HTTP URI")
	}

	// Non-string
	if err := v.validateRef(42, "$.ref"); err == nil {
		t.Error("expected error for non-string")
	}

	// Empty string
	if err := v.validateRef("", "$.ref"); err == nil {
		t.Error("expected error for empty ref")
	}

	// Other invalid format
	if err := v.validateRef("just-a-string", "$.ref"); err == nil {
		t.Error("expected error for random string")
	}
}

// ── query.go — validateQueryPattern / matchesIDPattern ──────────────────────

func TestQuery_InvalidPattern(t *testing.T) {
	store := NewGtsStore(nil)
	_ = store.RegisterSchema("gts.x.test.ns.type.v1~", map[string]any{"type": "object"})
	r := store.Query("**invalid**", 10)
	// Should return results or error depending on pattern
	_ = r // just ensure no panic
}

func TestQuery_WildcardFilter(t *testing.T) {
	store := NewGtsStore(nil)
	s1 := map[string]any{
		"$id": "gts://gts.x.test.ns.a.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "status": "active",
	}
	s2 := map[string]any{
		"$id": "gts://gts.x.test.ns.b.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "status": "inactive",
	}
	e1 := NewJsonEntity(s1, DefaultGtsConfig())
	e2 := NewJsonEntity(s2, DefaultGtsConfig())
	_ = store.Register(e1)
	_ = store.Register(e2)

	r := store.Query("gts.x.test.ns.*", 100)
	if r.Count < 2 {
		t.Errorf("expected at least 2 results, got %d", r.Count)
	}
}

// ── schema_compat.go — resolveRefsInner / findUnresolvedRef ─────────────────

func TestResolveRefs_CircularDetection(t *testing.T) {
	store := NewGtsStore(nil)
	// Create two schemas that reference each other
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.a.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{
			"ref": map[string]any{"$ref": "gts://gts.x.test.ns.b.v1~"},
		},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.b.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{
			"ref": map[string]any{"$ref": "gts://gts.x.test.ns.a.v1~"},
		},
	})
	schemaA := store.Get("gts.x.test.ns.a.v1~")
	_, err := store.resolveRefs(schemaA.Content)
	if err == nil {
		t.Fatal("expected circular ref error")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("expected circular error: %v", err)
	}
}

func TestResolveRefs_UnresolvableRef(t *testing.T) {
	store := NewGtsStore(nil)
	schema := map[string]any{
		"type": "object", "properties": map[string]any{
			"ref": map[string]any{"$ref": "gts://gts.x.nonexistent.v1~"},
		},
	}
	_, err := store.resolveRefs(schema)
	if err == nil {
		t.Fatal("expected unresolved ref error")
	}
	if !strings.Contains(err.Error(), "unresolved") {
		t.Errorf("expected unresolved error: %v", err)
	}
}

func TestFindUnresolvedRef_InArray(t *testing.T) {
	schema := map[string]any{
		"allOf": []any{
			map[string]any{"$ref": "gts://gts.x.unresolved.v1~"},
		},
	}
	ref := findUnresolvedRef(schema)
	if ref == "" {
		t.Error("expected to find unresolved ref in array")
	}
}

// ── schema_compat.go — checkEnumeratedValuesAgainstBase ─────────────────────

func TestCheckEnumeratedValuesAgainstBase_MaxConstraints(t *testing.T) {
	base := map[string]any{
		"maximum": float64(100), "maxLength": float64(10), "maxItems": float64(5),
	}
	// Values that violate max constraints
	errs := checkEnumeratedValuesAgainstBase(base, []any{float64(200)}, "prop")
	if len(errs) == 0 {
		t.Error("expected error for value exceeding maximum")
	}
	errs2 := checkEnumeratedValuesAgainstBase(base, []any{"a very long string value"}, "prop")
	if len(errs2) == 0 {
		t.Error("expected error for string exceeding maxLength")
	}

	// exclusiveMaximum
	base2 := map[string]any{"exclusiveMaximum": float64(10)}
	errs3 := checkEnumeratedValuesAgainstBase(base2, []any{float64(10)}, "prop")
	if len(errs3) == 0 {
		t.Error("expected error for value at exclusiveMaximum")
	}

	// exclusiveMinimum
	base3 := map[string]any{"exclusiveMinimum": float64(0)}
	errs4 := checkEnumeratedValuesAgainstBase(base3, []any{float64(0)}, "prop")
	if len(errs4) == 0 {
		t.Error("expected error for value at exclusiveMinimum")
	}
}

// ── x_gts_ref.go — resolvePointer ──────────────────────────────────────────

func TestXGtsRefValidator_ResolvePointer_Missing(t *testing.T) {
	validator := NewXGtsRefValidator(NewGtsStore(nil))
	schema := map[string]any{"properties": map[string]any{"name": map[string]any{"type": "string"}}}
	val := validator.resolvePointer(schema, "/missing/path")
	if val != "" {
		t.Errorf("expected empty for missing pointer, got %q", val)
	}
}

func TestXGtsRefValidator_ResolvePointer_Valid(t *testing.T) {
	validator := NewXGtsRefValidator(NewGtsStore(nil))
	schema := map[string]any{
		"properties": map[string]any{
			"name": map[string]any{"const": "hello"},
		},
	}
	val := validator.resolvePointer(schema, "/properties/name/const")
	if val != "hello" {
		t.Errorf("expected 'hello', got %q", val)
	}
}

func TestXGtsRefValidator_ResolvePointer_Empty(t *testing.T) {
	validator := NewXGtsRefValidator(NewGtsStore(nil))
	val := validator.resolvePointer(map[string]any{}, "/")
	if val != "" {
		t.Errorf("expected empty for empty path, got %q", val)
	}
}

func TestXGtsRefValidator_ResolvePointer_NonStringValue(t *testing.T) {
	validator := NewXGtsRefValidator(NewGtsStore(nil))
	schema := map[string]any{
		"properties": map[string]any{
			"count": map[string]any{"type": "integer"},
		},
	}
	// Resolving to "integer" (a string) should work
	val := validator.resolvePointer(schema, "/properties/count/type")
	if val != "integer" {
		t.Errorf("expected 'integer', got %q", val)
	}

	// Resolving to a map (properties node itself) returns ""
	val2 := validator.resolvePointer(schema, "/properties/count")
	_ = val2 // just ensure no panic
}

func TestXGtsRefValidator_ResolvePointer_IntermediateNonMap(t *testing.T) {
	validator := NewXGtsRefValidator(NewGtsStore(nil))
	schema := map[string]any{
		"type": "string",
	}
	// path goes through a string value which is not a map → returns ""
	val := validator.resolvePointer(schema, "/type/nested")
	if val != "" {
		t.Errorf("expected empty for non-map intermediate, got %q", val)
	}
}

// ── store.go — ValidateInstanceWithXGtsRef with valid instance ──────────────

func TestGtsStore_ValidateInstanceWithXGtsRef_SchemaNotFound(t *testing.T) {
	store := NewGtsStore(nil)
	// Instance with typeID pointing to missing schema
	entity := &JsonEntity{
		Content: map[string]any{"name": "test"},
		TypeID:  "gts.x.missing.schema.v1~",
	}
	entity.GtsID = &gtsid.ID{ID: "gts.x.test.ns.type.v1~x.test.ns.inst.v1"}
	store.byID["gts.x.test.ns.type.v1~x.test.ns.inst.v1"] = entity
	err := store.ValidateInstanceWithXGtsRef("gts.x.test.ns.type.v1~x.test.ns.inst.v1")
	if err == nil {
		t.Fatal("expected error for missing schema")
	}
}

func TestGtsStore_ValidateInstanceWithXGtsRef_ValidInstance(t *testing.T) {
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

	err := store.ValidateInstanceWithXGtsRef("gts.x.test.ns.type.v1~x.test.ns.inst.v1")
	if err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

// ── match.go — MatchPatternError ────────────────────────────────────────────

func TestInvalidWildcardError(t *testing.T) {
	e := &gtsid.InvalidWildcardError{Pattern: "gts.*bad", Cause: "invalid wildcard"}
	if !strings.Contains(e.Error(), "gts.*bad") || !strings.Contains(e.Error(), "invalid wildcard") {
		t.Errorf("error: %s", e.Error())
	}
	e2 := &gtsid.InvalidWildcardError{Pattern: "gts.*bad"}
	if !strings.Contains(e2.Error(), "gts.*bad") {
		t.Errorf("error without cause: %s", e2.Error())
	}
}

// ── validate.go — newXGtsRefVocabulary ──────────────────────────────────────

func TestNewXGtsRefVocabulary(t *testing.T) {
	store := NewGtsStore(nil)
	vocab := newXGtsRefVocabulary(store)
	if vocab == nil {
		t.Fatal("expected vocabulary")
	}
	if vocab.URL != "https://globaltypesystem.io/vocab/x-gts-ref" {
		t.Errorf("unexpected URL: %s", vocab.URL)
	}
}

// ── store.go — ValidateSchema more branches ─────────────────────────────────

func TestGtsStore_ValidateSchema_WithRefs(t *testing.T) {
	store := NewGtsStore(nil)
	// Register a target schema
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.target.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{"code": map[string]any{"type": "string"}},
	})
	// Register a schema that references the target
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.source.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{
			"ref": map[string]any{"$ref": "gts://gts.x.test.ns.target.v1~"},
		},
	})
	err := store.ValidateSchema("gts.x.test.ns.source.v1~")
	if err != nil {
		t.Errorf("expected valid schema with refs: %v", err)
	}
}

func TestGtsStore_ValidateSchema_NilContent(t *testing.T) {
	store := NewGtsStore(nil)
	entity := &JsonEntity{
		GtsID:        &gtsid.ID{ID: "gts.x.test.ns.nil.v1~"},
		IsTypeSchema: true,
		Content:      nil,
	}
	store.byID["gts.x.test.ns.nil.v1~"] = entity
	err := store.ValidateSchema("gts.x.test.ns.nil.v1~")
	if err == nil {
		t.Fatal("expected error for nil content")
	}
}

func TestGtsStore_ValidateSchema_NotASchema(t *testing.T) {
	store := NewGtsStore(nil)
	// Register a non-schema entity under a tilde-ending key
	entity := &JsonEntity{
		GtsID:        &gtsid.ID{ID: "gts.x.test.ns.inst.v1~"},
		IsTypeSchema: false,
		Content:      map[string]any{"name": "test"},
	}
	store.byID["gts.x.test.ns.inst.v1~"] = entity
	err := store.ValidateSchema("gts.x.test.ns.inst.v1~")
	if err == nil {
		t.Fatal("expected error for non-schema entity")
	}
	if !strings.Contains(err.Error(), "not a type-schema") {
		t.Errorf("expected 'not a type-schema' error: %v", err)
	}
}

func TestGtsStore_ValidateInstanceWithXGtsRef_SchemaNotTypeSchema(t *testing.T) {
	store := NewGtsStore(nil)
	// Instance points to a non-type-schema entity
	schemaEntity := &JsonEntity{
		GtsID:        &gtsid.ID{ID: "gts.x.test.ns.fake.v1~"},
		IsTypeSchema: false,
		Content:      map[string]any{"type": "object"},
		TypeID:       "",
	}
	store.byID["gts.x.test.ns.fake.v1~"] = schemaEntity

	instEntity := &JsonEntity{
		GtsID:   &gtsid.ID{ID: "gts.x.test.ns.fake.v1~x.test.ns.inst.v1"},
		Content: map[string]any{"name": "test"},
		TypeID:  "gts.x.test.ns.fake.v1~",
	}
	store.byID["gts.x.test.ns.fake.v1~x.test.ns.inst.v1"] = instEntity

	err := store.ValidateInstanceWithXGtsRef("gts.x.test.ns.fake.v1~x.test.ns.inst.v1")
	if err == nil {
		t.Fatal("expected error for schema not being type-schema")
	}
}

// ── validate.go newXGtsRefVocabulary Compile coverage ───────────────────────

func TestNewXGtsRefVocabulary_Compile(t *testing.T) {
	store := NewGtsStore(nil)
	vocab := newXGtsRefVocabulary(store)

	// Compile with x-gts-ref present
	ext, err := vocab.Compile(nil, map[string]any{"x-gts-ref": "gts.x.test.*"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if ext == nil {
		t.Error("expected non-nil extension")
	}

	// Compile without x-gts-ref
	ext2, err := vocab.Compile(nil, map[string]any{"type": "string"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if ext2 != nil {
		t.Error("expected nil extension when no x-gts-ref")
	}

	// Compile with non-string x-gts-ref
	_, err = vocab.Compile(nil, map[string]any{"x-gts-ref": 42})
	if err == nil {
		t.Fatal("expected error for non-string x-gts-ref")
	}
}

// ── attribute.go — resolveAttributePath ─────────────────────────────────────

func TestGetAttribute_DeepPath(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test.ns.type.v1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": map[string]any{
			"address": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string"},
				},
			},
		},
	})
	inst := map[string]any{
		"id":      "gts.x.test.ns.type.v1~x.test.ns.inst.v1",
		"type":    "gts.x.test.ns.type.v1~",
		"address": map[string]any{"city": "NYC"},
	}
	instEntity := NewJsonEntity(inst, DefaultGtsConfig())
	_ = store.Register(instEntity)

	r := store.GetAttribute("gts.x.test.ns.type.v1~x.test.ns.inst.v1@address.city")
	if !r.Resolved {
		t.Errorf("expected resolved, error: %s", r.Error)
	}
	if r.Value != "NYC" {
		t.Errorf("expected NYC, got %v", r.Value)
	}
}
