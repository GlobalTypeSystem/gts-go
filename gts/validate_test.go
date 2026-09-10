/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"testing"
)

func TestValidateInstance_ValidInstance(t *testing.T) {
	store := NewGtsStore(nil)

	// Register base event schema
	baseSchema := map[string]any{
		"$id":      "gts://gts.x.core.events.type.v1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"id", "type", "tenantId", "occurredAt"},
		"properties": map[string]any{
			"type":       map[string]any{"type": "string"},
			"id":         map[string]any{"type": "string"},
			"tenantId":   map[string]any{"type": "string", "format": "uuid"},
			"occurredAt": map[string]any{"type": "string", "format": "date-time"},
			"payload":    map[string]any{"type": "object"},
		},
	}
	baseEntity := NewJsonEntity(baseSchema, DefaultGtsConfig())
	if err := store.Register(baseEntity); err != nil {
		t.Fatalf("Failed to register base schema: %v", err)
	}

	// Register derived event schema
	derivedSchema := map[string]any{
		"$id":     "gts://gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.core.events.type.v1~"},
			map[string]any{
				"type":     "object",
				"required": []any{"type", "payload"},
				"properties": map[string]any{
					"type": map[string]any{"const": "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~"},
					"payload": map[string]any{
						"type":     "object",
						"required": []any{"orderId", "customerId", "totalAmount", "items"},
						"properties": map[string]any{
							"orderId":     map[string]any{"type": "string", "format": "uuid"},
							"customerId":  map[string]any{"type": "string", "format": "uuid"},
							"totalAmount": map[string]any{"type": "number"},
							"items":       map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						},
					},
				},
			},
		},
	}
	derivedEntity := NewJsonEntity(derivedSchema, DefaultGtsConfig())
	if err := store.Register(derivedEntity); err != nil {
		t.Fatalf("Failed to register derived schema: %v", err)
	}

	// Register valid instance
	instance := map[string]any{
		"type":       "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~",
		"id":         "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~x.y._.some_event.v1.0",
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
	instanceEntity := NewJsonEntity(instance, DefaultGtsConfig())
	if err := store.Register(instanceEntity); err != nil {
		t.Fatalf("Failed to register instance: %v", err)
	}

	// Validate the instance
	result := store.ValidateInstance("gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~x.y._.some_event.v1.0")

	if !result.OK {
		t.Errorf("Expected validation to succeed, got error: %s", result.Error)
	}
	if result.ID != "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~x.y._.some_event.v1.0" {
		t.Errorf("Expected ID to be set correctly, got: %s", result.ID)
	}
}

