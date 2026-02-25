/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"testing"
)

// =============================================================================
// walkSchema / normalizeDollarRefs / removeXGtsFields
// =============================================================================

func TestWalkSchema_SkipKeys(t *testing.T) {
	input := map[string]any{
		"type":         "object",
		"x-gts-traits": map[string]any{"color": "red"},
		"x-gts-ref":    "gts.x.foo.ns.bar.v1~",
		"properties":   map[string]any{"id": map[string]any{"type": "string", "x-gts-ref": "gts.*"}},
	}
	result := walkSchema(input, nil, func(k string) bool {
		return k == "x-gts-traits" || k == "x-gts-ref"
	})
	if _, ok := result["x-gts-traits"]; ok {
		t.Error("x-gts-traits should have been skipped")
	}
	if _, ok := result["x-gts-ref"]; ok {
		t.Error("x-gts-ref should have been skipped")
	}
	if _, ok := result["type"]; !ok {
		t.Error("type should be preserved")
	}
	// Nested removal
	props, _ := result["properties"].(map[string]any)
	idProp, _ := props["id"].(map[string]any)
	if _, ok := idProp["x-gts-ref"]; ok {
		t.Error("nested x-gts-ref should have been removed")
	}
}

func TestWalkSchema_RenameKeys(t *testing.T) {
	input := map[string]any{
		"$$ref": "gts.x.foo.ns.bar.v1~",
		"type":  "object",
	}
	result := normalizeDollarRefs(input)
	if _, ok := result["$ref"]; !ok {
		t.Error("$$ref should be renamed to $ref")
	}
	if _, ok := result["$$ref"]; ok {
		t.Error("$$ref should be gone after rename")
	}
}

func TestNormalizeDollarRefs_Nested(t *testing.T) {
	input := map[string]any{
		"allOf": []any{
			map[string]any{"$$ref": "gts.x.foo.ns.bar.v1~"},
		},
	}
	result := normalizeDollarRefs(input)
	allOf, _ := result["allOf"].([]any)
	if len(allOf) != 1 {
		t.Fatalf("expected 1 allOf item, got %d", len(allOf))
	}
	item, _ := allOf[0].(map[string]any)
	if _, ok := item["$ref"]; !ok {
		t.Error("nested $$ref should be renamed to $ref")
	}
}

func TestRemoveXGtsFields(t *testing.T) {
	input := map[string]any{
		"type":                "object",
		"x-gts-traits":        map[string]any{"k": "v"},
		"x-gts-traits-schema": map[string]any{"type": "object"},
		"properties": map[string]any{
			"id": map[string]any{
				"type":      "string",
				"x-gts-ref": "gts.*",
			},
		},
	}
	result := removeXGtsFields(input)
	for _, k := range []string{"x-gts-traits", "x-gts-traits-schema"} {
		if _, ok := result[k]; ok {
			t.Errorf("key %q should have been removed", k)
		}
	}
	props, _ := result["properties"].(map[string]any)
	idProp, _ := props["id"].(map[string]any)
	if _, ok := idProp["x-gts-ref"]; ok {
		t.Error("nested x-gts-ref should have been removed")
	}
}

// =============================================================================
// collectTraitSchemaFromValue
// =============================================================================

func TestCollectTraitSchemaFromValue(t *testing.T) {
	t.Run("top-level x-gts-traits-schema", func(t *testing.T) {
		schema := map[string]any{
			"x-gts-traits-schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"color": map[string]any{"type": "string"},
				},
			},
		}
		var out []map[string]any
		collectTraitSchemaFromValue(schema, &out, 0)
		if len(out) != 1 {
			t.Fatalf("expected 1 trait schema, got %d", len(out))
		}
		if out[0] == nil {
			t.Error("expected non-nil trait schema")
		}
	})

	t.Run("non-object x-gts-traits-schema appends nil sentinel", func(t *testing.T) {
		schema := map[string]any{
			"x-gts-traits-schema": "invalid",
		}
		var out []map[string]any
		collectTraitSchemaFromValue(schema, &out, 0)
		if len(out) != 1 || out[0] != nil {
			t.Error("expected one nil sentinel for non-object trait schema")
		}
	})

	t.Run("allOf nested x-gts-traits-schema", func(t *testing.T) {
		schema := map[string]any{
			"allOf": []any{
				map[string]any{
					"x-gts-traits-schema": map[string]any{"type": "object"},
				},
				map[string]any{
					"x-gts-traits-schema": map[string]any{"type": "object"},
				},
			},
		}
		var out []map[string]any
		collectTraitSchemaFromValue(schema, &out, 0)
		if len(out) != 2 {
			t.Errorf("expected 2 trait schemas from allOf, got %d", len(out))
		}
	})

	t.Run("no x-gts-traits-schema", func(t *testing.T) {
		var out []map[string]any
		collectTraitSchemaFromValue(map[string]any{"type": "object"}, &out, 0)
		if len(out) != 0 {
			t.Errorf("expected 0 trait schemas, got %d", len(out))
		}
	})

	t.Run("depth limit stops recursion", func(t *testing.T) {
		var out []map[string]any
		collectTraitSchemaFromValue(map[string]any{"x-gts-traits-schema": map[string]any{}}, &out, maxTraitsRecursionDepth)
		if len(out) != 0 {
			t.Error("should not collect at max depth")
		}
	})
}

