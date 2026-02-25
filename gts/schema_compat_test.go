/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"encoding/json"
	"strings"
	"testing"
)

// =============================================================================
// typeToSet
// =============================================================================

func TestTypeToSet(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  map[string]bool
	}{
		{"string", "string", map[string]bool{"string": true}},
		{"integer", "integer", map[string]bool{"integer": true}},
		{"slice two types", []any{"string", "null"}, map[string]bool{"string": true, "null": true}},
		{"slice one type", []any{"number"}, map[string]bool{"number": true}},
		{"unknown type int", 42, map[string]bool{}},
		{"nil", nil, map[string]bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typeToSet(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("missing key %q in result %v", k, got)
				}
			}
		})
	}
}

// =============================================================================
// toFloat64
// =============================================================================

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   float64
		wantOK bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(1.5), float64(float32(1.5)), true},
		{"int", int(5), 5, true},
		{"int32", int32(7), 7, true},
		{"int64", int64(100), 100, true},
		{"json.Number", json.Number("42.5"), 42.5, true},
		{"string", "nope", 0, false},
		{"bool", true, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok=%v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// jsonEqual / anySliceContains
// =============================================================================

func TestJsonEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b any
		want bool
	}{
		{"equal strings", "foo", "foo", true},
		{"different strings", "foo", "bar", false},
		{"equal numbers", 42.0, 42.0, true},
		{"different numbers", 1.0, 2.0, false},
		{"equal maps", map[string]any{"x": 1.0}, map[string]any{"x": 1.0}, true},
		{"different maps", map[string]any{"x": 1.0}, map[string]any{"x": 2.0}, false},
		{"nil equal", nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("jsonEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestAnySliceContains(t *testing.T) {
	slice := []any{"a", "b", 1.0}
	tests := []struct {
		val  any
		want bool
	}{
		{"a", true},
		{"b", true},
		{1.0, true},
		{"z", false},
		{2.0, false},
	}
	for _, tt := range tests {
		if got := anySliceContains(slice, tt.val); got != tt.want {
			t.Errorf("anySliceContains(%v) = %v, want %v", tt.val, got, tt.want)
		}
	}
}

// =============================================================================
// extractEffectiveSchema
// =============================================================================

func TestExtractEffectiveSchema(t *testing.T) {
	t.Run("direct properties and required", func(t *testing.T) {
		schema := map[string]any{
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"age":  map[string]any{"type": "integer"},
			},
			"required": []any{"name"},
		}
		eff := extractEffectiveSchema(schema)
		if _, ok := eff.properties["name"]; !ok {
			t.Error("expected 'name' in properties")
		}
		if _, ok := eff.properties["age"]; !ok {
			t.Error("expected 'age' in properties")
		}
		if !eff.required["name"] {
			t.Error("expected 'name' in required")
		}
		if !eff.requiredSet {
			t.Error("expected requiredSet=true")
		}
	})

	t.Run("allOf merges properties and required", func(t *testing.T) {
		schema := map[string]any{
			"allOf": []any{
				map[string]any{
					"properties": map[string]any{"a": map[string]any{"type": "string"}},
					"required":   []any{"a"},
				},
				map[string]any{
					"properties": map[string]any{"b": map[string]any{"type": "integer"}},
				},
			},
		}
		eff := extractEffectiveSchema(schema)
		if _, ok := eff.properties["a"]; !ok {
			t.Error("expected 'a' from allOf")
		}
		if _, ok := eff.properties["b"]; !ok {
			t.Error("expected 'b' from allOf")
		}
		if !eff.required["a"] {
			t.Error("expected 'a' required from allOf")
		}
	})

	t.Run("additionalProperties captured", func(t *testing.T) {
		eff := extractEffectiveSchema(map[string]any{"additionalProperties": false})
		if eff.additionalProperties != false {
			t.Errorf("expected false, got %v", eff.additionalProperties)
		}
	})

	t.Run("no required array leaves requiredSet false", func(t *testing.T) {
		eff := extractEffectiveSchema(map[string]any{
			"properties": map[string]any{"x": map[string]any{"type": "string"}},
		})
		if eff.requiredSet {
			t.Error("requiredSet should be false")
		}
	})
}

// =============================================================================
// checkTypeCompatibility
// =============================================================================

func TestCheckTypeCompatibility(t *testing.T) {
	tests := []struct {
		name      string
		base      map[string]any
		derived   map[string]any
		wantError bool
	}{
		{"same type", map[string]any{"type": "string"}, map[string]any{"type": "string"}, false},
		{"different type", map[string]any{"type": "string"}, map[string]any{"type": "integer"}, true},
		{"base has no type", map[string]any{}, map[string]any{"type": "string"}, false},
		{"derived omits type", map[string]any{"type": "string"}, map[string]any{}, true},
		{"derived narrows union", map[string]any{"type": []any{"string", "null"}}, map[string]any{"type": "string"}, false},
		{"derived expands type", map[string]any{"type": "string"}, map[string]any{"type": []any{"string", "integer"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := checkTypeCompatibility(tt.base, tt.derived, "field")
			if tt.wantError && len(errs) == 0 {
				t.Error("expected error but got none")
			}
			if !tt.wantError && len(errs) != 0 {
				t.Errorf("expected no error but got: %v", errs)
			}
		})
	}
}

// =============================================================================
// checkConstCompatibility
// =============================================================================

func TestCheckConstCompatibility(t *testing.T) {
	tests := []struct {
		name      string
		base      map[string]any
		derived   map[string]any
		wantError bool
	}{
		{"no base const", map[string]any{"type": "string"}, map[string]any{"type": "string"}, false},
		{"same const", map[string]any{"const": "v"}, map[string]any{"const": "v"}, false},
		{"different const", map[string]any{"const": "A"}, map[string]any{"const": "B"}, true},
		{"derived omits const", map[string]any{"const": "v"}, map[string]any{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := checkConstCompatibility(tt.base, tt.derived, "field")
			if tt.wantError && len(errs) == 0 {
				t.Error("expected error but got none")
			}
			if !tt.wantError && len(errs) != 0 {
				t.Errorf("expected no error but got: %v", errs)
			}
		})
	}
}

// =============================================================================
// checkPatternCompatibility
// =============================================================================

func TestCheckPatternCompatibility(t *testing.T) {
	tests := []struct {
		name      string
		base      map[string]any
		derived   map[string]any
		wantError bool
	}{
		{"same pattern", map[string]any{"pattern": "^[a-z]+$"}, map[string]any{"pattern": "^[a-z]+$"}, false},
		{"different pattern", map[string]any{"pattern": "^[a-z]+$"}, map[string]any{"pattern": "^[A-Z]+$"}, true},
		{"derived omits pattern", map[string]any{"pattern": "^[a-z]+$"}, map[string]any{}, true},
		{"no base pattern", map[string]any{}, map[string]any{"pattern": "^x$"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := checkPatternCompatibility(tt.base, tt.derived, "field")
			if tt.wantError && len(errs) == 0 {
				t.Error("expected error but got none")
			}
			if !tt.wantError && len(errs) != 0 {
				t.Errorf("expected no error but got: %v", errs)
			}
		})
	}
}

// =============================================================================
// checkEnumCompatibility
// =============================================================================

func TestCheckEnumCompatibility(t *testing.T) {
	tests := []struct {
		name      string
		base      map[string]any
		derived   map[string]any
		wantError bool
	}{
		{"no base enum", map[string]any{"type": "string"}, map[string]any{"type": "string"}, false},
		{"derived subset", map[string]any{"enum": []any{"a", "b", "c"}}, map[string]any{"enum": []any{"a", "b"}}, false},
		{"derived same", map[string]any{"enum": []any{"a", "b"}}, map[string]any{"enum": []any{"a", "b"}}, false},
		{"derived adds value", map[string]any{"enum": []any{"a", "b"}}, map[string]any{"enum": []any{"a", "b", "c"}}, true},
		{"derived const in enum", map[string]any{"enum": []any{"a", "b", "c"}}, map[string]any{"const": "b"}, false},
		{"derived const not in enum", map[string]any{"enum": []any{"a", "b"}}, map[string]any{"const": "z"}, true},
		{"derived omits enum", map[string]any{"enum": []any{"a", "b"}}, map[string]any{"type": "string"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := checkEnumCompatibility(tt.base, tt.derived, "field")
			if tt.wantError && len(errs) == 0 {
				t.Error("expected error but got none")
			}
			if !tt.wantError && len(errs) != 0 {
				t.Errorf("expected no error but got: %v", errs)
			}
		})
	}
}

// =============================================================================
// checkBound
// =============================================================================

func TestCheckBound(t *testing.T) {
	tests := []struct {
		name      string
		base      map[string]any
		derived   map[string]any
		keyword   string
		upper     bool
		wantError bool
	}{
		// upper bounds
		{"maxLength tightened", map[string]any{"maxLength": 100}, map[string]any{"maxLength": 50}, "maxLength", true, false},
		{"maxLength same", map[string]any{"maxLength": 50}, map[string]any{"maxLength": 50}, "maxLength", true, false},
		{"maxLength loosened", map[string]any{"maxLength": 50}, map[string]any{"maxLength": 100}, "maxLength", true, true},
		{"maximum derived omits", map[string]any{"maximum": 100}, map[string]any{}, "maximum", true, true},
		{"maxItems tightened", map[string]any{"maxItems": 10}, map[string]any{"maxItems": 5}, "maxItems", true, false},
		// lower bounds
		{"minLength tightened", map[string]any{"minLength": 5}, map[string]any{"minLength": 10}, "minLength", false, false},
		{"minLength loosened", map[string]any{"minLength": 10}, map[string]any{"minLength": 3}, "minLength", false, true},
		{"minimum derived omits", map[string]any{"minimum": 0}, map[string]any{}, "minimum", false, true},
		{"minItems tightened", map[string]any{"minItems": 1}, map[string]any{"minItems": 3}, "minItems", false, false},
		// base has no constraint
		{"no base bound", map[string]any{}, map[string]any{"maxLength": 50}, "maxLength", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := checkBound(tt.base, tt.derived, tt.keyword, "field", tt.upper)
			if tt.wantError && len(errs) == 0 {
				t.Error("expected error but got none")
			}
			if !tt.wantError && len(errs) != 0 {
				t.Errorf("expected no error but got: %v", errs)
			}
		})
	}
}

// =============================================================================
// collectDerivedEnumeratedValues
// =============================================================================

func TestCollectDerivedEnumeratedValues(t *testing.T) {
	tests := []struct {
		name    string
		prop    map[string]any
		wantLen int
		wantOK  bool
	}{
		{"const", map[string]any{"const": "only"}, 1, true},
		{"enum three", map[string]any{"enum": []any{"x", "y", "z"}}, 3, true},
		{"neither", map[string]any{"type": "string"}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vals, ok := collectDerivedEnumeratedValues(tt.prop)
			if ok != tt.wantOK {
				t.Errorf("ok=%v, want %v", ok, tt.wantOK)
			}
			if len(vals) != tt.wantLen {
				t.Errorf("len=%v, want %v", len(vals), tt.wantLen)
			}
		})
	}
}

