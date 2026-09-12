/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

// OP#8 – Type Schema Evolution Compatibility Checking (spec §4, v0.13).
//
// Compatibility is defined by accepted-instance-set inclusion:
//
//   - backward: Valid(old) ⊆ Valid(new)   — new consumers can read old data
//   - forward:  Valid(new) ⊆ Valid(old)   — old consumers can read new data
//   - full:     backward AND forward
//
// Each relation yields a tri-state verdict: "compatible", "incompatible", or
// "unknown" (when the checker cannot establish either).

import (
	"math"
	"reflect"
	"strings"
	"unicode/utf8"
)

// Verdict constants
const (
	VerdictCompatible   = "compatible"
	VerdictIncompatible = "incompatible"
	VerdictUnknown      = "unknown"
)

// CompatibilityResult is the JSON-serialisable response for the /compatibility endpoint.
type CompatibilityResult struct {
	OldID                 string `json:"old"`
	NewID                 string `json:"new"`
	BackwardCompatibility string `json:"backward_compatibility"`
	ForwardCompatibility  string `json:"forward_compatibility"`
	FullCompatibility     string `json:"full_compatibility"`
}

// ── Entry point ──────────────────────────────────────────────────────────────

// CheckCompatibility compares two type schemas and reports evolution compatibility (OP#8).
func (s *GtsStore) CheckCompatibility(oldTypeID, newTypeID string) *CompatibilityResult {
	unknownResult := &CompatibilityResult{
		OldID:                 oldTypeID,
		NewID:                 newTypeID,
		BackwardCompatibility: VerdictUnknown,
		ForwardCompatibility:  VerdictUnknown,
		FullCompatibility:     VerdictUnknown,
	}

	oldEntity := s.Get(oldTypeID)
	newEntity := s.Get(newTypeID)
	if oldEntity == nil || newEntity == nil || oldEntity.Content == nil || newEntity.Content == nil {
		return unknownResult
	}
	if dialectsDiffer(oldEntity.Content, newEntity.Content) {
		return unknownResult
	}

	// Normalize $$ref → $ref and resolve all $ref references so the
	// comparison operates on fully-resolved effective schemas.
	oldResolved, err1 := s.resolveRefs(normalizeDollarRefs(deepCopyMap(oldEntity.Content)))
	newResolved, err2 := s.resolveRefs(normalizeDollarRefs(deepCopyMap(newEntity.Content)))
	if err1 != nil || err2 != nil {
		return unknownResult
	}

	oldLowered, oldOK := lowerUnevaluatedProperties(oldResolved)
	newLowered, newOK := lowerUnevaluatedProperties(newResolved)
	if !oldOK || !newOK {
		return unknownResult
	}
	oldResolved = oldLowered
	newResolved = newLowered

	// backward: Valid(old) ⊆ Valid(new)
	backward := verdict(isSubschema(oldResolved, newResolved))
	// forward:  Valid(new) ⊆ Valid(old)
	forward := verdict(isSubschema(newResolved, oldResolved))

	return &CompatibilityResult{
		OldID:                 oldTypeID,
		NewID:                 newTypeID,
		BackwardCompatibility: backward,
		ForwardCompatibility:  forward,
		FullCompatibility:     fullVerdict(backward, forward),
	}
}

// ── Verdict helpers ──────────────────────────────────────────────────────────

func dialectsDiffer(oldSchema, newSchema map[string]any) bool {
	oldDialect, oldOK := oldSchema["$schema"].(string)
	newDialect, newOK := newSchema["$schema"].(string)
	canonical := func(dialect string) string {
		dialect = strings.TrimSuffix(dialect, "#")
		dialect = strings.TrimPrefix(dialect, "https://")
		return strings.TrimPrefix(dialect, "http://")
	}
	return oldOK && newOK && canonical(oldDialect) != canonical(newDialect)
}