// =============================================================================
// collectTraitsFromValue
// =============================================================================

func TestCollectTraitsFromValue(t *testing.T) {
	t.Run("top-level x-gts-traits", func(t *testing.T) {
		schema := map[string]any{
			"x-gts-traits": map[string]any{"color": "red", "size": "large"},
		}
		merged := make(map[string]any)
		collectTraitsFromValue(schema, merged, 0)
		if merged["color"] != "red" || merged["size"] != "large" {
			t.Errorf("expected traits to be merged, got %v", merged)
		}
	})

	t.Run("allOf nested x-gts-traits merged (rightmost wins)", func(t *testing.T) {
		schema := map[string]any{
			"allOf": []any{
				map[string]any{"x-gts-traits": map[string]any{"color": "red"}},
				map[string]any{"x-gts-traits": map[string]any{"color": "blue", "size": "small"}},
			},
		}
		merged := make(map[string]any)
		collectTraitsFromValue(schema, merged, 0)
		if merged["color"] != "blue" {
			t.Errorf("rightmost should win: expected blue, got %v", merged["color"])
		}
		if merged["size"] != "small" {
			t.Errorf("expected size=small, got %v", merged["size"])
		}
	})

	t.Run("no x-gts-traits", func(t *testing.T) {
		merged := make(map[string]any)
		collectTraitsFromValue(map[string]any{"type": "object"}, merged, 0)
		if len(merged) != 0 {
			t.Errorf("expected empty merged map, got %v", merged)
		}
	})

	t.Run("depth limit stops recursion", func(t *testing.T) {
		merged := make(map[string]any)
		collectTraitsFromValue(map[string]any{"x-gts-traits": map[string]any{"k": "v"}}, merged, maxTraitsRecursionDepth)
		if len(merged) != 0 {
			t.Error("should not collect at max depth")
		}
	})
}

// =============================================================================
// buildEffectiveTraitSchema
// =============================================================================

func TestBuildEffectiveTraitSchema(t *testing.T) {
	tests := []struct {
		name       string
		schemas    []map[string]any
		wantEmpty  bool
		wantAllOf  bool
		wantDirect bool
	}{
		{
			name:      "empty input returns empty map",
			schemas:   []map[string]any{},
			wantEmpty: true,
		},
		{
			name:      "single nil returns empty map",
			schemas:   []map[string]any{nil},
			wantEmpty: true,
		},
		{
			name:       "single non-nil returns it directly",
			schemas:    []map[string]any{{"type": "object"}},
			wantDirect: true,
		},
		{
			name:      "two schemas wrapped in allOf",
			schemas:   []map[string]any{{"type": "object"}, {"type": "object"}},
			wantAllOf: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildEffectiveTraitSchema(tt.schemas)
			if tt.wantEmpty && len(result) != 0 {
				t.Errorf("expected empty map, got %v", result)
			}
			if tt.wantAllOf {
				if _, ok := result["allOf"]; !ok {
					t.Errorf("expected allOf key in result, got %v", result)
				}
			}
			if tt.wantDirect {
				if _, ok := result["type"]; !ok {
					t.Errorf("expected direct schema pass-through, got %v", result)
				}
			}
		})
	}
}