// =============================================================================
// checkEnumeratedValuesAgainstBase
// =============================================================================

func TestCheckEnumeratedValuesAgainstBase(t *testing.T) {
	tests := []struct {
		name      string
		base      map[string]any
		values    []any
		wantError bool
	}{
		{"valid minimum", map[string]any{"minimum": 5.0}, []any{5.0, 10.0}, false},
		{"violates minimum", map[string]any{"minimum": 10.0}, []any{5.0}, true},
		{"valid maximum", map[string]any{"maximum": 100.0}, []any{50.0, 99.0}, false},
		{"violates maximum", map[string]any{"maximum": 100.0}, []any{101.0}, true},
		{"string ok minLength", map[string]any{"minLength": 2.0}, []any{"hi"}, false},
		{"string violates minLength", map[string]any{"minLength": 3.0}, []any{"hi"}, true},
		{"string violates maxLength", map[string]any{"maxLength": 3.0}, []any{"toolong"}, true},
		{"array ok minItems", map[string]any{"minItems": 1.0}, []any{[]any{"a"}}, false},
		{"array violates minItems", map[string]any{"minItems": 2.0}, []any{[]any{"a"}}, true},
		{"array violates maxItems", map[string]any{"maxItems": 1.0}, []any{[]any{"a", "b"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := checkEnumeratedValuesAgainstBase(tt.base, tt.values, "field")
			if tt.wantError && len(errs) == 0 {
				t.Error("expected error but got none")
			}
			if !tt.wantError && len(errs) != 0 {
				t.Errorf("expected no error but got: %v", errs)
			}
		})
	}
}

// =============================================================================
// checkItemsCompatibility
// =============================================================================

func TestCheckItemsCompatibility(t *testing.T) {
	tests := []struct {
		name      string
		base      map[string]any
		derived   map[string]any
		wantError bool
	}{
		{"no base items", map[string]any{"type": "array"}, map[string]any{"items": map[string]any{"type": "string"}}, false},
		{"derived omits items", map[string]any{"items": map[string]any{"type": "string"}}, map[string]any{}, true},
		{"compatible items (tightened)", map[string]any{"items": map[string]any{"type": "string", "minLength": 1.0}}, map[string]any{"items": map[string]any{"type": "string", "minLength": 3.0}}, false},
		{"incompatible items type change", map[string]any{"items": map[string]any{"type": "string"}}, map[string]any{"items": map[string]any{"type": "integer"}}, true},
		{"derived replaces object items with bool", map[string]any{"items": map[string]any{"type": "string"}}, map[string]any{"items": true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := checkItemsCompatibility(tt.base, tt.derived, "field")
			if tt.wantError && len(errs) == 0 {
				t.Error("expected error but got none")
			}
			if !tt.wantError && len(errs) != 0 {
				t.Errorf("expected no error but got: %v", errs)
			}
		})
	}
}

// =============================================================================
// checkRequiredRemoval
// =============================================================================

func TestCheckRequiredRemoval(t *testing.T) {
	tests := []struct {
		name       string
		baseReq    map[string]bool
		derivedReq map[string]bool
		derivedSet bool
		wantError  bool
	}{
		// At top level (nested=false), omitting required is treated as empty — error if base has required fields
		{"top-level derived not set — treated as empty", map[string]bool{"a": true}, map[string]bool{}, false, true},
		{"derived preserves all", map[string]bool{"a": true, "b": true}, map[string]bool{"a": true, "b": true, "c": true}, true, false},
		{"derived removes one", map[string]bool{"a": true, "b": true}, map[string]bool{"a": true}, true, true},
		{"derived removes all", map[string]bool{"a": true}, map[string]bool{}, true, true},
		{"base empty", map[string]bool{}, map[string]bool{"a": true}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := &effectiveSchema{required: tt.baseReq, requiredSet: true}
			derived := &effectiveSchema{required: tt.derivedReq, requiredSet: tt.derivedSet}
			errs := checkRequiredRemoval(base, derived, "base", "derived", false)
			if tt.wantError && len(errs) == 0 {
				t.Error("expected error but got none")
			}
			if !tt.wantError && len(errs) != 0 {
				t.Errorf("expected no error but got: %v", errs)
			}
		})
	}

	// In nested context, omitting required is a valid partial overlay
	t.Run("nested derived not set — allowed partial overlay", func(t *testing.T) {
		base := &effectiveSchema{required: map[string]bool{"a": true}, requiredSet: true}
		derived := &effectiveSchema{required: map[string]bool{}, requiredSet: false}
		errs := checkRequiredRemoval(base, derived, "base", "derived", true)
		if len(errs) != 0 {
			t.Errorf("nested partial overlay should be allowed, got: %v", errs)
		}
	})
}

