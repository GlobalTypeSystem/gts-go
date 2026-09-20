/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"strings"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
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

func invalidJSONRefErrors(path string, err error) []*XGtsRefValidationError {
	return []*XGtsRefValidationError{{
		FieldPath: path,
		Reason:    fmt.Sprintf("input must be valid JSON: %v", err),
	}}
}

// XGtsRefValidator validates x-gts-ref constraints in GTS schemas
type XGtsRefValidator struct {
	store                      *GtsStore
	mode                       GtsRefValidationMode
	referencedIDs              map[string]struct{}
	referencedWildcardPatterns map[string]struct{}
}

func NewXGtsRefValidator(store *GtsStore, modes ...GtsRefValidationMode) *XGtsRefValidator {
	mode := GtsRefValidationAnyValid
	if len(modes) > 0 {
		mode = modes[0]
	}
	return &XGtsRefValidator{
		store:                      store,
		mode:                       mode,
		referencedIDs:              make(map[string]struct{}),
		referencedWildcardPatterns: make(map[string]struct{}),
	}
}

func (v *XGtsRefValidator) ReferencedIDs() []string {
	ids := make([]string, 0, len(v.referencedIDs))
	for id := range v.referencedIDs {
		ids = append(ids, id)
	}
	return ids
}

func (v *XGtsRefValidator) ReferencedWildcardPatterns() []string {
	patterns := make([]string, 0, len(v.referencedWildcardPatterns))
	for pattern := range v.referencedWildcardPatterns {
		patterns = append(patterns, pattern)
	}
	return patterns
}

// ValidateInstance validates an instance against x-gts-ref constraints in schema
func (v *XGtsRefValidator) ValidateInstance(instance map[string]interface{}, schema map[string]interface{}, instancePath string, selectedTypeIDs ...string) []*XGtsRefValidationError {
	if err := validateJSONContent(instance); err != nil {
		return invalidJSONRefErrors(instancePath, err)
	}
	if err := validateJSONContent(schema); err != nil {
		return invalidJSONRefErrors(instancePath, err)
	}
	selectedTypeID := ""
	if len(selectedTypeIDs) > 0 {
		selectedTypeID = gtsid.NormalizeID(selectedTypeIDs[0])
	} else if schemaID, ok := schema["$id"].(string); ok {
		selectedTypeID = gtsid.NormalizeID(schemaID)
	}
	var errors []*XGtsRefValidationError
	v.visitInstance(instance, schema, instancePath, selectedTypeID, &errors)
	return errors
}

// ValidateSchema validates x-gts-ref fields in a schema definition
func (v *XGtsRefValidator) ValidateSchema(schema map[string]interface{}, schemaPath string) []*XGtsRefValidationError {
	if err := validateJSONContent(schema); err != nil {
		return invalidJSONRefErrors(schemaPath, err)
	}

	var errors []*XGtsRefValidationError
	v.visitSchema(schema, schemaPath, &errors)
	return errors
}

// visitInstance recursively visits instance nodes and validates x-gts-ref constraints
func (v *XGtsRefValidator) visitInstance(instance interface{}, schema map[string]interface{}, path, selectedTypeID string, errors *[]*XGtsRefValidationError) {
	if schema == nil {
		return
	}

	// Check for x-gts-ref constraint
	if xGtsRef, hasRef := schema["x-gts-ref"]; hasRef {
		if strInstance, ok := instance.(string); ok {
			if err := v.validateRefValue(strInstance, xGtsRef, path, selectedTypeID); err != nil {
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
							v.visitInstance(propValue, propSchemaMap, propPath, selectedTypeID, errors)
						}
					}
				}
			}
		}
	}

	// Recurse into array items
	if schemaType, ok := schema["type"].(string); ok && schemaType == "array" {
		if instanceArray, ok := instance.([]interface{}); ok {
			if prefixItems, ok := schema["prefixItems"].([]interface{}); ok {
				for idx, itemSchema := range prefixItems {
					if idx >= len(instanceArray) {
						break
					}
					if schemaMap, ok := itemSchema.(map[string]interface{}); ok {
						itemPath := fmt.Sprintf("%s[%d]", path, idx)
						v.visitInstance(instanceArray[idx], schemaMap, itemPath, selectedTypeID, errors)
					}
				}
				if items, ok := schema["items"].(map[string]interface{}); ok {
					for idx := len(prefixItems); idx < len(instanceArray); idx++ {
						itemPath := fmt.Sprintf("%s[%d]", path, idx)
						v.visitInstance(instanceArray[idx], items, itemPath, selectedTypeID, errors)
					}
				}
				return
			}
			switch items := schema["items"].(type) {
			case map[string]interface{}:
				for idx, item := range instanceArray {
					itemPath := fmt.Sprintf("%s[%d]", path, idx)
					v.visitInstance(item, items, itemPath, selectedTypeID, errors)
				}
			case []interface{}:
				for idx, itemSchema := range items {
					if idx >= len(instanceArray) {
						break
					}
					if schemaMap, ok := itemSchema.(map[string]interface{}); ok {
						itemPath := fmt.Sprintf("%s[%d]", path, idx)
						v.visitInstance(instanceArray[idx], schemaMap, itemPath, selectedTypeID, errors)
					}
				}
				if additionalItems, ok := schema["additionalItems"].(map[string]interface{}); ok {
					for idx := len(items); idx < len(instanceArray); idx++ {
						itemPath := fmt.Sprintf("%s[%d]", path, idx)
						v.visitInstance(instanceArray[idx], additionalItems, itemPath, selectedTypeID, errors)
					}
				}
			}
		}
	}
}