// lowerUnevaluatedProperties rewrites a root unevaluatedProperties constraint to
// additionalProperties when it is statically equivalent. It returns ok=false
// (unknown) when the constraint interacts with dynamic keywords whose effect on
// the accepted instance set cannot be decided structurally.
func lowerUnevaluatedProperties(schema map[string]any) (map[string]any, bool) {
	value, ok := schema["unevaluatedProperties"]
	if !ok {
		return schema, true
	}
	for _, key := range []string{"$ref", "$dynamicRef", "allOf", "anyOf", "oneOf", "not", "if", "then", "else", "dependentSchemas"} {
		if _, present := schema[key]; present {
			return nil, false
		}
	}
	if existing, present := schema["additionalProperties"]; present && !reflect.DeepEqual(existing, value) {
		return nil, false
	}
	result := deepCopyMap(schema)
	delete(result, "unevaluatedProperties")
	if _, exists := result["additionalProperties"]; !exists {
		result["additionalProperties"] = value
	}
	return result, true
}

func verdict(result *bool) string {
	if result == nil {
		return VerdictUnknown
	}
	if *result {
		return VerdictCompatible
	}
	return VerdictIncompatible
}

func fullVerdict(backward, forward string) string {
	if backward == VerdictIncompatible || forward == VerdictIncompatible {
		return VerdictIncompatible
	}
	if backward == VerdictCompatible && forward == VerdictCompatible {
		return VerdictCompatible
	}
	return VerdictUnknown
}

func boolPtr(b bool) *bool { return &b }

// ── Schema sanitisation ─────────────────────────────────────────────────────

// Meta and annotation keywords that do not change the accepted instance set.
var nonAssertionKeywords = map[string]bool{
	"$id": true, "$schema": true, "$comment": true,
	"$anchor": true, "$dynamicAnchor": true, "$defs": true, "definitions": true,
	"description": true, "title": true, "examples": true, "default": true,
	"deprecated": true, "readOnly": true, "writeOnly": true,
}

// sanitizeSchema strips non-assertion and x-gts-* keywords so only validation
// semantics remain. It also drops the "type" keyword when it is fully redundant
// with a sibling const/enum (matches the Python sanitize()).
func sanitizeSchema(schema any) any {
	switch v := schema.(type) {
	case map[string]any:
		dropType := typeRedundantWithValues(v)
		result := make(map[string]any, len(v))
		for key, val := range v {
			if nonAssertionKeywords[key] {
				continue
			}
			if strings.HasPrefix(key, "x-gts-") {
				continue
			}
			if key == "type" && dropType {
				continue
			}
			result[key] = sanitizeSchema(val)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = sanitizeSchema(item)
		}
		return result
	default:
		return schema
	}
}

// typeRedundantWithValues returns true when every const/enum value already
// satisfies the sibling "type" keyword, making "type" redundant for validation.
func typeRedundantWithValues(schema map[string]any) bool {
	typeVal, hasType := schema["type"]
	if !hasType {
		return false
	}
	typeStr, ok := typeVal.(string)
	if !ok {
		return false
	}
	if constVal, hasConst := schema["const"]; hasConst {
		return valueHasType(constVal, typeStr)
	}
	if enumVal, hasEnum := schema["enum"]; hasEnum {
		if arr, ok := enumVal.([]any); ok && len(arr) > 0 {
			for _, val := range arr {
				if !valueHasType(val, typeStr) {
					return false
				}
			}
			return true
		}
	}
	return false
}

// ── Accepted-instance-set inclusion ─────────────────────────────────────────

// isSubschema checks whether Valid(subset) ⊆ Valid(superset).
// Returns *true, *false, or nil (unknown/undecidable).
func isSubschema(subset, superset map[string]any) *bool {
	subSan, ok1 := sanitizeSchema(subset).(map[string]any)
	supSan, ok2 := sanitizeSchema(superset).(map[string]any)
	if !ok1 || !ok2 {
		return nil
	}
	return checkInclusion(subSan, supSan)
}

