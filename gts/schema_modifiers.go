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

// ValidateInstanceModifiers checks that schema-only keywords (x-gts-final, x-gts-abstract)
// do not appear in instance content.
func ValidateInstanceModifiers(content map[string]any) error {
	if _, ok := content[KeyXGtsFinal]; ok {
		return fmt.Errorf("%s is a schema-only keyword and must not appear in instances", KeyXGtsFinal)
	}
	if _, ok := content[KeyXGtsAbstract]; ok {
		return fmt.Errorf("%s is a schema-only keyword and must not appear in instances", KeyXGtsAbstract)
	}
	return nil
}
