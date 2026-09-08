/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

// This file contains integration tests that mirror the gts-spec v0.13.1
// conformance suite test_op8_compatibility_checking.py exactly.

import "testing"

// helper: register a schema using $$id/$$schema (as the conformance tests do)
func regSpec(t *testing.T, store *GtsStore, schema map[string]any) {
	t.Helper()
	// Normalize $$id → $id, $$schema → $schema (mimics the HTTP handler)
	if v, ok := schema["$$id"]; ok {
		schema["$id"] = v
		delete(schema, "$$id")
	}
	if v, ok := schema["$$schema"]; ok {
		schema["$schema"] = v
		delete(schema, "$$schema")
	}
	entity := NewJsonEntity(schema, DefaultGtsConfig())
	if err := store.Register(entity); err != nil {
		t.Fatalf("register: %v", err)
	}
}

func TestSpecOp8_BackwardCompatible(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.event.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "timestamp", "userId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "timestamp": map[string]any{"type": "string", "format": "date-time"},
			"userId": map[string]any{"type": "string"}, "metadata": map[string]any{"type": "object"},
		},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.event.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "timestamp", "userId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "timestamp": map[string]any{"type": "string", "format": "date-time"},
			"userId": map[string]any{"type": "string"},
		},
	})
	r := store.CheckCompatibility("gts.x.test8.compat.event.v1.0~", "gts.x.test8.compat.event.v1.1~")
	assertCompat(t, r, "compatible", "incompatible", "incompatible")
	if r.OldID != "gts.x.test8.compat.event.v1.0~" || r.NewID != "gts.x.test8.compat.event.v1.1~" {
		t.Errorf("echo: old=%q new=%q", r.OldID, r.NewID)
	}
}

func TestSpecOp8_BackwardIncompatible(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.breaking.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.breaking.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "newRequiredField"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "newRequiredField": map[string]any{"type": "string"},
		},
	})
	r := store.CheckCompatibility("gts.x.test8.compat.breaking.v1.0~", "gts.x.test8.compat.breaking.v1.1~")
	assertCompat(t, r, "incompatible", "compatible", "incompatible")
}

func TestSpecOp8_ForwardCompatible(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.forward.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties":            map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": true,
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.forward.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "newField": map[string]any{"type": "string"},
		},
		"additionalProperties": true,
	})
	r := store.CheckCompatibility("gts.x.test8.compat.forward.v1.0~", "gts.x.test8.compat.forward.v1.1~")
	assertCompat(t, r, "incompatible", "compatible", "incompatible")
}

func TestSpecOp8_FullyCompatible(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.full.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string", "description": "Event identifier"},
		},
		"additionalProperties": true,
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.full.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string", "description": "Stable event identifier", "examples": []any{"evt-123"}},
		},
		"additionalProperties": true,
	})
	r := store.CheckCompatibility("gts.x.test8.compat.full.v1.0~", "gts.x.test8.compat.full.v1.1~")
	assertCompat(t, r, "compatible", "compatible", "compatible")
}

func TestSpecOp8_ClosedModelAddOptional(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.closed_add.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}}, "additionalProperties": false,
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.closed_add.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"},
		}, "additionalProperties": false,
	})
	r := store.CheckCompatibility("gts.x.test8.compat.closed_add.v1.0~", "gts.x.test8.compat.closed_add.v1.1~")
	assertCompat(t, r, "compatible", "incompatible", "incompatible")
}

func TestSpecOp8_ClosedModelRemoveOptional(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.closed_remove.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"},
		}, "additionalProperties": false,
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.closed_remove.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}}, "additionalProperties": false,
	})
	r := store.CheckCompatibility("gts.x.test8.compat.closed_remove.v1.0~", "gts.x.test8.compat.closed_remove.v1.1~")
	assertCompat(t, r, "incompatible", "compatible", "incompatible")
}

func TestSpecOp8_TypeChange(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.typechange.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "count"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "count": map[string]any{"type": "number"}},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.typechange.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "count"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "count": map[string]any{"type": "string"}},
	})
	r := store.CheckCompatibility("gts.x.test8.compat.typechange.v1.0~", "gts.x.test8.compat.typechange.v1.1~")
	assertCompat(t, r, "incompatible", "incompatible", "incompatible")
}