func TestBuildEffectiveTraitSchema_NilsInMultiple(t *testing.T) {
	// Nils mixed with valid schemas: nils should be filtered out of allOf
	schemas := []map[string]any{nil, {"type": "object"}, nil, {"properties": map[string]any{}}}
	result := buildEffectiveTraitSchema(schemas)
	allOf, ok := result["allOf"].([]any)
	if !ok {
		t.Fatalf("expected allOf, got %v", result)
	}
	if len(allOf) != 2 {
		t.Errorf("expected 2 items in allOf (nils filtered), got %d", len(allOf))
	}
}

// =============================================================================
// collectAllProperties
// =============================================================================

func TestCollectAllProperties(t *testing.T) {
	t.Run("direct properties", func(t *testing.T) {
		schema := map[string]any{
			"properties": map[string]any{
				"color": map[string]any{"type": "string"},
				"size":  map[string]any{"type": "string"},
			},
		}
		props := collectAllProperties(schema, 0)
		names := make(map[string]bool)
		for _, p := range props {
			names[p.name] = true
		}
		if !names["color"] || !names["size"] {
			t.Errorf("expected color and size, got %v", props)
		}
	})

	t.Run("allOf nested properties merged", func(t *testing.T) {
		schema := map[string]any{
			"allOf": []any{
				map[string]any{"properties": map[string]any{
					"a": map[string]any{"type": "string"},
				}},
				map[string]any{"properties": map[string]any{
					"b": map[string]any{"type": "integer"},
				}},
			},
		}
		props := collectAllProperties(schema, 0)
		names := make(map[string]bool)
		for _, p := range props {
			names[p.name] = true
		}
		if !names["a"] || !names["b"] {
			t.Errorf("expected a and b from allOf, got %v", names)
		}
	})

	t.Run("later definition overwrites earlier (last-write wins)", func(t *testing.T) {
		schema := map[string]any{
			"allOf": []any{
				map[string]any{"properties": map[string]any{
					"x": map[string]any{"type": "string", "default": "first"},
				}},
				map[string]any{"properties": map[string]any{
					"x": map[string]any{"type": "string", "default": "second"},
				}},
			},
		}
		props := collectAllProperties(schema, 0)
		if len(props) != 1 {
			t.Fatalf("expected 1 deduplicated property, got %d", len(props))
		}
		if props[0].schema["default"] != "second" {
			t.Errorf("expected last-write-wins, got %v", props[0].schema["default"])
		}
	})

	t.Run("depth limit returns nil", func(t *testing.T) {
		schema := map[string]any{
			"properties": map[string]any{"x": map[string]any{"type": "string"}},
		}
		props := collectAllProperties(schema, maxTraitsRecursionDepth)
		if props != nil {
			t.Error("expected nil at max depth")
		}
	})
}

// =============================================================================
// applyDefaults
// =============================================================================

func TestApplyDefaults(t *testing.T) {
	t.Run("fills missing property with default", func(t *testing.T) {
		traitSchema := map[string]any{
			"properties": map[string]any{
				"color": map[string]any{"type": "string", "default": "blue"},
				"size":  map[string]any{"type": "string"},
			},
		}
		traits := map[string]any{"size": "large"}
		result := applyDefaults(traitSchema, traits, 0)
		if result["color"] != "blue" {
			t.Errorf("expected default 'blue' for color, got %v", result["color"])
		}
		if result["size"] != "large" {
			t.Errorf("expected existing 'large' for size, got %v", result["size"])
		}
	})

	t.Run("does not overwrite existing value", func(t *testing.T) {
		traitSchema := map[string]any{
			"properties": map[string]any{
				"color": map[string]any{"type": "string", "default": "blue"},
			},
		}
		traits := map[string]any{"color": "red"}
		result := applyDefaults(traitSchema, traits, 0)
		if result["color"] != "red" {
			t.Errorf("existing value should not be overwritten, got %v", result["color"])
		}
	})

	t.Run("no default — leaves property absent", func(t *testing.T) {
		traitSchema := map[string]any{
			"properties": map[string]any{
				"required_trait": map[string]any{"type": "string"},
			},
		}
		result := applyDefaults(traitSchema, map[string]any{}, 0)
		if _, ok := result["required_trait"]; ok {
			t.Error("property without default should not appear in result")
		}
	})

	t.Run("nested object defaults applied recursively", func(t *testing.T) {
		traitSchema := map[string]any{
			"properties": map[string]any{
				"meta": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"version": map[string]any{"type": "string", "default": "1.0"},
					},
				},
			},
		}
		traits := map[string]any{"meta": map[string]any{}}
		result := applyDefaults(traitSchema, traits, 0)
		meta, ok := result["meta"].(map[string]any)
		if !ok {
			t.Fatal("expected meta to be a map")
		}
		if meta["version"] != "1.0" {
			t.Errorf("expected nested default '1.0' for version, got %v", meta["version"])
		}
	})

	t.Run("depth limit returns original traits", func(t *testing.T) {
		traitSchema := map[string]any{
			"properties": map[string]any{
				"x": map[string]any{"default": "val"},
			},
		}
		traits := map[string]any{}
		result := applyDefaults(traitSchema, traits, maxTraitsRecursionDepth)
		if _, ok := result["x"]; ok {
			t.Error("should not apply defaults at max depth")
		}
	})
}

