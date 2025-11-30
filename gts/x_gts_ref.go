/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"strings"
)

// XGtsRefValidationError represents a validation error for x-gts-ref constraints
type XGtsRefValidationError struct {
	FieldPath  string
	Value      interface{}
	RefPattern string
	Reason     string
}

func (e *XGtsRefValidationError) Error() string {
	return fmt.Sprintf("x-gts-ref validation failed for field '%s': %s", e.FieldPath, e.Reason)
}

// XGtsRefValidator validates x-gts-ref constraints in GTS schemas
type XGtsRefValidator struct {
	store *GtsStore
}

// NewXGtsRefValidator creates a new x-gts-ref validator
func NewXGtsRefValidator(store *GtsStore) *XGtsRefValidator {
	return &XGtsRefValidator{
		store: store,
	}
}

// ValidateInstance validates an instance against x-gts-ref constraints in schema
func (v *XGtsRefValidator) ValidateInstance(instance map[string]interface{}, schema map[string]interface{}, instancePath string) []*XGtsRefValidationError {
	var errors []*XGtsRefValidationError
	v.visitInstance(instance, schema, instancePath, schema, &errors)
	return errors
}

// ValidateSchema validates x-gts-ref fields in a schema definition
func (v *XGtsRefValidator) ValidateSchema(schema map[string]interface{}, schemaPath string, rootSchema map[string]interface{}) []*XGtsRefValidationError {
	if rootSchema == nil {
		rootSchema = schema
	}
	
	var errors []*XGtsRefValidationError
	v.visitSchema(schema, schemaPath, rootSchema, &errors)
	return errors
}

// visitInstance recursively visits instance nodes and validates x-gts-ref constraints
func (v *XGtsRefValidator) visitInstance(instance interface{}, schema map[string]interface{}, path string, rootSchema map[string]interface{}, errors *[]*XGtsRefValidationError) {
	if schema == nil {
		return
	}

	// Check for x-gts-ref constraint
	if xGtsRef, hasRef := schema["x-gts-ref"]; hasRef {
		if strInstance, ok := instance.(string); ok {
			if err := v.validateRefValue(strInstance, xGtsRef, path, rootSchema); err != nil {
				*errors = append(*errors, err)
			}
		}
	}

	// Recurse into object properties
	if schemaType, ok := schema["type"].(string); ok && schemaType == "object" {
		if properties, hasProps := schema["properties"].(map[string]interface{}); hasProps {
			if instanceMap, ok := instance.(map[string]interface{}); ok {
				for propName, propSchema := range properties {
					if propValue, hasProp := instanceMap[propName]; hasProp {
						propPath := propName
						if path != "" {
							propPath = path + "." + propName
						}
						if propSchemaMap, ok := propSchema.(map[string]interface{}); ok {
							v.visitInstance(propValue, propSchemaMap, propPath, rootSchema, errors)
						}
					}
				}
			}
		}
	}

	// Recurse into array items
	if schemaType, ok := schema["type"].(string); ok && schemaType == "array" {
		if items, hasItems := schema["items"].(map[string]interface{}); hasItems {
			if instanceArray, ok := instance.([]interface{}); ok {
				for idx, item := range instanceArray {
					itemPath := fmt.Sprintf("%s[%d]", path, idx)
					v.visitInstance(item, items, itemPath, rootSchema, errors)
				}
			}
		}
	}
}

// visitSchema recursively visits schema nodes
func (v *XGtsRefValidator) visitSchema(schema map[string]interface{}, path string, rootSchema map[string]interface{}, errors *[]*XGtsRefValidationError) {
	if schema == nil {
		return
	}

	// Check for x-gts-ref field
	if xGtsRef, hasRef := schema["x-gts-ref"]; hasRef {
		refPath := "x-gts-ref"
		if path != "" {
			refPath = path + "/x-gts-ref"
		}
		if err := v.validateRefPattern(xGtsRef, refPath, rootSchema); err != nil {
			*errors = append(*errors, err)
		}
	}

	// Recurse into nested structures
	for key, value := range schema {
		if key == "x-gts-ref" {
			continue
		}
		
		nestedPath := key
		if path != "" {
			nestedPath = path + "/" + key
		}
		
		switch val := value.(type) {
		case map[string]interface{}:
			v.visitSchema(val, nestedPath, rootSchema, errors)
		case []interface{}:
			for idx, item := range val {
				if itemMap, ok := item.(map[string]interface{}); ok {
					v.visitSchema(itemMap, fmt.Sprintf("%s[%d]", nestedPath, idx), rootSchema, errors)
				}
			}
		}
	}
}

