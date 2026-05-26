/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"testing"
)

// TestExtractID_BasicEntityID tests extracting entity ID from JSON content
func TestExtractID_BasicEntityID(t *testing.T) {
	tests := []struct {
		name          string
		content       map[string]any
		expectedID    string
		expectedField string
	}{
		{
			name: "Extract from gtsId field",
			content: map[string]any{
				"gtsId": "gts.vendor.package.namespace.type.v0~a.b.c.d.v1",
				"name":  "Test Entity",
			},
			expectedID:    "gts.vendor.package.namespace.type.v0~a.b.c.d.v1",
			expectedField: "gtsId",
		},
		{
			name: "Extract from $id field",
			content: map[string]any{
				"$id":  "gts.vendor.package.namespace.type.v1~a.b.c.d.v1",
				"name": "Test Entity",
			},
			expectedID:    "gts.vendor.package.namespace.type.v1~a.b.c.d.v1",
			expectedField: "$id",
		},
		{
			name: "Extract from id field (fallback)",
			content: map[string]any{
				"id":   "gts.vendor.package.namespace.type.v2",
				"name": "Test Entity",
			},
			expectedID:    "gts.vendor.package.namespace.type.v2",
			expectedField: "id",
		},
		{
			name: "Priority: gtsId over id",
			content: map[string]any{
				"gtsId": "gts.vendor1.package.namespace.type.v0",
				"id":    "gts.vendor2.package.namespace.type.v0",
			},
			expectedID:    "gts.vendor1.package.namespace.type.v0",
			expectedField: "gtsId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractID(tt.content, nil)
			if result.ID != tt.expectedID {
				t.Errorf("Expected ID %q, got %q", tt.expectedID, result.ID)
			}
			if result.SelectedEntityField == nil || *result.SelectedEntityField != tt.expectedField {
				var got string
				if result.SelectedEntityField != nil {
					got = *result.SelectedEntityField
				}
				t.Errorf("Expected field %q, got %q", tt.expectedField, got)
			}
		})
	}
}

// TestExtractID_SchemaID tests extracting schema ID from JSON content
func TestExtractID_SchemaID(t *testing.T) {
	tests := []struct {
		name                string
		content             map[string]any
		expectedTypeID      string
		expectedSchemaField string
	}{
		{
			name: "Extract from $schema field",
			content: map[string]any{
				"gtsId":   "gts.vendor.package.namespace.type.v0.1",
				"$schema": "gts.vendor.package.namespace.type.v0~",
			},
			expectedTypeID:      "gts.vendor.package.namespace.type.v0~",
			expectedSchemaField: "$schema",
		},
		{
			name: "Extract from gtsTid field",
			content: map[string]any{
				"gtsId":  "gts.vendor.package.namespace.type.v0.1",
				"gtsTid": "gts.vendor.package.namespace.type.v0~",
			},
			expectedTypeID:      "gts.vendor.package.namespace.type.v0~",
			expectedSchemaField: "gtsTid",
		},
		{
			name: "Derive from entity ID with tilde",
			content: map[string]any{
				"gtsId": "gts.vendor.package.namespace.type.v0~a.b.c.d.v1.0",
			},
			expectedTypeID:      "gts.vendor.package.namespace.type.v0~",
			expectedSchemaField: "gtsId", // Derived from the chained ID
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractID(tt.content, nil)
			var gotTypeID string
			if result.TypeID != nil {
				gotTypeID = *result.TypeID
			}
			if gotTypeID != tt.expectedTypeID {
				t.Errorf("Expected TypeID %q, got %q", tt.expectedTypeID, gotTypeID)
			}

			// Handle both empty string expectation and actual value
			var got string
			if result.SelectedTypeIDField != nil {
				got = *result.SelectedTypeIDField
			}
			if got != tt.expectedSchemaField {
				t.Errorf("Expected schema field %q, got %q", tt.expectedSchemaField, got)
			}
		})
	}
}

