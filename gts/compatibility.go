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
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
)

// Verdict constants
const (
	VerdictCompatible   = "compatible"
	VerdictIncompatible = "incompatible"
	VerdictUnknown      = "unknown"
)

// maxCompatRecursionDepth caps how deep the accepted-instance-set inclusion
// checker descends. $ref resolution inlines other registered documents before
// the walk runs, so a resolved tree can be far deeper than anything authored;
// past the cap the checker reports "unknown" (nil) rather than a proof, so a
// truncated walk never reads as compatible or incompatible. Mirrors the
// traits walker's maxTraitsRecursionDepth and gts-rust's MAX_RECURSION_DEPTH.
const maxCompatRecursionDepth = 64

// Diagnostic direction constants.
const (
	CompatDirectionBackward = "backward"
	CompatDirectionForward  = "forward"
)

// Content-model labels for one object level (spec §4.4).
const (
	ContentModelOpen    = "open"
	ContentModelClosed  = "closed"
	ContentModelPartial = "partially_open"
)

// CompatibilityDiagnostic is one piece of structured evidence behind a
// non-compatible directional verdict, modelled after gts-rust's
// CompatibilityDiagnostic. Carrying direction + verdict + message separately
// (rather than a bare string) lets callers filter or group findings without
// parsing prose. Cross-implementation parity with gts-ts / gts-python.
type CompatibilityDiagnostic struct {
	Direction string `json:"direction"`
	Verdict   string `json:"verdict"`
	Message   string `json:"message"`
}

// ObjectLevel is the content model of one object level of a resolved schema.
// Path is "$" for the root and dotted/`[]` segments below it (e.g. "$.payload"
// or "$.items[]").
type ObjectLevel struct {
	Path         string `json:"path"`
	ContentModel string `json:"content_model"`
}

// CompatibilityResult is the JSON-serialisable response for the /compatibility endpoint.
type CompatibilityResult struct {
	OldID                 string `json:"old"`
	NewID                 string `json:"new"`
	BackwardCompatibility string `json:"backward_compatibility"`
	ForwardCompatibility  string `json:"forward_compatibility"`
	FullCompatibility     string `json:"full_compatibility"`
	IsBackwardCompatible  bool   `json:"is_backward_compatible"`
	IsForwardCompatible   bool   `json:"is_forward_compatible"`
	IsFullyCompatible     bool   `json:"is_fully_compatible"`
	// IncompatibilityReasons is the union of the directional error messages.
	IncompatibilityReasons []string `json:"incompatibility_reasons"`
	BackwardErrors         []string `json:"backward_errors"`
	ForwardErrors          []string `json:"forward_errors"`
	// Diagnostics is the structured form of the directional errors. The full
	// verdict equals verdictFromDiagnostics over the whole slice; a directional
	// verdict equals it over the entries whose Direction matches. Empty when both
	// directions are compatible.
	Diagnostics []CompatibilityDiagnostic `json:"diagnostics"`
	// CandidateObjectLevels classifies every object level of the resolved
	// "new" schema (spec §4.4), so a caller can see, per level, whether a
	// later definition can still add an optional property there.
	CandidateObjectLevels []ObjectLevel `json:"candidate_object_levels"`
}

// verdictFromDiagnostics reads a verdict off its evidence, mirroring gts-rust's
// CompatibilityVerdict::from_diagnostics: no diagnostics is a proof of
// compatibility, only inconclusive ones leave the relation unknown, and anything
// else breaks it. A diagnostic is inconclusive when its verdict is unknown — an
// inclusion the checker could neither prove nor disprove (e.g. incomparable
// dialects, or a branch it cannot order). Diagnostics are produced only for a
// non-compatible direction, so a compatible direction contributes none.
func verdictFromDiagnostics(diagnostics []CompatibilityDiagnostic) string {
	if len(diagnostics) == 0 {
		return VerdictCompatible
	}
	for _, d := range diagnostics {
		if d.Verdict != VerdictUnknown {
			return VerdictIncompatible
		}
	}
	return VerdictUnknown
}

// ── Entry point ──────────────────────────────────────────────────────────────