// =============================================================================
// validateSchemaCompatibility
// =============================================================================

func TestValidateSchemaCompatibility(t *testing.T) {
	tests := []struct {
		name      string
		base      *effectiveSchema
		derived   *effectiveSchema
		nested    bool
		wantError bool
	}{
		{
			name: "identical schemas",
			base: &effectiveSchema{
				properties:  map[string]any{"name": map[string]any{"type": "string"}},
				required:    map[string]bool{"name": true},
				requiredSet: true,
			},
			derived: &effectiveSchema{
				properties:  map[string]any{"name": map[string]any{"type": "string"}},
				required:    map[string]bool{"name": true},
				requiredSet: true,
			},
			wantError: false,
		},
		{
			name: "derived disables base property",
			base: &effectiveSchema{
				properties:  map[string]any{"name": map[string]any{"type": "string"}},
				required:    map[string]bool{},
				requiredSet: false,
			},
			derived: &effectiveSchema{
				properties:  map[string]any{"name": false},
				required:    map[string]bool{},
				requiredSet: false,
			},
			wantError: true,
		},
		{
			name: "derived omits base property at top level",
			base: &effectiveSchema{
				properties:  map[string]any{"name": map[string]any{"type": "string"}},
				required:    map[string]bool{},
				requiredSet: false,
			},
			derived: &effectiveSchema{
				properties:  map[string]any{},
				required:    map[string]bool{},
				requiredSet: false,
			},
			nested:    false,
			wantError: true,
		},
		{
			name: "derived omits base property in nested context (ok)",
			base: &effectiveSchema{
				properties:  map[string]any{"name": map[string]any{"type": "string"}},
				required:    map[string]bool{},
				requiredSet: false,
			},
			derived: &effectiveSchema{
				properties:  map[string]any{},
				required:    map[string]bool{},
				requiredSet: false,
			},
			nested:    true,
			wantError: false,
		},
		{
			name: "derived loosens additionalProperties from false",
			base: &effectiveSchema{
				properties:           map[string]any{},
				required:             map[string]bool{},
				requiredSet:          false,
				additionalProperties: false,
			},
			derived: &effectiveSchema{
				properties:           map[string]any{},
				required:             map[string]bool{},
				requiredSet:          false,
				additionalProperties: nil,
			},
			wantError: true,
		},
		{
			name: "derived adds new property when base has additionalProperties:false",
			base: &effectiveSchema{
				properties:           map[string]any{},
				required:             map[string]bool{},
				requiredSet:          false,
				additionalProperties: false,
			},
			derived: &effectiveSchema{
				properties:           map[string]any{"extra": map[string]any{"type": "string"}},
				required:             map[string]bool{},
				requiredSet:          false,
				additionalProperties: false,
			},
			wantError: true,
		},
		{
			name: "derived re-enables disabled base property",
			base: &effectiveSchema{
				properties:  map[string]any{"disabled": false},
				required:    map[string]bool{},
				requiredSet: false,
			},
			derived: &effectiveSchema{
				properties:  map[string]any{"disabled": map[string]any{"type": "string"}},
				required:    map[string]bool{},
				requiredSet: false,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateSchemaCompatibility(tt.base, tt.derived, "base", "derived", tt.nested)
			if tt.wantError && len(errs) == 0 {
				t.Error("expected error but got none")
			}
			if !tt.wantError && len(errs) != 0 {
				t.Errorf("expected no error but got: %v", errs)
			}
		})
	}
}

