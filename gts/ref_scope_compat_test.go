/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import "testing"

func TestNarrowerRefOperand_UnresolvedSelfRefIsKept(t *testing.T) {
	for _, tt := range []struct{ a, b any }{
		{XGtsRefSelf, "gts.x.scope.ns.a.v1~"},
		{"gts.x.scope.ns.a.v1~", XGtsRefSelf},
		{"not-a-gts-id", "gts.x.scope.ns.a.v1~"},
	} {
		if narrower, ok := narrowerRefOperand(KeyXGtsTypeRef, tt.a, tt.b); ok {
			t.Errorf("narrowerRefOperand(%v, %v) = %v, want incomparable so both are kept", tt.a, tt.b, narrower)
		}
	}
}

func TestMergeEffectiveProperty_KeepsUnresolvedSelfRefAsConjunction(t *testing.T) {
	prev := map[string]any{"type": "string", KeyXGtsTypeRef: XGtsRefSelf}
	next := map[string]any{"type": "string", KeyXGtsTypeRef: "gts.x.scope.ns.a.v1~"}

	merged, _ := mergeEffectiveProperty(prev, next).(map[string]any)
	if _, flattened := merged[KeyXGtsTypeRef]; flattened {
		t.Fatalf("incomparable operands must not be flattened to one keyword: %v", merged)
	}
	allOf, _ := merged["allOf"].([]any)
	if len(allOf) != 2 {
		t.Fatalf("want both operands kept as an allOf conjunction, got %v", merged)
	}
}

// An inherited "/$id" reference must survive the allOf merge with a wider
// conjunct in the derived schema: C's effective scope is C ∧ A = C ⊆ B.
func TestValidateSchemaChain_InheritedSelfRefSurvivesWiderConjunct(t *testing.T) {
	store := NewGtsStore(nil)
	const (
		a = "gts.x.scope.ns.a.v1~"
		b = a + "x.scope.ns.b.v1~"
		c = b + "x.scope.ns.c.v1~"
	)
	mustRegister(t, store, map[string]any{
		"$id":        "gts://" + a,
		"type":       "object",
		"properties": map[string]any{"p": map[string]any{"type": "string"}},
	})
	mustRegister(t, store, map[string]any{
		"$id":  "gts://" + b,
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "gts://" + a},
			map[string]any{"properties": map[string]any{
				"p": map[string]any{"type": "string", KeyXGtsTypeRef: XGtsRefSelf},
			}},
		},
	})
	mustRegister(t, store, map[string]any{
		"$id":  "gts://" + c,
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "gts://" + b},
			map[string]any{"properties": map[string]any{
				"p": map[string]any{"type": "string", KeyXGtsTypeRef: a},
			}},
		},
	})

	if res := store.ValidateSchemaChain(c); !res.OK {
		t.Fatalf("admissible narrowing rejected: %s", res.Error)
	}
}