// TestExtractID_IsSchema tests detecting JSON Schema documents
func TestExtractID_IsSchema(t *testing.T) {
	tests := []struct {
		name           string
		content        map[string]any
		expectedSchema bool
	}{
		{
			name: "JSON Schema with http://json-schema.org/",
			content: map[string]any{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"gtsId":   "gts.vendor.package.namespace.type.v0~",
			},
			expectedSchema: true,
		},
		{
			name: "JSON Schema with https://json-schema.org/",
			content: map[string]any{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"gtsId":   "gts.vendor.package.namespace.type.v0~",
			},
			expectedSchema: true,
		},
		{
			name: "GTS Schema with gts:// prefix",
			content: map[string]any{
				"$schema": "gts://vendor.package.namespace.type.v0~",
			},
			expectedSchema: true,
		},
		{
			name: "GTS Schema with gts. prefix",
			content: map[string]any{
				"$schema": "gts.vendor.package.namespace.type.v0~",
			},
			expectedSchema: true,
		},
		{
			name: "Regular entity (not a schema)",
			content: map[string]any{
				"gtsId": "gts.vendor.package.namespace.type.v0.1",
				"name":  "Test Entity",
			},
			expectedSchema: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractID(tt.content, nil)
			if result.IsTypeSchema != tt.expectedSchema {
				t.Errorf("Expected IsTypeSchema %v, got %v", tt.expectedSchema, result.IsTypeSchema)
			}
		})
	}
}

// TestExtractID_CustomConfig tests using custom configuration
func TestExtractID_CustomConfig(t *testing.T) {
	customCfg := &GtsConfig{
		EntityIDFields: []string{"customId", "id"},
		TypeIDFields:   []string{"customType", "type"},
	}

	content := map[string]any{
		"customId":   "gts.vendor.package.namespace.type.v0",
		"customType": "gts.vendor.package.namespace.type.v0~",
	}

	result := ExtractID(content, customCfg)

	if result.ID != "gts.vendor.package.namespace.type.v0" {
		t.Errorf("Expected ID from customId field, got %q", result.ID)
	}
	if result.SelectedEntityField == nil || *result.SelectedEntityField != "customId" {
		var got string
		if result.SelectedEntityField != nil {
			got = *result.SelectedEntityField
		}
		t.Errorf("Expected customId field, got %q", got)
	}
	var gotTypeID string
	if result.TypeID != nil {
		gotTypeID = *result.TypeID
	}
	if gotTypeID != "gts.vendor.package.namespace.type.v0~" {
		t.Errorf("Expected TypeID from customType field, got %q", gotTypeID)
	}
	if result.SelectedTypeIDField == nil || *result.SelectedTypeIDField != "customType" {
		var got string
		if result.SelectedTypeIDField != nil {
			got = *result.SelectedTypeIDField
		}
		t.Errorf("Expected customType field, got %q", got)
	}
}

// TestExtractID_NoValidID tests extraction when no valid GTS ID is found
func TestExtractID_NoValidID(t *testing.T) {
	content := map[string]any{
		"name":        "Test Entity",
		"description": "No GTS ID here",
	}

	result := ExtractID(content, nil)

	if result.ID != "" {
		t.Errorf("Expected empty ID, got %q", result.ID)
	}
	if result.SelectedEntityField != nil {
		t.Errorf("Expected nil SelectedEntityField, got %q", *result.SelectedEntityField)
	}
}

// TestExtractID_InvalidIDInField tests extraction when field contains invalid GTS ID
func TestExtractID_InvalidIDInField(t *testing.T) {
	content := map[string]any{
		"gtsId": "not-a-valid-gts-id",
		"id":    "gts.vendor.package.namespace.type.v0~a.b.c.d.v1",
	}

	result := ExtractID(content, nil)

	// Should fallback to the "id" field which has a valid GTS ID
	if result.ID != "gts.vendor.package.namespace.type.v0~a.b.c.d.v1" {
		t.Errorf("Expected fallback to valid ID, got %q", result.ID)
	}
}