// =============================================================================
// ValidateSchemaChain integration tests
// =============================================================================

func newSchemaEntity(content map[string]any) *JsonEntity {
	return NewJsonEntity(content, DefaultGtsConfig())
}

func mustRegister(t *testing.T, store *GtsStore, content map[string]any) {
	t.Helper()
	if _, ok := content["$schema"]; !ok {
		content["$schema"] = "http://json-schema.org/draft-07/schema#"
	}
	if err := store.Register(newSchemaEntity(content)); err != nil {
		t.Fatalf("register failed: %v", err)
	}
}

func TestValidateSchemaChain_SingleSegment(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":  "gts.x.chain.ns.base.v1~",
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	})
	result := store.ValidateSchemaChain("gts.x.chain.ns.base.v1~")
	if !result.OK {
		t.Errorf("single-segment schema should always be valid, got: %s", result.Error)
	}
}

func TestValidateSchemaChain_InvalidGtsID(t *testing.T) {
	store := NewGtsStore(nil)
	result := store.ValidateSchemaChain("not-a-valid-id")
	if result.OK {
		t.Error("expected failure for invalid GTS ID")
	}
}

func TestValidateSchemaChain_MissingBase(t *testing.T) {
	store := NewGtsStore(nil)
	// Only register derived, not base
	mustRegister(t, store, map[string]any{
		"$id":  "gts.x.chain.ns.base.v1~x.chain.ns.derived.v1~",
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	})
	result := store.ValidateSchemaChain("gts.x.chain.ns.base.v1~x.chain.ns.derived.v1~")
	if result.OK {
		t.Error("expected failure when base schema is missing")
	}
}

