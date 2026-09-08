/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"testing"
)

// assertCompat is a helper that checks all three verdict fields.
func assertCompat(t *testing.T, result *CompatibilityResult, backward, forward, full string) {
	t.Helper()
	if result.BackwardCompatibility != backward {
		t.Errorf("backward_compatibility: want %q, got %q", backward, result.BackwardCompatibility)
	}
	if result.ForwardCompatibility != forward {
		t.Errorf("forward_compatibility: want %q, got %q", forward, result.ForwardCompatibility)
	}
	if result.FullCompatibility != full {
		t.Errorf("full_compatibility: want %q, got %q", full, result.FullCompatibility)
	}
}

func registerSchema(t *testing.T, store *GtsStore, schema map[string]any) {
	t.Helper()
	entity := NewJsonEntity(schema, DefaultGtsConfig())
	if err := store.Register(entity); err != nil {
		t.Fatalf("Failed to register schema: %v", err)
	}
}

// ── Open-model tests ────────────────────────────────────────────────────────

func TestCheckCompatibility_BackwardCompatible_RemoveOptionalOpen(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.event.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "timestamp", "userId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "timestamp": map[string]any{"type": "string"},
			"userId": map[string]any{"type": "string"}, "metadata": map[string]any{"type": "object"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.event.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "timestamp", "userId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "timestamp": map[string]any{"type": "string"},
			"userId": map[string]any{"type": "string"},
		},
	})

	result := store.CheckCompatibility("gts.x.core.compat.event.v1.0~", "gts.x.core.compat.event.v1.1~")
	assertCompat(t, result, VerdictCompatible, VerdictIncompatible, VerdictIncompatible)
	if result.OldID != "gts.x.core.compat.event.v1.0~" {
		t.Errorf("old: want gts.x.core.compat.event.v1.0~, got %s", result.OldID)
	}
	if result.NewID != "gts.x.core.compat.event.v1.1~" {
		t.Errorf("new: want gts.x.core.compat.event.v1.1~, got %s", result.NewID)
	}
}

func TestCheckCompatibility_BackwardIncompatible_AddRequired(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.breaking.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.breaking.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "newRequiredField"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "newRequiredField": map[string]any{"type": "string"},
		},
	})

	result := store.CheckCompatibility("gts.x.core.compat.breaking.v1.0~", "gts.x.core.compat.breaking.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

func TestCheckCompatibility_ForwardCompatible_AddOptionalOpen(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.forward.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":            map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": true,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.forward.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "newField": map[string]any{"type": "string"},
		},
		"additionalProperties": true,
	})

	result := store.CheckCompatibility("gts.x.core.compat.forward.v1.0~", "gts.x.core.compat.forward.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

func TestCheckCompatibility_ForwardIncompatible_RemoveRequired(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.fwd_break.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "importantField"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "importantField": map[string]any{"type": "string"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.fwd_break.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}},
	})

	result := store.CheckCompatibility("gts.x.core.compat.fwd_break.v1.0~", "gts.x.core.compat.fwd_break.v1.1~")
	assertCompat(t, result, VerdictCompatible, VerdictIncompatible, VerdictIncompatible)
}

func TestCheckCompatibility_FullyCompatible_AnnotationsOnly(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.full.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string", "description": "Event identifier"},
		},
		"additionalProperties": true,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.full.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string", "description": "Stable event identifier", "examples": []any{"evt-123"}},
		},
		"additionalProperties": true,
	})

	result := store.CheckCompatibility("gts.x.core.compat.full.v1.0~", "gts.x.core.compat.full.v1.1~")
	assertCompat(t, result, VerdictCompatible, VerdictCompatible, VerdictCompatible)
}

// ── Closed-model tests ──────────────────────────────────────────────────────

func TestCheckCompatibility_ClosedModel_AddOptional(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.closed_add.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":            map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.closed_add.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	})

	result := store.CheckCompatibility("gts.x.core.compat.closed_add.v1.0~", "gts.x.core.compat.closed_add.v1.1~")
	assertCompat(t, result, VerdictCompatible, VerdictIncompatible, VerdictIncompatible)
}

