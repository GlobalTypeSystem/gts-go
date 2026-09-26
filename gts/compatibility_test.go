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
		"$id": "gts://gts.x.core.compat.event.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "timestamp", "userId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "timestamp": map[string]any{"type": "string"},
			"userId": map[string]any{"type": "string"}, "metadata": map[string]any{"type": "object"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.event.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.breaking.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.breaking.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.forward.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":           map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": true,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.forward.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.fwd_break.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "importantField"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "importantField": map[string]any{"type": "string"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.fwd_break.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}},
	})

	result := store.CheckCompatibility("gts.x.core.compat.fwd_break.v1.0~", "gts.x.core.compat.fwd_break.v1.1~")
	assertCompat(t, result, VerdictCompatible, VerdictIncompatible, VerdictIncompatible)
}

func TestCheckCompatibility_FullyCompatible_AnnotationsOnly(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.full.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string", "description": "Event identifier"},
		},
		"additionalProperties": true,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.full.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.closed_add.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":           map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.closed_add.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.closed_rm.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.closed_rm.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":           map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})

	result := store.CheckCompatibility("gts.x.core.compat.closed_rm.v1.0~", "gts.x.core.compat.closed_rm.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

func TestCheckCompatibility_ClosedModel_RemoveRequired(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.closed_req.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "importantField"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "importantField": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.closed_req.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":           map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})

	result := store.CheckCompatibility("gts.x.core.compat.closed_req.v1.0~", "gts.x.core.compat.closed_req.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictIncompatible, VerdictIncompatible)
}

// ── Content model transitions ───────────────────────────────────────────────

func TestCheckCompatibility_ClosingOpenObject(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.closing.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":           map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": true,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.closing.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":           map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})

	result := store.CheckCompatibility("gts.x.core.compat.closing.v1.0~", "gts.x.core.compat.closing.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)
}

func TestCheckCompatibility_OpeningClosedObject(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.opening.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":           map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.opening.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":           map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": true,
	})

	result := store.CheckCompatibility("gts.x.core.compat.opening.v1.0~", "gts.x.core.compat.opening.v1.1~")
	assertCompat(t, result, VerdictCompatible, VerdictIncompatible, VerdictIncompatible)
}

// ── Type changes ────────────────────────────────────────────────────────────

func TestCheckCompatibility_TypeChange(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.typechange.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "count"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "count": map[string]any{"type": "number"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.typechange.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.widen.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "amount"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "amount": map[string]any{"type": "integer"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.widen.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.narrow.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "amount"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "amount": map[string]any{"type": "number"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.narrow.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.enum.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "status"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"status":  map[string]any{"type": "string", "enum": []any{"active", "inactive"}},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.enum.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.enum_red.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "status"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"status":  map[string]any{"type": "string", "enum": []any{"active", "inactive", "pending"}},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.enum_red.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.const_id.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "kind"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"kind":    map[string]any{"type": "string", "const": "gts.x.core.compat.const_id.v1.0~"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.const_id.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.constraints.product.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"productId", "price"},
		"properties": map[string]any{
			"productId": map[string]any{"type": "string"},
			"price":     map[string]any{"type": "number", "minimum": float64(0), "maximum": float64(1000)},
			"name":      map[string]any{"type": "string", "minLength": float64(3), "maxLength": float64(50)},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.constraints.product.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.tight.item.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"itemId", "quantity"},
		"properties": map[string]any{
			"itemId":   map[string]any{"type": "string"},
			"quantity": map[string]any{"type": "integer", "minimum": float64(1), "maximum": float64(1000)},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.tight.item.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.nested_compat.order.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"orderId", "customer"},
		"properties": map[string]any{
			"orderId": map[string]any{"type": "string"},
			"customer": map[string]any{"type": "object", "required": []any{"customerId", "name"}, "properties": map[string]any{
				"customerId": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"},
			}},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.nested_compat.order.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.array_compat.list.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.array_compat.list.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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
		"$id": "gts://gts.x.core.compat.rename.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "userId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "userId": map[string]any{"type": "string"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.compat.rename.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
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

// ── OP#8 structured diagnostics (#2) ────────────────────────────────────────

func TestCheckCompatibility_Diagnostics_Incompatible(t *testing.T) {
	store := NewGtsStore(nil)
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.diag.thing.v1.0~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}},
	})
	registerSchema(t, store, map[string]any{
		"$id": "gts://gts.x.core.diag.thing.v1.1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "newRequiredField"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "newRequiredField": map[string]any{"type": "string"},
		},
	})

	result := store.CheckCompatibility("gts.x.core.diag.thing.v1.0~", "gts.x.core.diag.thing.v1.1~")
	assertCompat(t, result, VerdictIncompatible, VerdictCompatible, VerdictIncompatible)

	if result.IsBackwardCompatible || !result.IsForwardCompatible || result.IsFullyCompatible {
		t.Errorf("is_*_compatible flags disagree with verdicts: %+v", result)
	}
	if len(result.BackwardErrors) == 0 {
		t.Error("expected backward_errors for the incompatible direction")
	}
	if len(result.ForwardErrors) != 0 {
		t.Errorf("expected no forward_errors, got %v", result.ForwardErrors)
	}
	// Diagnostics must reconstruct the directional verdicts exactly.
	var backwardDiags, forwardDiags []CompatibilityDiagnostic
	for _, d := range result.Diagnostics {
		if d.Direction == CompatDirectionBackward {
			backwardDiags = append(backwardDiags, d)
		} else {
			forwardDiags = append(forwardDiags, d)
		}
	}
	if got := verdictFromDiagnostics(backwardDiags); got != result.BackwardCompatibility {
		t.Errorf("verdictFromDiagnostics(backward) = %q, want %q", got, result.BackwardCompatibility)
	}
	if got := verdictFromDiagnostics(forwardDiags); got != result.ForwardCompatibility {
		t.Errorf("verdictFromDiagnostics(forward) = %q, want %q", got, result.ForwardCompatibility)
	}
}

func TestCheckCompatibility_Diagnostics_MissingSchema(t *testing.T) {
	store := NewGtsStore(nil)
	result := store.CheckCompatibility("gts.x.core.diag.absent.v1.0~", "gts.x.core.diag.absent.v1.1~")
	assertCompat(t, result, VerdictUnknown, VerdictUnknown, VerdictUnknown)
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected diagnostics explaining the missing schemas")
	}
	for _, d := range result.Diagnostics {
		if d.Verdict != VerdictUnknown {
			t.Errorf("missing-schema diagnostic verdict = %q, want unknown", d.Verdict)
		}
	}
	// Slices must serialise as [] not null.
	if result.BackwardErrors == nil || result.ForwardErrors == nil || result.CandidateObjectLevels == nil {
		t.Error("result slices must be non-nil")
	}
}

func TestVerdictFromDiagnostics(t *testing.T) {
	if got := verdictFromDiagnostics(nil); got != VerdictCompatible {
		t.Errorf("no diagnostics: want compatible, got %q", got)
	}
	unknownOnly := []CompatibilityDiagnostic{{Verdict: VerdictUnknown}}
	if got := verdictFromDiagnostics(unknownOnly); got != VerdictUnknown {
		t.Errorf("unknown only: want unknown, got %q", got)
	}
	mixed := []CompatibilityDiagnostic{{Verdict: VerdictUnknown}, {Verdict: VerdictIncompatible}}
	if got := verdictFromDiagnostics(mixed); got != VerdictIncompatible {
		t.Errorf("incompatible dominates: want incompatible, got %q", got)
	}
}

// ── OP#8 content-model classification (#3) ──────────────────────────────────

func TestClassifyObjectLevels(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false, // root: closed
		"properties": map[string]any{
			"openBag": map[string]any{
				"type":                 "object",
				"additionalProperties": true, // open
			},
			"partialBag": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"}, // schema-valued -> partial
			},
		},
	}
	levels := classifyObjectLevels(schema)
	got := map[string]string{}
	for _, l := range levels {
		got[l.Path] = l.ContentModel
	}
	if got["$"] != ContentModelClosed {
		t.Errorf("root: want closed, got %q", got["$"])
	}
	if got["$.openBag"] != ContentModelOpen {
		t.Errorf("$.openBag: want open, got %q", got["$.openBag"])
	}
	if got["$.partialBag"] != ContentModelPartial {
		t.Errorf("$.partialBag: want partially_open, got %q", got["$.partialBag"])
	}
}

