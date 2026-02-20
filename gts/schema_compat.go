/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

// OP#12 – Schema-vs-schema compatibility validation.
//
// Given a chained GTS schema ID like `gts.A~B~C~`, this validates that
// each derived schema is compatible with its base:
//
//   - B (derived from A) must be compatible with A
//   - C (derived from A~B) must be compatible with A~B
//
// "Compatible" means every valid instance of the derived schema is also a valid
// instance of the base schema. The derived schema may only tighten (never loosen)
// constraints on properties inherited from the base.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// effectiveSchema holds the flattened view of a schema used for compatibility comparison.
type effectiveSchema struct {
	properties           map[string]any
	required             map[string]bool
	requiredSet          bool // true if the schema explicitly declared a "required" array
	additionalProperties any  // nil means not set
}

// extractEffectiveSchema extracts properties, required, and additionalProperties
// from a fully-resolved JSON Schema value, merging allOf items.
func extractEffectiveSchema(schema map[string]any) *effectiveSchema {
	eff := &effectiveSchema{
		properties: make(map[string]any),
		required:   make(map[string]bool),
	}
	extractEffectiveSchemaInto(schema, eff)
	return eff
}

func extractEffectiveSchemaInto(schema map[string]any, eff *effectiveSchema) {
	if schema == nil {
		return
	}

	// Direct properties
	if props, ok := schema["properties"].(map[string]any); ok {
		for k, v := range props {
			eff.properties[k] = v
		}
	}

	// Required
	if req, ok := schema["required"].([]any); ok {
		eff.requiredSet = true
		for _, v := range req {
			if s, ok := v.(string); ok {
				eff.required[s] = true
			}
		}
	}

	// additionalProperties
	if ap, ok := schema["additionalProperties"]; ok {
		eff.additionalProperties = ap
	}

	// allOf – merge from all items
	if allOf, ok := schema["allOf"].([]any); ok {
		for _, item := range allOf {
			if sub, ok := item.(map[string]any); ok {
				extractEffectiveSchemaInto(sub, eff)
			}
		}
	}
}

// validateSchemaCompatibility validates that a derived schema is compatible with its base.
// Returns a list of human-readable error descriptions (empty = compatible).
func validateSchemaCompatibility(base, derived *effectiveSchema, baseID, derivedID string) []string {
	var errors []string

	baseDisallowsAdditional := false
	if b, ok := base.additionalProperties.(bool); ok && !b {
		baseDisallowsAdditional = true
	}

	for propName, derivedProp := range derived.properties {
		if baseProp, exists := base.properties[propName]; exists {
			// Property exists in both – check for disabling (false)
			if b, ok := derivedProp.(bool); ok && !b {
				errors = append(errors, fmt.Sprintf(
					"property '%s': derived schema '%s' disables property defined in base '%s'",
					propName, derivedID, baseID,
				))
				continue
			}
			// Compare constraints
			if basePropMap, ok := baseProp.(map[string]any); ok {
				if derivedPropMap, ok := derivedProp.(map[string]any); ok {
					errors = append(errors, comparePropertyConstraints(basePropMap, derivedPropMap, propName)...)
				} else {
					// derived replaced schema object with non-object
					errors = append(errors, fmt.Sprintf(
						"property '%s': derived replaces schema object with a non-object value, loosening base constraints",
						propName,
					))
				}
			}
		} else if baseDisallowsAdditional {
			// New property in derived – base forbids additional properties
			errors = append(errors, fmt.Sprintf(
				"property '%s': derived schema '%s' adds new property but base '%s' has additionalProperties: false",
				propName, derivedID, baseID,
			))
		}
	}

	// Check if derived loosens additionalProperties constraint.
	// When base has additionalProperties: false, derived must also explicitly
	// set additionalProperties: false. Omitting it is also loosening.
	if baseDisallowsAdditional {
		derivedAlsoClosed := false
		if b, ok := derived.additionalProperties.(bool); ok && !b {
			derivedAlsoClosed = true
		}
		if !derivedAlsoClosed {
			errors = append(errors, fmt.Sprintf(
				"derived schema '%s' loosens additionalProperties from false in base '%s'",
				derivedID, baseID,
			))
		}
	}

	// Check that derived doesn't remove fields from base's required set
	errors = append(errors, checkRequiredRemoval(base, derived, baseID, derivedID)...)

	return errors
}