// checkInclusion is the recursive core of the inclusion checker.
func checkInclusion(subset, superset map[string]any) *bool {
	// Fast path: when subset constrains to a finite set of values (const/enum),
	// validate each value against the superset schema directly.
	if result := finiteSubsetCheck(subset, superset); result != nil {
		return result
	}

	subType := getString(subset, "type")
	supType := getString(superset, "type")

	// ── type compatibility ──────────────────────────────────────────────
	if subType != "" && supType != "" {
		if !isTypeSubsetOf(subType, supType) {
			return boolPtr(false)
		}
	} else if subType == "" && supType != "" {
		// subset accepts any type; superset restricts → not a subschema.
		return boolPtr(false)
	}

	// ── dispatch by schema shape ────────────────────────────────────────
	if subType == "object" && supType == "object" {
		return checkObjectInclusion(subset, superset)
	}
	if subType == "array" && supType == "array" {
		return checkArrayInclusion(subset, superset)
	}

	results := []*bool{checkPrimitiveInclusion(subset, superset, subType)}
	if subType == "" && supType == "" {
		if hasObjectKeywords(subset) || hasObjectKeywords(superset) {
			results = append(results, checkObjectInclusion(subset, superset))
		}
		if hasArrayKeywords(subset) || hasArrayKeywords(superset) {
			results = append(results, checkArrayInclusion(subset, superset))
		}
	}
	for _, result := range results {
		if result == nil {
			return nil
		}
		if !*result {
			return boolPtr(false)
		}
	}
	return boolPtr(true)
}

func hasObjectKeywords(schema map[string]any) bool {
	for _, key := range []string{"properties", "required", "additionalProperties", "patternProperties"} {
		if _, ok := schema[key]; ok {
			return true
		}
	}
	return false
}

func hasArrayKeywords(schema map[string]any) bool {
	for _, key := range []string{"items", "additionalItems", "minItems", "maxItems", "uniqueItems", "contains"} {
		if _, ok := schema[key]; ok {
			return true
		}
	}
	return false
}

// isTypeSubsetOf returns true when every value of subType is also of supType.
func isTypeSubsetOf(subType, supType string) bool {
	if subType == supType {
		return true
	}
	// JSON Schema: integer is a subtype of number.
	return subType == "integer" && supType == "number"
}

// ── Finite-value fast path ──────────────────────────────────────────────────

// finiteSubsetCheck returns non-nil when subset uses const/enum and every
// enumerated value is accepted by superset.
func finiteSubsetCheck(subset, superset map[string]any) *bool {
	values := getFiniteValues(subset)
	if values == nil {
		return nil
	}
	for _, val := range values {
		if !schemaAcceptsValue(superset, val) {
			return boolPtr(false)
		}
	}
	return boolPtr(true)
}

func getFiniteValues(schema map[string]any) []any {
	if c, ok := schema["const"]; ok {
		return []any{c}
	}
	if e, ok := schema["enum"].([]any); ok {
		return e
	}
	return nil
}

// schemaAcceptsValue performs a lightweight validation of a concrete value
// against a schema (type, const, enum, and numeric/string bounds).
func schemaAcceptsValue(schema map[string]any, value any) bool {
	if typeVal, ok := schema["type"].(string); ok {
		if !valueHasType(value, typeVal) {
			return false
		}
	}
	if constVal, ok := schema["const"]; ok {
		if !jsonEqual(value, constVal) {
			return false
		}
	}
	if enumVal, ok := schema["enum"].([]any); ok {
		if !anySliceContains(enumVal, value) {
			return false
		}
	}
	if f, ok := toFloat64(value); ok {
		if min, has := getFloat(schema, "minimum"); has && f < min {
			return false
		}
		if max, has := getFloat(schema, "maximum"); has && f > max {
			return false
		}
		if emin, has := getFloat(schema, "exclusiveMinimum"); has && f <= emin {
			return false
		}
		if emax, has := getFloat(schema, "exclusiveMaximum"); has && f >= emax {
			return false
		}
	}
	if str, ok := value.(string); ok {
		runeLen := float64(utf8.RuneCountInString(str))
		if min, has := getFloat(schema, "minLength"); has && runeLen < min {
			return false
		}
		if max, has := getFloat(schema, "maxLength"); has && runeLen > max {
			return false
		}
	}
	return true
}

// valueHasType checks whether a concrete Go value matches a JSON Schema type name.
func valueHasType(value any, typeName string) bool {
	switch typeName {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := toFloat64(value)
		return ok
	case "integer":
		f, ok := toFloat64(value)
		return ok && f == math.Floor(f)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	}
	return false
}

