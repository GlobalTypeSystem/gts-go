/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"strings"
	"testing"
)

// =============================================================================
// Helper functions
// =============================================================================

func mustRegisterMod(t *testing.T, store *GtsStore, content map[string]any) {
	t.Helper()
	if _, ok := content["$schema"]; !ok {
		content["$schema"] = "http://json-schema.org/draft-07/schema#"
	}
	if err := store.Register(NewJsonEntity(content, DefaultGtsConfig())); err != nil {
		t.Fatalf("register failed: %v", err)
	}
}

func mustRegisterInstance(t *testing.T, store *GtsStore, content map[string]any) {
	t.Helper()
	if err := store.Register(NewJsonEntity(content, DefaultGtsConfig())); err != nil {
		t.Fatalf("register instance failed: %v", err)
	}
}

// =============================================================================
// ValidateSchemaModifiers unit tests
// =============================================================================

func TestValidateSchemaModifiers_Default(t *testing.T) {
	if err := ValidateSchemaModifiers(map[string]any{"type": "object"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSchemaModifiers_FinalTrue(t *testing.T) {
	if err := ValidateSchemaModifiers(map[string]any{"x-gts-final": true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSchemaModifiers_AbstractTrue(t *testing.T) {
	if err := ValidateSchemaModifiers(map[string]any{"x-gts-abstract": true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSchemaModifiers_BothTrue_Error(t *testing.T) {
	err := ValidateSchemaModifiers(map[string]any{
		"x-gts-final":    true,
		"x-gts-abstract": true,
	})
	if err == nil {
		t.Error("expected error for both true")
	}
}

func TestValidateSchemaModifiers_NonBooleanFinal(t *testing.T) {
	err := ValidateSchemaModifiers(map[string]any{"x-gts-final": "yes"})
	if err == nil {
		t.Error("expected error for non-boolean x-gts-final")
	}
}

func TestValidateSchemaModifiers_NonBooleanAbstract(t *testing.T) {
	err := ValidateSchemaModifiers(map[string]any{"x-gts-abstract": 1.0})
	if err == nil {
		t.Error("expected error for non-boolean x-gts-abstract")
	}
}

func TestValidateSchemaModifiers_FalseIsNoop(t *testing.T) {
	err := ValidateSchemaModifiers(map[string]any{
		"x-gts-final":    false,
		"x-gts-abstract": false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSchemaModifiers_TopLevelWithAllOfOk(t *testing.T) {
	err := ValidateSchemaModifiers(map[string]any{
		"x-gts-final": true,
		"allOf": []any{
			map[string]any{"$ref": "gts.x.a.base.v1~"},
		},
	})
	if err != nil {
		t.Fatalf("top-level modifier should not be rejected: %v", err)
	}
}

func TestValidateSchemaModifiers_DirectAllOfRejected(t *testing.T) {
	err := ValidateSchemaModifiers(map[string]any{
		"allOf": []any{
			map[string]any{"x-gts-final": true},
		},
	})
	if err == nil {
		t.Error("expected error for x-gts-final inside allOf")
	}
}

func TestValidateSchemaModifiers_NestedAllOfFinalRejected(t *testing.T) {
	err := ValidateSchemaModifiers(map[string]any{
		"allOf": []any{
			map[string]any{
				"allOf": []any{
					map[string]any{"x-gts-final": true},
				},
			},
		},
	})
	if err == nil {
		t.Error("expected error for x-gts-final inside nested allOf")
	}
}

func TestValidateSchemaModifiers_NestedAllOfAbstractRejected(t *testing.T) {
	err := ValidateSchemaModifiers(map[string]any{
		"allOf": []any{
			map[string]any{"type": "object"},
			map[string]any{
				"allOf": []any{
					map[string]any{
						"allOf": []any{
							map[string]any{"x-gts-abstract": true},
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Error("expected error for x-gts-abstract deep inside nested allOf")
	}
}

func TestValidateInstanceModifiers_Clean(t *testing.T) {
	err := ValidateInstanceModifiers(map[string]any{"id": "test", "name": "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateInstanceModifiers_HasFinal(t *testing.T) {
	err := ValidateInstanceModifiers(map[string]any{"id": "test", "x-gts-final": true})
	if err == nil {
		t.Error("expected error for x-gts-final in instance")
	}
}

func TestValidateInstanceModifiers_HasAbstract(t *testing.T) {
	err := ValidateInstanceModifiers(map[string]any{"id": "test", "x-gts-abstract": true})
	if err == nil {
		t.Error("expected error for x-gts-abstract in instance")
	}
}

// =============================================================================
// x-gts-final integration tests
// =============================================================================

func TestFinal_RejectDerivedSchema(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":         "gts.x.testmod.final.base.v1~",
		"type":        "object",
		"x-gts-final": true,
		"properties":  map[string]any{"name": map[string]any{"type": "string"}},
	})
	mustRegisterMod(t, store, map[string]any{
		"$id":  "gts.x.testmod.final.base.v1~x.testmod._.derived.v1~",
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.final.base.v1~"},
			map[string]any{"type": "object", "properties": map[string]any{"extra": map[string]any{"type": "string"}}},
		},
	})
	result := store.ValidateSchemaChain("gts.x.testmod.final.base.v1~x.testmod._.derived.v1~")
	if result.OK {
		t.Error("expected validation to fail for derived from final base")
	}
	if !strings.Contains(result.Error, "final") {
		t.Errorf("error should mention 'final', got: %s", result.Error)
	}
}

func TestFinal_AllowWellKnownInstance(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":         "gts.x.testmod.final.inst.v1~",
		"type":        "object",
		"x-gts-final": true,
		"required":    []any{"id", "description"},
		"properties": map[string]any{
			"id":          map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
		},
	})
	mustRegisterInstance(t, store, map[string]any{
		"id":          "gts.x.testmod.final.inst.v1~x.testmod._.running.v1",
		"description": "Running state",
	})
	result := store.ValidateInstance("gts.x.testmod.final.inst.v1~x.testmod._.running.v1")
	if !result.OK {
		t.Errorf("expected instance of final type to pass, got: %s", result.Error)
	}
}

func TestFinal_MidChainFinal(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":        "gts.x.testmod.finalmid.base.v1~",
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	})
	mustRegisterMod(t, store, map[string]any{
		"$id":         "gts.x.testmod.finalmid.base.v1~x.testmod._.mid.v1~",
		"type":        "object",
		"x-gts-final": true,
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.finalmid.base.v1~"},
			map[string]any{"type": "object"},
		},
	})
	mustRegisterMod(t, store, map[string]any{
		"$id":  "gts.x.testmod.finalmid.base.v1~x.testmod._.mid.v1~x.testmod._.leaf.v1~",
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.finalmid.base.v1~x.testmod._.mid.v1~"},
			map[string]any{"type": "object"},
		},
	})
	result := store.ValidateSchemaChain("gts.x.testmod.finalmid.base.v1~x.testmod._.mid.v1~x.testmod._.leaf.v1~")
	if result.OK {
		t.Error("expected validation to fail - mid is final")
	}
}

func TestFinal_SiblingUnaffected(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":        "gts.x.testmod.finalsib.base.v1~",
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	})
	mustRegisterMod(t, store, map[string]any{
		"$id":         "gts.x.testmod.finalsib.base.v1~x.testmod._.final_b.v1~",
		"type":        "object",
		"x-gts-final": true,
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.finalsib.base.v1~"},
			map[string]any{"type": "object"},
		},
	})
	mustRegisterMod(t, store, map[string]any{
		"$id":  "gts.x.testmod.finalsib.base.v1~x.testmod._.sibling_c.v1~",
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.finalsib.base.v1~"},
			map[string]any{"type": "object", "properties": map[string]any{"extra": map[string]any{"type": "string"}}},
		},
	})
	result := store.ValidateSchemaChain("gts.x.testmod.finalsib.base.v1~x.testmod._.sibling_c.v1~")
	if !result.OK {
		t.Errorf("sibling should pass - base is not final, got: %s", result.Error)
	}
}

func TestFinal_FalseIsNoop(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":         "gts.x.testmod.finalfalse.base.v1~",
		"type":        "object",
		"x-gts-final": false,
		"properties":  map[string]any{"name": map[string]any{"type": "string"}},
	})
	mustRegisterMod(t, store, map[string]any{
		"$id":  "gts.x.testmod.finalfalse.base.v1~x.testmod._.derived.v1~",
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.finalfalse.base.v1~"},
			map[string]any{"type": "object"},
		},
	})
	result := store.ValidateSchemaChain("gts.x.testmod.finalfalse.base.v1~x.testmod._.derived.v1~")
	if !result.OK {
		t.Errorf("final=false should allow derivation, got: %s", result.Error)
	}
}

// =============================================================================
// x-gts-abstract integration tests
// =============================================================================

func TestAbstract_RejectDirectInstance(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":            "gts.x.testmod.abs.reject.v1~",
		"type":           "object",
		"x-gts-abstract": true,
		"required":       []any{"id", "name"},
		"properties": map[string]any{
			"id":   map[string]any{"type": "string"},
			"name": map[string]any{"type": "string"},
		},
	})
	mustRegisterInstance(t, store, map[string]any{
		"id":   "gts.x.testmod.abs.reject.v1~x.testmod._.item.v1",
		"name": "Direct item",
	})
	result := store.ValidateInstance("gts.x.testmod.abs.reject.v1~x.testmod._.item.v1")
	if result.OK {
		t.Error("expected instance of abstract type to fail validation")
	}
	if !strings.Contains(result.Error, "abstract") {
		t.Errorf("error should mention 'abstract', got: %s", result.Error)
	}
}

func TestAbstract_AllowDerivedSchema(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":            "gts.x.testmod.abs.derive.v1~",
		"type":           "object",
		"x-gts-abstract": true,
		"properties":     map[string]any{"name": map[string]any{"type": "string"}},
	})
	mustRegisterMod(t, store, map[string]any{
		"$id":  "gts.x.testmod.abs.derive.v1~x.testmod._.concrete.v1~",
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.abs.derive.v1~"},
			map[string]any{"type": "object", "properties": map[string]any{"extra": map[string]any{"type": "string"}}},
		},
	})
	result := store.ValidateSchemaChain("gts.x.testmod.abs.derive.v1~x.testmod._.concrete.v1~")
	if !result.OK {
		t.Errorf("derived from abstract should pass, got: %s", result.Error)
	}
}