// comparePropertyConstraints compares constraints between base and derived property schemas.
func comparePropertyConstraints(baseProp, derivedProp map[string]any, propName string) []string {
	var errors []string

	// Type compatibility
	errors = append(errors, checkTypeCompatibility(baseProp, derivedProp, propName)...)

	// Collect derived enumerated values (const or enum)
	derivedValues, derivedEnumerates := collectDerivedEnumeratedValues(derivedProp)

	// const compatibility
	errors = append(errors, checkConstCompatibility(baseProp, derivedProp, propName)...)

	if derivedEnumerates {
		// Derived enumerates values: verify every value satisfies base bounds
		errors = append(errors, checkEnumeratedValuesAgainstBase(baseProp, derivedValues, propName)...)
	} else {
		// No enumeration: require keyword-level constraints to be preserved/tightened
		errors = append(errors, checkPatternCompatibility(baseProp, derivedProp, propName)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "maxLength", propName, true)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "maximum", propName, true)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "maxItems", propName, true)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "minLength", propName, false)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "minimum", propName, false)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "minItems", propName, false)...)
	}

	// enum compatibility
	errors = append(errors, checkEnumCompatibility(baseProp, derivedProp, propName)...)

	// Array items sub-schema comparison
	errors = append(errors, checkItemsCompatibility(baseProp, derivedProp, propName)...)

	// Recurse for nested object properties
	baseType, _ := baseProp["type"].(string)
	derivedType, _ := derivedProp["type"].(string)
	if baseType == "object" && derivedType == "object" {
		if _, hasProps := baseProp["properties"]; hasProps {
			baseNested := extractEffectiveSchema(baseProp)
			derivedNested := extractEffectiveSchema(derivedProp)
			nestedErrors := validateSchemaCompatibility(baseNested, derivedNested, "base", "derived")
			for _, e := range nestedErrors {
				errors = append(errors, fmt.Sprintf("in nested object '%s': %s", propName, e))
			}
		}
	}

	return errors
}

func checkTypeCompatibility(baseProp, derivedProp map[string]any, propName string) []string {
	baseType, hasBase := baseProp["type"]
	if !hasBase {
		return nil
	}
	derivedType, hasDerived := derivedProp["type"]
	if !hasDerived {
		return []string{fmt.Sprintf(
			"property '%s': derived omits type constraint (%v) defined in base",
			propName, baseType,
		)}
	}
	if !reflect.DeepEqual(baseType, derivedType) {
		return []string{fmt.Sprintf(
			"property '%s': derived changes type from %v to %v",
			propName, baseType, derivedType,
		)}
	}
	return nil
}

func checkConstCompatibility(baseProp, derivedProp map[string]any, propName string) []string {
	baseConst, hasBaseConst := baseProp["const"]
	if !hasBaseConst {
		return nil
	}
	derivedConst, hasDerivedConst := derivedProp["const"]
	if !hasDerivedConst {
		return []string{fmt.Sprintf(
			"property '%s': derived omits const constraint (%v) defined in base",
			propName, baseConst,
		)}
	}
	// Compare using JSON equality
	if !jsonEqual(baseConst, derivedConst) {
		return []string{fmt.Sprintf(
			"property '%s': derived redefines const from %v to %v",
			propName, baseConst, derivedConst,
		)}
	}
	return nil
}

func checkPatternCompatibility(baseProp, derivedProp map[string]any, propName string) []string {
	basePat, hasBase := baseProp["pattern"]
	if !hasBase {
		return nil
	}
	derivedPat, hasDerived := derivedProp["pattern"]
	if !hasDerived {
		return []string{fmt.Sprintf(
			"property '%s': derived omits pattern constraint (%v) defined in base",
			propName, basePat,
		)}
	}
	if basePat != derivedPat {
		return []string{fmt.Sprintf(
			"property '%s': derived changes pattern from %v to %v",
			propName, basePat, derivedPat,
		)}
	}
	return nil
}

