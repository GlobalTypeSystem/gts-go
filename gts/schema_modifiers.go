/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
)

const (
	KeyXGtsFinal    = "x-gts-final"
	KeyXGtsAbstract = "x-gts-abstract"
)

// ValidateSchemaModifiers checks that x-gts-final and x-gts-abstract are well-formed:
// boolean type, not both true, and not placed inside allOf entries at any depth.
func ValidateSchemaExtensions(content map[string]any) error {
	allowed := map[string]bool{KeyXGtsFinal: true, KeyXGtsAbstract: true, KeyXGtsTraits: true, KeyXGtsTraitsSchema: true, KeyXGtsRef: true}
	var walk func(any) error
	walk = func(value any) error {
		switch node := value.(type) {
		case map[string]any:
			for key, child := range node {
				if IsXGtsExtension(key) && !allowed[key] {
					return fmt.Errorf("unknown GTS extension keyword: %s", key)
				}
				if err := walk(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range node {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(content)
}

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

// validateModifierPlacement enforces that x-gts-final / x-gts-abstract appear
// only at the schema document top level (gts-spec §9.11). They are type-level
// keywords describing the GTS Type as a whole; nesting either inside ANY
// subschema (allOf, properties, $defs/definitions, items, combinators, …) is a
// misplacement and is rejected rather than silently ignored.
func validateModifierPlacement(content map[string]any) error {
	for k, v := range content {
		if k == KeyXGtsFinal || k == KeyXGtsAbstract {
			continue
		}
		if containsKeyInValue(v, KeyXGtsFinal) {
			return fmt.Errorf("%s must be at the schema top level", KeyXGtsFinal)
		}
		if containsKeyInValue(v, KeyXGtsAbstract) {
			return fmt.Errorf("%s must be at the schema top level", KeyXGtsAbstract)
		}
	}
	return nil
}

// ValidateTraitPlacement enforces that x-gts-traits and x-gts-traits-schema
// appear only at the schema document top level (gts-spec §9.7.1/§9.11). Like
// the modifiers, these are type-level keywords; nesting either inside a
// subschema (allOf, properties, $defs/definitions, combinators, items, …) is a
// misplacement and is rejected (fail fast).
//
// The rule constrains only the *position* of the keyword, not the *contents*
// of its value: the top-level x-gts-traits-schema is an ordinary JSON Schema
// subschema whose body may legitimately carry x-gts-* members (e.g. when an
// existing GTS type is reused as a trait-schema source via $ref). The top-level
// keyword values are therefore not re-scanned.
func ValidateTraitPlacement(content map[string]any) error {
	for k, v := range content {
		// The four document-level keyword slots are allowed at the top level;
		// their own values are not re-scanned (see doc comment).
		if k == KeyXGtsFinal || k == KeyXGtsAbstract || k == KeyXGtsTraits || k == KeyXGtsTraitsSchema {
			continue
		}
		if containsKeyInValue(v, KeyXGtsTraitsSchema) {
			return fmt.Errorf("%s must be at the schema top level", KeyXGtsTraitsSchema)
		}
		if containsKeyInValue(v, KeyXGtsTraits) {
			return fmt.Errorf("%s must be at the schema top level", KeyXGtsTraits)
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
