/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"strings"
	"testing"
)

func TestXGtsRefValidator_ValidateSchema_BasicPatterns(t *testing.T) {
	store := NewGtsStore(nil)
	validator := NewXGtsRefValidator(store)

	tests := []struct {
		name        string
		schema      map[string]interface{}
		shouldFail  bool
		errorContains string
	}{
		{
			name: "valid absolute GTS pattern",
			schema: map[string]interface{}{
				"$id":   "gts.x.test.ns.module.v1~",
				"type":  "object",
				"properties": map[string]interface{}{
					"capability": map[string]interface{}{
						"type":      "string",
						"x-gts-ref": "gts.x.test.ns.capability.v1~",
					},
				},
			},
			shouldFail: false,
		},
		{
			name: "valid self-reference",
			schema: map[string]interface{}{
				"$id":   "gts.x.test.ns.module.v1~",
				"type":  "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":      "string",
						"x-gts-ref": "/$id",
					},
				},
			},
			shouldFail: false,
		},
		{
			name: "valid wildcard pattern",
			schema: map[string]interface{}{
				"$id":   "gts.x.test.ns.module.v1~",
				"type":  "object",
				"properties": map[string]interface{}{
					"capability": map[string]interface{}{
						"type":      "string",
						"x-gts-ref": "gts.x.test.*",
					},
				},
			},
			shouldFail: false,
		},
		{
			name: "invalid GTS identifier",
			schema: map[string]interface{}{
				"$id":   "gts.x.test.ns.module.v1~",
				"type":  "object",
				"properties": map[string]interface{}{
					"capability": map[string]interface{}{
						"type":      "string",
						"x-gts-ref": "gts.x.y.z",
					},
				},
			},
			shouldFail: true,
			errorContains: "Invalid GTS identifier: gts.x.y.z",
		},
		{
			name: "invalid non-GTS pattern",
			schema: map[string]interface{}{
				"$id":   "gts.x.test.ns.module.v1~",
				"type":  "object",
				"properties": map[string]interface{}{
					"capability": map[string]interface{}{
						"type":      "string",
						"x-gts-ref": "a.b.c",
					},
				},
			},
			shouldFail: true,
			errorContains: "Invalid x-gts-ref value: 'a.b.c' must start with 'gts.' or '/'",
		},
		{
			name: "invalid pointer resolution",
			schema: map[string]interface{}{
				"$id":   "gts.x.test.ns.module.v1~",
				"type":  "object",
				"properties": map[string]interface{}{
					"capability": map[string]interface{}{
						"type":      "string",
						"x-gts-ref": "/nonexistent",
					},
				},
			},
			shouldFail: true,
			errorContains: "Cannot resolve reference path '/nonexistent'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateSchema(tt.schema, "", nil)
			
			if tt.shouldFail {
				if len(errors) == 0 {
					t.Errorf("Expected validation to fail, but no errors were returned")
				} else if tt.errorContains != "" {
					found := false
					for _, err := range errors {
						if containsSubstring(err.Error(), tt.errorContains) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error containing '%s', got errors: %v", tt.errorContains, errors)
					}
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected validation to pass, but got errors: %v", errors)
				}
			}
		})
	}
}