func visitSchemaChildren(schema map[string]interface{}, path string, visit func(map[string]interface{}, string)) {
	for key, value := range schema {
		nestedPath := key
		if path != "" {
			nestedPath = path + "/" + key
		}
		switch key {
		case "properties", "patternProperties", "definitions", "$defs", "dependentSchemas", "dependencies":
			if named, ok := value.(map[string]interface{}); ok {
				for name, child := range named {
					if childSchema, ok := child.(map[string]interface{}); ok {
						visit(childSchema, nestedPath+"/"+name)
					}
				}
			}
		case "allOf", "anyOf", "oneOf", "prefixItems":
			if children, ok := value.([]interface{}); ok {
				for index, child := range children {
					if childSchema, ok := child.(map[string]interface{}); ok {
						visit(childSchema, fmt.Sprintf("%s[%d]", nestedPath, index))
					}
				}
			}
		case "items":
			switch children := value.(type) {
			case map[string]interface{}:
				visit(children, nestedPath)
			case []interface{}:
				for index, child := range children {
					if childSchema, ok := child.(map[string]interface{}); ok {
						visit(childSchema, fmt.Sprintf("%s[%d]", nestedPath, index))
					}
				}
			}
		case "not", "if", "then", "else", "contains", "additionalProperties", "additionalItems", "propertyNames", "unevaluatedProperties", "unevaluatedItems", KeyXGtsTraitsSchema:
			if childSchema, ok := value.(map[string]interface{}); ok {
				visit(childSchema, nestedPath)
			}
		}
	}
}

// visitSchema recursively visits schema nodes
func (v *XGtsRefValidator) visitSchema(schema map[string]interface{}, path string, errors *[]*XGtsRefValidationError) {
	if schema == nil {
		return
	}

	// Check for x-gts-ref field
	if xGtsRef, hasRef := schema["x-gts-ref"]; hasRef {
		refPath := "x-gts-ref"
		if path != "" {
			refPath = path + "/x-gts-ref"
		}
		if err := v.validateRefPattern(xGtsRef, refPath); err != nil {
			*errors = append(*errors, err)
		}
	}

	visitSchemaChildren(schema, path, func(child map[string]interface{}, childPath string) {
		v.visitSchema(child, childPath, errors)
	})
}

// validateRefValue validates an instance value against its x-gts-ref constraint
func (v *XGtsRefValidator) validateRefValue(value string, refPattern interface{}, fieldPath, selectedTypeID string) *XGtsRefValidationError {
	refPatternStr, ok := refPattern.(string)
	if !ok {
		return &XGtsRefValidationError{
			FieldPath:  fieldPath,
			Value:      value,
			RefPattern: fmt.Sprintf("%v", refPattern),
			Reason:     fmt.Sprintf("Value must be a string, got %T", refPattern),
		}
	}

	if IsXGtsRefSelf(refPatternStr) {
		if selectedTypeID == "" {
			return &XGtsRefValidationError{
				FieldPath: fieldPath, Value: value, RefPattern: refPatternStr,
				Reason: "Cannot resolve /$id without a selected GTS Type Schema",
			}
		}
		refPatternStr = selectedTypeID
	}

	// Validate against GTS pattern
	return v.validateGtsPattern(value, refPatternStr, fieldPath)
}

// validateRefPattern validates an x-gts-ref pattern in a schema definition
func (v *XGtsRefValidator) validateRefPattern(refPattern interface{}, fieldPath string) *XGtsRefValidationError {
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
	if gtsid.HasPrefix(refPatternStr) {
		return v.validateGtsIDOrPattern(refPatternStr, fieldPath)
	}

	if IsXGtsRefSelf(refPatternStr) {
		return nil
	}

	return &XGtsRefValidationError{
		FieldPath:  fieldPath,
		Value:      refPattern,
		RefPattern: refPatternStr,
		Reason:     fmt.Sprintf("Invalid x-gts-ref value: '%s' must be a GTS identifier, wildcard, or '%s'", refPatternStr, XGtsRefSelf),
	}
}

