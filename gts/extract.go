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
	GtsID               *GtsID
	TypeID              string
	SelectedEntityField string
	SelectedTypeIDField string
	IsTypeSchema        bool
	Content             map[string]any
	File                *JsonFile
	ListSequence        *int
	Label               string
	GtsRefs             []*GtsReference // All GTS ID references found in content
}

// ExtractIDResult holds the result of extracting ID information from JSON content
type ExtractIDResult struct {
	ID                  string  `json:"id"`
	TypeID              *string `json:"type_id"`
	SelectedEntityField *string `json:"selected_entity_field"`
	SelectedTypeIDField *string `json:"selected_type_id_field"`
	IsTypeSchema        bool    `json:"is_type_schema"`
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
		IsTypeSchema: isJSONSchema(content),
		File:         file,
		ListSequence: listSequence,
	}

	// Extract entity ID
	entityIDValue := entity.calcJSONEntityID(cfg)

	// Extract type ID
	entity.TypeID = entity.calcJSONTypeID(cfg, entityIDValue)

	// ID extraction logic based on entity type
	if entity.IsTypeSchema {
		// Validate that schema $id uses the gts:// URI form, not the bare gts.
		// prefix. Per gts-spec, schemas must place the identifier in $id as a
		// gts:// URI. Leaving GtsID nil here causes downstream registration to
		// fail with a "malformed or non-GTS $id" diagnostic (mirrors Rust
		// entities.rs). This lives in the core library so every client (server,
		// batch validator, direct callers) enforces it uniformly.
		validPrefix := true
		if rawID, ok := content["$id"].(string); ok {
			rawID = strings.TrimSpace(rawID)
			if strings.HasPrefix(rawID, GtsPrefix) && !strings.HasPrefix(rawID, GtsURIPrefix) {
				validPrefix = false
			}
		}
		if validPrefix && entityIDValue != "" && IsValidGtsID(entityIDValue) {
			gtsID, _ := NewGtsID(entityIDValue)
			entity.GtsID = gtsID
		}
	} else if entityIDValue != "" && IsValidGtsID(entityIDValue) {
		// Well-known instance: GTS ID in id field
		gtsID, _ := NewGtsID(entityIDValue)
		entity.GtsID = gtsID
		// Type ID should be derived from the chain if not explicitly set
		if entity.TypeID == "" && entity.SelectedEntityField != "" {
			entity.TypeID = entity.calcJSONTypeID(cfg, entityIDValue)
		}
	}
	// Anonymous instance (non-schema, non-GTS id): GtsID stays nil and
	// entity.TypeID is taken from the type field — nothing more to do here.

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
	if !e.IsTypeSchema && e.SelectedEntityField != "" {
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

	// Schema Detection: only canonical $schema is recognized.
	// The doubled-dollar $$schema form is an HttpRunner escaping artifact,
	// not a JSON Schema keyword.
	_, hasSchema := content["$schema"]
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

// calcJSONTypeID extracts the type ID from JSON content
func (e *JsonEntity) calcJSONTypeID(cfg *GtsConfig, entityIDValue string) string {
	if e.IsTypeSchema {
		// For derived type-schemas, derive parent type from chain
		if entityIDValue != "" && IsValidGtsID(entityIDValue) && strings.HasSuffix(entityIDValue, "~") {
			firstTilde := strings.Index(entityIDValue, "~")
			if firstTilde > 0 {
				secondTilde := strings.Index(entityIDValue[firstTilde+1:], "~")
				if secondTilde > 0 {
					// This is a derived type-schema, derive parent from chain
					e.SelectedTypeIDField = e.SelectedEntityField
					return entityIDValue[:firstTilde+1]
				}
			}
		}

		// For base type-schemas: get type ID from $schema field.
		// Per gts-spec v0.11, type_id MUST be a GTS Type Identifier or null —
		// JSON Schema dialect URLs (and other non-GTS values) are no longer
		// accepted as type_id. Leave SelectedTypeIDField set either way so
		// callers can see we did inspect $schema.
		if schemaValue := e.getFieldValue("$schema"); schemaValue != "" {
			e.SelectedTypeIDField = "$schema"
			if strings.HasSuffix(schemaValue, "~") && IsValidGtsID(schemaValue) {
				return schemaValue
			}
		}
		return ""
	}

	// For instances: try entity ID chain first, then TypeIDFields
	if entityIDValue != "" && IsValidGtsID(entityIDValue) {
		// For instances, find last ~ and return everything up to and including it
		// But skip if entity ID ends with ~ (that would be a type, not an instance)
		if !strings.HasSuffix(entityIDValue, "~") {
			lastTilde := strings.LastIndex(entityIDValue, "~")
			if lastTilde > 0 {
				e.SelectedTypeIDField = e.SelectedEntityField
				return entityIDValue[:lastTilde+1]
			}
		}
	}

	// If no entity ID found, use TypeIDFields to find type reference
	field, value := e.firstNonEmptyField(cfg.TypeIDFields)
	if value != "" {
		e.SelectedTypeIDField = field
		return value
	}

	return ""
}

// ExtractID extracts GTS ID information from JSON content
func ExtractID(content map[string]any, cfg *GtsConfig) *ExtractIDResult {
	entity := NewJsonEntity(content, cfg)

	result := &ExtractIDResult{
		IsTypeSchema: entity.IsTypeSchema,
	}

	// Set TypeID as pointer (nil if empty)
	if entity.TypeID != "" {
		result.TypeID = &entity.TypeID
	}

	// Set SelectedEntityField as pointer (nil if empty)
	if entity.SelectedEntityField != "" {
		result.SelectedEntityField = &entity.SelectedEntityField
	}

	// Set SelectedTypeIDField as pointer (nil if empty)
	if entity.SelectedTypeIDField != "" {
		result.SelectedTypeIDField = &entity.SelectedTypeIDField
	}

	// Diagnostic: for type-schemas and well-known instances, return the parsed
	// GTS ID. For non-type-schema entities without a valid GTS ID (anonymous or
	// malformed), fall back to the raw value of the selected id field. This
	// differs from EffectiveID (registrable), which requires a resolvable type.
	switch {
	case entity.GtsID != nil:
		result.ID = entity.GtsID.ID
	case !entity.IsTypeSchema && entity.SelectedEntityField != "":
		if val, ok := content[entity.SelectedEntityField].(string); ok {
			result.ID = val
		}
	}

	return result
}