// validateRefValue validates an instance value against its x-gts-ref constraint
func (v *XGtsRefValidator) validateRefValue(value string, refPattern interface{}, fieldPath string, schema map[string]interface{}) *XGtsRefValidationError {
	refPatternStr, ok := refPattern.(string)
	if !ok {
		return &XGtsRefValidationError{
			FieldPath:  fieldPath,
			Value:      value,
			RefPattern: fmt.Sprintf("%v", refPattern),
			Reason:     fmt.Sprintf("Value must be a string, got %T", refPattern),
		}
	}

	// Resolve pattern if it's a relative reference
	if strings.HasPrefix(refPatternStr, "/") {
		resolved := v.resolvePointer(schema, refPatternStr)
		if resolved == "" {
			return &XGtsRefValidationError{
				FieldPath:  fieldPath,
				Value:      value,
				RefPattern: refPatternStr,
				Reason:     fmt.Sprintf("Cannot resolve reference path '%s'", refPatternStr),
			}
		}
		// Check if the resolved value is a pointer that needs further resolution
		if strings.HasPrefix(resolved, "/") {
			// Recursive resolution
			furtherResolved := v.resolvePointer(schema, resolved)
			if furtherResolved == "" {
				return &XGtsRefValidationError{
					FieldPath:  fieldPath,
					Value:      value,
					RefPattern: refPatternStr,
					Reason:     fmt.Sprintf("Cannot resolve nested reference '%s' -> '%s'", refPatternStr, resolved),
				}
			}
			resolved = furtherResolved
		}
		
		if !strings.HasPrefix(resolved, "gts.") {
			return &XGtsRefValidationError{
				FieldPath:  fieldPath,
				Value:      value,
				RefPattern: refPatternStr,
				Reason:     fmt.Sprintf("Resolved reference '%s' -> '%s' is not a GTS pattern", refPatternStr, resolved),
			}
		}
		refPatternStr = resolved
	}

	// Validate against GTS pattern
	return v.validateGtsPattern(value, refPatternStr, fieldPath)
}

// validateRefPattern validates an x-gts-ref pattern in a schema definition
func (v *XGtsRefValidator) validateRefPattern(refPattern interface{}, fieldPath string, rootSchema map[string]interface{}) *XGtsRefValidationError {
	refPatternStr, ok := refPattern.(string)
	if !ok {
		return &XGtsRefValidationError{
			FieldPath:  fieldPath,
			Value:      refPattern,
			RefPattern: "",
			Reason:     fmt.Sprintf("x-gts-ref value must be a string, got %T", refPattern),
		}
	}

	// Case 1: Absolute GTS pattern
	if strings.HasPrefix(refPatternStr, "gts.") {
		return v.validateGtsIDOrPattern(refPatternStr, fieldPath)
	}

	// Case 2: Relative reference
	if strings.HasPrefix(refPatternStr, "/") {
		resolved := v.resolvePointer(rootSchema, refPatternStr)
		if resolved == "" {
			return &XGtsRefValidationError{
				FieldPath:  fieldPath,
				Value:      refPattern,
				RefPattern: refPatternStr,
				Reason:     fmt.Sprintf("Cannot resolve reference path '%s'", refPatternStr),
			}
		}
		if !IsValidGtsID(resolved) {
			return &XGtsRefValidationError{
				FieldPath:  fieldPath,
				Value:      refPattern,
				RefPattern: refPatternStr,
				Reason:     fmt.Sprintf("Resolved reference '%s' -> '%s' is not a valid GTS identifier", refPatternStr, resolved),
			}
		}
		return nil
	}

	return &XGtsRefValidationError{
		FieldPath:  fieldPath,
		Value:      refPattern,
		RefPattern: refPatternStr,
		Reason:     fmt.Sprintf("Invalid x-gts-ref value: '%s' must start with 'gts.' or '/'", refPatternStr),
	}
}

