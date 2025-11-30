/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"testing"
)

func TestCast_MinorVersionUpcast(t *testing.T) {
	store := NewGtsStore(nil)

	// Register base event schema
	baseSchema := map[string]any{
		"$id":      "gts.x.core.events.type.v1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"id", "type", "tenantId", "occurredAt"},
		"properties": map[string]any{
			"type":       map[string]any{"type": "string"},
			"id":         map[string]any{"type": "string", "format": "uuid"},
			"tenantId":   map[string]any{"type": "string", "format": "uuid"},
			"occurredAt": map[string]any{"type": "string", "format": "date-time"},
			"payload":    map[string]any{"type": "object"},
		},
		"additionalProperties": false,
	}
	baseEntity := NewJsonEntity(baseSchema, DefaultGtsConfig())
	if err := store.Register(baseEntity); err != nil {
		t.Fatalf("Failed to register base schema: %v", err)
	}

	// Register v1.0 schema
	v10Schema := map[string]any{
		"$id":     "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.core.events.type.v1~"},
			map[string]any{
				"type":     "object",
				"required": []any{"type", "payload"},
				"properties": map[string]any{
					"type": map[string]any{
						"const": "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~",
					},
					"payload": map[string]any{
						"type":     "object",
						"required": []any{"orderId", "customerId", "totalAmount", "items"},
						"properties": map[string]any{
							"orderId":     map[string]any{"type": "string", "format": "uuid"},
							"customerId":  map[string]any{"type": "string", "format": "uuid"},
							"totalAmount": map[string]any{"type": "number"},
							"items": map[string]any{
								"type":  "array",
								"items": map[string]any{"type": "object"},
							},
						},
					},
				},
			},
		},
	}
	v10Entity := NewJsonEntity(v10Schema, DefaultGtsConfig())
	if err := store.Register(v10Entity); err != nil {
		t.Fatalf("Failed to register v1.0 schema: %v", err)
	}

	// Register v1.1 schema (adds optional field with default)
	v11Schema := map[string]any{
		"$id":     "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.core.events.type.v1~"},
			map[string]any{
				"type":     "object",
				"required": []any{"type", "payload"},
				"properties": map[string]any{
					"type": map[string]any{
						"const": "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.1~",
					},
					"payload": map[string]any{
						"type":     "object",
						"required": []any{"orderId", "customerId", "totalAmount", "items"},
						"properties": map[string]any{
							"orderId":     map[string]any{"type": "string", "format": "uuid"},
							"customerId":  map[string]any{"type": "string", "format": "uuid"},
							"totalAmount": map[string]any{"type": "number"},
							"items": map[string]any{
								"type":  "array",
								"items": map[string]any{"type": "object"},
							},
							"new_field_in_v1_1": map[string]any{
								"type":    "string",
								"default": "some_value",
							},
						},
					},
				},
			},
		},
	}
	v11Entity := NewJsonEntity(v11Schema, DefaultGtsConfig())
	if err := store.Register(v11Entity); err != nil {
		t.Fatalf("Failed to register v1.1 schema: %v", err)
	}

	// Register v1.0 instance
	v10Instance := map[string]any{
		"type":       "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~",
		"id":         "af0e3c1b-8f1e-4a27-9a9b-b7b9b70c1f01",
		"tenantId":   "11111111-2222-3333-4444-555555555555",
		"occurredAt": "2025-09-20T18:35:00Z",
		"payload": map[string]any{
			"orderId":     "af0e3c1b-8f1e-4a27-9a9b-b7b9b70c1f01",
			"customerId":  "0f2e4a9b-1c3d-4e5f-8a9b-0c1d2e3f4a5b",
			"totalAmount": 149.99,
			"items": []any{
				map[string]any{
					"sku":   "SKU-ABC-001",
					"name":  "Wireless Mouse",
					"qty":   1,
					"price": 49.99,
				},
			},
		},
	}
	v10InstanceEntity := NewJsonEntity(v10Instance, DefaultGtsConfig())
	if err := store.Register(v10InstanceEntity); err != nil {
		t.Fatalf("Failed to register v1.0 instance: %v", err)
	}

	// Cast from v1.0 to v1.1
	result, err := store.Cast(
		"gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~",
		"gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.1~",
	)

	if err != nil {
		t.Fatalf("Cast failed: %v", err)
	}

	// Check the casted entity has the new field with default value
	if result.CastedEntity == nil {
		t.Fatal("Expected casted entity, got nil")
	}

	payload, ok := result.CastedEntity["payload"].(map[string]any)
	if !ok {
		t.Fatal("Expected payload to be a map")
	}

	newField, ok := payload["new_field_in_v1_1"]
	if !ok {
		t.Error("Expected new_field_in_v1_1 to be present")
	} else if newField != "some_value" {
		t.Errorf("Expected new_field_in_v1_1 to be 'some_value', got: %v", newField)
	}

	// Check const field was updated
	if typeField, ok := result.CastedEntity["type"].(string); ok {
		expected := "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.1~"
		if typeField != expected {
			t.Errorf("Expected type to be updated to %s, got: %s", expected, typeField)
		}
	}

	// Check added properties
	if len(result.AddedProperties) == 0 {
		t.Error("Expected some properties to be added")
	}
}