// =============================================================================
// validateTraitsAgainstSchema
// =============================================================================

func TestValidateTraitsAgainstSchema(t *testing.T) {
	t.Run("valid traits pass", func(t *testing.T) {
		traitSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"color": map[string]any{"type": "string"},
				"count": map[string]any{"type": "integer"},
			},
			"required": []any{"color"},
		}
		traits := map[string]any{"color": "red", "count": 3.0}
		errs := validateTraitsAgainstSchema(traitSchema, traits, false)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("missing required trait fails", func(t *testing.T) {
		traitSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"color": map[string]any{"type": "string"},
			},
			"required": []any{"color"},
		}
		errs := validateTraitsAgainstSchema(traitSchema, map[string]any{}, false)
		if len(errs) == 0 {
			t.Error("expected error for missing required trait")
		}
	})

	t.Run("wrong type fails", func(t *testing.T) {
		traitSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{"type": "integer"},
			},
		}
		traits := map[string]any{"count": "not-a-number"}
		errs := validateTraitsAgainstSchema(traitSchema, traits, false)
		if len(errs) == 0 {
			t.Error("expected error for wrong type")
		}
	})

	t.Run("checkUnresolved flags unresolved properties without defaults", func(t *testing.T) {
		traitSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"unresolved": map[string]any{"type": "string"},
			},
		}
		errs := validateTraitsAgainstSchema(traitSchema, map[string]any{}, true)
		if len(errs) == 0 {
			t.Error("expected error for unresolved trait without default")
		}
	})

	t.Run("unresolved property with default is ok", func(t *testing.T) {
		traitSchema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"color": map[string]any{"type": "string", "default": "blue"},
			},
		}
		errs := validateTraitsAgainstSchema(traitSchema, map[string]any{}, true)
		if len(errs) != 0 {
			t.Errorf("property with default should not be flagged as unresolved, got %v", errs)
		}
	})
}

// =============================================================================
// ValidateSchemaTraits integration tests
// =============================================================================

func mustRegisterTraits(t *testing.T, store *GtsStore, content map[string]any) {
	t.Helper()
	if _, ok := content["$schema"]; !ok {
		content["$schema"] = "http://json-schema.org/draft-07/schema#"
	}
	if err := store.Register(NewJsonEntity(content, DefaultGtsConfig())); err != nil {
		t.Fatalf("register failed: %v", err)
	}
}

func TestValidateSchemaTraits_InvalidGtsID(t *testing.T) {
	store := NewGtsStore(nil)
	result := store.ValidateSchemaTraits("not-valid")
	if result.OK {
		t.Error("expected failure for invalid GTS ID")
	}
}

func TestValidateSchemaTraits_MissingSchema(t *testing.T) {
	store := NewGtsStore(nil)
	result := store.ValidateSchemaTraits("gts.x.traits.ns.missing.v1~")
	if result.OK {
		t.Error("expected failure for missing schema")
	}
}

func TestValidateSchemaTraits_NoTraitsAnywhere_OK(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.traits.ns.plain.v1~",
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	})
	result := store.ValidateSchemaTraits("gts.x.traits.ns.plain.v1~")
	if !result.OK {
		t.Errorf("schema with no traits at all should be valid, got: %s", result.Error)
	}
}