func TestCheckCompatibility_ClosedModel_RemoveOptional(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.closed_rm.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.closed_rm.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":            map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})

	result := store.CheckCompatibility("gts.x.core.compat.closed_rm.v1.0~", "gts.x.core.compat.closed_rm.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

func TestCheckCompatibility_ClosedModel_RemoveRequired(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.closed_req.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "importantField"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "importantField": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.closed_req.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":            map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})

	result := store.CheckCompatibility("gts.x.core.compat.closed_req.v1.0~", "gts.x.core.compat.closed_req.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictIncompatible, VerdictIncompatible)
}

// ── Content model transitions ───────────────────────────────────────────────

func TestCheckCompatibility_ClosingOpenObject(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.closing.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":            map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": true,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.closing.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":            map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})

	result := store.CheckCompatibility("gts.x.core.compat.closing.v1.0~", "gts.x.core.compat.closing.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

func TestCheckCompatibility_OpeningClosedObject(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.opening.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":            map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.opening.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":            map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": true,
	})

	result := store.CheckCompatibility("gts.x.core.compat.opening.v1.0~", "gts.x.core.compat.opening.v1.1~")
	assertCompat(t, result, VerdictCompatible, VerdictIncompatible, VerdictIncompatible)
}

// ── Type changes ────────────────────────────────────────────────────────────

func TestCheckCompatibility_TypeChange(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.typechange.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "count"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "count": map[string]any{"type": "number"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.typechange.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "count"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "count": map[string]any{"type": "string"},
		},
	})

	result := store.CheckCompatibility("gts.x.core.compat.typechange.v1.0~", "gts.x.core.compat.typechange.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictIncompatible, VerdictIncompatible)
}

func TestCheckCompatibility_NumericWidening(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.widen.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "amount"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "amount": map[string]any{"type": "integer"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.widen.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "amount"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "amount": map[string]any{"type": "number"},
		},
	})

	result := store.CheckCompatibility("gts.x.core.compat.widen.v1.0~", "gts.x.core.compat.widen.v1.1~")
	assertCompat(t, result, VerdictCompatible, VerdictIncompatible, VerdictIncompatible)
}

func TestCheckCompatibility_NumericNarrowing(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.narrow.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "amount"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "amount": map[string]any{"type": "number"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.narrow.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "amount"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "amount": map[string]any{"type": "integer"},
		},
	})

	result := store.CheckCompatibility("gts.x.core.compat.narrow.v1.0~", "gts.x.core.compat.narrow.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

// ── Enum changes ────────────────────────────────────────────────────────────

func TestCheckCompatibility_EnumExpansion(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.enum.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "status"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"status":  map[string]any{"type": "string", "enum": []any{"active", "inactive"}},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.enum.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "status"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"status":  map[string]any{"type": "string", "enum": []any{"active", "inactive", "pending"}},
		},
	})

	result := store.CheckCompatibility("gts.x.core.compat.enum.v1.0~", "gts.x.core.compat.enum.v1.1~")
	assertCompat(t, result, VerdictCompatible, VerdictIncompatible, VerdictIncompatible)
}

func TestCheckCompatibility_EnumReduction(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.enum_red.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "status"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"status":  map[string]any{"type": "string", "enum": []any{"active", "inactive", "pending"}},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.enum_red.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "status"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"status":  map[string]any{"type": "string", "enum": []any{"active", "inactive"}},
		},
	})

	result := store.CheckCompatibility("gts.x.core.compat.enum_red.v1.0~", "gts.x.core.compat.enum_red.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

// ── Const changes ───────────────────────────────────────────────────────────

func TestCheckCompatibility_ConstIdentityChange(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.const_id.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "kind"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"kind":    map[string]any{"type": "string", "const": "gts.x.core.compat.const_id.v1.0~"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.const_id.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "kind"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"kind":    map[string]any{"type": "string", "const": "gts.x.core.compat.const_id.v1.1~"},
		},
	})

	result := store.CheckCompatibility("gts.x.core.compat.const_id.v1.0~", "gts.x.core.compat.const_id.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictIncompatible, VerdictIncompatible)
}

// ── Constraint changes ──────────────────────────────────────────────────────

func TestCheckCompatibility_ConstraintRelaxation(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.constraints.product.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"productId", "price"},
		"properties": map[string]any{
			"productId": map[string]any{"type": "string"},
			"price":     map[string]any{"type": "number", "minimum": float64(0), "maximum": float64(1000)},
			"name":      map[string]any{"type": "string", "minLength": float64(3), "maxLength": float64(50)},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.constraints.product.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"productId", "price"},
		"properties": map[string]any{
			"productId": map[string]any{"type": "string"},
			"price":     map[string]any{"type": "number", "minimum": float64(0), "maximum": float64(10000)},
			"name":      map[string]any{"type": "string", "minLength": float64(1), "maxLength": float64(100)},
		},
	})

	result := store.CheckCompatibility("gts.x.core.constraints.product.v1.0~", "gts.x.core.constraints.product.v1.1~")
	assertCompat(t, result, VerdictCompatible, VerdictIncompatible, VerdictIncompatible)
}