// validateGtsIDOrPattern validates a GTS ID or pattern in schema definition
func (v *XGtsRefValidator) validateGtsIDOrPattern(pattern, fieldPath string) *XGtsRefValidationError {
	if pattern == gtsid.Prefix+gtsid.WildcardMarker {
		return nil // Valid wildcard
	}

	if gtsid.HasWildcard(pattern) {
		// Wildcard pattern - validate prefix
		prefix := strings.TrimSuffix(pattern, gtsid.WildcardMarker)
		if !gtsid.HasPrefix(prefix) {
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
	if !gtsid.IsValid(pattern) {
		return &XGtsRefValidationError{
			FieldPath:  fieldPath,
			Value:      pattern,
			RefPattern: pattern,
			Reason:     fmt.Sprintf("Invalid GTS identifier: %s", pattern),
		}
	}
	return nil
}

// ValidateSchemaRefExistence checks concrete, wildcard, and /$id constraints
// against the registry when the selected mode requires it.
func (v *XGtsRefValidator) ValidateSchemaRefExistence(schema map[string]interface{}, schemaPath string, selectedTypeIDs ...string) []*XGtsRefValidationError {
	if err := validateJSONContent(schema); err != nil {
		return invalidJSONRefErrors(schemaPath, err)
	}
	var errors []*XGtsRefValidationError
	if v.store == nil || v.mode == GtsRefValidationNone {
		return errors
	}
	selectedTypeID := ""
	if len(selectedTypeIDs) > 0 {
		selectedTypeID = gtsid.NormalizeID(selectedTypeIDs[0])
	} else if schemaID, ok := schema["$id"].(string); ok {
		selectedTypeID = gtsid.NormalizeID(schemaID)
	}
	v.visitSchemaRefExistence(schema, schemaPath, selectedTypeID, &errors)
	return errors
}

// visitSchemaRefExistence recursively checks concrete x-gts-ref targets exist.
func (v *XGtsRefValidator) visitSchemaRefExistence(schema map[string]interface{}, path, selectedTypeID string, errors *[]*XGtsRefValidationError) {
	if schema == nil {
		return
	}

	if xGtsRef, hasRef := schema["x-gts-ref"]; hasRef {
		if refStr, ok := xGtsRef.(string); ok {
			targetID := refStr
			if IsXGtsRefSelf(refStr) {
				targetID = selectedTypeID
			}
			if gtsid.HasPrefix(targetID) {
				refPath := "x-gts-ref"
				if path != "" {
					refPath = path + "/x-gts-ref"
				}
				if gtsid.HasWildcard(targetID) {
					matched := false
					for entityID := range v.store.Items() {
						if gtsid.Match(entityID, targetID).Match {
							matched = true
							break
						}
					}
					if !matched {
						*errors = append(*errors, &XGtsRefValidationError{
							FieldPath: refPath, Value: refStr, RefPattern: targetID,
							Reason: fmt.Sprintf("x-gts-ref wildcard constraint '%s' has no registered match", targetID),
						})
					} else {
						v.referencedWildcardPatterns[targetID] = struct{}{}
					}
				} else if v.store.Get(targetID) == nil {
					*errors = append(*errors, &XGtsRefValidationError{
						FieldPath: refPath, Value: refStr, RefPattern: targetID,
						Reason: fmt.Sprintf("x-gts-ref constraint '%s' is not registered", targetID),
					})
				} else {
					v.referencedIDs[targetID] = struct{}{}
				}
			}
		}
	}

	visitSchemaChildren(schema, path, func(child map[string]interface{}, childPath string) {
		v.visitSchemaRefExistence(child, childPath, selectedTypeID, errors)
	})
}

// validateGtsPattern validates value matches a GTS pattern
func (v *XGtsRefValidator) validateGtsPattern(value, pattern, fieldPath string) *XGtsRefValidationError {
	// Validate it's a valid GTS ID
	if !gtsid.IsValid(value) {
		return &XGtsRefValidationError{
			FieldPath:  fieldPath,
			Value:      value,
			RefPattern: pattern,
			Reason:     fmt.Sprintf("Value '%s' is not a valid GTS identifier", value),
		}
	}

	// Check pattern match
	if pattern == gtsid.Prefix+gtsid.WildcardMarker {
		// Any valid GTS ID matches
	} else if strings.HasSuffix(pattern, gtsid.WildcardMarker) {
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

	// The referenced value must resolve to a registered entity when a store is
	// available and existence enforcement is enabled. Existence is enforced
	// uniformly for all constraint forms, including the bare "gts.*" wildcard
	// (gts-spec §9.6).
	if v.store != nil && v.mode != GtsRefValidationNone {
		if v.store.Get(value) == nil {
			return &XGtsRefValidationError{
				FieldPath:  fieldPath,
				Value:      value,
				RefPattern: pattern,
				Reason:     fmt.Sprintf("Referenced entity '%s' not found in registry", value),
			}
		}
		v.referencedIDs[value] = struct{}{}
	}

	return nil
}