func checkEnumCompatibility(baseProp, derivedProp map[string]any, propName string) []string {
	baseEnum, ok := baseProp["enum"].([]any)
	if !ok {
		return nil
	}
	// Check if derived has enum (subset check)
	if derivedEnum, ok := derivedProp["enum"].([]any); ok {
		var errors []string
		for _, val := range derivedEnum {
			if !anySliceContains(baseEnum, val) {
				errors = append(errors, fmt.Sprintf(
					"property '%s': derived enum contains value %v not in base enum",
					propName, val,
				))
			}
		}
		return errors
	}
	// Check if derived has const (must be in base enum)
	if derivedConst, ok := derivedProp["const"]; ok {
		if !anySliceContains(baseEnum, derivedConst) {
			return []string{fmt.Sprintf(
				"property '%s': derived const %v is not in base enum",
				propName, derivedConst,
			)}
		}
		return nil
	}
	// Neither enum nor const — loosening
	return []string{fmt.Sprintf(
		"property '%s': derived omits enum constraint defined in base",
		propName,
	)}
}

func checkItemsCompatibility(baseProp, derivedProp map[string]any, propName string) []string {
	baseItems, hasBase := baseProp["items"]
	if !hasBase {
		return nil
	}
	derivedItems, hasDerived := derivedProp["items"]
	if !hasDerived {
		return []string{fmt.Sprintf(
			"property '%s': derived omits items constraint defined in base",
			propName,
		)}
	}
	itemsName := propName + ".items"
	baseItemsMap, baseOk := baseItems.(map[string]any)
	derivedItemsMap, derivedOk := derivedItems.(map[string]any)
	if baseOk && derivedOk {
		return comparePropertyConstraints(baseItemsMap, derivedItemsMap, itemsName)
	}
	return nil
}

func checkRequiredRemoval(base, derived *effectiveSchema, baseID, derivedID string) []string {
	// Only skip if derived omits required entirely; an explicit empty list must be validated
	if !derived.requiredSet {
		return nil
	}
	var errors []string
	for baseReq := range base.required {
		if !derived.required[baseReq] {
			errors = append(errors, fmt.Sprintf(
				"derived schema '%s' removes required field '%s' defined in base '%s'",
				derivedID, baseReq, baseID,
			))
		}
	}
	return errors
}

// checkBound checks that a numeric constraint is preserved or tightened in the derived schema.
// upper=true means derived value must be <= base (e.g. maxLength); upper=false means >= (e.g. minimum).
func checkBound(baseProp, derivedProp map[string]any, keyword, propName string, upper bool) []string {
	baseVal, hasBase := getFloat(baseProp, keyword)
	if !hasBase {
		return nil
	}
	derivedVal, hasDerived := getFloat(derivedProp, keyword)
	if !hasDerived {
		return []string{fmt.Sprintf(
			"property '%s': derived omits %s constraint (%v) defined in base",
			propName, keyword, baseVal,
		)}
	}
	loosened := (upper && derivedVal > baseVal) || (!upper && derivedVal < baseVal)
	if loosened {
		return []string{fmt.Sprintf(
			"property '%s': derived %s (%v) loosens base %s (%v)",
			propName, keyword, derivedVal, keyword, baseVal,
		)}
	}
	return nil
}

// collectDerivedEnumeratedValues returns the concrete values from const or enum.
func collectDerivedEnumeratedValues(derivedProp map[string]any) ([]any, bool) {
	if c, ok := derivedProp["const"]; ok {
		return []any{c}, true
	}
	if arr, ok := derivedProp["enum"].([]any); ok {
		return arr, true
	}
	return nil, false
}

// checkEnumeratedValuesAgainstBase verifies every enumerated value satisfies base bounds.
func checkEnumeratedValuesAgainstBase(baseProp map[string]any, values []any, propName string) []string {
	var errors []string

	for _, keyword := range []string{"minimum", "minLength", "minItems"} {
		baseVal, hasBase := getFloat(baseProp, keyword)
		if !hasBase {
			continue
		}
		for _, val := range values {
			n, ok := numericValueFor(val, keyword)
			if ok && n < baseVal {
				errors = append(errors, fmt.Sprintf(
					"property '%s': derived const/enum value %v violates base %s (%v)",
					propName, val, keyword, baseVal,
				))
			}
		}
	}

	for _, keyword := range []string{"maximum", "maxLength", "maxItems"} {
		baseVal, hasBase := getFloat(baseProp, keyword)
		if !hasBase {
			continue
		}
		for _, val := range values {
			n, ok := numericValueFor(val, keyword)
			if ok && n > baseVal {
				errors = append(errors, fmt.Sprintf(
					"property '%s': derived const/enum value %v violates base %s (%v)",
					propName, val, keyword, baseVal,
				))
			}
		}
	}

	return errors
}