func TestBooleanSchemaValue(t *testing.T) {
	cases := []struct {
		name   string
		schema any
		want   *bool
	}{
		{"true", true, boolPtr(true)},
		{"false", false, boolPtr(false)},
		{"empty object", map[string]any{}, boolPtr(true)},
		{"annotations only", map[string]any{"title": "x", "description": "y"}, boolPtr(true)},
		{"not empty", map[string]any{"not": map[string]any{}}, boolPtr(false)},
		{"constrained", map[string]any{"type": "string"}, nil},
		{"multiple assertions", map[string]any{"type": "string", "minLength": float64(1)}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := booleanSchemaValue(tc.schema)
			switch {
			case tc.want == nil && got != nil:
				t.Errorf("want nil, got %v", *got)
			case tc.want != nil && got == nil:
				t.Errorf("want %v, got nil", *tc.want)
			case tc.want != nil && got != nil && *got != *tc.want:
				t.Errorf("want %v, got %v", *tc.want, *got)
			}
		})
	}
}

func TestClassifyObjectLevels_DepthGuard(t *testing.T) {
	// Deeper than the cap: the classifier must terminate, not overflow.
	node := map[string]any{"type": "object", "additionalProperties": false}
	for i := 0; i < maxCompatRecursionDepth+10; i++ {
		node = map[string]any{
			"type":       "object",
			"properties": map[string]any{"child": node},
		}
	}
	levels := classifyObjectLevels(node)
	if len(levels) == 0 {
		t.Error("expected at least the shallow levels to be classified")
	}
}

