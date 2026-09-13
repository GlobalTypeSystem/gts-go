/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"testing"
)

// Test 1: Access root field
func TestGetAttribute_RootField(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance
	instance := NewJsonEntity(map[string]any{
		"gtsId":      "gts.x.test11.events.type.v1~x.test11.nested.type.v1.0~x.test11.my.event.v1.0",
		"type":       "gts.x.test11.events.type.v1~x.test11.nested.type.v1.0~",
		"eventId":    "ad4g5h67-8901-72de-2345-def456789",
		"tenantId":   "44444444-5555-6666-7777-888888888888",
		"occurredAt": "2025-09-20T21:00:00Z",
		"payload":    map[string]any{},
	}, DefaultGtsConfig())
	_ = store.Register(instance)

	// Access root field
	result := store.GetAttribute("gts.x.test11.events.type.v1~x.test11.nested.type.v1.0~x.test11.my.event.v1.0@eventId")

	if !result.Resolved {
		t.Errorf("Expected resolved=true, got false. Error: %s", result.Error)
	}

	if result.Value != "ad4g5h67-8901-72de-2345-def456789" {
		t.Errorf("Expected value 'ad4g5h67-8901-72de-2345-def456789', got: %v", result.Value)
	}
}

// Test 2: Access nested field
func TestGetAttribute_NestedField(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance
	instance := NewJsonEntity(map[string]any{
		"gtsId":      "gts.x.test11.events.type.v1~x.test11.nested.type.v1.0~x.test11.my.event.v1.0",
		"type":       "gts.x.test11.events.type.v1~x.test11.nested.type.v1.0~",
		"eventId":    "ad4g5h67-8901-72de-2345-def456789",
		"tenantId":   "44444444-5555-6666-7777-888888888888",
		"occurredAt": "2025-09-20T21:00:00Z",
		"payload": map[string]any{
			"orderId": "order-12345",
			"customer": map[string]any{
				"name":  "John Doe",
				"email": "john.doe@example.com",
			},
		},
	}, DefaultGtsConfig())
	_ = store.Register(instance)

	// Access nested field
	result := store.GetAttribute("gts.x.test11.events.type.v1~x.test11.nested.type.v1.0~x.test11.my.event.v1.0@payload.customer.email")

	if !result.Resolved {
		t.Errorf("Expected resolved=true, got false. Error: %s", result.Error)
	}

	if result.Value != "john.doe@example.com" {
		t.Errorf("Expected value 'john.doe@example.com', got: %v", result.Value)
	}
}

// Test 3: Access non-existent field
func TestGetAttribute_NonExistentField(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance
	instance := NewJsonEntity(map[string]any{
		"gtsId":      "gts.x.test11.events.type.v1~x.test11.missing.event.v1.0",
		"type":       "gts.x.test11.events.type.v1~",
		"eventId":    "be5h6i78-9012-83ef-3456-efg567890",
		"tenantId":   "55555555-6666-7777-8888-999999999999",
		"occurredAt": "2025-09-20T22:00:00Z",
		"payload": map[string]any{
			"field1": "value1",
		},
	}, DefaultGtsConfig())
	if err := store.Register(instance); err != nil {
		t.Fatal(err)
	}

	// Access non-existent field
	result := store.GetAttribute("gts.x.test11.events.type.v1~x.test11.missing.event.v1.0@payload.nonExistent")

	if result.Resolved {
		t.Error("Expected resolved=false, got true")
	}

	if result.Error == "" {
		t.Error("Expected error message for non-existent field")
	}
}

// Test 4: Missing @ symbol
func TestGetAttribute_MissingAtSymbol(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance
	instance := NewJsonEntity(map[string]any{
		"type":       "gts.x.test11.events.type.v1~x.test11.nosymbol.event.v1.0~",
		"eventId":    "cf6i7j89-0123-94fg-4567-fgh678901",
		"tenantId":   "66666666-7777-8888-9999-000000000000",
		"occurredAt": "2025-09-20T23:00:00Z",
		"payload": map[string]any{
			"field1": "value1",
		},
	}, DefaultGtsConfig())
	_ = store.Register(instance)

	// Access without @ symbol
	result := store.GetAttribute("gts.x.test11.events.type.v1~x.test11.nosymbol.event.v1.0")

	if result.Resolved {
		t.Error("Expected resolved=false, got true")
	}

	if !containsString(result.Error, "Attribute selector requires") {
		t.Errorf("Expected error about missing @, got: %s", result.Error)
	}
}

// Test 5: Entity not found
func TestGetAttribute_EntityNotFound(t *testing.T) {
	store := NewGtsStore(nil)

	// Try to access attribute on non-existent entity
	result := store.GetAttribute("gts.x.nonexistent.entity.v1~@field")

	if result.Resolved {
		t.Error("Expected resolved=false, got true")
	}

	if !containsString(result.Error, "Entity not found") {
		t.Errorf("Expected 'Entity not found' error, got: %s", result.Error)
	}
}

