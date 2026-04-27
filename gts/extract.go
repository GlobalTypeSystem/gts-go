/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"strings"
)

// JsonFile represents a JSON file containing one or more entities
type JsonFile struct {
	Path    string
	Name    string
	Content any
}

// JsonEntity represents a JSON object with extracted GTS identifiers
type JsonEntity struct {
	GtsID                 *GtsID
	SchemaID              string
	SelectedEntityField   string
	SelectedSchemaIDField string
	IsSchema              bool
	Content               map[string]any
	File                  *JsonFile
	ListSequence          *int
	Label                 string
	GtsRefs               []*GtsReference // All GTS ID references found in content
}

// ExtractIDResult holds the result of extracting ID information from JSON content
type ExtractIDResult struct {
	ID                    string  `json:"id"`
	SchemaID              *string `json:"schema_id"`
	SelectedEntityField   *string `json:"selected_entity_field"`
	SelectedSchemaIDField *string `json:"selected_schema_id_field"`
	IsSchema              bool    `json:"is_schema"`
}

// NewJsonEntity creates a JsonEntity from JSON content using the provided config
func NewJsonEntity(content map[string]any, cfg *GtsConfig) *JsonEntity {
	return NewJsonEntityWithFile(content, cfg, nil, nil)
}

// NewJsonEntityWithFile creates a JsonEntity with file and sequence information
func NewJsonEntityWithFile(content map[string]any, cfg *GtsConfig, file *JsonFile, listSequence *int) *JsonEntity {
	if cfg == nil {
		cfg = DefaultGtsConfig()
	}

	entity := &JsonEntity{
		Content:      content,
		IsSchema:     isJSONSchema(content),
		File:         file,
		ListSequence: listSequence,
	}

	// Extract entity ID
	entityIDValue := entity.calcJSONEntityID(cfg)

	// Extract schema ID
	entity.SchemaID = entity.calcJSONSchemaID(cfg, entityIDValue)

	// ID extraction logic based on entity type
	if entity.IsSchema {
		// For schemas: use entity ID (should be from $id field)
		if entityIDValue != "" && IsValidGtsID(entityIDValue) {
			gtsID, _ := NewGtsID(entityIDValue)
			entity.GtsID = gtsID
		}
	} else {
		// For instances: different logic based on well-known vs anonymous
		if entityIDValue != "" && IsValidGtsID(entityIDValue) {
			// Well-known instance: GTS ID in id field
			gtsID, _ := NewGtsID(entityIDValue)
			entity.GtsID = gtsID
			// Schema ID should be derived from the chain if not explicitly set
			if entity.SchemaID == "" && entity.SelectedEntityField != "" {
				entity.SchemaID = entity.calcJSONSchemaID(cfg, entityIDValue)
			}
		} else {
			// Anonymous instance: non-GTS ID in id field, GTS type in type field
			// GtsID remains nil for anonymous instances
			// entity.SchemaID should be set from type field
		}
	}

	// Extract GTS references from content
	entity.GtsRefs = extractGtsReferences(content)

	// Set label
	entity.setLabel()

	return entity
}

// EffectiveID returns the identifier used to key this entity in a registry
// and echo back to clients. Resolution order:
//  1. Parsed GTS ID (schemas and well-known instances).
//  2. Raw id field value for non-schemas (anonymous instances, spec §3.7) —
//     used even when the schema reference cannot be resolved; schema presence
//     is enforced at validation time, not at registration time.
//  3. File path (+ list sequence for multi-entity files) when the entity
//     originated from a file. Mirrors gts-rust.
//
// Returns "" if none of the above apply.
func (e *JsonEntity) EffectiveID() string {
	if e.GtsID != nil && e.GtsID.ID != "" {
		return e.GtsID.ID
	}
	if !e.IsSchema && e.SelectedEntityField != "" {
		if val, ok := e.Content[e.SelectedEntityField].(string); ok {
			if id := strings.TrimSpace(val); id != "" {
				return id
			}
		}
	}
	if e.File != nil {
		if e.ListSequence != nil {
			return fmt.Sprintf("%s#%d", e.File.Path, *e.ListSequence)
		}
		return e.File.Path
	}
	return ""
}

// setLabel sets the entity's label based on file, sequence, or GTS ID
func (e *JsonEntity) setLabel() {
	if e.File != nil && e.ListSequence != nil {
		e.Label = fmt.Sprintf("%s#%d", e.File.Name, *e.ListSequence)
	} else if e.File != nil {
		e.Label = e.File.Name
	} else if e.GtsID != nil {
		e.Label = e.GtsID.ID
	} else {
		e.Label = ""
	}
}

// isJSONSchema checks if the content represents a JSON Schema
// A JSON document is a schema if and only if it has a $schema field
func isJSONSchema(content map[string]any) bool {
	if content == nil {
		return false
	}

	// Schema Detection: a JSON document is a schema if and only if it has a $schema field
	_, hasSchema := content["$schema"]
	if !hasSchema {
		// Try alternative field name
		_, hasSchema = content["$$schema"]
	}

	return hasSchema
}

