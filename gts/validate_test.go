/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"errors"
	"strings"
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

func TestValidateTransientJSON_RejectsMixedDialectSchemaGraph(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":     "gts://gts.x.validate.ns.foreign.v1~",
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object",
	})
	mustRegister(t, store, map[string]any{
		"$id":     "gts://gts.x.validate.ns.host.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"allOf": []any{
			map[string]any{"$ref": "gts://gts.x.validate.ns.foreign.v1~"},
		},
	})

	result := store.ValidateTransientJSON(map[string]any{
		"id":   "gts.x.validate.ns.host.v1~x.validate.ns.item.v1",
		"type": "gts.x.validate.ns.host.v1~",
	}, "")
	if result.OK || result.Error == "" {
		t.Fatalf("expected mixed-dialect schema graph failure, got: %+v", result)
	}
}

func TestValidateWithSchemaLoadsReferencedSchemasOnDemand(t *testing.T) {
	store := NewGtsStore(nil)
	baseID := "gts.x.validate.ns.ondemand.v1~"
	derivedID := "gts.x.validate.ns.ondemand.v1~x.validate.ns.child.v1~"
	mustRegister(t, store, map[string]any{
		"$id":     "gts://" + baseID,
		"$schema": "http://json-schema.org/draft-07/schema#",
		"$defs": map[string]any{
			"named": map[string]any{
				"type":     "object",
				"required": []any{"name"},
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	})
	mustRegister(t, store, map[string]any{
		"$id":     "gts://" + derivedID,
		"$schema": "http://json-schema.org/draft-07/schema#",
		"allOf": []any{
			map[string]any{"$ref": "gts://" + baseID + "#/$defs/named"},
		},
	})

	derived := store.Get(derivedID)
	if err := store.validateWithSchema(map[string]any{"name": "ok"}, derived.Content); err != nil {
		t.Fatalf("expected on-demand reference resolution to pass: %v", err)
	}
	if err := store.validateWithSchema(map[string]any{}, derived.Content); err == nil {
		t.Fatal("expected referenced required constraint to fail")
	}
}

// TestCompileSchema_GenerationGuard verifies the compiled-schema cache rejects an
// entry left over from before an invalidation, closing the race where a compile
// that ran concurrently with a store mutation publishes a stale result.
func TestCompileSchema_GenerationGuard(t *testing.T) {
	store := NewGtsStore(nil)
	schema := map[string]any{
		"$id":     "gts://gts.x.core.cache.thing.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
	}
	normalized := normalizeSchemaForCompile(schema)
	schemaID, _ := normalized["$id"].(string)
	key := schemaID + "\x00" + contentHash(normalized)

	// First compile populates the cache at the current generation.
	if c, err := store.compileSchema(schemaID, normalized, schema); err != nil || c == nil {
		t.Fatalf("initial compile failed: %v", err)
	}
	if _, ok := store.schemaCache.Load(key); !ok {
		t.Fatal("expected the successful compile to be cached")
	}

	// A mutation advances the generation and clears the cache.
	genBefore := store.schemaCacheGen.Load()
	store.invalidateSchemaCache()
	if store.schemaCacheGen.Load() != genBefore+1 {
		t.Fatalf("invalidation must advance the generation: %d -> %d", genBefore, store.schemaCacheGen.Load())
	}

	// Simulate a compile that lost the Store/Clear race: an entry carrying the
	// pre-mutation generation lands in the map. It must not be reused.
	store.schemaCache.Store(key, &compiledSchemaEntry{
		err: errors.New("stale cache entry (test)"),
		gen: store.schemaCacheGen.Load() - 1,
	})
	c, err := store.compileSchema(schemaID, normalized, schema)
	if err != nil {
		t.Fatalf("stale-generation entry must be ignored, got: %v", err)
	}
	if c == nil {
		t.Fatal("expected a fresh compile after the stale entry was rejected")
	}
}

// onDemandRef points into an unknown keyword, which becomes a schema position
// only through the reference (gts-spec README §11.0.1).
var onDemandRef = map[string]any{"$ref": "#/custom/unused"}

func onDemandSchema(dialect string, body map[string]any) map[string]any {
	schema := map[string]any{
		"$id":     "gts://gts.x.validate.regex.ondemand.v1~",
		"$schema": dialect,
		"type":    "object",
		"custom":  map[string]any{"unused": map[string]any{"pattern": "a(?=b)"}},
	}
	for k, v := range body {
		schema[k] = v
	}
	return schema
}

// Inactive subschemas must follow $refs and reject unsupported patterns;
// literal data and keywords outside the dialect remain unscanned.
func TestValidateJSONSchema_OnDemandSubschemas(t *testing.T) {
	const (
		d7   = "http://json-schema.org/draft-07/schema#"
		d19  = "https://json-schema.org/draft/2019-09/schema"
		d20  = "https://json-schema.org/draft/2020-12/schema"
		fail = true
		pass = false
	)
	cases := []struct {
		name    string
		dialect string
		body    map[string]any
		wantErr bool
	}{
		{"definitions", d7, map[string]any{"definitions": map[string]any{"e": onDemandRef}}, fail},
		{"defs_2019", d19, map[string]any{"$defs": map[string]any{"e": onDemandRef}}, fail},
		{"defs_2020", d20, map[string]any{"$defs": map[string]any{"e": onDemandRef}}, fail},
		{"legacy_definitions_2020", d20, map[string]any{"definitions": map[string]any{"e": onDemandRef}}, fail},
		{"nested_in_property", d7, map[string]any{"properties": map[string]any{
			"p": map[string]any{"definitions": map[string]any{"e": map[string]any{"anyOf": []any{true, onDemandRef}}}},
		}}, fail},
		{"embedded_id_scope", d20, map[string]any{"$defs": map[string]any{"e": map[string]any{
			"$id":    "http://example.com/embedded",
			"$ref":   "#/inner/unused",
			"inner":  map[string]any{"unused": map[string]any{"pattern": "a(?=b)"}},
			"custom": map[string]any{},
		}}}, fail},
		{"percent_encoded_pointer", d7, map[string]any{
			"definitions": map[string]any{"e": map[string]any{"$ref": "#/custom/un%75sed"}},
		}, fail},
		{"then_without_if", d7, map[string]any{"then": onDemandRef}, fail},
		{"then_after_false_if", d7, map[string]any{"if": false, "then": onDemandRef}, fail},
		{"else_after_true_if", d20, map[string]any{"if": true, "else": onDemandRef}, fail},
		{"additional_items_draft7", d7, map[string]any{"additionalItems": onDemandRef}, fail},
		{"additional_items_2019", d19, map[string]any{"additionalItems": onDemandRef}, fail},
		{"content_schema_2019", d19, map[string]any{"contentSchema": onDemandRef}, fail},
		{"content_schema_2020", d20, map[string]any{"contentSchema": onDemandRef}, fail},
		{"traits_draft7", d7, map[string]any{KeyXGtsTraitsSchema: map[string]any{"properties": map[string]any{"value": map[string]any{"pattern": "a(?=b)"}}}}, fail},
		{"traits_2019", d19, map[string]any{KeyXGtsTraitsSchema: onDemandRef}, fail},
		{"traits_2020", d20, map[string]any{KeyXGtsTraitsSchema: onDemandRef}, fail},
		{"traits_ref_draft7", d7, map[string]any{KeyXGtsTraitsSchema: onDemandRef}, fail},
		// Draft-07 ignores keywords beside "$ref" during evaluation; they are
		// still schema positions, checked like inactive branches.
		{"ref_sibling_properties_draft7", d7, refSiblings("properties", map[string]any{"p": onDemandRef}), fail},
		{"ref_sibling_allof_draft7", d7, refSiblings("allOf", []any{true, onDemandRef}), fail},
		{"ref_sibling_anyof_draft7", d7, refSiblings("anyOf", []any{true, onDemandRef}), fail},
		{"ref_sibling_not_draft7", d7, refSiblings("not", onDemandRef), fail},
		{"ref_sibling_items_draft7", d7, refSiblings("items", []any{onDemandRef}), fail},
		{"ref_sibling_dependencies_draft7", d7, refSiblings("dependencies", map[string]any{"x": onDemandRef}), fail},
		{"ref_sibling_property_names_draft7", d7, refSiblings("propertyNames", onDemandRef), fail},
		{"ref_sibling_contains_draft7", d7, refSiblings("contains", onDemandRef), fail},
		{"ref_sibling_if_draft7", d7, refSiblings("if", onDemandRef), fail},
		{"ref_sibling_properties_2020", d20, refSiblings("properties", map[string]any{"p": onDemandRef}), fail},

		// Controls: not schema positions in the declared dialect.
		{"dollar_defs_draft7", d7, map[string]any{"$defs": map[string]any{"e": onDemandRef}}, pass},
		{"content_schema_draft7", d7, map[string]any{"contentSchema": onDemandRef}, pass},
		{"additional_items_2020", d20, map[string]any{"additionalItems": onDemandRef}, pass},
		{"literal_data", d7, map[string]any{
			"const":   map[string]any{"definitions": map[string]any{"e": onDemandRef}},
			"default": map[string]any{"then": onDemandRef},
		}, pass},
		{"unknown_keyword", d7, map[string]any{"other": map[string]any{"definitions": map[string]any{"e": onDemandRef}}}, pass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewGtsStore(nil)
			err := store.validateJSONSchema(onDemandSchema(tc.dialect, tc.body))
			if tc.wantErr && (err == nil || !strings.Contains(err.Error(), "regex")) {
				t.Fatalf("expected unsupported-regex error, got %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected schema to be accepted, got %v", err)
			}
		})
	}

	// Control: the same references to a supported pattern are accepted.
	for _, supported := range []map[string]any{
		onDemandSchema(d20, map[string]any{"$defs": map[string]any{"e": onDemandRef}, "then": onDemandRef}),
		onDemandSchema(d7, refSiblings("allOf", []any{true, onDemandRef})),
		onDemandSchema(d7, map[string]any{KeyXGtsTraitsSchema: onDemandRef}),
	} {
		supported["custom"] = map[string]any{"unused": map[string]any{"pattern": "^a+$"}}
		if err := NewGtsStore(nil).validateJSONSchema(supported); err != nil {
			t.Fatalf("expected supported pattern to be accepted, got %v", err)
		}
	}
}

// refSiblings places keyword beside a root "$ref" to a valid definition.
func refSiblings(keyword string, value any) map[string]any {
	return map[string]any{
		"$ref":        "#/definitions/main",
		"definitions": map[string]any{"main": map[string]any{"type": "object"}},
		keyword:       value,
	}
}

// Deferred validation must also reject unsupported patterns in external schemas.
func TestOnDemandSubschemas_DeferredAndExternal(t *testing.T) {
	store := NewGtsStore(nil)
	target := onDemandSchema("http://json-schema.org/draft-07/schema#",
		map[string]any{"definitions": map[string]any{"e": onDemandRef}})
	mustRegister(t, store, target)

	if err := store.validateWithSchema(map[string]any{}, target); err == nil || !strings.Contains(err.Error(), "regex") {
		t.Fatalf("expected instance validation to fail on the unsupported pattern, got %v", err)
	}

	host := map[string]any{
		"$id":     "gts://gts.x.validate.regex.host.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"anyOf":   []any{true, map[string]any{"$ref": "gts://gts.x.validate.regex.ondemand.v1~"}},
	}
	if err := store.validateJSONSchema(host); err == nil || !strings.Contains(err.Error(), "regex") {
		t.Fatalf("expected referenced schema's unsupported pattern to fail, got %v", err)
	}
}

// Synthesized trait schemas inherit the host dialect.
func TestValidateTraitsAgainstSchema_OnDemandSubschemas(t *testing.T) {
	host := map[string]any{"$schema": "http://json-schema.org/draft-07/schema#"}
	traitSchema := func(container string) map[string]any {
		return map[string]any{
			"type":    "object",
			container: map[string]any{"e": onDemandRef},
			"custom":  map[string]any{"unused": map[string]any{"pattern": "a(?=b)"}},
		}
	}
	errs := validateTraitsAgainstSchema(traitSchema("definitions"), map[string]any{}, host, false)
	if len(errs) == 0 || !strings.Contains(errs[0], "regex") {
		t.Fatalf("expected trait schema compile error, got %v", errs)
	}
	// Draft-07 does not define $defs, so its value is an annotation.
	if errs := validateTraitsAgainstSchema(traitSchema("$defs"), map[string]any{}, host, false); len(errs) != 0 {
		t.Fatalf("expected draft-07 $defs to be ignored, got %v", errs)
	}
}