func TestXGtsRefValidator_ValidateInstance_PrefixValidation(t *testing.T) {
	store := NewGtsStore(nil)
	validator := NewXGtsRefValidator(store)

	// Register base capability schema
	capabilitySchema := map[string]interface{}{
		"$id":   "gts.x.testref.ns.capability.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":  "object",
		"required": []interface{}{"id", "description"},
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":      "string",
				"x-gts-ref": "/$id",
			},
			"description": map[string]interface{}{
				"type": "string",
			},
		},
		"additionalProperties": false,
	}

	capabilityEntity := NewJsonEntity(capabilitySchema, DefaultGtsConfig())
	store.Register(capabilityEntity)

	// Register valid capability instance
	validCapability := map[string]interface{}{
		"id":          "gts.x.testref.ns.capability.v1~x.vendor._.has_ws.v1",
		"description": "Has WebSocket",
	}
	validCapabilityEntity := NewJsonEntity(validCapability, DefaultGtsConfig())
	store.Register(validCapabilityEntity)

	// Register module schema that references capabilities
	moduleSchema := map[string]interface{}{
		"$id":   "gts.x.testref.ns.module.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":  "object",
		"required": []interface{}{"type", "id", "capabilities"},
		"properties": map[string]interface{}{
			"type": map[string]interface{}{
				"type":      "string",
				"x-gts-ref": "/$id",
			},
			"id": map[string]interface{}{
				"type": "string",
			},
			"capabilities": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":      "string",
					"x-gts-ref": "gts.x.testref.ns.capability.v1~",
				},
				"minItems":    0,
				"uniqueItems": true,
			},
		},
		"additionalProperties": false,
	}

	moduleEntity := NewJsonEntity(moduleSchema, DefaultGtsConfig())
	store.Register(moduleEntity)

	tests := []struct {
		name        string
		instance    map[string]interface{}
		shouldFail  bool
		errorContains string
	}{
		{
			name: "valid module with matching capability prefix",
			instance: map[string]interface{}{
				"type": "gts.x.testref.ns.module.v1~",
				"id":   "gts.x.testref.ns.module.v1~x.vendor._.chat.v1",
				"capabilities": []interface{}{
					"gts.x.testref.ns.capability.v1~x.vendor._.has_ws.v1",
				},
			},
			shouldFail: false,
		},
		{
			name: "invalid module with wrong capability prefix",
			instance: map[string]interface{}{
				"type": "gts.x.testref.ns.module.v1~",
				"id":   "gts.x.testref.ns.module.v1~x.vendor._.chat2.v1",
				"capabilities": []interface{}{
					"gts.y.other._.capability.v1~x.vendor._.foo.v1",
				},
			},
			shouldFail: true,
			errorContains: "does not match pattern",
		},
		{
			name: "invalid module with type mismatch",
			instance: map[string]interface{}{
				"type": "gts.x.testref._.wrong.v1~",
				"id":   "gts.x.testref.ns.module.v1~x.vendor._.chat3.v1",
				"capabilities": []interface{}{},
			},
			shouldFail: true,
			errorContains: "does not match pattern",
		},
		{
			name: "module with non-existent capability",
			instance: map[string]interface{}{
				"type": "gts.x.testref.ns.module.v1~",
				"id":   "gts.x.testref.ns.module.v1~x.vendor._.chat4.v1",
				"capabilities": []interface{}{
					"gts.x.testref.ns.capability.v1~x.vendor._.nonexistent.v1",
				},
			},
			shouldFail: true,
			errorContains: "not found in registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateInstance(tt.instance, moduleSchema, "")
			
			if tt.shouldFail {
				if len(errors) == 0 {
					t.Errorf("Expected validation to fail, but no errors were returned")
				} else if tt.errorContains != "" {
					found := false
					for _, err := range errors {
						if containsSubstring(err.Error(), tt.errorContains) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error containing '%s', got errors: %v", tt.errorContains, errors)
					}
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected validation to pass, but got errors: %v", errors)
				}
			}
		})
	}
}