// CheckCompatibility compares two type schemas and reports evolution compatibility (OP#8).
func (s *GtsStore) CheckCompatibility(oldTypeID, newTypeID string) *CompatibilityResult {
	// Inconclusive-input paths report "unknown" with a reason applied to both
	// directions, rather than a bare verdict, so a caller learns why the check
	// could not run.
	inconclusive := func(reason string, levels []ObjectLevel) *CompatibilityResult {
		msg := []string{reason}
		return buildCompatResult(oldTypeID, newTypeID, VerdictUnknown, VerdictUnknown, msg, msg, levels)
	}

	oldEntity := s.Get(oldTypeID)
	newEntity := s.Get(newTypeID)
	var missing []string
	if oldEntity == nil || oldEntity.Content == nil {
		missing = append(missing, fmt.Sprintf("compatibility is unknown: old type schema not found: %s", oldTypeID))
	}
	if newEntity == nil || newEntity.Content == nil {
		missing = append(missing, fmt.Sprintf("compatibility is unknown: new type schema not found: %s", newTypeID))
	}
	if len(missing) > 0 {
		return buildCompatResult(oldTypeID, newTypeID, VerdictUnknown, VerdictUnknown, missing, missing, nil)
	}

	if dialectsDiffer(oldEntity.Content, newEntity.Content) {
		return inconclusive("compatibility is unknown: the two definitions declare different JSON Schema dialects, so their accepted-instance sets are not comparable", nil)
	}

	// Normalize $$ref → $ref and resolve all $ref references so the
	// comparison operates on fully-resolved effective schemas.
	oldResolved, err1 := s.resolveRefs(normalizeDollarRefs(deepCopyMap(oldEntity.Content)))
	newResolved, err2 := s.resolveRefs(normalizeDollarRefs(deepCopyMap(newEntity.Content)))
	if err1 != nil || err2 != nil {
		return inconclusive(unknownDirectionReason, nil)
	}

	oldLowered, oldOK := lowerUnevaluatedProperties(oldResolved)
	newLowered, newOK := lowerUnevaluatedProperties(newResolved)
	if !oldOK || !newOK {
		return inconclusive(unknownDirectionReason, classifyObjectLevels(newResolved))
	}
	oldResolved = oldLowered
	newResolved = newLowered

	// backward: Valid(old) ⊆ Valid(new)
	backward := verdict(isSubschema(oldResolved, newResolved))
	// forward:  Valid(new) ⊆ Valid(old)
	forward := verdict(isSubschema(newResolved, oldResolved))

	return buildCompatResult(
		oldTypeID, newTypeID, backward, forward,
		explainVerdict(true, backward), explainVerdict(false, forward),
		classifyObjectLevels(newResolved),
	)
}

// unknownDirectionReason is the message for a direction whose accepted-instance-set
// inclusion the checker could neither prove nor disprove.
const unknownDirectionReason = "compatibility is unknown: the accepted-instance-set inclusion could not be proved or disproved for this direction"

// explainVerdict returns human-readable reasons for a non-compatible directional
// verdict (empty when compatible). The reasons explain the accepted-instance-set
// relation, so they never contradict the verdict. Mirrors gts-python's
// explain_verdict.
func explainVerdict(backward bool, v string) []string {
	switch v {
	case VerdictCompatible:
		return nil
	case VerdictUnknown:
		return []string{unknownDirectionReason}
	default: // incompatible
		if backward {
			return []string{"backward incompatible: Valid(old) is not a subset of Valid(new); the new definition rejects instances the old definition accepts"}
		}
		return []string{"forward incompatible: Valid(new) is not a subset of Valid(old); the old definition rejects instances the new definition accepts"}
	}
}

// buildCompatResult assembles the response, deriving the structured diagnostics
// and full verdict from the directional verdicts and their error messages. All
// slices are non-nil so they serialise as [] rather than null.
func buildCompatResult(oldID, newID, backward, forward string, backwardErrors, forwardErrors []string, candidateLevels []ObjectLevel) *CompatibilityResult {
	full := fullVerdict(backward, forward)

	diagnostics := make([]CompatibilityDiagnostic, 0, len(backwardErrors)+len(forwardErrors))
	for _, msg := range backwardErrors {
		diagnostics = append(diagnostics, CompatibilityDiagnostic{Direction: CompatDirectionBackward, Verdict: backward, Message: msg})
	}
	for _, msg := range forwardErrors {
		diagnostics = append(diagnostics, CompatibilityDiagnostic{Direction: CompatDirectionForward, Verdict: forward, Message: msg})
	}

	if candidateLevels == nil {
		candidateLevels = []ObjectLevel{}
	}

	return &CompatibilityResult{
		OldID:                  oldID,
		NewID:                  newID,
		BackwardCompatibility:  backward,
		ForwardCompatibility:   forward,
		FullCompatibility:      full,
		IsBackwardCompatible:   backward == VerdictCompatible,
		IsForwardCompatible:    forward == VerdictCompatible,
		IsFullyCompatible:      full == VerdictCompatible,
		IncompatibilityReasons: dedupeStrings(backwardErrors, forwardErrors),
		BackwardErrors:         nonNilStrings(backwardErrors),
		ForwardErrors:          nonNilStrings(forwardErrors),
		Diagnostics:            diagnostics,
		CandidateObjectLevels:  candidateLevels,
	}
}