func TestAbstract_AllowInstanceOfConcreteDerived(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":            "gts.x.testmod.abs.concinst.v1~",
		"type":           "object",
		"x-gts-abstract": true,
		"required":       []any{"id", "name"},
		"properties": map[string]any{
			"id":   map[string]any{"type": "string"},
			"name": map[string]any{"type": "string"},
		},
	})
	mustRegisterMod(t, store, map[string]any{
		"$id":  "gts.x.testmod.abs.concinst.v1~x.testmod._.concrete.v1~",
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.abs.concinst.v1~"},
			map[string]any{"type": "object"},
		},
	})
	mustRegisterInstance(t, store, map[string]any{
		"id":   "gts.x.testmod.abs.concinst.v1~x.testmod._.concrete.v1~x.testmod._.item.v1",
		"name": "My Item",
	})
	result := store.ValidateInstance("gts.x.testmod.abs.concinst.v1~x.testmod._.concrete.v1~x.testmod._.item.v1")
	if !result.OK {
		t.Errorf("instance of concrete derived should pass, got: %s", result.Error)
	}
}

func TestAbstract_ChainOfAbstracts(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":            "gts.x.testmod.abs.chain.v1~",
		"type":           "object",
		"x-gts-abstract": true,
		"required":       []any{"id"},
		"properties":     map[string]any{"id": map[string]any{"type": "string"}},
	})
	mustRegisterMod(t, store, map[string]any{
		"$id":            "gts.x.testmod.abs.chain.v1~x.testmod._.mid.v1~",
		"type":           "object",
		"x-gts-abstract": true,
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.abs.chain.v1~"},
			map[string]any{"type": "object"},
		},
	})
	mustRegisterMod(t, store, map[string]any{
		"$id":  "gts.x.testmod.abs.chain.v1~x.testmod._.mid.v1~x.testmod._.leaf.v1~",
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.abs.chain.v1~x.testmod._.mid.v1~"},
			map[string]any{"type": "object"},
		},
	})
	// Instance of concrete leaf — should pass
	mustRegisterInstance(t, store, map[string]any{
		"id": "gts.x.testmod.abs.chain.v1~x.testmod._.mid.v1~x.testmod._.leaf.v1~x.testmod._.item.v1",
	})
	result := store.ValidateInstance("gts.x.testmod.abs.chain.v1~x.testmod._.mid.v1~x.testmod._.leaf.v1~x.testmod._.item.v1")
	if !result.OK {
		t.Errorf("instance of concrete leaf should pass, got: %s", result.Error)
	}
	// Instance of abstract mid — should fail
	mustRegisterInstance(t, store, map[string]any{
		"id": "gts.x.testmod.abs.chain.v1~x.testmod._.mid.v1~x.testmod._.direct.v1",
	})
	result2 := store.ValidateInstance("gts.x.testmod.abs.chain.v1~x.testmod._.mid.v1~x.testmod._.direct.v1")
	if result2.OK {
		t.Error("instance of abstract mid should fail")
	}
}