func TestXGtsRefValidator_ValidateInstance_JsonPointerResolution(t *testing.T) {
	store := NewGtsStore(nil)
	validator := NewXGtsRefValidator(store)

	// Register schema with JSON pointer references
	pointerSchema := map[string]interface{}{
		"$id":   "gts.x.testref.ns.pointer.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title": "PTR-TITLE",
		"description": "PTR-DESC",
		"type":  "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":      "string",
				"x-gts-ref": "/$id",
			},
			"type": map[string]interface{}{
				"type":      "string",
				"x-gts-ref": "/properties/id/x-gts-ref", // This should resolve to "/$id"
			},
		},
		"required": []interface{}{"id"},
		"additionalProperties": false,
	}

	pointerEntity := NewJsonEntity(pointerSchema, DefaultGtsConfig())
	store.Register(pointerEntity)

	// Register the instance entity that will be referenced
	instanceEntity := map[string]interface{}{
		"id":   "gts.x.testref.ns.pointer.v1~x.vendor._.ptr_ok.v1",
		"type": "gts.x.testref.ns.pointer.v1~",
	}
	instanceEntityObj := NewJsonEntity(instanceEntity, DefaultGtsConfig())
	store.Register(instanceEntityObj)

	tests := []struct {
		name        string
		instance    map[string]interface{}
		shouldFail  bool
		errorContains string
	}{
		{
			name: "valid instance with correct pointer resolution",
			instance: map[string]interface{}{
				"id":   "gts.x.testref.ns.pointer.v1~x.vendor._.ptr_ok.v1",
				"type": "gts.x.testref.ns.pointer.v1~",
			},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateInstance(tt.instance, pointerSchema, "")
			
			if tt.shouldFail {
				if len(errors) == 0 {
					t.Errorf("Expected validation to fail, but no errors were returned")
				} else if tt.errorContains != "" {
					found := false
					for _, err := range errors {
						if containsSubstring(err.Error(), tt.errorContains) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error containing '%s', got errors: %v", tt.errorContains, errors)
					}
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected validation to pass, but got errors: %v", errors)
				}
			}
		})
	}
}