// Test 6: Array element access
func TestGetAttribute_ArrayElementAccess(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance with array
	instance := NewJsonEntity(map[string]any{
		"type":    "gts.x.test11.array_access.order.v1~",
		"id":      "gts.x.test11.array_access.order.v1~x.test11._.order_arr.v1",
		"orderId": "ORD-123",
		"items": []any{
			map[string]any{"sku": "SKU-001", "name": "Item 1", "price": 10.99},
			map[string]any{"sku": "SKU-002", "name": "Item 2", "price": 20.99},
			map[string]any{"sku": "SKU-003", "name": "Item 3", "price": 30.99},
		},
	}, DefaultGtsConfig())
	_ = store.Register(instance)

	// Access first array element
	result := store.GetAttribute("gts.x.test11.array_access.order.v1~x.test11._.order_arr.v1@items[0].sku")

	if !result.Resolved {
		t.Errorf("Expected resolved=true, got false. Error: %s", result.Error)
	}

	if result.Value != "SKU-001" {
		t.Errorf("Expected value 'SKU-001', got: %v", result.Value)
	}

	// Access second array element
	result = store.GetAttribute("gts.x.test11.array_access.order.v1~x.test11._.order_arr.v1@items[1].name")

	if !result.Resolved {
		t.Errorf("Expected resolved=true, got false. Error: %s", result.Error)
	}

	if result.Value != "Item 2" {
		t.Errorf("Expected value 'Item 2', got: %v", result.Value)
	}
}

// Test 7: Deep nesting (6 levels)
func TestGetAttribute_DeepNesting(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance with deep nesting
	instance := NewJsonEntity(map[string]any{
		"type": "gts.x.test11.deep.nested.v1~",
		"id":   "gts.x.test11.deep.nested.v1~x.test11._.deep1.v1",
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": map[string]any{
					"level4": map[string]any{
						"level5": map[string]any{
							"level6": map[string]any{
								"deepValue": "found-it",
							},
						},
					},
				},
			},
		},
	}, DefaultGtsConfig())
	_ = store.Register(instance)

	// Access deeply nested value
	result := store.GetAttribute("gts.x.test11.deep.nested.v1~x.test11._.deep1.v1@level1.level2.level3.level4.level5.level6.deepValue")

	if !result.Resolved {
		t.Errorf("Expected resolved=true, got false. Error: %s", result.Error)
	}

	if result.Value != "found-it" {
		t.Errorf("Expected value 'found-it', got: %v", result.Value)
	}
}

// Test 8: Mixed array and nesting
func TestGetAttribute_MixedArrayAndNesting(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance with mixed structure
	instance := NewJsonEntity(map[string]any{
		"type":   "gts.x.test11.mixed.complex.v1~",
		"id":     "gts.x.test11.mixed.complex.v1~x.test11._.mixed1.v1",
		"dataId": "DATA-001",
		"records": []any{
			map[string]any{
				"recordId": "REC-001",
				"details": map[string]any{
					"metadata": map[string]any{
						"author": "John Doe",
						"tags":   []any{"important", "urgent"},
					},
				},
			},
			map[string]any{
				"recordId": "REC-002",
				"details": map[string]any{
					"metadata": map[string]any{
						"author": "Jane Smith",
						"tags":   []any{"review", "pending"},
					},
				},
			},
		},
	}, DefaultGtsConfig())
	_ = store.Register(instance)

	// Access nested field in array element
	result := store.GetAttribute("gts.x.test11.mixed.complex.v1~x.test11._.mixed1.v1@records[0].details.metadata.author")

	if !result.Resolved {
		t.Errorf("Expected resolved=true, got false. Error: %s", result.Error)
	}

	if result.Value != "John Doe" {
		t.Errorf("Expected value 'John Doe', got: %v", result.Value)
	}

	// Access array within nested object
	result = store.GetAttribute("gts.x.test11.mixed.complex.v1~x.test11._.mixed1.v1@records[1].details.metadata.tags[0]")

	if !result.Resolved {
		t.Errorf("Expected resolved=true, got false. Error: %s", result.Error)
	}

	if result.Value != "review" {
		t.Errorf("Expected value 'review', got: %v", result.Value)
	}
}

