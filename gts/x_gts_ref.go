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
	store         *GtsStore
	referencedIDs map[string]struct{}
	// enforceExistence, when true and a store is provided, requires an
	// x-gts-ref value to resolve to a registered entity or validation fails.
	// Existence is enforced uniformly for all constraint forms, including the
	// bare "gts.*" wildcard (gts-spec §9.6). When false, only well-formedness
	// and pattern matching are checked. It defaults to true.
	enforceExistence bool
}

// NewXGtsRefValidator creates a new x-gts-ref validator with reference-existence
// enforcement enabled (the reference-implementation default, gts-spec §9.6).
func NewXGtsRefValidator(store *GtsStore) *XGtsRefValidator {
	return &XGtsRefValidator{
		store:            store,
		referencedIDs:    make(map[string]struct{}),
		enforceExistence: true,
	}
}

func (v *XGtsRefValidator) ReferencedIDs() []string {
	ids := make([]string, 0, len(v.referencedIDs))
	for id := range v.referencedIDs {
		ids = append(ids, id)
	}
	return ids
}

// ValidateInstance validates an instance against x-gts-ref constraints in schema
func (v *XGtsRefValidator) ValidateInstance(instance map[string]interface{}, schema map[string]interface{}, instancePath string) []*XGtsRefValidationError {
	if err := validateJSONContent(instance); err != nil {
		return invalidJSONRefErrors(instancePath, err)
	}
	if err := validateJSONContent(schema); err != nil {
		return invalidJSONRefErrors(instancePath, err)
	}
	var errors []*XGtsRefValidationError
	v.visitInstance(instance, schema, instancePath, schema, &errors)
	return errors
}

