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

// RefValidationError represents a validation error for $ref values
type RefValidationError struct {
	FieldPath string
	RefValue  string
	Reason    string
}

func (e *RefValidationError) Error() string {
	return fmt.Sprintf("$ref validation failed for field '%s': %s", e.FieldPath, e.Reason)
}

// RefValidator validates $ref constraints in GTS schemas
type RefValidator struct {
}

// NewRefValidator creates a new $ref validator
func NewRefValidator() *RefValidator {
	return &RefValidator{}
}

// ValidateSchemaRefs validates all $ref values in a schema
func (v *RefValidator) ValidateSchemaRefs(schema map[string]interface{}, schemaPath string) []*RefValidationError {
	var errors []*RefValidationError
	v.visitSchemaForRefs(schema, schemaPath, &errors)
	return errors
}

// visitSchemaForRefs recursively visits schema nodes to find and validate $ref values
func (v *RefValidator) visitSchemaForRefs(schema map[string]interface{}, path string, errors *[]*RefValidationError) {
	if schema == nil {
		return
	}

	// Check for $ref field
	if refValue, hasRef := schema["$ref"]; hasRef {
		refPath := "$ref"
		if path != "" {
			refPath = path + "/$ref"
		}
		if err := v.validateRef(refValue, refPath); err != nil {
			*errors = append(*errors, err)
		}
	}

	// Recurse into nested structures
	for key, value := range schema {
		if key == "$ref" {
			continue // Already processed above
		}

		nestedPath := key
		if path != "" {
			nestedPath = path + "/" + key
		}

		switch val := value.(type) {
		case map[string]interface{}:
			v.visitSchemaForRefs(val, nestedPath, errors)
		case []interface{}:
			for idx, item := range val {
				if itemMap, ok := item.(map[string]interface{}); ok {
					v.visitSchemaForRefs(itemMap, fmt.Sprintf("%s[%d]", nestedPath, idx), errors)
				}
			}
		}
	}
}

// validateRef validates a single $ref value according to GTS specification
func (v *RefValidator) validateRef(refValue interface{}, fieldPath string) *RefValidationError {
	refStr, ok := refValue.(string)
	if !ok {
		return &RefValidationError{
			FieldPath: fieldPath,
			RefValue:  fmt.Sprintf("%v", refValue),
			Reason:    fmt.Sprintf("$ref value must be a string, got %T", refValue),
		}
	}

	refStr = strings.TrimSpace(refStr)
	if refStr == "" {
		return &RefValidationError{
			FieldPath: fieldPath,
			RefValue:  refStr,
			Reason:    "$ref value cannot be empty",
		}
	}

	// $ref must be a same-document JSON Pointer ("#...") or a GTS URI
	// ("gts://<valid gts id>"). Every other form is rejected with the same
	// guidance message.
	invalid := &RefValidationError{
		FieldPath: fieldPath,
		RefValue:  refStr,
		Reason:    "must be a local ref (starting with '#') or a GTS URI (starting with 'gts://')",
	}

	switch ClassifyRef(refStr) {
	case RefLocalPointer:
		return nil // Valid local reference
	case RefGtsURI:
		// Strip prefix and validate the GTS ID
		gtsID := gtsid.NormalizeID(refStr)
		if !gtsid.IsValid(gtsID) {
			return &RefValidationError{
				FieldPath: fieldPath,
				RefValue:  refStr,
				Reason:    fmt.Sprintf("contains invalid GTS identifier '%s'", gtsID),
			}
		}
		return nil // Valid GTS URI reference
	default:
		// Bare GTS IDs, HTTP(S) URIs and anything else are invalid $ref forms.
		return invalid
	}
}