func TestValidateSchemaTraits_TraitValuesWithoutSchema_Fails(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":          "gts.x.traits.ns.noschema.v1~",
		"type":         "object",
		"x-gts-traits": map[string]any{"color": "red"},
	})
	result := store.ValidateSchemaTraits("gts.x.traits.ns.noschema.v1~")
	if result.OK {
		t.Error("expected failure: trait values provided but no trait schema defined")
	}
}

func TestValidateSchemaTraits_NonObjectTraitSchema_Fails(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":                 "gts.x.traits.ns.badschema.v1~",
		"type":                "object",
		"x-gts-traits-schema": "not-an-object",
		"x-gts-traits":        map[string]any{"k": "v"},
	})
	result := store.ValidateSchemaTraits("gts.x.traits.ns.badschema.v1~")
	if result.OK {
		t.Error("expected failure: x-gts-traits-schema is not an object")
	}
}

func TestValidateSchemaTraits_ValidTraits_SingleLevel(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.traits.ns.typed.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"color": map[string]any{"type": "string"},
				"count": map[string]any{"type": "integer"},
			},
			"required":             []any{"color"},
			"additionalProperties": false,
		},
		"x-gts-traits": map[string]any{"color": "red", "count": 3.0},
	})
	result := store.ValidateSchemaTraits("gts.x.traits.ns.typed.v1~")
	if !result.OK {
		t.Errorf("expected valid traits, got: %s", result.Error)
	}
}

func TestValidateSchemaTraits_MissingRequiredTrait_Fails(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.traits.ns.missingreq.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"color": map[string]any{"type": "string"},
			},
			"required":             []any{"color"},
			"additionalProperties": false,
		},
		// x-gts-traits omitted → unresolved required property
	})
	result := store.ValidateSchemaTraits("gts.x.traits.ns.missingreq.v1~")
	if result.OK {
		t.Error("expected failure: required trait 'color' is not provided")
	}
}

func TestValidateSchemaTraits_DefaultFillsMissingTrait(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.traits.ns.defaults.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"color": map[string]any{"type": "string", "default": "blue"},
			},
			"required":             []any{"color"},
			"additionalProperties": false,
		},
		// No x-gts-traits — default should fill 'color'
	})
	result := store.ValidateSchemaTraits("gts.x.traits.ns.defaults.v1~")
	if !result.OK {
		t.Errorf("expected default to fill required trait, got: %s", result.Error)
	}
}

func TestValidateSchemaTraits_WrongTraitType_Fails(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.traits.ns.wrongtype.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{"type": "integer"},
			},
			"additionalProperties": false,
		},
		"x-gts-traits": map[string]any{"count": "not-a-number"},
	})
	result := store.ValidateSchemaTraits("gts.x.traits.ns.wrongtype.v1~")
	if result.OK {
		t.Error("expected failure: trait 'count' has wrong type")
	}
}

func TestValidateSchemaTraits_InheritedTraitSchema_TwoLevel(t *testing.T) {
	store := NewGtsStore(nil)
	// Base defines trait schema
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.traits2.ns.base.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"color": map[string]any{"type": "string"},
			},
			"additionalProperties": false,
		},
	})
	// Derived provides the trait value
	mustRegisterTraits(t, store, map[string]any{
		"$id":          "gts.x.traits2.ns.base.v1~x.traits2.ns.child.v1~",
		"type":         "object",
		"x-gts-traits": map[string]any{"color": "green"},
	})
	result := store.ValidateSchemaTraits("gts.x.traits2.ns.base.v1~x.traits2.ns.child.v1~")
	if !result.OK {
		t.Errorf("expected valid inherited trait, got: %s", result.Error)
	}
}

func TestValidateSchemaTraits_TraitSchemaContainsTraitValues_Fails(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.traits.ns.selfcontained.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type":         "object",
			"x-gts-traits": map[string]any{"color": "red"},
			"properties": map[string]any{
				"color": map[string]any{"type": "string"},
			},
			"additionalProperties": false,
		},
		"x-gts-traits": map[string]any{"color": "red"},
	})
	result := store.ValidateSchemaTraits("gts.x.traits.ns.selfcontained.v1~")
	if result.OK {
		t.Error("expected failure: x-gts-traits inside x-gts-traits-schema is not allowed")
	}
}