func TestCheckCompatibility_ConstraintTightening(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.tight.item.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"itemId", "quantity"},
		"properties": map[string]any{
			"itemId":   map[string]any{"type": "string"},
			"quantity": map[string]any{"type": "integer", "minimum": float64(1), "maximum": float64(1000)},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.tight.item.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"itemId", "quantity"},
		"properties": map[string]any{
			"itemId":   map[string]any{"type": "string"},
			"quantity": map[string]any{"type": "integer", "minimum": float64(1), "maximum": float64(100)},
		},
	})

	result := store.CheckCompatibility("gts.x.core.tight.item.v1.0~", "gts.x.core.tight.item.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

// ── Nested / array ──────────────────────────────────────────────────────────

func TestCheckCompatibility_NestedObjectChanges(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.nested_compat.order.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"orderId", "customer"},
		"properties": map[string]any{
			"orderId": map[string]any{"type": "string"},
			"customer": map[string]any{"type": "object", "required": []any{"customerId", "name"}, "properties": map[string]any{
				"customerId": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"},
			}},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.nested_compat.order.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"orderId", "customer"},
		"properties": map[string]any{
			"orderId": map[string]any{"type": "string"},
			"customer": map[string]any{"type": "object", "required": []any{"customerId", "name"}, "properties": map[string]any{
				"customerId": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"},
				"email": map[string]any{"type": "string"},
			}},
		},
	})

	result := store.CheckCompatibility("gts.x.core.nested_compat.order.v1.0~", "gts.x.core.nested_compat.order.v1.1~")
	// Adding optional field to open nested object: forward only
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

func TestCheckCompatibility_ArrayItemSchemaChange(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.array_compat.list.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"listId", "items"},
		"properties": map[string]any{
			"listId": map[string]any{"type": "string"},
			"items": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "required": []any{"id", "value"}, "properties": map[string]any{
					"id": map[string]any{"type": "string"}, "value": map[string]any{"type": "number"},
				},
			}},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.array_compat.list.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"listId", "items"},
		"properties": map[string]any{
			"listId": map[string]any{"type": "string"},
			"items": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "required": []any{"id", "value"}, "properties": map[string]any{
					"id": map[string]any{"type": "string"}, "value": map[string]any{"type": "number"},
					"label": map[string]any{"type": "string"},
				},
			}},
		},
	})

	result := store.CheckCompatibility("gts.x.core.array_compat.list.v1.0~", "gts.x.core.array_compat.list.v1.1~")
	// Adding optional field to open array-item object: forward only
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

// ── Rename property ─────────────────────────────────────────────────────────

func TestCheckCompatibility_RenameProperty(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.rename.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "userId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "userId": map[string]any{"type": "string"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts.x.core.compat.rename.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "accountId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "accountId": map[string]any{"type": "string"},
		},
	})

	result := store.CheckCompatibility("gts.x.core.compat.rename.v1.0~", "gts.x.core.compat.rename.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictIncompatible, VerdictIncompatible)
}

// ── Entity not found ────────────────────────────────────────────────────────

func TestCheckCompatibility_EntityNotFound(t *testing.T) {
	store := NewGtsStore(nil)
	result := store.CheckCompatibility("gts.x.nonexistent.schema.v1.0~", "gts.x.nonexistent.schema.v1.1~")
	assertCompat(t, result, VerdictUnknown, VerdictUnknown, VerdictUnknown)
}

// ── Direction inference ─────────────────────────────────────────────────────

func TestInferDirection(t *testing.T) {
	tests := []struct {
		name     string
		fromID   string
		toID     string
		expected string
	}{
		{"Up direction (v1.0 to v1.1)", "gts.x.core.schema.test.v1.0~", "gts.x.core.schema.test.v1.1~", "up"},
		{"Down direction (v1.5 to v1.2)", "gts.x.core.schema.test.v1.5~", "gts.x.core.schema.test.v1.2~", "down"},
		{"None direction (same version)", "gts.x.core.schema.test.v1.0~", "gts.x.core.schema.test.v1.0~", "none"},
		{"Unknown direction (invalid ID)", "invalid", "also-invalid", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferDirection(tt.fromID, tt.toID)
			if result != tt.expected {
				t.Errorf("Expected direction %s, got %s", tt.expected, result)
			}
		})
	}
}
