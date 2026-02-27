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
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ValidateSchemaChainResult is the result of OP#12 schema chain validation.
type ValidateSchemaChainResult struct {
	SchemaID string `json:"schema_id"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// ValidateSchemaChain validates each derived schema against its base across the chain (OP#12).
func (s *GtsStore) ValidateSchemaChain(schemaID string) *ValidateSchemaChainResult {
	gid, err := NewGtsID(schemaID)
	if err != nil {
		return &ValidateSchemaChainResult{
			SchemaID: schemaID,
			OK:       false,
			Error:    fmt.Sprintf("Invalid GTS ID: %v", err),
		}
	}

	if len(gid.Segments) < 2 {
		return &ValidateSchemaChainResult{SchemaID: schemaID, OK: true}
	}

	segments := gid.Segments
	for i := 0; i < len(segments)-1; i++ {
		baseID := buildIDFromSegments(segments[:i+1])
		derivedID := buildIDFromSegments(segments[:i+2])

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

		errs := validateSchemaCompatibility(baseEff, derivedEff, baseID, derivedID, false)
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

// ── Effective schema extraction ───────────────────────────────────────────────

// effectiveSchema is a flattened view of a schema used for compatibility comparison.
type effectiveSchema struct {
	properties           map[string]any
	propertiesSet        bool // true if schema explicitly declared a "properties" key
	required             map[string]bool
	requiredSet          bool           // true if schema explicitly declared a "required" key
	additionalProperties any            // nil means not set
	extra                map[string]any // top-level combinators: not, anyOf, oneOf, if, then, else
}

// extractEffectiveSchema builds an effectiveSchema from a fully-resolved JSON Schema map.
func extractEffectiveSchema(schema map[string]any) *effectiveSchema {
	eff := &effectiveSchema{
		properties: make(map[string]any),
		required:   make(map[string]bool),
		extra:      make(map[string]any),
	}
	extractEffectiveSchemaInto(schema, eff)
	return eff
}

func extractEffectiveSchemaInto(schema map[string]any, eff *effectiveSchema) {
	if schema == nil {
		return
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		eff.propertiesSet = true
		for k, v := range props {
			eff.properties[k] = v
		}
	}
	if req, ok := schema["required"].([]any); ok {
		eff.requiredSet = true
		for _, v := range req {
			if s, ok := v.(string); ok {
				eff.required[s] = true
			}
		}
	}
	if ap, ok := schema["additionalProperties"]; ok {
		eff.additionalProperties = ap
	}
	for _, kw := range []string{"not", "anyOf", "oneOf", "if", "then", "else"} {
		if v, ok := schema[kw]; ok {
			eff.extra[kw] = v
		}
	}
	if allOf, ok := schema["allOf"].([]any); ok {
		for _, item := range allOf {
			if sub, ok := item.(map[string]any); ok {
				extractEffectiveSchemaInto(sub, eff)
			}
		}
	}
}

// ── Core compatibility validation ─────────────────────────────────────────────

// validateSchemaCompatibility returns compatibility errors between base and derived.
// nested suppresses top-level-only checks (e.g. omitted base properties) when called
// recursively for nested object properties.
func validateSchemaCompatibility(base, derived *effectiveSchema, baseID, derivedID string, nested bool) []string {
	var errors []string

	baseDisallowsAdditional := false
	if b, ok := base.additionalProperties.(bool); ok && !b {
		baseDisallowsAdditional = true
	}

	for propName, derivedProp := range derived.properties {
		if baseProp, exists := base.properties[propName]; exists {
			if b, ok := derivedProp.(bool); ok && !b {
				errors = append(errors, fmt.Sprintf(
					"property '%s': derived schema '%s' disables property defined in base '%s'",
					propName, derivedID, baseID,
				))
				continue
			}
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
			errors = append(errors, fmt.Sprintf(
				"property '%s': derived schema '%s' adds new property but base '%s' has additionalProperties: false",
				propName, derivedID, baseID,
			))
		}
	}

	for propName, baseProp := range base.properties {
		derivedProp, exists := derived.properties[propName]
		if b, ok := baseProp.(bool); ok && !b {
			if !exists {
				// omitting a disabled property re-enables it only if derived allows additional properties
				derivedAllowsAdditional := true
				if ap, ok := derived.additionalProperties.(bool); ok && !ap {
					derivedAllowsAdditional = false
				}
				if derivedAllowsAdditional {
					errors = append(errors, fmt.Sprintf(
						"property '%s': derived schema '%s' re-enables property disabled in base '%s'",
						propName, derivedID, baseID,
					))
				}
			} else if db, dOk := derivedProp.(bool); !dOk || db {
				errors = append(errors, fmt.Sprintf(
					"property '%s': derived schema '%s' re-enables property disabled in base '%s'",
					propName, derivedID, baseID,
				))
			}
		} else if !nested {
			// Only flag omission when derived is explicitly closed (has a properties key or AP:false).
			// An open-model derived schema implicitly accepts all properties.
			derivedIsClosed := derived.propertiesSet
			if ap, ok := derived.additionalProperties.(bool); ok && !ap {
				derivedIsClosed = true
			}
			if !exists && derivedIsClosed {
				errors = append(errors, fmt.Sprintf(
					"property '%s': derived schema '%s' omits property defined in base '%s'",
					propName, derivedID, baseID,
				))
			} else if _, baseIsObj := baseProp.(map[string]any); baseIsObj && exists {
				if _, derivedIsObj := derivedProp.(map[string]any); !derivedIsObj {
					errors = append(errors, fmt.Sprintf(
						"property '%s': derived schema '%s' replaces object schema with a non-object value, loosening base '%s' constraints",
						propName, derivedID, baseID,
					))
				}
			}
		}
	}

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

	errors = append(errors, checkRequiredRemoval(base, derived, baseID, derivedID, nested)...)
	errors = append(errors, checkTopLevelLooseningKeywords(base, derived, baseID, derivedID)...)

	return errors
}

// comparePropertyConstraints compares all keyword-level constraints between base and derived.
func comparePropertyConstraints(baseProp, derivedProp map[string]any, propName string) []string {
	var errors []string

	errors = append(errors, checkTypeCompatibility(baseProp, derivedProp, propName)...)

	derivedValues, derivedEnumerates := collectDerivedEnumeratedValues(derivedProp)

	errors = append(errors, checkConstCompatibility(baseProp, derivedProp, propName)...)

	if derivedEnumerates {
		errors = append(errors, checkEnumeratedValuesAgainstBase(baseProp, derivedValues, propName)...)
	} else {
		errors = append(errors, checkPatternCompatibility(baseProp, derivedProp, propName)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "maxLength", propName, true)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "maximum", propName, true)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "exclusiveMaximum", propName, true)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "maxItems", propName, true)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "minLength", propName, false)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "minimum", propName, false)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "exclusiveMinimum", propName, false)...)
		errors = append(errors, checkBound(baseProp, derivedProp, "minItems", propName, false)...)
		errors = append(errors, checkMultipleOf(baseProp, derivedProp, propName)...)
	}

	// enum subset check runs regardless of derivedEnumerates (no new values outside base enum)
	errors = append(errors, checkEnumCompatibility(baseProp, derivedProp, propName)...)
	errors = append(errors, checkItemsCompatibility(baseProp, derivedProp, propName)...)
	errors = append(errors, checkLooseningKeywords(baseProp, derivedProp, propName)...)

	baseType, _ := baseProp["type"].(string)
	derivedType, _ := derivedProp["type"].(string)
	if baseType == "object" && derivedType == "object" {
		if _, hasProps := baseProp["properties"]; hasProps {
			baseNested := extractEffectiveSchema(baseProp)
			derivedNested := extractEffectiveSchema(derivedProp)
			nestedErrors := validateSchemaCompatibility(baseNested, derivedNested, "base", "derived", true)
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
	baseSet := typeToSet(baseType)
	derivedType, hasDerived := derivedProp["type"]
	if !hasDerived {
		// A const or enum implicitly constrains the type; verify it is compatible.
		if constVal, hasConst := derivedProp["const"]; hasConst {
			if constType := jsonValueType(constVal); constType != "" {
				if !valueTypeCompatible(constType, baseSet) {
					return []string{fmt.Sprintf(
						"property '%s': derived const value has type %s, incompatible with base type %v",
						propName, constType, baseType,
					)}
				}
			}
			return nil
		}
		if enumVal, hasEnum := derivedProp["enum"]; hasEnum {
			if arr, ok := enumVal.([]any); ok {
				for _, item := range arr {
					if itemType := jsonValueType(item); itemType != "" {
						if !valueTypeCompatible(itemType, baseSet) {
							return []string{fmt.Sprintf(
								"property '%s': derived enum value %v has type %s, incompatible with base type %v",
								propName, item, itemType, baseType,
							)}
						}
					}
				}
			}
			return nil
		}
		return []string{fmt.Sprintf(
			"property '%s': derived omits type constraint (%v) defined in base",
			propName, baseType,
		)}
	}
	derivedSet := typeToSet(derivedType)
	for dt := range derivedSet {
		if !schemaTypeCompatible(dt, baseSet) {
			return []string{fmt.Sprintf(
				"property '%s': derived changes type from %v to %v",
				propName, baseType, derivedType,
			)}
		}
	}
	return nil
}

// schemaTypeCompatible reports if dt is in baseSet. No widening: integer↔number are distinct.
func schemaTypeCompatible(dt string, baseSet map[string]bool) bool {
	return baseSet[dt]
}

// valueTypeCompatible is like schemaTypeCompatible but allows integer↔number for
// const/enum value checks, since JSON decoder ambiguously types whole numbers.
func valueTypeCompatible(vt string, baseSet map[string]bool) bool {
	if baseSet[vt] {
		return true
	}
	if vt == "number" && baseSet["integer"] {
		return true
	}
	if vt == "integer" && baseSet["number"] {
		return true
	}
	return false
}

// typeToSet normalises a JSON Schema "type" value into a set of type strings.
func typeToSet(t any) map[string]bool {
	switch v := t.(type) {
	case string:
		return map[string]bool{v: true}
	case []any:
		s := make(map[string]bool, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				s[str] = true
			}
		}
		return s
	default:
		return map[string]bool{}
	}
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
	// Per §4.4.3: GTS-ID discriminator consts (strings containing '~') may differ across versions.
	if baseStr, ok := baseConst.(string); ok {
		if derivedStr, ok := derivedConst.(string); ok {
			if strings.Contains(baseStr, "~") && strings.Contains(derivedStr, "~") {
				return nil
			}
		}
	}
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
	// No regex subset analysis available — any change is conservatively rejected.
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
	if derivedConst, ok := derivedProp["const"]; ok {
		if !anySliceContains(baseEnum, derivedConst) {
			return []string{fmt.Sprintf(
				"property '%s': derived const %v is not in base enum",
				propName, derivedConst,
			)}
		}
		return nil
	}
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
	if baseOk && !derivedOk {
		return []string{fmt.Sprintf(
			"property '%s': derived replaced object schema with a non-object items value, loosening base constraints",
			itemsName,
		)}
	}
	return nil
}

func checkRequiredRemoval(base, derived *effectiveSchema, baseID, derivedID string, nested bool) []string {
	// In nested context, only enforce when derived explicitly declares required.
	if nested && !derived.requiredSet {
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

// checkMultipleOf verifies derived multipleOf is a multiple of base (stricter divisibility).
// E.g. base=2, derived=6 → valid; derived=3 → invalid.
func checkMultipleOf(baseProp, derivedProp map[string]any, propName string) []string {
	baseVal, hasBase := getFloat(baseProp, "multipleOf")
	if !hasBase {
		return nil
	}
	derivedVal, hasDerived := getFloat(derivedProp, "multipleOf")
	if !hasDerived {
		return []string{fmt.Sprintf(
			"property '%s': derived omits multipleOf constraint (%v) defined in base",
			propName, baseVal,
		)}
	}
	if baseVal == 0 {
		return nil
	}
	remainder := math.Mod(derivedVal, baseVal)
	if remainder != 0 {
		return []string{fmt.Sprintf(
			"property '%s': derived multipleOf (%v) is not a multiple of base multipleOf (%v)",
			propName, derivedVal, baseVal,
		)}
	}
	return nil
}

// checkBound verifies a numeric constraint is preserved or tightened.
// upper=true: derived must be ≤ base (e.g. maxLength). upper=false: derived must be ≥ base (e.g. minimum).
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

// checkTopLevelLooseningKeywords flags 'not'/'if' introduced at the derived top level when
// absent in base. anyOf/oneOf/then/else are excluded — they narrow rather than loosen.
func checkTopLevelLooseningKeywords(base, derived *effectiveSchema, baseID, derivedID string) []string {
	var errors []string
	for _, kw := range []string{"not", "if"} {
		_, derivedHas := derived.extra[kw]
		_, baseHas := base.extra[kw]
		if derivedHas && !baseHas {
			errors = append(errors, fmt.Sprintf(
				"derived schema '%s' introduces top-level '%s' keyword not present in base '%s', which may loosen constraints",
				derivedID, kw, baseID,
			))
		}
	}
	return errors
}

// checkLooseningKeywords is the per-property equivalent of checkTopLevelLooseningKeywords.
func checkLooseningKeywords(baseProp, derivedProp map[string]any, propName string) []string {
	var errors []string
	for _, kw := range []string{"not", "if"} {
		_, derivedHas := derivedProp[kw]
		_, baseHas := baseProp[kw]
		if derivedHas && !baseHas {
			errors = append(errors, fmt.Sprintf(
				"property '%s': derived introduces '%s' keyword not present in base, which may loosen constraints",
				propName, kw,
			))
		}
	}
	return errors
}

// ── Enumerated value helpers ─────────────────────────────────────────────────

// collectDerivedEnumeratedValues returns (values, true) when derived uses const or enum.
func collectDerivedEnumeratedValues(derivedProp map[string]any) ([]any, bool) {
	if c, ok := derivedProp["const"]; ok {
		return []any{c}, true
	}
	if arr, ok := derivedProp["enum"].([]any); ok {
		return arr, true
	}
	return nil, false
}

// checkEnumeratedValuesAgainstBase verifies every const/enum value satisfies base bounds.
func checkEnumeratedValuesAgainstBase(baseProp map[string]any, values []any, propName string) []string {
	var errors []string

	for _, keyword := range []string{"minimum", "minLength", "minItems", "exclusiveMinimum"} {
		baseVal, hasBase := getFloat(baseProp, keyword)
		if !hasBase {
			continue
		}
		for _, val := range values {
			n, ok := numericValueFor(val, keyword)
			if !ok {
				continue
			}
			violates := keyword == "exclusiveMinimum" && n <= baseVal
			violates = violates || (keyword != "exclusiveMinimum" && n < baseVal)
			if violates {
				errors = append(errors, fmt.Sprintf(
					"property '%s': derived const/enum value %v violates base %s (%v)",
					propName, val, keyword, baseVal,
				))
			}
		}
	}

	for _, keyword := range []string{"maximum", "maxLength", "maxItems", "exclusiveMaximum"} {
		baseVal, hasBase := getFloat(baseProp, keyword)
		if !hasBase {
			continue
		}
		for _, val := range values {
			n, ok := numericValueFor(val, keyword)
			if !ok {
				continue
			}
			violates := keyword == "exclusiveMaximum" && n >= baseVal
			violates = violates || (keyword != "exclusiveMaximum" && n > baseVal)
			if violates {
				errors = append(errors, fmt.Sprintf(
					"property '%s': derived const/enum value %v violates base %s (%v)",
					propName, val, keyword, baseVal,
				))
			}
		}
	}

	if baseMultiple, hasBase := getFloat(baseProp, "multipleOf"); hasBase && baseMultiple != 0 {
		for _, val := range values {
			n, ok := toFloat64(val)
			if ok && math.Mod(n, baseMultiple) != 0 {
				errors = append(errors, fmt.Sprintf(
					"property '%s': derived const/enum value %v violates base multipleOf (%v)",
					propName, val, baseMultiple,
				))
			}
		}
	}

	if basePat, ok := baseProp["pattern"].(string); ok && basePat != "" {
		re, err := regexp.Compile(basePat)
		if err == nil {
			for _, val := range values {
				if s, ok := val.(string); ok {
					if !re.MatchString(s) {
						errors = append(errors, fmt.Sprintf(
							"property '%s': derived const/enum value %q does not match base pattern %q",
							propName, s, basePat,
						))
					}
				}
			}
		}
	}

	return errors
}

// ── Primitive helpers ────────────────────────────────────────────────────────

// numericValueFor extracts a comparable numeric value from a JSON value for a given keyword.
func numericValueFor(val any, keyword string) (float64, bool) {
	switch keyword {
	case "minLength", "maxLength":
		if s, ok := val.(string); ok {
			return float64(utf8.RuneCountInString(s)), true
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

// jsonValueType returns the JSON Schema type name for a decoded Go value, or "" for null.
func jsonValueType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, float32, int, int32, int64, json.Number:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return ""
}

func stringSliceContains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func anySliceContains(slice []any, val any) bool {
	for _, item := range slice {
		if jsonEqual(item, val) {
			return true
		}
	}
	return false
}

func jsonEqual(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// ── Ref resolution ────────────────────────────────────────────────────────────

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

// resolveRefs resolves all $ref references in a schema map, detecting cycles and duplicates.
func (s *GtsStore) resolveRefs(schema map[string]any) (map[string]any, error) {
	visited := make(map[string]bool)
	cycleFound := false
	dupFound := false
	resolved := s.resolveRefsInner(schema, visited, &cycleFound, &dupFound)
	if cycleFound {
		return nil, fmt.Errorf("circular $ref detected")
	}
	if dupFound {
		return nil, fmt.Errorf("duplicate sibling $ref in allOf")
	}
	if m, ok := resolved.(map[string]any); ok {
		if ref := findUnresolvedRef(m); ref != "" {
			return nil, fmt.Errorf("unresolved $ref: %v", ref)
		}
		return m, nil
	}
	return schema, nil
}

// findUnresolvedRef returns the first non-local $ref still present after resolution, or "".
func findUnresolvedRef(schema any) string {
	switch v := schema.(type) {
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok && !strings.HasPrefix(ref, "#") {
			return ref
		}
		for _, val := range v {
			if found := findUnresolvedRef(val); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range v {
			if found := findUnresolvedRef(item); found != "" {
				return found
			}
		}
	}
	return ""
}

func (s *GtsStore) resolveRefsInner(schema any, visited map[string]bool, cycleFound *bool, dupFound *bool) any {
	switch v := schema.(type) {
	case map[string]any:
		// Handle $ref
		if refVal, ok := v["$ref"].(string); ok {
			if strings.HasPrefix(refVal, "#") { // local refs kept as-is
				result := make(map[string]any)
				for k, val := range v {
					result[k] = s.resolveRefsInner(val, visited, cycleFound, dupFound)
				}
				return result
			}

			canonical := strings.TrimPrefix(refVal, GtsURIPrefix)
			if visited[canonical] {
				*cycleFound = true
				result := make(map[string]any)
				for k, val := range v {
					if k != "$ref" {
						result[k] = s.resolveRefsInner(val, visited, cycleFound, dupFound)
					}
				}
				if len(result) == 0 {
					return schema
				}
				return result
			}

			entity := s.Get(canonical)
			if entity != nil && entity.IsSchema {
				visited[canonical] = true
				resolved := s.resolveRefsInner(entity.Content, visited, cycleFound, dupFound)
				delete(visited, canonical)

				if resolvedMap, ok := resolved.(map[string]any); ok {
					copy := make(map[string]any, len(resolvedMap))
					for k, val := range resolvedMap {
						copy[k] = val
					}
					delete(copy, "$id")
					delete(copy, "$schema")

					if len(v) == 1 { // pure $ref — return resolved copy directly
						return copy
					}

					// caller's own keys override the resolved base
					merged := make(map[string]any)
					for k, val := range copy {
						merged[k] = val
					}
					for k, val := range v {
						if k != "$ref" {
							merged[k] = s.resolveRefsInner(val, visited, cycleFound, dupFound)
						}
					}
					return merged
				}
			}

			// unresolvable — preserve $ref so callers can detect it
			result := make(map[string]any)
			for k, val := range v {
				if k == "$ref" {
					result[k] = val
				} else {
					result[k] = s.resolveRefsInner(val, visited, cycleFound, dupFound)
				}
			}
			return result
		}

		// Flatten allOf: merge properties/required with union semantics; hoist other keywords
		// rightmost-wins. AP is excluded from $ref-originated items to prevent base AP:false
		// bleeding onto the derived schema.
		if allOf, ok := v["allOf"].([]any); ok {
			var resolvedAllOf []any
			mergedProps := make(map[string]any)
			var mergedRequired []string
			mergedOther := make(map[string]any)
			anyMerged := false

			seenRefs := make(map[string]bool)
			for _, item := range allOf {
				if itemMap, ok := item.(map[string]any); ok {
					if refVal, ok := itemMap["$ref"].(string); ok {
						canonical := strings.TrimPrefix(refVal, GtsURIPrefix)
						if seenRefs[canonical] {
							*dupFound = true
							continue
						}
						seenRefs[canonical] = true
					}
				}

				itemWasRef := false
				if itemMap, ok := item.(map[string]any); ok {
					if _, hasRef := itemMap["$ref"].(string); hasRef {
						itemWasRef = true
					}
				}

				resolved := s.resolveRefsInner(item, visited, cycleFound, dupFound)
				if resolvedMap, ok := resolved.(map[string]any); ok {
					if _, stillHasRef := resolvedMap["$ref"]; stillHasRef {
						resolvedAllOf = append(resolvedAllOf, resolved)
					} else {
						anyMerged = true
						if props, ok := resolvedMap["properties"].(map[string]any); ok {
							for k, pv := range props {
								mergedProps[k] = pv
							}
						}
						if req, ok := resolvedMap["required"].([]any); ok {
							for _, rv := range req {
								if str, ok := rv.(string); ok {
									if !stringSliceContains(mergedRequired, str) {
										mergedRequired = append(mergedRequired, str)
									}
								}
							}
						}
						for k, val := range resolvedMap {
							switch k {
							case "properties", "required", "$id", "$schema":
								continue
							case "additionalProperties":
								if itemWasRef {
									continue
								}
							}
							mergedOther[k] = val
						}
					}
				} else {
					resolvedAllOf = append(resolvedAllOf, resolved)
				}
			}

			if anyMerged {
				merged := make(map[string]any)
				for k, val := range v {
					if k != "allOf" {
						merged[k] = val
					}
				}
				for k, val := range mergedOther { // parent keys take precedence
					if _, exists := merged[k]; !exists {
						merged[k] = val
					}
				}
				if parentProps, ok := v["properties"].(map[string]any); ok {
					for k, pv := range parentProps { // parent props overlay allOf props
						mergedProps[k] = pv
					}
				}
				if len(mergedProps) > 0 {
					merged["properties"] = mergedProps
				}
				if parentReq, ok := v["required"].([]any); ok {
					for _, rv := range parentReq {
						if str, ok := rv.(string); ok {
							if !stringSliceContains(mergedRequired, str) {
								mergedRequired = append(mergedRequired, str)
							}
						}
					}
				}
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

		result := make(map[string]any)
		for k, val := range v {
			result[k] = s.resolveRefsInner(val, visited, cycleFound, dupFound)
		}
		return result

	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = s.resolveRefsInner(item, visited, cycleFound, dupFound)
		}
		return result

	default:
		return schema
	}
}