func TestValidateSchemaChain_TwoLevel_Compatible(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":      "gts.x.chain.ns.animal.v1~",
		"type":     "object",
		"required": []any{"name"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	})
	// Derived tightens name with a minLength — compatible
	mustRegister(t, store, map[string]any{
		"$id":      "gts.x.chain.ns.animal.v1~x.chain.ns.dog.v1~",
		"type":     "object",
		"required": []any{"name"},
		"properties": map[string]any{
			"name":  map[string]any{"type": "string", "minLength": 1.0},
			"breed": map[string]any{"type": "string"},
		},
	})
	result := store.ValidateSchemaChain("gts.x.chain.ns.animal.v1~x.chain.ns.dog.v1~")
	if !result.OK {
		t.Errorf("expected compatible two-level chain, got: %s", result.Error)
	}
}

func TestValidateSchemaChain_TwoLevel_TypeChange(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":  "gts.x.chain.ns.item.v1~",
		"type": "object",
		"properties": map[string]any{
			"count": map[string]any{"type": "integer"},
		},
	})
	// Derived changes count type — incompatible
	mustRegister(t, store, map[string]any{
		"$id":  "gts.x.chain.ns.item.v1~x.chain.ns.item2.v1~",
		"type": "object",
		"properties": map[string]any{
			"count": map[string]any{"type": "string"},
		},
	})
	result := store.ValidateSchemaChain("gts.x.chain.ns.item.v1~x.chain.ns.item2.v1~")
	if result.OK {
		t.Error("expected failure for type change in derived schema")
	}
}