func TestAbstract_FalseIsNoop(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":            "gts.x.testmod.absfalse.base.v1~",
		"type":           "object",
		"x-gts-abstract": false,
		"required":       []any{"id"},
		"properties":     map[string]any{"id": map[string]any{"type": "string"}},
	})
	mustRegisterInstance(t, store, map[string]any{
		"id": "gts.x.testmod.absfalse.base.v1~x.testmod._.item.v1",
	})
	result := store.ValidateInstance("gts.x.testmod.absfalse.base.v1~x.testmod._.item.v1")
	if !result.OK {
		t.Errorf("abstract=false should allow instances, got: %s", result.Error)
	}
}

// =============================================================================
// Interaction tests
// =============================================================================

func TestBothModifiers_Rejected(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":            "gts.x.testmod.both.invalid.v1~",
		"type":           "object",
		"x-gts-final":    true,
		"x-gts-abstract": true,
		"properties":     map[string]any{"name": map[string]any{"type": "string"}},
	})
	result := store.ValidateEntity("gts.x.testmod.both.invalid.v1~")
	if result.OK {
		t.Error("expected validation to fail for both final+abstract")
	}
}

func TestAbstractBaseFinalDerived(t *testing.T) {
	store := NewGtsStore(nil)
	// Abstract base
	mustRegisterMod(t, store, map[string]any{
		"$id":            "gts.x.testmod.absfinal.base.v1~",
		"type":           "object",
		"x-gts-abstract": true,
		"required":       []any{"id", "name"},
		"properties": map[string]any{
			"id":   map[string]any{"type": "string"},
			"name": map[string]any{"type": "string"},
		},
	})
	// Concrete + final derived
	mustRegisterMod(t, store, map[string]any{
		"$id":         "gts.x.testmod.absfinal.base.v1~x.testmod._.concrete.v1~",
		"type":        "object",
		"x-gts-final": true,
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.absfinal.base.v1~"},
			map[string]any{"type": "object", "properties": map[string]any{"extra": map[string]any{"type": "string"}}},
		},
	})
	// B is valid schema
	chainResult := store.ValidateSchemaChain("gts.x.testmod.absfinal.base.v1~x.testmod._.concrete.v1~")
	if !chainResult.OK {
		t.Errorf("concrete derived from abstract should pass, got: %s", chainResult.Error)
	}
	// Instance of B — should pass
	mustRegisterInstance(t, store, map[string]any{
		"id":    "gts.x.testmod.absfinal.base.v1~x.testmod._.concrete.v1~x.testmod._.item.v1",
		"name":  "My Item",
		"extra": "value",
	})
	instResult := store.ValidateInstance("gts.x.testmod.absfinal.base.v1~x.testmod._.concrete.v1~x.testmod._.item.v1")
	if !instResult.OK {
		t.Errorf("instance of concrete final should pass, got: %s", instResult.Error)
	}
	// Derived from B — should fail (B is final)
	mustRegisterMod(t, store, map[string]any{
		"$id":  "gts.x.testmod.absfinal.base.v1~x.testmod._.concrete.v1~x.testmod._.sub.v1~",
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.absfinal.base.v1~x.testmod._.concrete.v1~"},
			map[string]any{"type": "object"},
		},
	})
	subResult := store.ValidateSchemaChain("gts.x.testmod.absfinal.base.v1~x.testmod._.concrete.v1~x.testmod._.sub.v1~")
	if subResult.OK {
		t.Error("derived from final should fail")
	}
	// Direct instance of A — should fail (A is abstract)
	mustRegisterInstance(t, store, map[string]any{
		"id":   "gts.x.testmod.absfinal.base.v1~x.testmod._.direct.v1",
		"name": "Direct from abstract",
	})
	directResult := store.ValidateInstance("gts.x.testmod.absfinal.base.v1~x.testmod._.direct.v1")
	if directResult.OK {
		t.Error("direct instance of abstract should fail")
	}
}