// getFieldValue retrieves a string value from content field
// For the "$id" field (JSON Schema), it strips the "gts://" URI prefix if present
func (e *JsonEntity) getFieldValue(field string) string {
	if e.Content == nil {
		return ""
	}

	val, ok := e.Content[field]
	if !ok {
		return ""
	}

	strVal, ok := val.(string)
	if !ok {
		return ""
	}

	trimmed := strings.TrimSpace(strVal)
	if trimmed == "" {
		return ""
	}

	// Strip the "gts://" URI prefix ONLY for $id field (JSON Schema compatibility)
	// The gts:// prefix is ONLY valid in the $id field of JSON Schema
	if field == "$id" {
		trimmed = strings.TrimPrefix(trimmed, GtsURIPrefix)
	}

	return trimmed
}

// firstNonEmptyField finds the first non-empty field, preferring valid GTS IDs
func (e *JsonEntity) firstNonEmptyField(fields []string) (string, string) {
	// First pass: look for valid GTS IDs
	for _, field := range fields {
		val := e.getFieldValue(field)
		if val != "" && IsValidGtsID(val) {
			return field, val
		}
	}

	// Second pass: any non-empty string
	for _, field := range fields {
		val := e.getFieldValue(field)
		if val != "" {
			return field, val
		}
	}

	return "", ""
}

// calcJSONEntityID extracts the entity ID from JSON content
func (e *JsonEntity) calcJSONEntityID(cfg *GtsConfig) string {
	field, value := e.firstNonEmptyField(cfg.EntityIDFields)
	e.SelectedEntityField = field
	return value
}

// calcJSONSchemaID extracts the schema ID from JSON content
func (e *JsonEntity) calcJSONSchemaID(cfg *GtsConfig, entityIDValue string) string {
	if e.IsSchema {
		// For derived schemas, derive parent type from chain
		if entityIDValue != "" && IsValidGtsID(entityIDValue) && strings.HasSuffix(entityIDValue, "~") {
			firstTilde := strings.Index(entityIDValue, "~")
			if firstTilde > 0 {
				secondTilde := strings.Index(entityIDValue[firstTilde+1:], "~")
				if secondTilde > 0 {
					// This is a derived schema, derive parent from chain
					e.SelectedSchemaIDField = e.SelectedEntityField
					return entityIDValue[:firstTilde+1]
				}
			}
		}

		// For base schemas: get schema ID from $schema field
		if schemaValue := e.getFieldValue("$schema"); schemaValue != "" {
			e.SelectedSchemaIDField = "$schema"
			return schemaValue
		}
		return ""
	}

	// For instances: try entity ID chain first, then SchemaIDFields
	if entityIDValue != "" && IsValidGtsID(entityIDValue) {
		// For instances, find last ~ and return everything up to and including it
		// But skip if entity ID ends with ~ (that would be a type, not an instance)
		if !strings.HasSuffix(entityIDValue, "~") {
			lastTilde := strings.LastIndex(entityIDValue, "~")
			if lastTilde > 0 {
				e.SelectedSchemaIDField = e.SelectedEntityField
				return entityIDValue[:lastTilde+1]
			}
		}
	}

	// If no entity ID found, use SchemaIDFields to find schema reference
	field, value := e.firstNonEmptyField(cfg.SchemaIDFields)
	if value != "" {
		e.SelectedSchemaIDField = field
		return value
	}

	return ""
}

// ExtractID extracts GTS ID information from JSON content
func ExtractID(content map[string]any, cfg *GtsConfig) *ExtractIDResult {
	entity := NewJsonEntity(content, cfg)

	result := &ExtractIDResult{
		IsSchema: entity.IsSchema,
	}

	// Set SchemaID as pointer (nil if empty)
	if entity.SchemaID != "" {
		result.SchemaID = &entity.SchemaID
	}

	// Set SelectedEntityField as pointer (nil if empty)
	if entity.SelectedEntityField != "" {
		result.SelectedEntityField = &entity.SelectedEntityField
	}

	// Set SelectedSchemaIDField as pointer (nil if empty)
	if entity.SelectedSchemaIDField != "" {
		result.SelectedSchemaIDField = &entity.SelectedSchemaIDField
	}

	// Diagnostic: for schemas and well-known instances, return the parsed GTS ID.
	// For non-schema entities without a valid GTS ID (anonymous or malformed),
	// fall back to the raw value of the selected id field. This differs from
	// EffectiveID (registrable), which requires a resolvable schema.
	switch {
	case entity.GtsID != nil:
		result.ID = entity.GtsID.ID
	case !entity.IsSchema && entity.SelectedEntityField != "":
		if val, ok := content[entity.SelectedEntityField].(string); ok {
			result.ID = val
		}
	}

	return result
}