// ── Object schema inclusion ─────────────────────────────────────────────────

func checkObjectInclusion(subset, superset map[string]any) *bool {
	subProps := getPropertiesMap(subset)
	supProps := getPropertiesMap(superset)
	subRequired := getRequiredSet(subset)
	supRequired := getRequiredSet(superset)
	subOpen := isOpenModel(subset)
	supOpen := isOpenModel(superset)

	// 1. Every property that superset requires must also be required by subset.
	//    Otherwise subset has valid instances missing a superset-required field.
	for prop := range supRequired {
		if !subRequired[prop] {
			return boolPtr(false)
		}
	}

	// 2. If subset is open and superset is closed, subset-valid instances may
	//    carry undeclared properties that superset rejects.
	if subOpen && !supOpen {
		return boolPtr(false)
	}

	// 3. Properties declared in subset but not in superset.
	for prop := range subProps {
		if _, inSup := supProps[prop]; inSup {
			continue // handled in step 4
		}
		if !supOpen {
			// Superset is closed and has no schema for this property →
			// subset-valid instances carrying this property are rejected.
			return boolPtr(false)
		}
		// Superset is open → accepts any value for undeclared properties → OK.
	}

	// 4. Properties declared in both schemas: recursive inclusion check.
	for prop, subPropVal := range subProps {
		supPropVal, inSup := supProps[prop]
		if !inSup {
			continue
		}
		subPropMap, subOk := subPropVal.(map[string]any)
		supPropMap, supOk := supPropVal.(map[string]any)
		if !subOk || !supOk {
			// Can't compare non-object property schemas.
			return nil
		}
		result := checkInclusion(subPropMap, supPropMap)
		if result == nil {
			return nil
		}
		if !*result {
			return boolPtr(false)
		}
	}

	// 5. Properties declared in superset but not in subset.
	for prop, supPropVal := range supProps {
		if _, inSub := subProps[prop]; inSub {
			continue
		}
		if subOpen {
			// Subset is open: subset-valid instances can carry this property
			// with any value. If superset constrains the property, a subset-
			// valid instance may violate the constraint → not a subschema,
			// UNLESS the superset property schema accepts everything.
			if !isAcceptAllSchema(supPropVal) {
				return boolPtr(false)
			}
		}
		// Subset is closed: subset-valid instances never carry this property.
		// We already ensured superset-required ⊆ subset-required in step 1.
	}

	return boolPtr(true)
}

// isOpenModel returns true when the schema does NOT reject undeclared properties.
func isOpenModel(schema map[string]any) bool {
	ap, ok := schema["additionalProperties"]
	if !ok {
		return true // default is open
	}
	if b, ok := ap.(bool); ok {
		return b
	}
	return true // schema-valued → partially open
}

// isAcceptAllSchema returns true when a property schema accepts every possible value.
func isAcceptAllSchema(schema any) bool {
	if b, ok := schema.(bool); ok {
		return b
	}
	if m, ok := schema.(map[string]any); ok {
		for k := range m {
			if !nonAssertionKeywords[k] && !strings.HasPrefix(k, "x-gts-") {
				return false
			}
		}
		return true
	}
	return false
}

// ── Array schema inclusion ──────────────────────────────────────────────────

func checkArrayInclusion(subset, superset map[string]any) *bool {
	subItems := getMap(subset, "items")
	supItems := getMap(superset, "items")

	if subItems != nil && supItems != nil {
		result := checkInclusion(subItems, supItems)
		if result != nil && !*result {
			return boolPtr(false)
		}
		if result == nil {
			return nil
		}
	} else if subItems == nil && supItems != nil {
		// subset accepts any items; superset constrains → not subschema.
		if !isAcceptAllSchema(supItems) {
			return boolPtr(false)
		}
	}

	// Check minItems / maxItems bounds.
	if r := checkBoundsInclusion(subset, superset, "minItems", false); r != nil && !*r {
		return boolPtr(false)
	}
	if r := checkBoundsInclusion(subset, superset, "maxItems", true); r != nil && !*r {
		return boolPtr(false)
	}

	return boolPtr(true)
}

// ── Primitive schema inclusion ──────────────────────────────────────────────