func TestValidateSchemaChain_TwoLevel_LoosensAdditionalProperties(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":                  "gts.x.chain.ns.closed.v1~",
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	})
	// Derived omits additionalProperties:false — loosening
	mustRegister(t, store, map[string]any{
		"$id":  "gts.x.chain.ns.closed.v1~x.chain.ns.open.v1~",
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	})
	result := store.ValidateSchemaChain("gts.x.chain.ns.closed.v1~x.chain.ns.open.v1~")
	if result.OK {
		t.Error("expected failure when derived loosens additionalProperties")
	}
}

func TestValidateSchemaChain_TwoLevel_RemovesRequired(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":      "gts.x.chain.ns.req.v1~",
		"type":     "object",
		"required": []any{"id", "name"},
		"properties": map[string]any{
			"id":   map[string]any{"type": "string"},
			"name": map[string]any{"type": "string"},
		},
	})
	// Derived drops 'name' from required
	mustRegister(t, store, map[string]any{
		"$id":      "gts.x.chain.ns.req.v1~x.chain.ns.req2.v1~",
		"type":     "object",
		"required": []any{"id"},
		"properties": map[string]any{
			"id":   map[string]any{"type": "string"},
			"name": map[string]any{"type": "string"},
		},
	})
	result := store.ValidateSchemaChain("gts.x.chain.ns.req.v1~x.chain.ns.req2.v1~")
	if result.OK {
		t.Error("expected failure when derived removes a required field")
	}
}

