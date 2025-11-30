/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"strings"
	"testing"
)

func TestRegistryConfig(t *testing.T) {
	t.Run("DefaultRegistryConfig", func(t *testing.T) {
		config := DefaultRegistryConfig()
		if config == nil {
			t.Fatal("DefaultRegistryConfig returned nil")
		}
		if config.ValidateGtsReferences {
			t.Error("Default config should have ValidateGtsReferences=false")
		}
	})
}

func TestNewGtsStoreWithConfig(t *testing.T) {
	t.Run("WithNilConfig", func(t *testing.T) {
		store := NewGtsStoreWithConfig(nil, nil)
		if store == nil {
			t.Fatal("NewGtsStoreWithConfig returned nil")
		}
		if store.config == nil {
			t.Fatal("Store config should not be nil")
		}
		if store.config.ValidateGtsReferences {
			t.Error("Default config should have ValidateGtsReferences=false")
		}
	})

	t.Run("WithValidationEnabled", func(t *testing.T) {
		config := &RegistryConfig{ValidateGtsReferences: true}
		store := NewGtsStoreWithConfig(nil, config)
		if store == nil {
			t.Fatal("NewGtsStoreWithConfig returned nil")
		}
		if !store.config.ValidateGtsReferences {
			t.Error("Config should have ValidateGtsReferences=true")
		}
	})
}

func TestGtsReferenceValidation(t *testing.T) {
	t.Run("SuccessfulValidation", func(t *testing.T) {
		// Create store with validation enabled
		config := &RegistryConfig{ValidateGtsReferences: true}
		store := NewGtsStoreWithConfig(nil, config)

		// First register a schema
		schema := NewJsonEntity(map[string]any{
			"$id":     "gts.test.pkg.ns.user.v1~",
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type":    "object",
			"properties": map[string]any{
				"id":   map[string]any{"type": "string"},
				"name": map[string]any{"type": "string"},
			},
		}, DefaultGtsConfig())

		err := store.Register(schema)
		if err != nil {
			t.Fatalf("Failed to register schema: %v", err)
		}

		// Now register an instance that references the schema
		instance := NewJsonEntity(map[string]any{
			"gtsId":   "gts.test.pkg.ns.user.v1.0",
			"$schema": "gts.test.pkg.ns.user.v1~",
			"id":      "user-123",
			"name":    "John Doe",
		}, DefaultGtsConfig())

		err = store.Register(instance)
		if err != nil {
			t.Fatalf("Failed to register instance: %v", err)
		}
	})

	t.Run("ValidationDisabled", func(t *testing.T) {
		// Create store with validation disabled (default)
		store := NewGtsStore(nil)

		// Register an instance that references non-existent schema
		instance := NewJsonEntity(map[string]any{
			"gtsId":   "gts.test.pkg.ns.user.v1.0",
			"$schema": "gts.test.pkg.ns.nonexistent.v1~",
			"id":      "user-123",
			"name":    "John Doe",
		}, DefaultGtsConfig())

		// Should succeed since validation is disabled
		err := store.Register(instance)
		if err != nil {
			t.Fatalf("Registration should succeed with validation disabled: %v", err)
		}
	})

	t.Run("MissingReference", func(t *testing.T) {
		// Create store with validation enabled
		config := &RegistryConfig{ValidateGtsReferences: true}
		store := NewGtsStoreWithConfig(nil, config)

		// Register an instance that references non-existent schema
		instance := NewJsonEntity(map[string]any{
			"gtsId":   "gts.test.pkg.ns.user.v1.0",
			"$schema": "gts.test.pkg.ns.nonexistent.v1~",
			"id":      "user-123",
			"name":    "John Doe",
		}, DefaultGtsConfig())

		err := store.Register(instance)
		if err == nil {
			t.Fatal("Expected validation error for missing reference")
		}
		if !strings.Contains(err.Error(), "referenced entity not found") {
			t.Errorf("Expected 'referenced entity not found' error, got: %v", err)
		}
	})

	t.Run("SelfReferenceSkipped", func(t *testing.T) {
		// Create store with validation enabled
		config := &RegistryConfig{ValidateGtsReferences: true}
		store := NewGtsStoreWithConfig(nil, config)

		// Register a schema that references itself (should be allowed)
		schema := NewJsonEntity(map[string]any{
			"$id":     "gts.test.pkg.ns.recursive.v1~",
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type":    "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
				"child": map[string]any{
					"$ref": "gts.test.pkg.ns.recursive.v1~",
				},
			},
		}, DefaultGtsConfig())

		err := store.Register(schema)
		if err != nil {
			t.Fatalf("Failed to register self-referencing schema: %v", err)
		}
	})

	t.Run("JSONSchemaMetaSchemaSkipped", func(t *testing.T) {
		// Create store with validation enabled
		config := &RegistryConfig{ValidateGtsReferences: true}
		store := NewGtsStoreWithConfig(nil, config)

		// Register a schema that references JSON Schema meta-schema (should be allowed)
		schema := NewJsonEntity(map[string]any{
			"$id":     "gts.test.pkg.ns.schema.v1~",
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type":    "object",
		}, DefaultGtsConfig())

		err := store.Register(schema)
		if err != nil {
			t.Fatalf("Failed to register schema with meta-schema reference: %v", err)
		}
	})
}