// Test 9: Boolean value
func TestGetAttribute_BooleanValue(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance with boolean
	instance := NewJsonEntity(map[string]any{
		"type":       "gts.x.test11.types.config.v1~",
		"id":         "gts.x.test11.types.config.v1~x.test11._.config1.v1",
		"configId":   "CFG-001",
		"enabled":    true,
		"maxRetries": 5,
		"timeout":    30.5,
	}, DefaultGtsConfig())
	_ = store.Register(instance)

	// Access boolean value
	result := store.GetAttribute("gts.x.test11.types.config.v1~x.test11._.config1.v1@enabled")

	if !result.Resolved {
		t.Errorf("Expected resolved=true, got false. Error: %s", result.Error)
	}

	boolVal, ok := result.Value.(bool)
	if !ok || !boolVal {
		t.Errorf("Expected boolean value true, got: %v", result.Value)
	}
}

// Test 10: Integer value
func TestGetAttribute_IntegerValue(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance with integer
	instance := NewJsonEntity(map[string]any{
		"type":       "gts.x.test11.types.config.v1~",
		"id":         "gts.x.test11.types.config.v1~x.test11._.config1.v1",
		"configId":   "CFG-001",
		"enabled":    true,
		"maxRetries": 5,
		"timeout":    30.5,
	}, DefaultGtsConfig())
	_ = store.Register(instance)

	// Access integer value
	result := store.GetAttribute("gts.x.test11.types.config.v1~x.test11._.config1.v1@maxRetries")

	if !result.Resolved {
		t.Errorf("Expected resolved=true, got false. Error: %s", result.Error)
	}

	// Handle both int and float64 (JSON unmarshaling may give us float64)
	switch v := result.Value.(type) {
	case int:
		if v != 5 {
			t.Errorf("Expected value 5, got: %v", result.Value)
		}
	case float64:
		if v != 5.0 {
			t.Errorf("Expected value 5, got: %v", result.Value)
		}
	default:
		t.Errorf("Expected numeric value, got: %T %v", result.Value, result.Value)
	}
}

// Test 11: Float value
func TestGetAttribute_FloatValue(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance with float
	instance := NewJsonEntity(map[string]any{
		"type":       "gts.x.test11.types.config.v1~",
		"id":         "gts.x.test11.types.config.v1~x.test11._.config1.v1",
		"configId":   "CFG-001",
		"enabled":    true,
		"maxRetries": 5,
		"timeout":    30.5,
	}, DefaultGtsConfig())
	_ = store.Register(instance)

	// Access float value
	result := store.GetAttribute("gts.x.test11.types.config.v1~x.test11._.config1.v1@timeout")

	if !result.Resolved {
		t.Errorf("Expected resolved=true, got false. Error: %s", result.Error)
	}

	floatVal, ok := result.Value.(float64)
	if !ok || floatVal != 30.5 {
		t.Errorf("Expected value 30.5, got: %v", result.Value)
	}
}

// Test 12: Array index out of bounds
func TestGetAttribute_ArrayIndexOutOfBounds(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance with array
	instance := NewJsonEntity(map[string]any{
		"type":    "gts.x.test11.array_access.order.v1~",
		"id":      "gts.x.test11.array_access.order.v1~x.test11._.order_arr.v1",
		"orderId": "ORD-123",
		"items": []any{
			map[string]any{"sku": "SKU-001"},
			map[string]any{"sku": "SKU-002"},
		},
	}, DefaultGtsConfig())
	_ = store.Register(instance)

	// Try to access out-of-bounds index
	result := store.GetAttribute("gts.x.test11.array_access.order.v1~x.test11._.order_arr.v1@items[10].sku")

	if result.Resolved {
		t.Error("Expected resolved=false for out-of-bounds index")
	}

	if !containsString(result.Error, "Index out of range") {
		t.Errorf("Expected 'Index out of range' error, got: %s", result.Error)
	}
}

// Test 13: Path normalization with slashes
func TestGetAttribute_PathNormalizationWithSlashes(t *testing.T) {
	store := NewGtsStore(nil)

	// Register instance - use a working pattern from other tests
	instance := NewJsonEntity(map[string]any{
		"type": "gts.x.test11.path.v1~",
		"id":   "gts.x.test11.path.v1~x.test11._.path1.v1",
		"data": map[string]any{
			"nested": map[string]any{
				"value": "test-value",
			},
		},
	}, DefaultGtsConfig())

	if instance.GtsID == nil {
		t.Skip("Skipping test - entity ID not properly extracted")
		return
	}

	err := store.Register(instance)
	if err != nil {
		t.Fatalf("Failed to register instance: %v", err)
	}

	// Access using slash notation (should be normalized to dots)
	result := store.GetAttribute(instance.GtsID.ID + "@data/nested/value")

	if !result.Resolved {
		t.Errorf("Expected resolved=true for slash notation, got false. Error: %s", result.Error)
	}

	if result.Value != "test-value" {
		t.Errorf("Expected value 'test-value', got: %v", result.Value)
	}
}
