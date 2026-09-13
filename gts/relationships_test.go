/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"testing"
)

func TestBuildSchemaGraph_ValidChain(t *testing.T) {
	store := NewGtsStore(nil)

	// Register base event schema
	baseSchema := map[string]any{
		"$id":      "gts://gts.x.core.events.type.v1~",
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

	// Register derived event schema
	derivedSchema := map[string]any{
		"$id":     "gts://gts.x.core.events.type.v1~x.test7.graph.event.v1.0~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.core.events.type.v1~"},
			map[string]any{
				"type":     "object",
				"required": []any{"type", "payload"},
				"properties": map[string]any{
					"type": map[string]any{"const": "gts.x.core.events.type.v1~x.test7.graph.event.v1.0~"},
					"payload": map[string]any{
						"type":     "object",
						"required": []any{"testField"},
						"properties": map[string]any{
							"testField": map[string]any{"type": "string"},
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

	// Build schema graph
	graph := store.BuildSchemaGraph("gts.x.core.events.type.v1~x.test7.graph.event.v1.0~")

	if graph == nil {
		t.Fatal("Expected graph to be non-nil")
	}
	if graph.ID != "gts.x.core.events.type.v1~x.test7.graph.event.v1.0~" {
		t.Errorf("Expected ID to match, got: %s", graph.ID)
	}
	if len(graph.Refs) == 0 {
		t.Error("Expected graph to have refs")
	}
	if len(graph.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", graph.Errors)
	}
}

func TestBuildSchemaGraph_BrokenReference(t *testing.T) {
	store := NewGtsStore(nil)

	// Register schema with broken reference
	schema := map[string]any{
		"$id":     "gts://gts.x.core.broken.schema.v1.0~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.nonexistent.base.type.v1~"},
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"field": map[string]any{"type": "string"},
				},
			},
		},
	}
	entity := NewJsonEntity(schema, DefaultGtsConfig())
	if err := store.Register(entity); err != nil {
		t.Fatalf("Failed to register schema: %v", err)
	}

	// Build schema graph - should detect broken reference
	graph := store.BuildSchemaGraph("gts.x.core.broken.schema.v1.0~")

	if graph == nil {
		t.Fatal("Expected graph to be non-nil")
	}
	if graph.ID != "gts.x.core.broken.schema.v1.0~" {
		t.Errorf("Expected ID to match, got: %s", graph.ID)
	}
	// Should have refs with errors
	if len(graph.Refs) == 0 {
		t.Error("Expected graph to have refs")
	} else {
		foundError := false
		for _, ref := range graph.Refs {
			if len(ref.Errors) > 0 {
				foundError = true
				break
			}
		}
		if !foundError {
			t.Error("Expected to find broken reference error in refs")
		}
	}
}

func TestBuildSchemaGraph_ComplexChain(t *testing.T) {
	store := NewGtsStore(nil)

	// Register base schema
	baseSchema := map[string]any{
		"$id":      "gts://gts.x.core.base.type.v1~",
		"$schema":  "http://json-schema.org/draft-07/schema#",
		"type":     "object",
		"required": []any{"id"},
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	}
	baseEntity := NewJsonEntity(baseSchema, DefaultGtsConfig())
	if err := store.Register(baseEntity); err != nil {
		t.Fatalf("Failed to register base schema: %v", err)
	}

	// Register level 1 derived schema
	derived1Schema := map[string]any{
		"$id":     "gts://gts.x.core.base.type.v1~x.test7.derived1.type.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.core.base.type.v1~"},
			map[string]any{
				"type":     "object",
				"required": []any{"field1"},
				"properties": map[string]any{
					"field1": map[string]any{"type": "string"},
				},
			},
		},
	}
	derived1Entity := NewJsonEntity(derived1Schema, DefaultGtsConfig())
	if err := store.Register(derived1Entity); err != nil {
		t.Fatalf("Failed to register level 1 schema: %v", err)
	}

	// Register level 2 derived schema
	derived2Schema := map[string]any{
		"$id":     "gts://gts.x.core.base.type.v1~x.test7.derived1.type.v1~x.test7.derived2.type.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"allOf": []any{
			map[string]any{"$ref": "gts.x.core.base.type.v1~x.test7.derived1.type.v1~"},
			map[string]any{
				"type":     "object",
				"required": []any{"field2"},
				"properties": map[string]any{
					"field2": map[string]any{"type": "string"},
				},
			},
		},
	}
	derived2Entity := NewJsonEntity(derived2Schema, DefaultGtsConfig())
	if err := store.Register(derived2Entity); err != nil {
		t.Fatalf("Failed to register level 2 schema: %v", err)
	}

	// Build schema graph for complex chain
	graph := store.BuildSchemaGraph("gts.x.core.base.type.v1~x.test7.derived1.type.v1~x.test7.derived2.type.v1~")

	if graph == nil {
		t.Fatal("Expected graph to be non-nil")
	}
	if graph.ID != "gts.x.core.base.type.v1~x.test7.derived1.type.v1~x.test7.derived2.type.v1~" {
		t.Errorf("Expected ID to match, got: %s", graph.ID)
	}
	if len(graph.Refs) == 0 {
		t.Error("Expected graph to have refs")
	}
	if len(graph.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", graph.Errors)
	}

	// Verify that nested references are present
	hasNestedRefs := false
	for _, ref := range graph.Refs {
		if len(ref.Refs) > 0 {
			hasNestedRefs = true
			break
		}
	}
	if !hasNestedRefs {
		t.Error("Expected to find nested references in the graph")
	}
}