// nonNilStrings returns s, or an empty non-nil slice when s is nil, so JSON
// encodes [] rather than null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// dedupeStrings concatenates the inputs, dropping duplicates while preserving
// first-seen order. A reason that applies to both directions is reported once.
func dedupeStrings(groups ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, group := range groups {
		for _, s := range group {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

// ── Verdict helpers ──────────────────────────────────────────────────────────

func dialectsDiffer(oldSchema, newSchema map[string]any) bool {
	oldDialect, oldOK := oldSchema["$schema"].(string)
	newDialect, newOK := newSchema["$schema"].(string)
	canonical := func(dialect string) string {
		dialect = strings.TrimSuffix(dialect, LocalRefPrefix)
		dialect = strings.TrimPrefix(dialect, HTTPSPrefix)
		return strings.TrimPrefix(dialect, HTTPPrefix)
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
			if IsXGtsExtension(key) {
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
	return checkInclusion(subSan, supSan, 0)
}

// checkInclusion is the recursive core of the inclusion checker. depth is the
// current object/array nesting level; it grows by one for every property or
// item schema descended into.
func checkInclusion(subset, superset map[string]any, depth int) *bool {
	// Stop descending past the recursion cap and report "unknown" rather than
	// guessing a verdict for a tree deeper than the checker walks.
	if depth >= maxCompatRecursionDepth {
		return nil
	}

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
		return checkObjectInclusion(subset, superset, depth)
	}
	if subType == "array" && supType == "array" {
		return checkArrayInclusion(subset, superset, depth)
	}

	results := []*bool{checkPrimitiveInclusion(subset, superset, subType)}
	if subType == "" && supType == "" {
		if hasObjectKeywords(subset) || hasObjectKeywords(superset) {
			results = append(results, checkObjectInclusion(subset, superset, depth))
		}
		if hasArrayKeywords(subset) || hasArrayKeywords(superset) {
			results = append(results, checkArrayInclusion(subset, superset, depth))
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

func checkObjectInclusion(subset, superset map[string]any, depth int) *bool {
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
		result := checkInclusion(subPropMap, supPropMap, depth+1)
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

// ── Content-model classification (spec §4.4) ─────────────────────────────────

// booleanSchemaValue reduces a schema to a boolean when it is boolean-equivalent:
// true for an accept-all schema (true or {}), false for a reject-all schema
// ({"not": {}}), and nil when the schema constrains values and cannot be reduced.
// Annotation and x-gts-* keywords are ignored. Mirrors gts-rust's
// boolean_schema_value.
func booleanSchemaValue(schema any) *bool {
	if b, ok := schema.(bool); ok {
		return boolPtr(b)
	}
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	var assertionKeys []string
	for k := range m {
		if nonAssertionKeywords[k] || IsXGtsExtension(k) {
			continue
		}
		assertionKeys = append(assertionKeys, k)
	}
	if len(assertionKeys) > 1 {
		return nil
	}
	if len(assertionKeys) == 0 {
		return boolPtr(true)
	}
	if assertionKeys[0] == "not" {
		inner := booleanSchemaValue(m["not"])
		if inner == nil {
			return nil
		}
		return boolPtr(!*inner)
	}
	return nil
}

// draftSupportsUnevaluated reports whether the schema's declared JSON Schema
// dialect evaluates unevaluatedProperties. Draft-07 and earlier do not; 2019-09,
// 2020-12, and an absent/unrecognized dialect (treated as the default) do.
// Mirrors gts-rust's draft_supports_unevaluated over the detected draft.
func draftSupportsUnevaluated(schema map[string]any) bool {
	dialect, ok := schema["$schema"].(string)
	if !ok || dialect == "" {
		return true
	}
	return !strings.Contains(dialect, "draft-07") &&
		!strings.Contains(dialect, "draft-06") &&
		!strings.Contains(dialect, "draft-04")
}

// allPatternValuesAre reports whether every patternProperties subschema reduces
// to the given boolean via booleanSchemaValue.
func allPatternValuesAre(patterns map[string]any, want bool) bool {
	for _, constraint := range patterns {
		if b := booleanSchemaValue(constraint); b == nil || *b != want {
			return false
		}
	}
	return true
}

// levelContentModel classifies how one object level treats undeclared
// properties: open (accepts any), closed (rejects all), or partially_open
// (accepts some names or constrains their values). supportsUnevaluated selects
// whether unevaluatedProperties acts as the undeclared-property fallback (its
// dialect must evaluate it). Faithful port of gts-rust's classify_content_model.
func levelContentModel(schema map[string]any, supportsUnevaluated bool) string {
	var patternProps map[string]any
	if pp, ok := schema["patternProperties"].(map[string]any); ok && len(pp) > 0 {
		patternProps = pp
	}
	patternsAllOpen := patternProps != nil && allPatternValuesAre(patternProps, true)
	patternsAllClosed := patternProps != nil && allPatternValuesAre(patternProps, false)

	// propertyNames reduced to a boolean, if it is one.
	var propertyNamesModel *bool
	_, hasPropertyNames := schema["propertyNames"]
	if hasPropertyNames {
		propertyNamesModel = booleanSchemaValue(schema["propertyNames"])
	}
	if propertyNamesModel != nil && !*propertyNamesModel {
		return ContentModelClosed
	}

	// The undeclared-property fallback is additionalProperties, or (only when the
	// dialect evaluates it and additionalProperties is absent) unevaluatedProperties.
	fallback, hasFallback := schema["additionalProperties"]
	if !hasFallback && supportsUnevaluated {
		fallback, hasFallback = schema["unevaluatedProperties"]
	}
	fallbackModel := boolPtr(true) // absent fallback accepts undeclared names
	if hasFallback {
		fallbackModel = booleanSchemaValue(fallback)
	}

	// A schema-valued propertyNames/fallback constrains without closing.
	constrainsPropertyNames := propertyNamesModel == nil && hasPropertyNames
	constrainsFallback := fallbackModel == nil

	if patternProps != nil {
		switch {
		case fallbackModel != nil && !*fallbackModel && patternsAllClosed:
			return ContentModelClosed
		case fallbackModel != nil && *fallbackModel && patternsAllOpen && !constrainsPropertyNames:
			return ContentModelOpen
		default:
			return ContentModelPartial
		}
	}
	if fallbackModel != nil && !*fallbackModel {
		return ContentModelClosed
	}
	if constrainsPropertyNames || constrainsFallback {
		return ContentModelPartial
	}
	return ContentModelOpen
}

// isObjectLevel reports whether a schema node describes an object level.
func isObjectLevel(node map[string]any) bool {
	if t, ok := node["type"].(string); ok && t == "object" {
		return true
	}
	for _, key := range []string{"properties", "additionalProperties", "unevaluatedProperties", "patternProperties", "propertyNames"} {
		if _, ok := node[key]; ok {
			return true
		}
	}
	return false
}

// classifyObjectLevels returns the content model of every object level of a
// (resolved) schema, e.g. [{Path: "$", ContentModel: "closed"}, ...]. Callers
// use it to report, per level, whether a later definition can add an optional
// property there (only a closed level can, spec §4.4).
//
// Faithful port of gts-rust's classify_object_levels/collect_object_levels: the
// dialect (and thus unevaluatedProperties support) is read once from the root;
// each level folds its own allOf into the effective node before classifying; and
// the walk descends only into properties and a single-schema items. Branches
// under anyOf/oneOf/not/if have no single content model and are not reported.
// The schema MUST already be $ref-resolved. Recursion is capped at
// maxCompatRecursionDepth; a level below the cap is simply not reported (the
// classification is advisory and never decides a compatibility relation).
func classifyObjectLevels(schema map[string]any) []ObjectLevel {
	supportsUnevaluated := draftSupportsUnevaluated(schema)
	levels := []ObjectLevel{}

	var walk func(node any, path string, depth int)
	walk = func(node any, path string, depth int) {
		if depth >= maxCompatRecursionDepth {
			return
		}
		m, ok := node.(map[string]any)
		if !ok {
			return
		}
		// Fold allOf into the effective node so the content model reflects the
		// intersection, matching the resolved schema §4.4 requires.
		if _, hasAllOf := m["allOf"]; hasAllOf {
			m = flattenSchema(m)
		}
		if isObjectLevel(m) {
			levels = append(levels, ObjectLevel{Path: path, ContentModel: levelContentModel(m, supportsUnevaluated)})
		}
		if props, ok := m["properties"].(map[string]any); ok {
			// Sort names for deterministic output (Go map order is random).
			names := make([]string, 0, len(props))
			for name := range props {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				walk(props[name], path+"."+name, depth+1)
			}
		}
		if items, ok := m["items"].(map[string]any); ok {
			walk(items, path+"[]", depth+1)
		}
	}
	walk(schema, "$", 0)
	return levels
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

// isAcceptAllSchema returns true when a property schema accepts every possible
// value — the accept-all case of booleanSchemaValue.
func isAcceptAllSchema(schema any) bool {
	b := booleanSchemaValue(schema)
	return b != nil && *b
}

// ── Array schema inclusion ──────────────────────────────────────────────────

func checkArrayInclusion(subset, superset map[string]any, depth int) *bool {
	subItems := getMap(subset, "items")
	supItems := getMap(superset, "items")

	if subItems != nil && supItems != nil {
		result := checkInclusion(subItems, supItems, depth+1)
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
	if m == nil {
		return nil
	}
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
	fromGtsID, err1 := gtsid.New(fromID)
	toGtsID, err2 := gtsid.New(toID)
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