func TestValidateInstance_InvalidInstance_MissingRequiredField(t *testing.T) {
	store := NewGtsStore(nil)

	// Register base event schema
	baseSchema := map[string]any{
		"$id":      "gts://gts.x.core.events.type.v1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"id", "type", "tenantId", "occurredAt"},
		"properties": map[string]any{
			"type":       map[string]any{"type": "string"},
			"id":         map[string]any{"type": "string"},
			"tenantId":   map[string]any{"type": "string", "format": "uuid"},
			"occurredAt": map[string]any{"type": "string", "format": "date-time"},
			"payload":    map[string]any{"type": "object"},
		},
	}
	baseEntity := NewJsonEntity(baseSchema, DefaultGtsConfig())
	if err := store.Register(baseEntity); err != nil {
		t.Fatalf("Failed to register base schema: %v", err)
	}

	// Register derived event schema with required field
	derivedSchema := map[string]any{
		"$id":     "gts://gts.x.core.events.type.v1~x.test6.invalid.event.v1.0~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.core.events.type.v1~"},
			map[string]any{
				"type":     "object",
				"required": []any{"type", "payload"},
				"properties": map[string]any{
					"type": map[string]any{"const": "gts.x.core.events.type.v1~x.test6.invalid.event.v1.0~"},
					"payload": map[string]any{
						"type":     "object",
						"required": []any{"requiredField"},
						"properties": map[string]any{
							"requiredField": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}
	derivedEntity := NewJsonEntity(derivedSchema, DefaultGtsConfig())
	if err := store.Register(derivedEntity); err != nil {
		t.Fatalf("Failed to register derived schema: %v", err)
	}

	// Register invalid instance (missing requiredField in payload)
	instance := map[string]any{
		"type":       "gts.x.core.events.type.v1~x.test6.invalid.event.v1.0~",
		"id":         "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~x.y._.some_event2.v1.0",
		"tenantId":   "11111111-2222-3333-4444-555555555555",
		"occurredAt": "2025-09-20T18:35:00Z",
		"payload": map[string]any{
			"someOtherField": "value",
		},
	}
	instanceEntity := NewJsonEntity(instance, DefaultGtsConfig())
	if err := store.Register(instanceEntity); err != nil {
		t.Fatalf("Failed to register instance: %v", err)
	}

	// Validate the instance - should fail
	result := store.ValidateInstance("gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~x.y._.some_event2.v1.0")

	if result.OK {
		t.Errorf("Expected validation to fail, but it succeeded")
	}
	if result.ID != "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~x.y._.some_event2.v1.0" {
		t.Errorf("Expected ID to be set correctly, got: %s", result.ID)
	}
	if result.Error == "" {
		t.Errorf("Expected error message, got empty string")
	}
}

func TestValidateInstance_NotFound(t *testing.T) {
	store := NewGtsStore(nil)

	// Validate non-existent instance
	result := store.ValidateInstance("gts.x.nonexistent.pkg.ns.type.v1.0")

	if result.OK {
		t.Errorf("Expected validation to fail for non-existent instance")
	}
	if result.Error == "" {
		t.Errorf("Expected error message for non-existent instance")
	}
}

func TestValidateInstance_FormatValidation(t *testing.T) {
	store := NewGtsStore(nil)

	// Register schema with format constraints
	schema := map[string]any{
		"$id":      "gts://gts.x.test6.formats.user.v1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"userId", "email", "createdAt"},
		"properties": map[string]any{
			"userId":    map[string]any{"type": "string", "format": "uuid"},
			"email":     map[string]any{"type": "string", "format": "email"},
			"createdAt": map[string]any{"type": "string", "format": "date-time"},
		},
	}
	schemaEntity := NewJsonEntity(schema, DefaultGtsConfig())
	if err := store.Register(schemaEntity); err != nil {
		t.Fatalf("Failed to register schema: %v", err)
	}

	// Register valid instance with correct formats
	instance := map[string]any{
		"type":      "gts.x.test6.formats.user.v1~",
		"id":        "gts.x.test6.formats.user.v1~x.test6._.user_inst.v1",
		"userId":    "550e8400-e29b-41d4-a716-446655440000",
		"email":     "user@example.com",
		"createdAt": "2025-01-15T10:30:00Z",
	}
	instanceEntity := NewJsonEntity(instance, DefaultGtsConfig())
	if err := store.Register(instanceEntity); err != nil {
		t.Fatalf("Failed to register instance: %v", err)
	}

	// Validate the instance
	result := store.ValidateInstance("gts.x.test6.formats.user.v1~x.test6._.user_inst.v1")

	if !result.OK {
		t.Errorf("Expected validation to succeed, got error: %s", result.Error)
	}
}

func TestValidateInstance_NestedObjects(t *testing.T) {
	store := NewGtsStore(nil)

	// Register schema with nested objects
	schema := map[string]any{
		"$id":      "gts://gts.x.test6.nested.order.v1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"orderId", "customer", "items"},
		"properties": map[string]any{
			"orderId": map[string]any{"type": "string"},
			"customer": map[string]any{
				"type":     "object",
				"required": []any{"customerId", "name", "address"},
				"properties": map[string]any{
					"customerId": map[string]any{"type": "string"},
					"name":       map[string]any{"type": "string"},
					"address": map[string]any{
						"type":     "object",
						"required": []any{"street", "city", "country"},
						"properties": map[string]any{
							"street":     map[string]any{"type": "string"},
							"city":       map[string]any{"type": "string"},
							"country":    map[string]any{"type": "string"},
							"postalCode": map[string]any{"type": "string"},
						},
					},
				},
			},
			"items": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type":     "object",
					"required": []any{"sku", "quantity", "price"},
					"properties": map[string]any{
						"sku":      map[string]any{"type": "string"},
						"quantity": map[string]any{"type": "integer", "minimum": 1},
						"price":    map[string]any{"type": "number", "minimum": 0},
					},
				},
			},
		},
	}
	schemaEntity := NewJsonEntity(schema, DefaultGtsConfig())
	if err := store.Register(schemaEntity); err != nil {
		t.Fatalf("Failed to register schema: %v", err)
	}

	// Register valid nested instance
	instance := map[string]any{
		"type":    "gts.x.test6.nested.order.v1~",
		"id":      "gts.x.test6.nested.order.v1~x.test6._.order1.v1",
		"orderId": "ORD-12345",
		"customer": map[string]any{
			"customerId": "CUST-001",
			"name":       "John Doe",
			"address": map[string]any{
				"street":     "123 Main St",
				"city":       "New York",
				"country":    "USA",
				"postalCode": "10001",
			},
		},
		"items": []any{
			map[string]any{"sku": "SKU-001", "quantity": 2, "price": 29.99},
			map[string]any{"sku": "SKU-002", "quantity": 1, "price": 49.99},
		},
	}
	instanceEntity := NewJsonEntity(instance, DefaultGtsConfig())
	if err := store.Register(instanceEntity); err != nil {
		t.Fatalf("Failed to register instance: %v", err)
	}

	// Validate nested instance
	result := store.ValidateInstance("gts.x.test6.nested.order.v1~x.test6._.order1.v1")

	if !result.OK {
		t.Errorf("Expected validation to succeed, got error: %s", result.Error)
	}
}

func TestValidateInstance_EnumConstraints(t *testing.T) {
	store := NewGtsStore(nil)

	// Register schema with enum
	schema := map[string]any{
		"$id":      "gts://gts.x.test6.enum.status.v1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"statusId", "status"},
		"properties": map[string]any{
			"statusId": map[string]any{"type": "string"},
			"status": map[string]any{
				"type": "string",
				"enum": []any{"pending", "approved", "rejected", "completed"},
			},
			"priority": map[string]any{
				"type": "string",
				"enum": []any{"low", "medium", "high", "critical"},
			},
		},
	}
	schemaEntity := NewJsonEntity(schema, DefaultGtsConfig())
	if err := store.Register(schemaEntity); err != nil {
		t.Fatalf("Failed to register schema: %v", err)
	}

	// Register valid instance with enum values
	instance := map[string]any{
		"type":     "gts.x.test6.enum.status.v1~",
		"id":       "gts.x.test6.enum.status.v1~x.test6._.status1.v1",
		"statusId": "STATUS-001",
		"status":   "approved",
		"priority": "high",
	}
	instanceEntity := NewJsonEntity(instance, DefaultGtsConfig())
	if err := store.Register(instanceEntity); err != nil {
		t.Fatalf("Failed to register instance: %v", err)
	}

	// Validate enum instance
	result := store.ValidateInstance("gts.x.test6.enum.status.v1~x.test6._.status1.v1")

	if !result.OK {
		t.Errorf("Expected validation to succeed, got error: %s", result.Error)
	}
}

func TestValidateInstance_ArrayConstraints(t *testing.T) {
	store := NewGtsStore(nil)

	// Register schema with array constraints
	schema := map[string]any{
		"$id":      "gts://gts.x.test6.array.tags.v1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"itemId", "tags"},
		"properties": map[string]any{
			"itemId": map[string]any{"type": "string"},
			"tags": map[string]any{
				"type":     "array",
				"minItems": 1,
				"maxItems": 5,
				"items":    map[string]any{"type": "string"},
			},
		},
	}
	schemaEntity := NewJsonEntity(schema, DefaultGtsConfig())
	if err := store.Register(schemaEntity); err != nil {
		t.Fatalf("Failed to register schema: %v", err)
	}

	// Register valid instance with array
	instance := map[string]any{
		"type":   "gts.x.test6.array.tags.v1~",
		"id":     "gts.x.test6.array.tags.v1~x.test6._.item1.v1",
		"itemId": "ITEM-001",
		"tags":   []any{"electronics", "sale", "featured"},
	}
	instanceEntity := NewJsonEntity(instance, DefaultGtsConfig())
	if err := store.Register(instanceEntity); err != nil {
		t.Fatalf("Failed to register instance: %v", err)
	}

	// Validate array instance
	result := store.ValidateInstance("gts.x.test6.array.tags.v1~x.test6._.item1.v1")

	if !result.OK {
		t.Errorf("Expected validation to succeed, got error: %s", result.Error)
	}
}

func TestValidateInstance_NoSchemaID(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance without schema ID
	instance := map[string]any{
		"id":        "gts.x.test6.noschem.item.v1~a.b.c.d.v1",
		"someField": "value",
	}
	instanceEntity := NewJsonEntity(instance, DefaultGtsConfig())
	if err := store.Register(instanceEntity); err != nil {
		t.Fatalf("Failed to register instance: %v", err)
	}

	// Validate instance without schema - should fail
	result := store.ValidateInstance("gts.x.test6.noschem.item.v1~a.b.c.d.v1")

	if result.OK {
		t.Errorf("Expected validation to fail for instance without schema")
	}
	if result.Error == "" {
		t.Errorf("Expected error message for instance without schema")
	}
}