func TestValidateSchemaTraits_LockedTraitOverride_Fails(t *testing.T) {
	store := NewGtsStore(nil)
	// Base defines trait value without a schema property — trait becomes locked
	mustRegisterTraits(t, store, map[string]any{
		"$id":          "gts.x.traits3.ns.base.v1~",
		"type":         "object",
		"x-gts-traits": map[string]any{"color": "red"},
	})
	// Child tries to override with a different value — must fail
	mustRegisterTraits(t, store, map[string]any{
		"$id":          "gts.x.traits3.ns.base.v1~x.traits3.ns.child.v1~",
		"type":         "object",
		"x-gts-traits": map[string]any{"color": "blue"},
	})
	result := store.ValidateSchemaTraits("gts.x.traits3.ns.base.v1~x.traits3.ns.child.v1~")
	if result.OK {
		t.Error("expected failure: child overrides locked trait value")
	}
}

func TestValidateSchemaTraits_DuplicateDefaultAcrossChain_Fails(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.traits4.ns.base.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"color": map[string]any{"type": "string", "default": "red"},
			},
			"additionalProperties": false,
		},
	})
	// Child redefines default for the same property with a different value — must fail
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.traits4.ns.base.v1~x.traits4.ns.child.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"color": map[string]any{"type": "string", "default": "blue"},
			},
			"additionalProperties": false,
		},
	})
	result := store.ValidateSchemaTraits("gts.x.traits4.ns.base.v1~x.traits4.ns.child.v1~")
	if result.OK {
		t.Error("expected failure: child overrides ancestor's trait schema default")
	}
}

// =============================================================================
// ValidateEntity integration tests
// =============================================================================

func TestValidateEntity_NotFound(t *testing.T) {
	store := NewGtsStore(nil)
	result := store.ValidateEntity("gts.x.entity.ns.ghost.v1~")
	if result.OK {
		t.Error("expected failure for non-existent entity")
	}
}

func TestValidateEntity_Schema_Valid(t *testing.T) {
	store := NewGtsStore(nil)
	// Single-segment schema with a closed trait schema and a matching trait value
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.entity.ns.valid.v1~",
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
		"x-gts-traits-schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"color": map[string]any{"type": "string", "default": "blue"},
			},
			"additionalProperties": false,
		},
		"x-gts-traits": map[string]any{"color": "red"},
	})
	result := store.ValidateEntity("gts.x.entity.ns.valid.v1~")
	if !result.OK {
		t.Errorf("expected valid entity, got: %s", result.Error)
	}
	if result.EntityType != "schema" {
		t.Errorf("expected EntityType=schema, got %s", result.EntityType)
	}
}

func TestValidateEntity_Schema_ChainIncompatible(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.entity.ns.typea.v1~",
		"type": "object",
		"properties": map[string]any{
			"val": map[string]any{"type": "string"},
		},
	})
	// Derived changes val to integer — incompatible
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.entity.ns.typea.v1~x.entity.ns.typeb.v1~",
		"type": "object",
		"properties": map[string]any{
			"val": map[string]any{"type": "integer"},
		},
	})
	result := store.ValidateEntity("gts.x.entity.ns.typea.v1~x.entity.ns.typeb.v1~")
	if result.OK {
		t.Error("expected failure due to incompatible chain")
	}
	if result.EntityType != "schema" {
		t.Errorf("expected EntityType=schema, got %s", result.EntityType)
	}
}

func TestValidateEntity_Schema_TraitSchemaWithoutValues_Fails(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.entity.ns.novals.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"color": map[string]any{"type": "string"}},
			"additionalProperties": false,
		},
		// No x-gts-traits — entity-level check must fail
	})
	result := store.ValidateEntity("gts.x.entity.ns.novals.v1~")
	if result.OK {
		t.Error("expected failure: entity has trait schema but no trait values")
	}
}

func TestValidateEntity_Schema_TraitSchemaNotClosed_Fails(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterTraits(t, store, map[string]any{
		"$id":  "gts.x.entity.ns.notclosed.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type":       "object",
			"properties": map[string]any{"color": map[string]any{"type": "string"}},
			// additionalProperties intentionally absent — must fail entity-level check
		},
		"x-gts-traits": map[string]any{"color": "red"},
	})
	result := store.ValidateEntity("gts.x.entity.ns.notclosed.v1~")
	if result.OK {
		t.Error("expected failure: entity trait schema must have additionalProperties:false")
	}
}