func TestSchemaKeywordsInInstance_FinalRejected(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":      "gts.x.testmod.kwininst.base.v1~",
		"type":     "object",
		"required": []any{"id"},
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	})
	mustRegisterInstance(t, store, map[string]any{
		"id":          "gts.x.testmod.kwininst.base.v1~x.testmod._.item.v1",
		"x-gts-final": true,
	})
	result := store.ValidateEntity("gts.x.testmod.kwininst.base.v1~x.testmod._.item.v1")
	if result.OK {
		t.Error("instance with x-gts-final should fail entity validation")
	}
	if !strings.Contains(result.Error, "x-gts-final") {
		t.Errorf("error should mention x-gts-final, got: %s", result.Error)
	}
}

func TestSchemaKeywordsInInstance_AbstractRejected(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":      "gts.x.testmod.kwininst2.base.v1~",
		"type":     "object",
		"required": []any{"id"},
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	})
	mustRegisterInstance(t, store, map[string]any{
		"id":             "gts.x.testmod.kwininst2.base.v1~x.testmod._.item.v1",
		"x-gts-abstract": true,
	})
	result := store.ValidateEntity("gts.x.testmod.kwininst2.base.v1~x.testmod._.item.v1")
	if result.OK {
		t.Error("instance with x-gts-abstract should fail entity validation")
	}
	if !strings.Contains(result.Error, "x-gts-abstract") {
		t.Errorf("error should mention x-gts-abstract, got: %s", result.Error)
	}
}