func TestCast_MinorVersionDowncast(t *testing.T) {
	store := NewGtsStore(nil)

	// Register base event schema
	baseSchema := map[string]any{
		"$id":      "gts.x.core.events.type.v1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"id", "type", "tenantId", "occurredAt"},
		"properties": map[string]any{
			"type":       map[string]any{"type": "string"},
			"id":         map[string]any{"type": "string", "format": "uuid"},
			"tenantId":   map[string]any{"type": "string", "format": "uuid"},
			"occurredAt": map[string]any{"type": "string", "format": "date-time"},
			"payload":    map[string]any{"type": "object"},
		},
		"additionalProperties": false,
	}
	baseEntity := NewJsonEntity(baseSchema, DefaultGtsConfig())
	if err := store.Register(baseEntity); err != nil {
		t.Fatalf("Failed to register base schema: %v", err)
	}

	// Register v1.0 schema
	v10Schema := map[string]any{
		"$id":     "gts.x.core.events.type.v1~x.test9.cast.event.v1.0~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.core.events.type.v1~"},
			map[string]any{
				"type":     "object",
				"required": []any{"type", "payload"},
				"properties": map[string]any{
					"type": map[string]any{
						"const": "gts.x.core.events.type.v1~x.test9.cast.event.v1.0~",
					},
					"payload": map[string]any{
						"type":                 "object",
						"required":             []any{"field1"},
						"additionalProperties": false,
						"properties": map[string]any{
							"field1": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}
	v10Entity := NewJsonEntity(v10Schema, DefaultGtsConfig())
	if err := store.Register(v10Entity); err != nil {
		t.Fatalf("Failed to register v1.0 schema: %v", err)
	}

	// Register v1.1 schema
	v11Schema := map[string]any{
		"$id":     "gts.x.core.events.type.v1~x.test9.cast.event.v1.1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.core.events.type.v1~"},
			map[string]any{
				"type":     "object",
				"required": []any{"type", "payload"},
				"properties": map[string]any{
					"type": map[string]any{
						"const": "gts.x.core.events.type.v1~x.test9.cast.event.v1.1~",
					},
					"payload": map[string]any{
						"type":                 "object",
						"required":             []any{"field1"},
						"additionalProperties": false,
						"properties": map[string]any{
							"field1": map[string]any{"type": "string"},
							"field2": map[string]any{
								"type":    "string",
								"default": "default_value",
							},
						},
					},
				},
			},
		},
	}
	v11Entity := NewJsonEntity(v11Schema, DefaultGtsConfig())
	if err := store.Register(v11Entity); err != nil {
		t.Fatalf("Failed to register v1.1 schema: %v", err)
	}

	// Register v1.1 instance
	v11Instance := map[string]any{
		"type":       "gts.x.core.events.type.v1~x.test9.cast.event.v1.1~",
		"id":         "8b2e3f45-6789-50bc-0123-bcdef234567",
		"tenantId":   "22222222-3333-4444-5555-666666666666",
		"occurredAt": "2025-09-20T19:00:00Z",
		"payload": map[string]any{
			"field1": "value1",
			"field2": "value2",
		},
	}
	v11InstanceEntity := NewJsonEntity(v11Instance, DefaultGtsConfig())
	if err := store.Register(v11InstanceEntity); err != nil {
		t.Fatalf("Failed to register v1.1 instance: %v", err)
	}

	// Cast from v1.1 to v1.0 (downcast)
	result, err := store.Cast(
		"gts.x.core.events.type.v1~x.test9.cast.event.v1.1~",
		"gts.x.core.events.type.v1~x.test9.cast.event.v1.0~",
	)

	if err != nil {
		t.Fatalf("Cast failed: %v", err)
	}

	// Check the casted entity
	if result.CastedEntity == nil {
		t.Fatal("Expected casted entity, got nil")
	}

	payload, ok := result.CastedEntity["payload"].(map[string]any)
	if !ok {
		t.Fatal("Expected payload to be a map")
	}

	// field2 should not be present in v1.0 (removed during downcast)
	if _, hasField2 := payload["field2"]; hasField2 {
		t.Error("Expected field2 to be removed during downcast")
	}

	// field1 should still be present
	if field1, ok := payload["field1"]; !ok {
		t.Error("Expected field1 to be present")
	} else if field1 != "value1" {
		t.Errorf("Expected field1 to be 'value1', got: %v", field1)
	}

	// Check const field was updated
	if typeField, ok := result.CastedEntity["type"].(string); ok {
		expected := "gts.x.core.events.type.v1~x.test9.cast.event.v1.0~"
		if typeField != expected {
			t.Errorf("Expected type to be updated to %s, got: %s", expected, typeField)
		}
	}
}

func TestCast_NestedObjects(t *testing.T) {
	store := NewGtsStore(nil)

	// Register v1.0 schema with nested objects
	v10Schema := map[string]any{
		"$id":      "gts.x.core.nested.type.v1.0~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"id", "details"},
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
			"details": map[string]any{
				"type":     "object",
				"required": []any{"name"},
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}
	v10Entity := NewJsonEntity(v10Schema, DefaultGtsConfig())
	if err := store.Register(v10Entity); err != nil {
		t.Fatalf("Failed to register v1.0 schema: %v", err)
	}

	// Register v1.1 schema with additional nested field
	v11Schema := map[string]any{
		"$id":      "gts.x.core.nested.type.v1.1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"id", "details"},
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
			"details": map[string]any{
				"type":     "object",
				"required": []any{"name"},
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age": map[string]any{
						"type":    "number",
						"default": 0,
					},
				},
			},
		},
	}
	v11Entity := NewJsonEntity(v11Schema, DefaultGtsConfig())
	if err := store.Register(v11Entity); err != nil {
		t.Fatalf("Failed to register v1.1 schema: %v", err)
	}

	// Register v1.0 instance
	v10Instance := map[string]any{
		"gtsId":   "gts.x.core.nested.type.v1.0",
		"$schema": "gts.x.core.nested.type.v1.0~",
		"id":      "test-123",
		"details": map[string]any{
			"name": "John",
		},
	}
	v10InstanceEntity := NewJsonEntity(v10Instance, DefaultGtsConfig())
	if err := store.Register(v10InstanceEntity); err != nil {
		t.Fatalf("Failed to register v1.0 instance: %v", err)
	}

	// Cast from v1.0 to v1.1
	result, err := store.Cast("gts.x.core.nested.type.v1.0", "gts.x.core.nested.type.v1.1~")

	if err != nil {
		t.Fatalf("Cast failed: %v", err)
	}

	if result.CastedEntity == nil {
		t.Fatal("Expected casted entity, got nil")
	}

	details, ok := result.CastedEntity["details"].(map[string]any)
	if !ok {
		t.Fatal("Expected details to be a map")
	}

	// Check nested default was added
	if age, ok := details["age"]; !ok {
		t.Error("Expected age field to be added")
	} else {
		// Could be int or float64 depending on how the value was set
		switch v := age.(type) {
		case float64:
			if v != 0 {
				t.Errorf("Expected age to be 0, got: %v", age)
			}
		case int:
			if v != 0 {
				t.Errorf("Expected age to be 0, got: %v", age)
			}
		default:
			t.Errorf("Expected age to be numeric, got: %T", age)
		}
	}
}