// ValidateSchema validates x-gts-ref fields in a schema definition
func (v *XGtsRefValidator) ValidateSchema(schema map[string]interface{}, schemaPath string, rootSchema map[string]interface{}) []*XGtsRefValidationError {
	if err := validateJSONContent(schema); err != nil {
		return invalidJSONRefErrors(schemaPath, err)
	}
	if rootSchema == nil {
		rootSchema = schema
	} else if err := validateJSONContent(rootSchema); err != nil {
		return invalidJSONRefErrors(schemaPath, err)
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
		if instanceArray, ok := instance.([]interface{}); ok {
			switch items := schema["items"].(type) {
			case map[string]interface{}:
				for idx, item := range instanceArray {
					itemPath := fmt.Sprintf("%s[%d]", path, idx)
					v.visitInstance(item, items, itemPath, rootSchema, errors)
				}
			case []interface{}:
				for idx, itemSchema := range items {
					if idx >= len(instanceArray) {
						break
					}
					if schemaMap, ok := itemSchema.(map[string]interface{}); ok {
						itemPath := fmt.Sprintf("%s[%d]", path, idx)
						v.visitInstance(instanceArray[idx], schemaMap, itemPath, rootSchema, errors)
					}
				}
				if additionalItems, ok := schema["additionalItems"].(map[string]interface{}); ok {
					for idx := len(items); idx < len(instanceArray); idx++ {
						itemPath := fmt.Sprintf("%s[%d]", path, idx)
						v.visitInstance(instanceArray[idx], additionalItems, itemPath, rootSchema, errors)
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
		case "not", "if", "then", "else", "contains", "additionalProperties", "additionalItems", "propertyNames", "unevaluatedProperties", "unevaluatedItems":
			if childSchema, ok := value.(map[string]interface{}); ok {
				visit(childSchema, nestedPath)
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

	visitSchemaChildren(schema, path, func(child map[string]interface{}, childPath string) {
		v.visitSchema(child, childPath, rootSchema, errors)
	})
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
	if strings.HasPrefix(refPatternStr, PointerPrefix) {
		resolved := v.resolveRefPointer(schema, refPatternStr)
		if resolved == "" {
			return &XGtsRefValidationError{
				FieldPath:  fieldPath,
				Value:      value,
				RefPattern: refPatternStr,
				Reason:     fmt.Sprintf("Cannot resolve reference path '%s'", refPatternStr),
			}
		}
		// Check if the resolved value is a pointer that needs further resolution
		if strings.HasPrefix(resolved, PointerPrefix) {
			// Recursive resolution
			furtherResolved := v.resolveRefPointer(schema, resolved)
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

		if !gtsid.HasPrefix(resolved) {
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
	if gtsid.HasPrefix(refPatternStr) {
		return v.validateGtsIDOrPattern(refPatternStr, fieldPath)
	}

	// Case 2: Relative reference
	if strings.HasPrefix(refPatternStr, PointerPrefix) {
		resolved := v.resolveRefPointer(rootSchema, refPatternStr)
		if resolved == "" {
			return &XGtsRefValidationError{
				FieldPath:  fieldPath,
				Value:      refPattern,
				RefPattern: refPatternStr,
				Reason:     fmt.Sprintf("Cannot resolve reference path '%s'", refPatternStr),
			}
		}
		if !gtsid.IsValid(resolved) {
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

// ValidateSchemaRefExistence walks a schema and verifies that every concrete
// (non-wildcard, non-relative) x-gts-ref names a constraint type that is
// registered. This enforces the reference-implementation rule that an x-gts-ref
// must point at an existing constraint type even when no value is supplied:
// gts-spec §9.6 leaves reference-existence checking to the implementation, and
// the reference implementation treats a dangling x-gts-ref target like a
// dangling $ref. Wildcard patterns (which name a family, not a single type) and
// relative pointer references (validated elsewhere) are skipped. Existence is
// only checked when a store is available and enforcement is enabled.
func (v *XGtsRefValidator) ValidateSchemaRefExistence(schema map[string]interface{}, schemaPath string) []*XGtsRefValidationError {
	if err := validateJSONContent(schema); err != nil {
		return invalidJSONRefErrors(schemaPath, err)
	}
	var errors []*XGtsRefValidationError
	if v.store == nil || !v.enforceExistence {
		return errors
	}
	v.visitSchemaRefExistence(schema, schemaPath, schema, &errors)
	return errors
}

// visitSchemaRefExistence recursively checks concrete x-gts-ref targets exist.
func (v *XGtsRefValidator) visitSchemaRefExistence(schema map[string]interface{}, path string, rootSchema map[string]interface{}, errors *[]*XGtsRefValidationError) {
	if schema == nil {
		return
	}

	if xGtsRef, hasRef := schema["x-gts-ref"]; hasRef {
		if refStr, ok := xGtsRef.(string); ok {
			targetID := refStr
			if strings.HasPrefix(refStr, PointerPrefix) {
				targetID = v.resolveRefPointer(rootSchema, refStr)
			}
			if gtsid.HasPrefix(targetID) && !gtsid.HasWildcard(targetID) {
				refPath := "x-gts-ref"
				if path != "" {
					refPath = path + "/x-gts-ref"
				}
				entity := v.store.Get(targetID)
				if entity == nil || !entity.IsTypeSchema {
					*errors = append(*errors, &XGtsRefValidationError{
						FieldPath:  refPath,
						Value:      refStr,
						RefPattern: refStr,
						Reason:     fmt.Sprintf("x-gts-ref constraint type '%s' is not registered as a type schema", targetID),
					})
				} else {
					v.referencedIDs[targetID] = struct{}{}
				}
			}
		}
	}

	visitSchemaChildren(schema, path, func(child map[string]interface{}, childPath string) {
		v.visitSchemaRefExistence(child, childPath, rootSchema, errors)
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
	if v.store != nil && v.enforceExistence {
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

// resolvePointer resolves a JSON Pointer in the schema
// Note: For /$id references, the gts:// prefix is stripped from the value
// as per GTS specification (relative self-reference should match the $id without the prefix).
func (v *XGtsRefValidator) resolveRefPointer(schema map[string]interface{}, pointer string) string {
	resolved := v.resolvePointer(schema, pointer)
	if resolved == "" && strings.HasPrefix(pointer, "/"+KeyXGtsTraitsSchema+"/") {
		resolved = v.resolvePointer(schema, strings.TrimPrefix(pointer, "/"+KeyXGtsTraitsSchema))
	}
	return resolved
}

func (v *XGtsRefValidator) resolvePointer(schema map[string]interface{}, pointer string) string {
	path := strings.TrimPrefix(pointer, PointerPrefix)
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

	// If current is a string, return it (stripping gts:// prefix if present).
	// For /$id relative references the schema's $id may be a full GTS URI
	// (e.g. "gts://gts.x.example._.user.v1~") but the instance value matches
	// without the prefix, so normalize here.
	if str, ok := current.(string); ok {
		return gtsid.NormalizeID(str)
	}

	// If current is a dict with x-gts-ref, resolve it
	if currentMap, ok := current.(map[string]interface{}); ok {
		if xGtsRef, hasRef := currentMap["x-gts-ref"]; hasRef {
			if refStr, ok := xGtsRef.(string); ok {
				if strings.HasPrefix(refStr, PointerPrefix) {
					return v.resolveRefPointer(schema, refStr)
				}
				return refStr
			}
		}
	}

	return ""
}