func TestValidateSchema(t *testing.T) {
	t.Run("ValidSchema", func(t *testing.T) {
		store := NewGtsStore(nil)

		// Register a valid schema
		schema := NewJsonEntity(map[string]any{
			"$id":     "gts.test.pkg.ns.valid.v1~",
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type":    "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
		}, DefaultGtsConfig())

		err := store.Register(schema)
		if err != nil {
			t.Fatalf("Failed to register schema: %v", err)
		}

		// Validate the schema
		err = store.ValidateSchema("gts.test.pkg.ns.valid.v1~")
		if err != nil {
			t.Fatalf("Schema validation failed: %v", err)
		}
	})

	t.Run("NonSchemaID", func(t *testing.T) {
		store := NewGtsStore(nil)

		err := store.ValidateSchema("gts.test.pkg.ns.instance.v1.0")
		if err == nil {
			t.Fatal("Expected error for non-schema ID")
		}
		if !strings.Contains(err.Error(), "is not a schema") {
			t.Errorf("Expected 'is not a schema' error, got: %v", err)
		}
	})

	t.Run("SchemaNotFound", func(t *testing.T) {
		store := NewGtsStore(nil)

		err := store.ValidateSchema("gts.test.pkg.ns.nonexistent.v1~")
		if err == nil {
			t.Fatal("Expected error for non-existent schema")
		}
		_, ok := err.(*StoreGtsSchemaNotFoundError)
		if !ok {
			t.Errorf("Expected StoreGtsSchemaNotFoundError, got: %T", err)
		}
	})

	t.Run("EntityIsNotSchema", func(t *testing.T) {
		store := NewGtsStore(nil)

		// Register a regular entity with schema-like ID
		instance := NewJsonEntity(map[string]any{
			"gtsId": "gts.test.pkg.ns.instance.v1~",
			"name":  "Test Instance",
		}, DefaultGtsConfig())

		// Force it to be treated as non-schema
		instance.IsSchema = false
		err := store.Register(instance)
		if err != nil {
			t.Fatalf("Failed to register instance: %v", err)
		}

		err = store.ValidateSchema("gts.test.pkg.ns.instance.v1~")
		if err == nil {
			t.Fatal("Expected error for entity that is not a schema")
		}
		if !strings.Contains(err.Error(), "is not a schema") {
			t.Errorf("Expected 'is not a schema' error, got: %v", err)
		}
	})
}

func TestRegistryIntegration(t *testing.T) {
	t.Run("CompleteWorkflow", func(t *testing.T) {
		// Create store with validation enabled
		config := &RegistryConfig{ValidateGtsReferences: true}
		store := NewGtsStoreWithConfig(nil, config)

		// 1. Register base schemas first
		userSchema := NewJsonEntity(map[string]any{
			"$id":     "gts.test.pkg.ns.user.v1~",
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type":    "object",
			"properties": map[string]any{
				"id":   map[string]any{"type": "string"},
				"name": map[string]any{"type": "string"},
			},
		}, DefaultGtsConfig())

		err := store.Register(userSchema)
		if err != nil {
			t.Fatalf("Failed to register user schema: %v", err)
		}

		// 2. Register a schema that extends the base schema
		extendedSchema := NewJsonEntity(map[string]any{
			"$id":     "gts.test.pkg.ns.admin.v1~",
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"allOf": []any{
				map[string]any{"$ref": "gts.test.pkg.ns.user.v1~"},
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"permissions": map[string]any{"type": "array"},
					},
				},
			},
		}, DefaultGtsConfig())

		err = store.Register(extendedSchema)
		if err != nil {
			t.Fatalf("Failed to register extended schema: %v", err)
		}

		// 3. Register instances
		userInstance := NewJsonEntity(map[string]any{
			"gtsId":   "gts.test.pkg.ns.user.v1.0",
			"$schema": "gts.test.pkg.ns.user.v1~",
			"id":      "user-123",
			"name":    "John Doe",
		}, DefaultGtsConfig())

		err = store.Register(userInstance)
		if err != nil {
			t.Fatalf("Failed to register user instance: %v", err)
		}

		adminInstance := NewJsonEntity(map[string]any{
			"gtsId":       "gts.test.pkg.ns.admin.v1.0",
			"$schema":     "gts.test.pkg.ns.admin.v1~",
			"id":          "admin-456",
			"name":        "Jane Admin",
			"permissions": []string{"read", "write"},
		}, DefaultGtsConfig())

		err = store.Register(adminInstance)
		if err != nil {
			t.Fatalf("Failed to register admin instance: %v", err)
		}

		// 4. Validate schemas
		err = store.ValidateSchema("gts.test.pkg.ns.user.v1~")
		if err != nil {
			t.Fatalf("User schema validation failed: %v", err)
		}

		err = store.ValidateSchema("gts.test.pkg.ns.admin.v1~")
		if err != nil {
			t.Fatalf("Admin schema validation failed: %v", err)
		}

		// 5. Query the store
		result := store.Query("gts.test.pkg.ns.*", 10)
		if result.Error != "" {
			t.Fatalf("Query failed: %s", result.Error)
		}
		if result.Count != 4 {
			t.Errorf("Expected 4 entities, got %d", result.Count)
		}

		// 6. Verify all entities are in store
		if store.Count() != 4 {
			t.Errorf("Expected 4 total entities in store, got %d", store.Count())
		}
	})
}