func checkPrimitiveInclusion(subset, superset map[string]any, typeName string) *bool {
	// superset const/enum constrains to a finite set that subset may exceed.
	if supConst, ok := superset["const"]; ok {
		if subConst, subHasConst := subset["const"]; subHasConst {
			return boolPtr(jsonEqual(subConst, supConst))
		}
		return boolPtr(false) // subset is more permissive
	}
	if supEnum, ok := superset["enum"].([]any); ok {
		if subEnum, ok := subset["enum"].([]any); ok {
			for _, val := range subEnum {
				if !anySliceContains(supEnum, val) {
					return boolPtr(false)
				}
			}
			return boolPtr(true)
		}
		return boolPtr(false) // subset has no enum → may exceed superset's enum
	}

	switch typeName {
	case "number", "integer":
		return checkNumericInclusion(subset, superset)
	case "string":
		return checkStringInclusion(subset, superset)
	}
	return boolPtr(true)
}

// checkNumericInclusion: subset range must be within superset range.
func checkNumericInclusion(subset, superset map[string]any) *bool {
	for _, kw := range []string{"minimum", "exclusiveMinimum"} {
		if r := checkBoundsInclusion(subset, superset, kw, false); r != nil && !*r {
			return boolPtr(false)
		}
	}
	for _, kw := range []string{"maximum", "exclusiveMaximum"} {
		if r := checkBoundsInclusion(subset, superset, kw, true); r != nil && !*r {
			return boolPtr(false)
		}
	}
	return boolPtr(true)
}

// checkStringInclusion: subset length range must be within superset range.
func checkStringInclusion(subset, superset map[string]any) *bool {
	if r := checkBoundsInclusion(subset, superset, "minLength", false); r != nil && !*r {
		return boolPtr(false)
	}
	if r := checkBoundsInclusion(subset, superset, "maxLength", true); r != nil && !*r {
		return boolPtr(false)
	}
	return boolPtr(true)
}

// checkBoundsInclusion checks that subset's range is within superset's range
// for a single keyword. upper=true for maximum-like keywords, false for minimum-like.
//
// For subset to be within superset:
//
//	minimum-like: subset.min >= superset.min (subset's floor is no lower)
//	maximum-like: subset.max <= superset.max (subset's ceiling is no higher)
func checkBoundsInclusion(subset, superset map[string]any, keyword string, upper bool) *bool {
	supVal := getNumber(superset, keyword)
	if supVal == nil {
		return nil // superset imposes no bound → OK
	}
	subVal := getNumber(subset, keyword)
	if subVal == nil {
		// superset has a bound but subset does not → subset is wider → not subschema
		return boolPtr(false)
	}
	if upper {
		if *subVal > *supVal {
			return boolPtr(false) // subset ceiling exceeds superset
		}
	} else {
		if *subVal < *supVal {
			return boolPtr(false) // subset floor is below superset
		}
	}
	return nil // bound is satisfied, but not the only criterion
}

// ── Deep-copy helper ────────────────────────────────────────────────────────

func deepCopyMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = deepCopyValue(v)
	}
	return result
}

func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyMap(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = deepCopyValue(item)
		}
		return result
	default:
		return v
	}
}

// ── Direction inference (kept for any callers; not in the v0.13 response) ────

// inferDirection determines if going up/down based on minor version.
func inferDirection(fromID, toID string) string {
	fromGtsID, err1 := NewGtsID(fromID)
	toGtsID, err2 := NewGtsID(toID)
	if err1 != nil || err2 != nil {
		return "unknown"
	}
	if len(fromGtsID.Segments) == 0 || len(toGtsID.Segments) == 0 {
		return "unknown"
	}
	fromSeg := fromGtsID.Segments[len(fromGtsID.Segments)-1]
	toSeg := toGtsID.Segments[len(toGtsID.Segments)-1]
	if fromSeg.VerMinor != nil && toSeg.VerMinor != nil {
		if *toSeg.VerMinor > *fromSeg.VerMinor {
			return "up"
		}
		if *toSeg.VerMinor < *fromSeg.VerMinor {
			return "down"
		}
		return "none"
	}
	return "unknown"
}