func TestFinal_WithTraitsFullyResolved(t *testing.T) {
	store := NewGtsStore(nil)
	// Base with trait schema (required priority, no default)
	mustRegisterMod(t, store, map[string]any{
		"$id":  "gts.x.testmod.finaltrait.base.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"retention": map[string]any{"type": "string", "default": "P30D"},
				"priority":  map[string]any{"type": "integer"},
			},
		},
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	})
	// Final derived that provides all traits
	mustRegisterMod(t, store, map[string]any{
		"$id":         "gts.x.testmod.finaltrait.base.v1~x.testmod._.leaf.v1~",
		"type":        "object",
		"x-gts-final": true,
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.finaltrait.base.v1~"},
			map[string]any{
				"type": "object",
				"x-gts-traits": map[string]any{
					"priority": 5,
				},
			},
		},
	})
	result := store.ValidateSchemaTraits("gts.x.testmod.finaltrait.base.v1~x.testmod._.leaf.v1~")
	if !result.OK {
		t.Errorf("final with resolved traits should pass, got: %s", result.Error)
	}
}

func TestFinal_WithTraitsMissing(t *testing.T) {
	store := NewGtsStore(nil)
	// Base with required trait (no default)
	mustRegisterMod(t, store, map[string]any{
		"$id":  "gts.x.testmod.finalmiss.base.v1~",
		"type": "object",
		"x-gts-traits-schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"priority": map[string]any{"type": "integer"},
			},
			"required": []any{"priority"},
		},
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	})
	// Final derived that does NOT provide the required trait
	mustRegisterMod(t, store, map[string]any{
		"$id":         "gts.x.testmod.finalmiss.base.v1~x.testmod._.leaf.v1~",
		"type":        "object",
		"x-gts-final": true,
		"allOf": []any{
			map[string]any{"$ref": "gts.x.testmod.finalmiss.base.v1~"},
			map[string]any{"type": "object"},
		},
	})
	result := store.ValidateSchemaTraits("gts.x.testmod.finalmiss.base.v1~x.testmod._.leaf.v1~")
	if result.OK {
		t.Error("final with missing required traits should fail")
	}
	if !strings.Contains(result.Error, "priority") {
		t.Errorf("error should mention missing trait 'priority', got: %s", result.Error)
	}
}

func TestAbstract_WithIncompleteTraitsOk(t *testing.T) {
	store := NewGtsStore(nil)
	// Abstract base with required trait (no default) — should pass because abstract
	mustRegisterMod(t, store, map[string]any{
		"$id":            "gts.x.testmod.abstrait.base.v1~",
		"type":           "object",
		"x-gts-abstract": true,
		"x-gts-traits-schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"priority": map[string]any{"type": "integer"},
			},
			"required": []any{"priority"},
		},
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	})
	result := store.ValidateSchemaTraits("gts.x.testmod.abstrait.base.v1~")
	if !result.OK {
		t.Errorf("abstract with incomplete traits should pass, got: %s", result.Error)
	}
}

func TestFinal_NonBooleanRejected(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":         "gts.x.testmod.finalbadval.base.v1~",
		"type":        "object",
		"x-gts-final": "yes",
		"properties":  map[string]any{"name": map[string]any{"type": "string"}},
	})
	result := store.ValidateEntity("gts.x.testmod.finalbadval.base.v1~")
	if result.OK {
		t.Error("non-boolean x-gts-final should fail entity validation")
	}
}

func TestAbstract_NonBooleanRejected(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegisterMod(t, store, map[string]any{
		"$id":            "gts.x.testmod.absbadval.base.v1~",
		"type":           "object",
		"x-gts-abstract": 1.0,
		"properties":     map[string]any{"name": map[string]any{"type": "string"}},
	})
	result := store.ValidateEntity("gts.x.testmod.absbadval.base.v1~")
	if result.OK {
		t.Error("non-boolean x-gts-abstract should fail entity validation")
	}
}