func TestCast_ArrayOfObjects(t *testing.T) {
	store := NewGtsStore(nil)

	// Register v1.0 schema with array of objects
	v10Schema := map[string]any{
		"$id":      "gts.x.core.array.type.v1.0~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"items"},
		"properties": map[string]any{
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"id"},
					"properties": map[string]any{
						"id": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	v10Entity := NewJsonEntity(v10Schema, DefaultGtsConfig())
	if err := store.Register(v10Entity); err != nil {
		t.Fatalf("Failed to register v1.0 schema: %v", err)
	}

	// Register v1.1 schema with additional field in array items
	v11Schema := map[string]any{
		"$id":      "gts.x.core.array.type.v1.1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"items"},
		"properties": map[string]any{
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"id"},
					"properties": map[string]any{
						"id": map[string]any{"type": "string"},
						"status": map[string]any{
							"type":    "string",
							"default": "active",
						},
					},
				},
			},
		},
	}
	v11Entity := NewJsonEntity(v11Schema, DefaultGtsConfig())
	if err := store.Register(v11Entity); err != nil {
		t.Fatalf("Failed to register v1.1 schema: %v", err)
	}

	// Register v1.0 instance with array
	v10Instance := map[string]any{
		"gtsId":   "gts.x.core.array.type.v1.0",
		"$schema": "gts.x.core.array.type.v1.0~",
		"items": []any{
			map[string]any{"id": "item1"},
			map[string]any{"id": "item2"},
		},
	}
	v10InstanceEntity := NewJsonEntity(v10Instance, DefaultGtsConfig())
	if err := store.Register(v10InstanceEntity); err != nil {
		t.Fatalf("Failed to register v1.0 instance: %v", err)
	}

	// Cast from v1.0 to v1.1
	result, err := store.Cast("gts.x.core.array.type.v1.0", "gts.x.core.array.type.v1.1~")

	if err != nil {
		t.Fatalf("Cast failed: %v", err)
	}

	if result.CastedEntity == nil {
		t.Fatal("Expected casted entity, got nil")
	}

	items, ok := result.CastedEntity["items"].([]any)
	if !ok {
		t.Fatal("Expected items to be an array")
	}

	// Check each item has the default status
	for i, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			t.Errorf("Expected item %d to be a map", i)
			continue
		}
		if status, ok := itemMap["status"]; !ok {
			t.Errorf("Expected item %d to have status field", i)
		} else if status != "active" {
			t.Errorf("Expected item %d status to be 'active', got: %v", i, status)
		}
	}
}