func TestBuildSchemaGraph_EntityNotFound(t *testing.T) {
	store := NewGtsStore(nil)

	// Build graph for non-existent entity
	graph := store.BuildSchemaGraph("gts.x.nonexistent.entity.v1")

	if graph == nil {
		t.Fatal("Expected graph to be non-nil")
	}
	if len(graph.Errors) == 0 {
		t.Error("Expected error for non-existent entity")
	}
	if graph.Errors[0] != "Entity not found" {
		t.Errorf("Expected 'Entity not found' error, got: %s", graph.Errors[0])
	}
}

func TestBuildSchemaGraph_CycleDetection(t *testing.T) {
	store := NewGtsStore(nil)

	// Create a schema that references another which references back (cycle)
	schema1 := map[string]any{
		"$id":     "gts://gts.x.core.cycle.a.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]any{
			"nested": map[string]any{
				"$ref": "gts.x.core.cycle.b.v1~",
			},
		},
	}
	entity1 := NewJsonEntity(schema1, DefaultGtsConfig())
	if err := store.Register(entity1); err != nil {
		t.Fatalf("Failed to register schema1: %v", err)
	}

	schema2 := map[string]any{
		"$id":     "gts://gts.x.core.cycle.b.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]any{
			"nested": map[string]any{
				"$ref": "gts.x.core.cycle.a.v1~",
			},
		},
	}
	entity2 := NewJsonEntity(schema2, DefaultGtsConfig())
	if err := store.Register(entity2); err != nil {
		t.Fatalf("Failed to register schema2: %v", err)
	}

	// Build graph - should handle cycle gracefully (not infinite loop)
	graph := store.BuildSchemaGraph("gts.x.core.cycle.a.v1~")

	if graph == nil {
		t.Fatal("Expected graph to be non-nil")
	}
	// Should detect and handle the cycle
	if len(graph.Refs) == 0 {
		t.Error("Expected graph to have refs")
	}
	// The cycle should be detected - schema B should not recursively include schema A again
	if len(graph.Refs) > 0 {
		for _, ref := range graph.Refs {
			if ref.ID == "gts.x.core.cycle.b.v1~" {
				// Schema B should have been visited, so it shouldn't have its refs populated again
				// This ensures we don't infinite loop
				return
			}
		}
	}
}

func TestExtractGtsReferences(t *testing.T) {
	content := map[string]any{
		"$id":  "gts://gts.x.test.core.schema.v1~",
		"$ref": "gts.x.test.core.base.v1~",
		"properties": map[string]any{
			"field1": map[string]any{
				"$ref": "gts.x.test.core.field.v1~",
			},
		},
		"items": []any{
			map[string]any{
				"$ref": "gts.x.test.core.item.v1~",
			},
		},
	}

	refs := extractGtsReferences(content)

	if len(refs) == 0 {
		t.Fatalf("Expected to extract references, got %d", len(refs))
	}

	// Should find all GTS IDs
	foundIDs := make(map[string]bool)
	for _, ref := range refs {
		t.Logf("Found GTS ID: %s at path: %s", ref.ID, ref.SourcePath)
		foundIDs[ref.ID] = true
	}

	expectedIDs := []string{
		"gts.x.test.core.schema.v1~",
		"gts.x.test.core.base.v1~",
		"gts.x.test.core.field.v1~",
		"gts.x.test.core.item.v1~",
	}

	for _, expectedID := range expectedIDs {
		if !foundIDs[expectedID] {
			t.Errorf("Expected to find GTS ID: %s", expectedID)
		}
	}
}