func TestSpecOp8_EnumExpansion(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.enum.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "status"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "status": map[string]any{"type": "string", "enum": []any{"active", "inactive"}}},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.enum.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "status"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "status": map[string]any{"type": "string", "enum": []any{"active", "inactive", "pending"}}},
	})
	r := store.CheckCompatibility("gts.x.test8.compat.enum.v1.0~", "gts.x.test8.compat.enum.v1.1~")
	assertCompat(t, r, "compatible", "incompatible", "incompatible")
}

func TestSpecOp8_EnumReduction(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.enum_reduction.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "status"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "status": map[string]any{"type": "string", "enum": []any{"active", "inactive", "pending"}}},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.enum_reduction.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "status"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "status": map[string]any{"type": "string", "enum": []any{"active", "inactive"}}},
	})
	r := store.CheckCompatibility("gts.x.test8.compat.enum_reduction.v1.0~", "gts.x.test8.compat.enum_reduction.v1.1~")
	assertCompat(t, r, "incompatible", "compatible", "incompatible")
}

func TestSpecOp8_RemoveRequiredClosedModel(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.closed_req.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "importantField"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "importantField": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.closed_req.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}},
		"additionalProperties": false,
	})
	r := store.CheckCompatibility("gts.x.test8.compat.closed_req.v1.0~", "gts.x.test8.compat.closed_req.v1.1~")
	assertCompat(t, r, "incompatible", "incompatible", "incompatible")
}

func TestSpecOp8_ClosingOpenObject(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.closing.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}}, "additionalProperties": true,
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.closing.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}}, "additionalProperties": false,
	})
	r := store.CheckCompatibility("gts.x.test8.compat.closing.v1.0~", "gts.x.test8.compat.closing.v1.1~")
	assertCompat(t, r, "incompatible", "compatible", "incompatible")
}

func TestSpecOp8_OpeningClosedObject(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.opening.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}}, "additionalProperties": false,
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.opening.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}}, "additionalProperties": true,
	})
	r := store.CheckCompatibility("gts.x.test8.compat.opening.v1.0~", "gts.x.test8.compat.opening.v1.1~")
	assertCompat(t, r, "compatible", "incompatible", "incompatible")
}

func TestSpecOp8_ConstIdentityChange(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.const_id.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "kind"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"kind":    map[string]any{"type": "string", "const": "gts.x.test8.compat.const_id.v1.0~"},
		},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.const_id.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "kind"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"kind":    map[string]any{"type": "string", "const": "gts.x.test8.compat.const_id.v1.1~"},
		},
	})
	r := store.CheckCompatibility("gts.x.test8.compat.const_id.v1.0~", "gts.x.test8.compat.const_id.v1.1~")
	assertCompat(t, r, "incompatible", "incompatible", "incompatible")
}

func TestSpecOp8_RenameProperty(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.rename.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "userId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "userId": map[string]any{"type": "string"}},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.rename.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "accountId"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "accountId": map[string]any{"type": "string"}},
	})
	r := store.CheckCompatibility("gts.x.test8.compat.rename.v1.0~", "gts.x.test8.compat.rename.v1.1~")
	assertCompat(t, r, "incompatible", "incompatible", "incompatible")
}

func TestSpecOp8_NumericWidening(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.widen.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "amount"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "amount": map[string]any{"type": "integer"}},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.widen.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "amount"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "amount": map[string]any{"type": "number"}},
	})
	r := store.CheckCompatibility("gts.x.test8.compat.widen.v1.0~", "gts.x.test8.compat.widen.v1.1~")
	assertCompat(t, r, "compatible", "incompatible", "incompatible")
}

func TestSpecOp8_NumericNarrowing(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.narrow.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "amount"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "amount": map[string]any{"type": "number"}},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.narrow.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "amount"},
		"properties": map[string]any{"eventId": map[string]any{"type": "string"}, "amount": map[string]any{"type": "integer"}},
	})
	r := store.CheckCompatibility("gts.x.test8.compat.narrow.v1.0~", "gts.x.test8.compat.narrow.v1.1~")
	assertCompat(t, r, "incompatible", "compatible", "incompatible")
}