func TestCast_InstanceNotFound(t *testing.T) {
	store := NewGtsStore(nil)

	_, err := store.Cast("gts.x.nonexistent.instance.v1.0", "gts.x.nonexistent.schema.v1.1~")

	if err == nil {
		t.Error("Expected error for non-existent instance")
	}
}

func TestCast_SchemaNotFound(t *testing.T) {
	store := NewGtsStore(nil)

	// Register schema first
	schema := map[string]any{
		"$id":      "gts.x.core.test.type.v1.0~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"id"},
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	}
	schemaEntity := NewJsonEntity(schema, DefaultGtsConfig())
	if err := store.Register(schemaEntity); err != nil {
		t.Fatalf("Failed to register schema: %v", err)
	}

	// Register instance with schema
	instance := map[string]any{
		"$schema": "gts.x.core.test.type.v1.0~",
		"id":      "test-123",
	}
	instanceEntity := NewJsonEntity(instance, DefaultGtsConfig())
	if err := store.Register(instanceEntity); err != nil {
		t.Fatalf("Failed to register instance: %v", err)
	}

	_, err := store.Cast("gts.x.core.test.type.v1.0~", "gts.x.nonexistent.schema.v1.1~")

	if err == nil {
		t.Error("Expected error for non-existent target schema")
	}
}

func TestCast_MissingRequiredFieldNoDefault(t *testing.T) {
	store := NewGtsStore(nil)

	// Register v1.0 schema
	v10Schema := map[string]any{
		"$id":      "gts.x.core.required.type.v1.0~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"id"},
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	}
	v10Entity := NewJsonEntity(v10Schema, DefaultGtsConfig())
	if err := store.Register(v10Entity); err != nil {
		t.Fatalf("Failed to register v1.0 schema: %v", err)
	}

	// Register v1.1 schema with new required field WITHOUT default
	v11Schema := map[string]any{
		"$id":      "gts.x.core.required.type.v1.1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"id", "newRequired"},
		"properties": map[string]any{
			"id":          map[string]any{"type": "string"},
			"newRequired": map[string]any{"type": "string"},
		},
	}
	v11Entity := NewJsonEntity(v11Schema, DefaultGtsConfig())
	if err := store.Register(v11Entity); err != nil {
		t.Fatalf("Failed to register v1.1 schema: %v", err)
	}

	// Register v1.0 instance
	v10Instance := map[string]any{
		"gtsId":   "gts.x.core.required.type.v1.0",
		"$schema": "gts.x.core.required.type.v1.0~",
		"id":      "test-123",
	}
	v10InstanceEntity := NewJsonEntity(v10Instance, DefaultGtsConfig())
	if err := store.Register(v10InstanceEntity); err != nil {
		t.Fatalf("Failed to register v1.0 instance: %v", err)
	}

	// Cast from v1.0 to v1.1 should fail
	result, err := store.Cast("gts.x.core.required.type.v1.0", "gts.x.core.required.type.v1.1~")

	if err != nil {
		t.Fatalf("Cast should not error at top level: %v", err)
	}

	// But it should have incompatibility reasons
	if len(result.IncompatibilityReasons) == 0 {
		t.Error("Expected incompatibility reasons for missing required field")
	}
}