func TestValidateSchemaChain_ThreeLevel_AllCompatible(t *testing.T) {
	store := NewGtsStore(nil)
	// A
	mustRegister(t, store, map[string]any{
		"$id":      "gts.x.chain3.ns.a.v1~",
		"type":     "object",
		"required": []any{"id"},
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	})
	// A~B — adds optional field
	mustRegister(t, store, map[string]any{
		"$id":      "gts.x.chain3.ns.a.v1~x.chain3.ns.b.v1~",
		"type":     "object",
		"required": []any{"id"},
		"properties": map[string]any{
			"id":    map[string]any{"type": "string"},
			"label": map[string]any{"type": "string"},
		},
	})
	// A~B~C — tightens label minLength
	mustRegister(t, store, map[string]any{
		"$id":      "gts.x.chain3.ns.a.v1~x.chain3.ns.b.v1~x.chain3.ns.c.v1~",
		"type":     "object",
		"required": []any{"id"},
		"properties": map[string]any{
			"id":    map[string]any{"type": "string"},
			"label": map[string]any{"type": "string", "minLength": 1.0},
		},
	})
	result := store.ValidateSchemaChain("gts.x.chain3.ns.a.v1~x.chain3.ns.b.v1~x.chain3.ns.c.v1~")
	if !result.OK {
		t.Errorf("expected three-level chain to be valid, got: %s", result.Error)
	}
}

func TestValidateSchemaChain_ThreeLevel_MiddleIncompatible(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":  "gts.x.chain3b.ns.a.v1~",
		"type": "object",
		"properties": map[string]any{
			"val": map[string]any{"type": "string"},
		},
	})
	// B changes val type — incompatible with A
	mustRegister(t, store, map[string]any{
		"$id":  "gts.x.chain3b.ns.a.v1~x.chain3b.ns.b.v1~",
		"type": "object",
		"properties": map[string]any{
			"val": map[string]any{"type": "integer"},
		},
	})
	mustRegister(t, store, map[string]any{
		"$id":  "gts.x.chain3b.ns.a.v1~x.chain3b.ns.b.v1~x.chain3b.ns.c.v1~",
		"type": "object",
		"properties": map[string]any{
			"val": map[string]any{"type": "integer"},
		},
	})
	result := store.ValidateSchemaChain("gts.x.chain3b.ns.a.v1~x.chain3b.ns.b.v1~x.chain3b.ns.c.v1~")
	if result.OK {
		t.Error("expected failure when middle of chain is incompatible")
	}
}

func TestValidateSchemaChain_CircularRef(t *testing.T) {
	store := NewGtsStore(nil)
	// Schema that directly references itself via $ref
	mustRegister(t, store, map[string]any{
		"$id":  "gts.x.cyclic.ns.self.v1~",
		"type": "object",
		"properties": map[string]any{
			"child": map[string]any{"$ref": "gts.x.cyclic.ns.self.v1~"},
		},
	})
	mustRegister(t, store, map[string]any{
		"$id":  "gts.x.cyclic.ns.self.v1~x.cyclic.ns.derived.v1~",
		"type": "object",
		"properties": map[string]any{
			"child": map[string]any{"$ref": "gts.x.cyclic.ns.self.v1~"},
		},
	})
	result := store.ValidateSchemaChain("gts.x.cyclic.ns.self.v1~x.cyclic.ns.derived.v1~")
	if result.OK {
		t.Fatalf("expected ValidateSchemaChain to fail for circular $ref schema, got ok=true")
	}
	if !strings.Contains(strings.ToLower(result.Error), "circular") {
		t.Errorf("expected error to contain 'circular', got: %s", result.Error)
	}
}

// =============================================================================
// comparePropertyConstraints — nested object recursion
// =============================================================================

func TestComparePropertyConstraints_NestedObject(t *testing.T) {
	t.Run("compatible nested object", func(t *testing.T) {
		base := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
		}
		derived := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
		}
		errs := comparePropertyConstraints(base, derived, "obj")
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("nested type change is an error", func(t *testing.T) {
		base := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{"type": "integer"},
			},
		}
		derived := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{"type": "string"},
			},
		}
		errs := comparePropertyConstraints(base, derived, "obj")
		if len(errs) == 0 {
			t.Error("expected error for nested type change")
		}
	})
}