// validateGtsIDOrPattern validates a GTS ID or pattern in schema definition
func (v *XGtsRefValidator) validateGtsIDOrPattern(pattern, fieldPath string) *XGtsRefValidationError {
	if pattern == "gts.*" {
		return nil // Valid wildcard
	}

	if strings.Contains(pattern, "*") {
		// Wildcard pattern - validate prefix
		prefix := strings.TrimSuffix(pattern, "*")
		if !strings.HasPrefix(prefix, "gts.") {
			return &XGtsRefValidationError{
				FieldPath:  fieldPath,
				Value:      pattern,
				RefPattern: pattern,
				Reason:     fmt.Sprintf("Invalid GTS wildcard pattern: %s", pattern),
			}
		}
		return nil
	}

	// Specific GTS ID
	if !IsValidGtsID(pattern) {
		return &XGtsRefValidationError{
			FieldPath:  fieldPath,
			Value:      pattern,
			RefPattern: pattern,
			Reason:     fmt.Sprintf("Invalid GTS identifier: %s", pattern),
		}
	}
	return nil
}

// validateGtsPattern validates value matches a GTS pattern
func (v *XGtsRefValidator) validateGtsPattern(value, pattern, fieldPath string) *XGtsRefValidationError {
	// Validate it's a valid GTS ID
	if !IsValidGtsID(value) {
		return &XGtsRefValidationError{
			FieldPath:  fieldPath,
			Value:      value,
			RefPattern: pattern,
			Reason:     fmt.Sprintf("Value '%s' is not a valid GTS identifier", value),
		}
	}

	// Check pattern match
	if pattern == "gts.*" {
		// Any valid GTS ID matches
	} else if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		if !strings.HasPrefix(value, prefix) {
			return &XGtsRefValidationError{
				FieldPath:  fieldPath,
				Value:      value,
				RefPattern: pattern,
				Reason:     fmt.Sprintf("Value '%s' does not match pattern '%s'", value, pattern),
			}
		}
	} else if !strings.HasPrefix(value, pattern) {
		return &XGtsRefValidationError{
			FieldPath:  fieldPath,
			Value:      value,
			RefPattern: pattern,
			Reason:     fmt.Sprintf("Value '%s' does not match pattern '%s'", value, pattern),
		}
	}

	// Optionally check if entity exists in store
	if v.store != nil {
		entity := v.store.Get(value)
		if entity == nil {
			return &XGtsRefValidationError{
				FieldPath:  fieldPath,
				Value:      value,
				RefPattern: pattern,
				Reason:     fmt.Sprintf("Referenced entity '%s' not found in registry", value),
			}
		}
	}

	return nil
}

// resolvePointer resolves a JSON Pointer in the schema
func (v *XGtsRefValidator) resolvePointer(schema map[string]interface{}, pointer string) string {
	path := strings.TrimPrefix(pointer, "/")
	if path == "" {
		return ""
	}

	parts := strings.Split(path, "/")
	var current interface{} = schema

	for _, part := range parts {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = currentMap[part]
		if current == nil {
			return ""
		}
	}

	// If current is a string, return it
	if str, ok := current.(string); ok {
		return str
	}

	// If current is a dict with x-gts-ref, resolve it
	if currentMap, ok := current.(map[string]interface{}); ok {
		if xGtsRef, hasRef := currentMap["x-gts-ref"]; hasRef {
			if refStr, ok := xGtsRef.(string); ok {
				if strings.HasPrefix(refStr, "/") {
					return v.resolvePointer(schema, refStr)
				}
				return refStr
			}
		}
	}

	return ""
}