// ── content-model classifier: gts-rust parity (#1–#4) ───────────────────────

func levelMap(schema map[string]any) map[string]string {
	out := map[string]string{}
	for _, l := range classifyObjectLevels(schema) {
		out[l.Path] = l.ContentModel
	}
	return out
}

func TestClassifyObjectLevels_PatternProperties(t *testing.T) {
	// All patterns closed + additionalProperties:false → closed.
	got := levelMap(map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"patternProperties":    map[string]any{"^x-": false},
	})
	if got["$"] != ContentModelClosed {
		t.Errorf("all-closed patterns + AP:false: want closed, got %q", got["$"])
	}

	// All patterns open + additionalProperties:true → open.
	got = levelMap(map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"patternProperties":    map[string]any{"^x-": true},
	})
	if got["$"] != ContentModelOpen {
		t.Errorf("all-open patterns + AP:true: want open, got %q", got["$"])
	}

	// A constraining pattern → partially_open.
	got = levelMap(map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"patternProperties":    map[string]any{"^x-": map[string]any{"type": "string"}},
	})
	if got["$"] != ContentModelPartial {
		t.Errorf("constraining pattern: want partially_open, got %q", got["$"])
	}
}

func TestClassifyObjectLevels_PropertyNames(t *testing.T) {
	// Boolean propertyNames:false closes the level regardless of the fallback.
	got := levelMap(map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"propertyNames":        false,
	})
	if got["$"] != ContentModelClosed {
		t.Errorf("propertyNames:false: want closed, got %q", got["$"])
	}

	// A schema-valued propertyNames only constrains → partially_open.
	got = levelMap(map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"propertyNames":        map[string]any{"pattern": "^[a-z]+$"},
	})
	if got["$"] != ContentModelPartial {
		t.Errorf("schema-valued propertyNames: want partially_open, got %q", got["$"])
	}
}

func TestClassifyObjectLevels_UnevaluatedPropertiesIsDialectAware(t *testing.T) {
	// 2020-12 evaluates unevaluatedProperties, so it acts as the fallback.
	got := levelMap(map[string]any{
		"$schema":               "https://json-schema.org/draft/2020-12/schema",
		"type":                  "object",
		"unevaluatedProperties": false,
		"properties":            map[string]any{"a": map[string]any{"type": "string"}},
	})
	if got["$"] != ContentModelClosed {
		t.Errorf("2020-12 unevaluatedProperties:false: want closed, got %q", got["$"])
	}

	// draft-07 does not evaluate unevaluatedProperties, so it is not a fallback;
	// with no additionalProperties the level stays open.
	got = levelMap(map[string]any{
		"$schema":               "http://json-schema.org/draft-07/schema#",
		"type":                  "object",
		"unevaluatedProperties": false,
		"properties":            map[string]any{"a": map[string]any{"type": "string"}},
	})
	if got["$"] != ContentModelOpen {
		t.Errorf("draft-07 unevaluatedProperties:false: want open, got %q", got["$"])
	}
}

func TestClassifyObjectLevels_FlattensAllOf(t *testing.T) {
	// allOf carrying additionalProperties:false closes the effective level.
	got := levelMap(map[string]any{
		"type": "object",
		"allOf": []any{
			map[string]any{"additionalProperties": false},
			map[string]any{"properties": map[string]any{"a": map[string]any{"type": "string"}}},
		},
	})
	if got["$"] != ContentModelClosed {
		t.Errorf("allOf additionalProperties:false: want closed, got %q", got["$"])
	}
}

func TestClassifyObjectLevels_IgnoresBranchLevels(t *testing.T) {
	// Object levels reachable only through anyOf/oneOf have no single content
	// model and must not be reported (only $ here).
	levels := classifyObjectLevels(map[string]any{
		"type": "object",
		"anyOf": []any{
			map[string]any{"type": "object", "additionalProperties": false},
			map[string]any{"type": "object", "additionalProperties": true},
		},
	})
	if len(levels) != 1 || levels[0].Path != "$" {
		t.Errorf("anyOf branches must not be reported as levels, got %+v", levels)
	}
}