func TestSpecOp8_ReferencedTypeWidened(t *testing.T) {
	store := NewGtsStore(nil)
	// Register target v1.0 and v1.1
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.target.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"code"},
		"properties": map[string]any{"code": map[string]any{"type": "string", "enum": []any{"a", "b"}}},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.target.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"code"},
		"properties": map[string]any{"code": map[string]any{"type": "string", "enum": []any{"a", "b", "c"}}},
	})
	// Register container v1.0 referencing target v1.0 via $$ref
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.container.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "detail"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"detail":  map[string]any{"$$ref": "gts://gts.x.test8.compat.target.v1.0~"},
		},
	})
	// Register container v1.1 referencing target v1.1 via $$ref
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.compat.container.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"eventId", "detail"},
		"properties": map[string]any{
			"eventId": map[string]any{"type": "string"},
			"detail":  map[string]any{"$$ref": "gts://gts.x.test8.compat.target.v1.1~"},
		},
	})
	r := store.CheckCompatibility("gts.x.test8.compat.container.v1.0~", "gts.x.test8.compat.container.v1.1~")
	assertCompat(t, r, "compatible", "incompatible", "incompatible")
}

func TestSpecOp8_NestedObjectChanges(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.nested_compat.order.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"orderId", "customer"},
		"properties": map[string]any{
			"orderId": map[string]any{"type": "string"},
			"customer": map[string]any{"type": "object", "required": []any{"customerId", "name"}, "properties": map[string]any{
				"customerId": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"},
			}},
		},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.nested_compat.order.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"orderId", "customer"},
		"properties": map[string]any{
			"orderId": map[string]any{"type": "string"},
			"customer": map[string]any{"type": "object", "required": []any{"customerId", "name"}, "properties": map[string]any{
				"customerId": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}, "email": map[string]any{"type": "string"},
			}},
		},
	})
	r := store.CheckCompatibility("gts.x.test8.nested_compat.order.v1.0~", "gts.x.test8.nested_compat.order.v1.1~")
	assertCompat(t, r, "incompatible", "compatible", "incompatible")
}

func TestSpecOp8_ConstraintRelaxation(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.constraints.product.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"productId", "price"},
		"properties": map[string]any{
			"productId": map[string]any{"type": "string"},
			"price":     map[string]any{"type": "number", "minimum": float64(0), "maximum": float64(1000)},
			"name":      map[string]any{"type": "string", "minLength": float64(3), "maxLength": float64(50)},
		},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.constraints.product.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"productId", "price"},
		"properties": map[string]any{
			"productId": map[string]any{"type": "string"},
			"price":     map[string]any{"type": "number", "minimum": float64(0), "maximum": float64(10000)},
			"name":      map[string]any{"type": "string", "minLength": float64(1), "maxLength": float64(100)},
		},
	})
	r := store.CheckCompatibility("gts.x.test8.constraints.product.v1.0~", "gts.x.test8.constraints.product.v1.1~")
	assertCompat(t, r, "compatible", "incompatible", "incompatible")
}

func TestSpecOp8_ConstraintTightening(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.tight.item.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"itemId", "quantity"},
		"properties": map[string]any{
			"itemId":   map[string]any{"type": "string"},
			"quantity": map[string]any{"type": "integer", "minimum": float64(1), "maximum": float64(1000)},
		},
	})
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.tight.item.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"itemId", "quantity"},
		"properties": map[string]any{
			"itemId":   map[string]any{"type": "string"},
			"quantity": map[string]any{"type": "integer", "minimum": float64(1), "maximum": float64(100)},
		},
	})
	r := store.CheckCompatibility("gts.x.test8.tight.item.v1.0~", "gts.x.test8.tight.item.v1.1~")
	assertCompat(t, r, "incompatible", "compatible", "incompatible")
}

func TestSpecOp8_ArrayItemSchemaChange(t *testing.T) {
	store := NewGtsStore(nil)
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.array_compat.list.v1.0~", "$$schema": "http://json-schema.org/draft-07/schema#",
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
	regSpec(t, store, map[string]any{
		"$$id": "gts://gts.x.test8.array_compat.list.v1.1~", "$$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "required": []any{"listId", "items"},
		"properties": map[string]any{
			"listId": map[string]any{"type": "string"},
			"items": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "required": []any{"id", "value"}, "properties": map[string]any{
					"id": map[string]any{"type": "string"}, "value": map[string]any{"type": "number"}, "label": map[string]any{"type": "string"},
				},
			}},
		},
	})
	r := store.CheckCompatibility("gts.x.test8.array_compat.list.v1.0~", "gts.x.test8.array_compat.list.v1.1~")
	assertCompat(t, r, "incompatible", "compatible", "incompatible")
}