func TestXGtsRefValidator_ResolvePointer(t *testing.T) {
	validator := NewXGtsRefValidator(nil)

	schema := map[string]interface{}{
		"$id": "gts.x.test.ns.module.v1~",
		"properties": map[string]interface{}{
			"type": map[string]interface{}{
				"const": "gts.x.test.ns.type.v1~",
			},
			"nested": map[string]interface{}{
				"properties": map[string]interface{}{
					"anchor": map[string]interface{}{
						"const": "gts.x.test.ns.anchor.v1~",
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		pointer  string
		expected string
	}{
		{
			name:     "resolve $id",
			pointer:  "/$id",
			expected: "gts.x.test.ns.module.v1~",
		},
		{
			name:     "resolve property const",
			pointer:  "/properties/type/const",
			expected: "gts.x.test.ns.type.v1~",
		},
		{
			name:     "resolve nested property const",
			pointer:  "/properties/nested/properties/anchor/const",
			expected: "gts.x.test.ns.anchor.v1~",
		},
		{
			name:     "resolve non-existent path",
			pointer:  "/properties/nonexistent",
			expected: "",
		},
		{
			name:     "resolve empty path",
			pointer:  "/",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.resolvePointer(schema, tt.pointer)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestXGtsRefValidator_ValidateGtsPattern(t *testing.T) {
	validator := NewXGtsRefValidator(nil) // No store to avoid entity existence checks

	tests := []struct {
		name        string
		value       string
		pattern     string
		shouldFail  bool
		errorContains string
	}{
		{
			name:       "exact match",
			value:      "gts.x.test.ns.module.v1~",
			pattern:    "gts.x.test.ns.module.v1~",
			shouldFail: false,
		},
		{
			name:       "prefix match",
			value:      "gts.x.test.ns.module.v1~x.vendor._.instance.v1",
			pattern:    "gts.x.test.ns.module.v1~",
			shouldFail: false,
		},
		{
			name:       "wildcard match",
			value:      "gts.x.test.ns.anything.v1~",
			pattern:    "gts.x.test.*",
			shouldFail: false,
		},
		{
			name:       "global wildcard match",
			value:      "gts.anything.works.ns.type.v1~",
			pattern:    "gts.*",
			shouldFail: false,
		},
		{
			name:        "prefix mismatch",
			value:       "gts.y.other.ns.module.v1~",
			pattern:     "gts.x.test.*",
			shouldFail:  true,
			errorContains: "does not match pattern",
		},
		{
			name:        "invalid GTS ID",
			value:       "not.gts.format",
			pattern:     "gts.*",
			shouldFail:  true,
			errorContains: "not a valid GTS identifier",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateGtsPattern(tt.value, tt.pattern, "test_field")
			
			if tt.shouldFail {
				if err == nil {
					t.Errorf("Expected validation to fail, but no error was returned")
				} else if tt.errorContains != "" && !containsSubstring(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected validation to pass, but got error: %s", err.Error())
				}
			}
		})
	}
}

func TestGtsStore_ValidateSchemaWithXGtsRef(t *testing.T) {
	store := NewGtsStore(nil)

	// Register the capability schema that will be referenced
	capabilitySchema := map[string]interface{}{
		"$id":   "gts.x.test.ns.capability.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":  "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type": "string",
			},
		},
	}
	capabilityEntity := NewJsonEntity(capabilitySchema, DefaultGtsConfig())
	store.Register(capabilityEntity)

	// Register a schema with x-gts-ref
	schema := map[string]interface{}{
		"$id":   "gts.x.test.ns.module.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":  "object",
		"properties": map[string]interface{}{
			"type": map[string]interface{}{
				"type":      "string",
				"x-gts-ref": "/$id",
			},
			"capability": map[string]interface{}{
				"type":      "string",
				"x-gts-ref": "gts.x.test.ns.capability.v1~",
			},
		},
	}

	entity := NewJsonEntity(schema, DefaultGtsConfig())
	store.Register(entity)

	// Test validation
	err := store.ValidateSchema("gts.x.test.ns.module.v1~")
	if err != nil {
		t.Errorf("Expected schema validation to pass, but got error: %s", err.Error())
	}
}

func TestGtsStore_ValidateInstanceWithXGtsRef(t *testing.T) {
	store := NewGtsStore(nil)

	// Register capability schema
	capabilitySchema := map[string]interface{}{
		"$id":   "gts.x.test.ns.capability.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":  "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type": "string",
			},
		},
	}
	capabilityEntity := NewJsonEntity(capabilitySchema, DefaultGtsConfig())
	store.Register(capabilityEntity)

	// Register capability instance
	capability := map[string]interface{}{
		"id": "gts.x.test.ns.capability.v1~x.vendor._.ws.v1",
	}
	capabilityInstanceEntity := NewJsonEntity(capability, DefaultGtsConfig())
	store.Register(capabilityInstanceEntity)

	// Register module schema with x-gts-ref
	moduleSchema := map[string]interface{}{
		"$id":   "gts.x.test.ns.module.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":  "object",
		"properties": map[string]interface{}{
			"type": map[string]interface{}{
				"type":      "string",
				"x-gts-ref": "/$id",
			},
			"capability": map[string]interface{}{
				"type":      "string",
				"x-gts-ref": "gts.x.test.ns.capability.v1~",
			},
		},
	}
	moduleEntity := NewJsonEntity(moduleSchema, DefaultGtsConfig())
	store.Register(moduleEntity)

	// Register valid module instance
	validModule := map[string]interface{}{
		"id":         "gts.x.test.ns.module.v1~x.vendor._.test.v1",
		"type":       "gts.x.test.ns.module.v1~",
		"capability": "gts.x.test.ns.capability.v1~x.vendor._.ws.v1",
	}
	validModuleEntity := NewJsonEntity(validModule, DefaultGtsConfig())
	store.Register(validModuleEntity)

	// Test validation  
	err := store.ValidateInstanceWithXGtsRef("gts.x.test.ns.module.v1~x.vendor._.test.v1")
	if err != nil {
		t.Errorf("Expected instance validation to pass, but got error: %s", err.Error())
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}