// TestExtractID_SchemaIDFallback tests schema ID extraction for schemas with $schema field
func TestExtractID_SchemaIDFallback(t *testing.T) {
	content := map[string]any{
		"$id":     "gts.vendor.package.namespace.type.v0~",
		"$schema": "http://json-schema.org/draft-07/schema#",
	}

	result := ExtractID(content, nil)

	// For schemas, ID comes from $id field
	if result.ID != "gts.vendor.package.namespace.type.v0~" {
		t.Errorf("Expected ID from $id field, got %q", result.ID)
	}
	var gotTypeID string
	if result.TypeID != nil {
		gotTypeID = *result.TypeID
	}
	// For base schemas, schema_id comes from $schema field
	if gotTypeID != "http://json-schema.org/draft-07/schema#" {
		t.Errorf("Expected TypeID from $schema field, got %q", gotTypeID)
	}
}

// =============================================================================
// Tests for URI prefix "gts://" in JSON Schema $id field
// The gts:// prefix is used in JSON Schema for URI compatibility.
// GtsEntity strips it when parsing so the GtsID works with normal "gts." format.
// =============================================================================

// TestExtractID_GtsURIPrefix_InDollarIdField tests that gts:// prefix is stripped from $id field
func TestExtractID_GtsURIPrefix_InDollarIdField(t *testing.T) {
	content := map[string]any{
		"$id":     "gts://gts.vendor.package.namespace.type.v1.0~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
	}

	result := ExtractID(content, nil)

	// The gts:// prefix should be stripped from the $id field
	if result.ID != "gts.vendor.package.namespace.type.v1.0~" {
		t.Errorf("Expected ID without gts:// prefix %q, got %q", "gts.vendor.package.namespace.type.v1.0~", result.ID)
	}
	if !result.IsTypeSchema {
		t.Errorf("Expected IsTypeSchema to be true")
	}
}

// TestExtractID_GtsURIPrefix_NotStrippedFromOtherFields tests that gts:// prefix is NOT stripped from non-$id fields
func TestExtractID_GtsURIPrefix_NotStrippedFromOtherFields(t *testing.T) {
	// gts:// prefix in non-$id field should NOT be stripped
	// The value "gts://gts.vendor..." is not a valid GTS ID, so it's treated as an anonymous instance
	content := map[string]any{
		"id": "gts://gts.vendor.package.namespace.type.v1~a.b.c.d.v1.0",
	}

	result := ExtractID(content, nil)

	// The "id" field is not $id, so the gts:// prefix is NOT stripped
	// The raw value is returned as-is for anonymous instances (non-GTS IDs)
	expectedID := "gts://gts.vendor.package.namespace.type.v1~a.b.c.d.v1.0"
	if result.ID != expectedID {
		t.Errorf("Expected ID %q (raw value for anonymous instance), got %q", expectedID, result.ID)
	}
}

// TestExtractID_GtsColonPrefix_NotValid tests that gts: (without //) is NOT a valid prefix
func TestExtractID_GtsColonPrefix_NotValid(t *testing.T) {
	// "gts:" (without //) is NOT a valid prefix - only "gts://" is valid
	content := map[string]any{
		"$id":     "gts:gts.vendor.package.namespace.type.v1.0~",
		"$schema": "http://json-schema.org/draft-07/schema#",
	}

	result := ExtractID(content, nil)

	// With "gts:" prefix (not "gts://"), the ID is not stripped and won't be valid
	// The entity should NOT have a valid GTS ID
	if result.ID != "" {
		t.Errorf("Expected empty ID (gts: prefix without // should not be stripped), got %q", result.ID)
	}
}

// TestExtractID_GtsURIPrefix_WithoutPrefix tests that IDs without prefix still work
func TestExtractID_GtsURIPrefix_WithoutPrefix(t *testing.T) {
	// IDs without gts:// prefix should work as before
	content := map[string]any{
		"$id":     "gts.vendor.package.namespace.type.v1.0~",
		"$schema": "http://json-schema.org/draft-07/schema#",
	}

	result := ExtractID(content, nil)

	if result.ID != "gts.vendor.package.namespace.type.v1.0~" {
		t.Errorf("Expected ID %q, got %q", "gts.vendor.package.namespace.type.v1.0~", result.ID)
	}
}