// numericValueFor extracts a numeric value from a JSON value for a given keyword.
func numericValueFor(val any, keyword string) (float64, bool) {
	switch keyword {
	case "minLength", "maxLength":
		if s, ok := val.(string); ok {
			return float64(len(s)), true
		}
	case "minItems", "maxItems":
		if arr, ok := val.([]any); ok {
			return float64(len(arr)), true
		}
	default:
		return toFloat64(val)
	}
	return 0, false
}

// getFloat safely extracts a float64 from a map.
func getFloat(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	return toFloat64(v)
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// stringSliceContains checks if a string is in a slice.
func stringSliceContains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// anySliceContains checks if a value is in a slice using JSON equality.
func anySliceContains(slice []any, val any) bool {
	for _, item := range slice {
		if jsonEqual(item, val) {
			return true
		}
	}
	return false
}

// jsonEqual compares two values using JSON serialization for deep equality.
func jsonEqual(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// ValidateSchemaChainResult is the result of OP#12 schema chain validation.
type ValidateSchemaChainResult struct {
	SchemaID string `json:"schema_id"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// ValidateSchemaChain validates a chained schema ID by checking each derived schema
// against its base (OP#12).
func (s *GtsStore) ValidateSchemaChain(schemaID string) *ValidateSchemaChainResult {
	gid, err := NewGtsID(schemaID)
	if err != nil {
		return &ValidateSchemaChainResult{
			SchemaID: schemaID,
			OK:       false,
			Error:    fmt.Sprintf("Invalid GTS ID: %v", err),
		}
	}

	// Single-segment schemas have no parent to validate against
	if len(gid.Segments) < 2 {
		return &ValidateSchemaChainResult{SchemaID: schemaID, OK: true}
	}

	// Build pairs of (base_id, derived_id) for each adjacent level
	segments := gid.Segments
	for i := 0; i < len(segments)-1; i++ {
		baseID := buildIDFromSegments(segments[:i+1])
		derivedID := buildIDFromSegments(segments[:i+2])

		// Check for circular refs in both schemas
		baseContent, err := s.resolveSchemaRefsChecked(baseID)
		if err != nil {
			return &ValidateSchemaChainResult{
				SchemaID: schemaID,
				OK:       false,
				Error:    fmt.Sprintf("Schema '%s' has %v", baseID, err),
			}
		}
		derivedContent, err := s.resolveSchemaRefsChecked(derivedID)
		if err != nil {
			return &ValidateSchemaChainResult{
				SchemaID: schemaID,
				OK:       false,
				Error:    fmt.Sprintf("Schema '%s' has %v", derivedID, err),
			}
		}

		baseEff := extractEffectiveSchema(baseContent)
		derivedEff := extractEffectiveSchema(derivedContent)

		errs := validateSchemaCompatibility(baseEff, derivedEff, baseID, derivedID)
		if len(errs) > 0 {
			return &ValidateSchemaChainResult{
				SchemaID: schemaID,
				OK:       false,
				Error: fmt.Sprintf(
					"Schema '%s' is not compatible with base '%s': %s",
					derivedID, baseID, strings.Join(errs, "; "),
				),
			}
		}
	}

	return &ValidateSchemaChainResult{SchemaID: schemaID, OK: true}
}

// buildIDFromSegments reconstructs a GTS ID string from a slice of segments.
func buildIDFromSegments(segments []*GtsIDSegment) string {
	sb := strings.Builder{}
	sb.WriteString(GtsPrefix)
	for _, seg := range segments {
		sb.WriteString(seg.Segment)
	}
	return sb.String()
}

// resolveSchemaRefsChecked resolves $ref references in a named schema, detecting cycles.
func (s *GtsStore) resolveSchemaRefsChecked(schemaID string) (map[string]any, error) {
	entity := s.Get(schemaID)
	if entity == nil {
		return nil, fmt.Errorf("schema '%s' not found", schemaID)
	}
	if !entity.IsSchema {
		return nil, fmt.Errorf("entity '%s' is not a schema", schemaID)
	}
	return s.resolveRefs(entity.Content)
}

// resolveRefs resolves $ref references in a raw schema map, detecting cycles.
func (s *GtsStore) resolveRefs(schema map[string]any) (map[string]any, error) {
	visited := make(map[string]bool)
	cycleFound := false
	resolved := s.resolveRefsInner(schema, visited, &cycleFound, true)
	if cycleFound {
		return nil, fmt.Errorf("circular $ref detected")
	}
	if m, ok := resolved.(map[string]any); ok {
		return m, nil
	}
	return schema, nil
}

// resolveRefsInner recursively resolves $ref references in a schema value.
func (s *GtsStore) resolveRefsInner(schema any, visited map[string]bool, cycleFound *bool, strict bool) any {
	switch v := schema.(type) {
	case map[string]any:
		// Handle $ref
		if refVal, ok := v["$ref"].(string); ok {
			// Local refs are kept as-is
			if strings.HasPrefix(refVal, "#") {
				result := make(map[string]any)
				for k, val := range v {
					result[k] = s.resolveRefsInner(val, visited, cycleFound, strict)
				}
				return result
			}

			// Normalize: strip gts:// prefix
			canonical := strings.TrimPrefix(refVal, GtsURIPrefix)

			// Cycle detection
			if visited[canonical] {
				*cycleFound = true
				result := make(map[string]any)
				for k, val := range v {
					if k != "$ref" {
						result[k] = s.resolveRefsInner(val, visited, cycleFound, strict)
					}
				}
				if len(result) == 0 {
					return schema
				}
				return result
			}

			// Try to resolve
			entity := s.Get(canonical)
			if entity != nil && entity.IsSchema {
				visited[canonical] = true
				resolved := s.resolveRefsInner(entity.Content, visited, cycleFound, strict)
				if !strict {
					delete(visited, canonical)
				}

				// Remove $id and $schema from resolved content
				if resolvedMap, ok := resolved.(map[string]any); ok {
					delete(resolvedMap, "$id")
					delete(resolvedMap, "$schema")

					// If original object has only $ref, return resolved schema
					if len(v) == 1 {
						return resolvedMap
					}

					// Merge resolved schema with other properties
					merged := make(map[string]any)
					for k, val := range resolvedMap {
						merged[k] = val
					}
					for k, val := range v {
						if k != "$ref" {
							merged[k] = s.resolveRefsInner(val, visited, cycleFound, strict)
						}
					}
					return merged
				}
			}

			// Can't resolve — remove $ref, keep other properties
			result := make(map[string]any)
			for k, val := range v {
				if k != "$ref" {
					result[k] = s.resolveRefsInner(val, visited, cycleFound, strict)
				}
			}
			if len(result) > 0 {
				return result
			}
			return schema
		}

		// Special handling for allOf: merge properties+required from resolved items
		// (but NOT additionalProperties — matches Rust resolve_schema_refs_inner behavior)
		if allOf, ok := v["allOf"].([]any); ok {
			var resolvedAllOf []any
			mergedProps := make(map[string]any)
			var mergedRequired []string

			for _, item := range allOf {
				resolved := s.resolveRefsInner(item, visited, cycleFound, strict)
				if resolvedMap, ok := resolved.(map[string]any); ok {
					if _, stillHasRef := resolvedMap["$ref"]; stillHasRef {
						resolvedAllOf = append(resolvedAllOf, resolved)
					} else {
						// Merge only properties and required
						if props, ok := resolvedMap["properties"].(map[string]any); ok {
							for k, pv := range props {
								mergedProps[k] = pv
							}
						}
						if req, ok := resolvedMap["required"].([]any); ok {
							for _, rv := range req {
								if s, ok := rv.(string); ok {
									if !stringSliceContains(mergedRequired, s) {
										mergedRequired = append(mergedRequired, s)
									}
								}
							}
						}
					}
				} else {
					resolvedAllOf = append(resolvedAllOf, resolved)
				}
			}

			if len(mergedProps) > 0 {
				// Build merged schema without allOf
				merged := make(map[string]any)
				for k, val := range v {
					if k != "allOf" {
						merged[k] = val
					}
				}
				merged["properties"] = mergedProps
				if len(mergedRequired) > 0 {
					reqAny := make([]any, len(mergedRequired))
					for i, r := range mergedRequired {
						reqAny[i] = r
					}
					merged["required"] = reqAny
				}
				if len(resolvedAllOf) > 0 {
					merged["allOf"] = resolvedAllOf
				}
				return merged
			}
		}

		// Recursively process all properties
		result := make(map[string]any)
		for k, val := range v {
			result[k] = s.resolveRefsInner(val, visited, cycleFound, strict)
		}
		return result

	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = s.resolveRefsInner(item, visited, cycleFound, strict)
		}
		return result

	default:
		return schema
	}
}
