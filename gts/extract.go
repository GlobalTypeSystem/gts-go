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
	SchemaID              string  `json:"schema_id"`
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

	// If no valid GTS ID found in entity fields, use schema ID as fallback
	if entityIDValue == "" || !IsValidGtsID(entityIDValue) {
		if entity.SchemaID != "" && IsValidGtsID(entity.SchemaID) {
			entityIDValue = entity.SchemaID
		}
	}

	// Create GtsID if valid
	if entityIDValue != "" && IsValidGtsID(entityIDValue) {
		gtsID, _ := NewGtsID(entityIDValue)
		entity.GtsID = gtsID
	}

	// Extract GTS references from content
	entity.GtsRefs = extractGtsReferences(content)

	// Set label
	entity.setLabel()

	return entity
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
func isJSONSchema(content map[string]any) bool {
	if content == nil {
		return false
	}

	schemaURL, ok := content["$schema"]
	if !ok {
		schemaURL, ok = content["$$schema"]
		if !ok {
			return false
		}
	}

	schemaStr, ok := schemaURL.(string)
	if !ok {
		return false
	}

	// Check if this is a JSON Schema meta-schema reference
	if strings.HasPrefix(schemaStr, "http://json-schema.org/") ||
		strings.HasPrefix(schemaStr, "https://json-schema.org/") {
		return true
	}

	// Special GTS schema protocol
	if strings.HasPrefix(schemaStr, "gts://") {
		return true
	}

	// If $schema points to a GTS type ID, determine if this is a schema or instance
	if strings.HasPrefix(schemaStr, "gts.") {
		// Check for entity ID fields that might indicate this is a schema
		entityIDFields := []string{"$id", "gtsId", "gtsIid", "gtsOid", "gtsI", "gts_id", "gts_oid", "gts_iid", "id"}

		for _, field := range entityIDFields {
			if idVal, hasID := content[field]; hasID {
				if idStr, ok := idVal.(string); ok && strings.HasSuffix(idStr, "~") {
					// Entity ID ends with ~ - this is definitely a schema
					return true
				}
			}
		}

		// No entity ID field ending with ~
		// Check if this could be a schema without explicit entity ID based on content
		if strings.HasSuffix(schemaStr, "~") {
			// Additional heuristic: if it has schema-like properties, consider it a schema
			if _, hasType := content["type"]; hasType {
				if _, hasProps := content["properties"]; hasProps {
					return true
				}
				if _, hasItems := content["items"]; hasItems {
					return true
				}
				if _, hasEnum := content["enum"]; hasEnum {
					return true
				}
			}

			// Check if it has NO entity ID fields at all (pure schema)
			hasEntityID := false
			for _, field := range entityIDFields {
				if _, exists := content[field]; exists {
					hasEntityID = true
					break
				}
			}

			if !hasEntityID {
				// No entity ID and $schema ends with ~ - likely a schema definition
				return true
			}
		}

		// Has entity ID but doesn't end with ~, or has other characteristics of an instance
		return false
	}

	return false
}

// getFieldValue retrieves a string value from content field
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

	return strings.TrimSpace(strVal)
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
	field, value := e.firstNonEmptyField(cfg.SchemaIDFields)
	if value != "" {
		e.SelectedSchemaIDField = field
		return value
	}

	// If no schema ID field found, try to derive from entity ID
	if entityIDValue != "" && IsValidGtsID(entityIDValue) {
		// If entity ID ends with ~, it's already a type ID
		if strings.HasSuffix(entityIDValue, "~") {
			// Don't set SelectedSchemaIDField - the entity ID itself is a type
			return entityIDValue
		}

		// Find last ~ and return everything up to and including it
		lastTilde := strings.LastIndex(entityIDValue, "~")
		if lastTilde > 0 {
			// Set SelectedSchemaIDField to the entity field since we extracted from it
			e.SelectedSchemaIDField = e.SelectedEntityField
			return entityIDValue[:lastTilde+1]
		}
	}

	return ""
}

// ExtractID extracts GTS ID information from JSON content
func ExtractID(content map[string]any, cfg *GtsConfig) *ExtractIDResult {
	entity := NewJsonEntity(content, cfg)

	result := &ExtractIDResult{
		SchemaID: entity.SchemaID,
		IsSchema: entity.IsSchema,
	}

	// Set SelectedEntityField as pointer (nil if empty)
	if entity.SelectedEntityField != "" {
		result.SelectedEntityField = &entity.SelectedEntityField
	}

	// Set SelectedSchemaIDField as pointer (nil if empty)
	if entity.SelectedSchemaIDField != "" {
		result.SelectedSchemaIDField = &entity.SelectedSchemaIDField
	}

	if entity.GtsID != nil {
		result.ID = entity.GtsID.ID
	}

	return result
}
