/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import "fmt"

const (
	KeyXGtsFinal    = "x-gts-final"
	KeyXGtsAbstract = "x-gts-abstract"
)

// ValidateSchemaModifiers checks that x-gts-final and x-gts-abstract are well-formed:
// boolean type, not both true, and not placed inside allOf entries at any depth.
func ValidateSchemaModifiers(content map[string]any) error {
	final, err := readBoolModifier(content, KeyXGtsFinal)
	if err != nil {
		return err
	}
	abstract, err := readBoolModifier(content, KeyXGtsAbstract)
	if err != nil {
		return err
	}
	if final && abstract {
		return fmt.Errorf("schema cannot declare both %s and %s as true", KeyXGtsFinal, KeyXGtsAbstract)
	}
	return validateModifierPlacement(content)
}

func readBoolModifier(content map[string]any, key string) (bool, error) {
	val, ok := content[key]
	if !ok {
		return false, nil
	}
	b, isBool := val.(bool)
	if !isBool {
		return false, fmt.Errorf("%s must be a boolean, got %T", key, val)
	}
	return b, nil
}

func validateModifierPlacement(content map[string]any) error {
	allOf, ok := content["allOf"].([]any)
	if !ok {
		return nil
	}
	for _, item := range allOf {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if _, has := entry[KeyXGtsFinal]; has {
			return fmt.Errorf("%s must be at the schema top level, not inside allOf", KeyXGtsFinal)
		}
		if _, has := entry[KeyXGtsAbstract]; has {
			return fmt.Errorf("%s must be at the schema top level, not inside allOf", KeyXGtsAbstract)
		}
		if err := validateModifierPlacement(entry); err != nil {
			return err
		}
	}
	return nil
}

// schemaOnlyInstanceKeywords lists the x-gts-* keywords that are valid only on
// type-schema documents and MUST be rejected when found inside instance
// documents (gts-spec §9.7.1, §9.11.1).
var schemaOnlyInstanceKeywords = []string{
	KeyXGtsFinal,
	KeyXGtsAbstract,
	KeyXGtsTraitsSchema,
	KeyXGtsTraits,
}

// ValidateInstanceModifiers checks that schema-only keywords (x-gts-final,
// x-gts-abstract, x-gts-traits-schema, x-gts-traits) do not appear anywhere
// in instance content. Per gts-spec §9.7.1 / §9.11.1 these annotations are
// only valid on JSON Schema (type-schema) documents and implementations MUST
// reject instances that contain them.
//
// The check is recursive over both objects and arrays so a stray keyword
// nested under any property is also flagged.
func ValidateInstanceModifiers(content map[string]any) error {
	for _, key := range schemaOnlyInstanceKeywords {
		if containsKeyRecursive(content, key) {
			return fmt.Errorf("%s is a schema-only keyword and must not appear in instances", key)
		}
	}
	return nil
}

// containsKeyRecursive reports whether the given key appears as a key in the
// content or anywhere in its nested objects/arrays.
func containsKeyRecursive(content map[string]any, key string) bool {
	if content == nil {
		return false
	}
	if _, ok := content[key]; ok {
		return true
	}
	for _, v := range content {
		if containsKeyInValue(v, key) {
			return true
		}
	}
	return false
}

func containsKeyInValue(v any, key string) bool {
	switch vv := v.(type) {
	case map[string]any:
		return containsKeyRecursive(vv, key)
	case []any:
		for _, item := range vv {
			if containsKeyInValue(item, key) {
				return true
			}
		}
	}
	return false
}
