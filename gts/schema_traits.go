/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

// OP#13 – Schema Traits Validation (x-gts-traits-schema / x-gts-traits)
//
// Validates that trait values provided in derived schemas conform to the
// effective trait schema built from the entire inheritance chain.
//
// Algorithm:
// 1. Walk the chain from leftmost (base) to rightmost (leaf) segment.
// 2. For each schema in the chain, collect:
//   - x-gts-traits-schema objects → compose via allOf into the effective trait schema.
//   - x-gts-traits objects → shallow-merge (rightmost wins) into the effective traits object.
//
// 3. Apply defaults from the effective trait schema to fill unresolved trait properties.
// 4. Validate the effective traits object against the effective trait schema.

import (
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// KeyXGtsTraitsSchema is the JSON Schema annotation keyword that defines the
// shape of trait properties available to a GTS type and its descendants.
// Schema-only — MUST NOT appear in instances (see gts-spec §9.7.1).
const KeyXGtsTraitsSchema = "x-gts-traits-schema"

// KeyXGtsTraits is the JSON Schema annotation keyword that supplies concrete
// values for trait properties declared via KeyXGtsTraitsSchema. Schema-only —
// MUST NOT appear in instances (see gts-spec §9.7.1).
const KeyXGtsTraits = "x-gts-traits"

const maxTraitsRecursionDepth = 64

// walkAllOf calls fn on the given schema and recursively on every item inside its allOf array.
// Recursion is capped at maxTraitsRecursionDepth to prevent infinite loops on cyclic schemas.
func walkAllOf(value map[string]any, depth int, fn func(map[string]any)) {
	if depth >= maxTraitsRecursionDepth {
		return
	}
	fn(value)
	if allOf, ok := value["allOf"].([]any); ok {
		for _, item := range allOf {
			if sub, ok := item.(map[string]any); ok {
				walkAllOf(sub, depth+1, fn)
			}
		}
	}
}

// collectTraitSchemaFromValue recursively searches a schema value for x-gts-traits-schema entries.
// Handles both top-level and allOf-nested occurrences.
//
// Each x-gts-traits-schema is an ordinary JSON Schema subschema (ADR-0002): its
// value MAY be a JSON object, the boolean `true` (any traits pass), or the
// boolean `false` (no traits permitted). Objects are collected as map[string]any,
// booleans as bool; any other JSON type is collected verbatim so it can be
// rejected later as an invalid subschema form.
func collectTraitSchemaFromValue(value map[string]any, out *[]any, depth int) {
	walkAllOf(value, depth, func(node map[string]any) {
		if ts, ok := node[KeyXGtsTraitsSchema]; ok {
			*out = append(*out, ts)
		}
	})
}

// collectTraitsFromValue recursively searches a schema value for x-gts-traits entries and merges them.
//
// `null` values are preserved verbatim — they carry RFC 7396 "delete this key"
// semantics and must reach the cross-level merge step (mergeRFC7396Into) intact.
func collectTraitsFromValue(value map[string]any, merged map[string]any, depth int) {
	walkAllOf(value, depth, func(node map[string]any) {
		if traits, ok := node[KeyXGtsTraits].(map[string]any); ok {
			for k, v := range traits {
				merged[k] = v
			}
		}
	})
}

// mergeRFC7396Into merges patch into target per RFC 7396 (JSON Merge Patch),
// used to compose x-gts-traits along the $id chain (root → leaf).
//
// Semantics:
//   - a `null` patch value deletes the corresponding key from target;
//   - an object patch value merges recursively (keys not restated by patch are
//     preserved), replacing wholesale when target holds a non-object;
//   - any other value (scalar or array) replaces the existing value wholesale.
//
// Object patch values are deep-copied on insert so stored entity content is
// never mutated through shared map references.
//
// Recursion over nested objects is bounded by maxTraitsRecursionDepth to
// prevent stack overflow on deeply-nested (or maliciously crafted) trait
// values supplied via x-gts-traits.
func mergeRFC7396Into(target map[string]any, patch map[string]any) {
	mergeRFC7396Recursive(target, patch, 0)
}

func mergeRFC7396Recursive(target map[string]any, patch map[string]any, depth int) {
	if depth >= maxTraitsRecursionDepth {
		return
	}
	for k, v := range patch {
		switch pv := v.(type) {
		case nil:
			delete(target, k)
		case map[string]any:
			if existing, ok := target[k].(map[string]any); ok {
				mergeRFC7396Recursive(existing, pv, depth+1)
			} else {
				fresh := make(map[string]any)
				mergeRFC7396Recursive(fresh, pv, depth+1)
				target[k] = fresh
			}
		default:
			target[k] = pv
		}
	}
}

// buildEffectiveTraitSchema composes all collected trait schemas via allOf
// (ADR-0002 chain aggregation). Each element is a JSON Schema subschema — an
// object, the boolean `true`, or the boolean `false`.
//
// Returns:
//   - the boolean `false` if any subschema along the chain is `false`
//     (allOf(false, …) ≡ false — the effective schema is unsatisfiable);
//   - otherwise an object schema: `true` subschemas are identity elements and
//     dropped, object subschemas are composed via allOf (a single object is
//     returned directly, none yields the empty/accept-all schema {}).
func buildEffectiveTraitSchema(schemas []any) any {
	objs := make([]map[string]any, 0, len(schemas))
	for _, s := range schemas {
		// A `false` anywhere in the chain (a bare boolean, or nested inside an
		// allOf) makes the composed schema unsatisfiable — allOf(false, …) ≡ false.
		if schemaIsFalse(s, 0) {
			return false
		}
		switch v := s.(type) {
		case bool:
			// `true` is the identity element under allOf — contributes nothing.
		case map[string]any:
			objs = append(objs, v)
		}
	}

	switch len(objs) {
	case 0:
		return map[string]any{}
	case 1:
		return objs[0]
	default:
		allOf := make([]any, len(objs))
		for i, o := range objs {
			allOf[i] = o
		}
		return map[string]any{
			"type":  "object",
			"allOf": allOf,
		}
	}
}

// schemaIsFalse reports whether a JSON Schema subschema is the unsatisfiable
// schema: the boolean `false`, or an object whose `allOf` contains (recursively)
// a `false`. Under allOf composition such a subschema rejects every value, which
// GTS treats as the "traits prohibited" signal (ADR-0002). Recursion is bounded
// by maxTraitsRecursionDepth.
func schemaIsFalse(v any, depth int) bool {
	if depth >= maxTraitsRecursionDepth {
		return false
	}
	switch s := v.(type) {
	case bool:
		return !s
	case map[string]any:
		if allOf, ok := s["allOf"].([]any); ok {
			for _, item := range allOf {
				if schemaIsFalse(item, depth+1) {
					return true
				}
			}
		}
	}
	return false
}

type namedProp struct {
	name   string
	schema map[string]any
}

// collectAllProperties collects all property definitions from a schema, handling allOf composition.
// Later definitions override earlier ones (rightmost-wins semantics).
func collectAllProperties(schema map[string]any, depth int) []namedProp {
	if depth >= maxTraitsRecursionDepth {
		return nil
	}

	// Use ordered insertion into a map to deduplicate (last write wins).
	order := make([]string, 0)
	byName := make(map[string]map[string]any)

	var collect func(s map[string]any, d int)
	collect = func(s map[string]any, d int) {
		if d >= maxTraitsRecursionDepth {
			return
		}
		if propsMap, ok := s["properties"].(map[string]any); ok {
			for k, v := range propsMap {
				if propSchema, ok := v.(map[string]any); ok {
					if _, seen := byName[k]; !seen {
						order = append(order, k)
					}
					byName[k] = propSchema
				}
			}
		}
		if allOf, ok := s["allOf"].([]any); ok {
			for _, item := range allOf {
				if sub, ok := item.(map[string]any); ok {
					collect(sub, d+1)
				}
			}
		}
	}
	collect(schema, depth)

	result := make([]namedProp, 0, len(order))
	for _, name := range order {
		result = append(result, namedProp{name, byName[name]})
	}
	return result
}

// collectAllRequired returns the union of `required` property names declared at
// the top level or within any allOf branch of the (effective) trait schema.
// Mirrors collectAllProperties so completeness enforcement matches JSON Schema's
// own `required` aggregation across the composed chain.
func collectAllRequired(schema map[string]any, depth int) map[string]bool {
	req := make(map[string]bool)
	var collect func(s map[string]any, d int)
	collect = func(s map[string]any, d int) {
		if d >= maxTraitsRecursionDepth {
			return
		}
		if required, ok := s["required"].([]any); ok {
			for _, item := range required {
				if name, ok := item.(string); ok {
					req[name] = true
				}
			}
		}
		if allOf, ok := s["allOf"].([]any); ok {
			for _, item := range allOf {
				if sub, ok := item.(map[string]any); ok {
					collect(sub, d+1)
				}
			}
		}
	}
	collect(schema, depth)
	return req
}

// applyDefaults applies JSON Schema default values from the effective trait schema
// to the merged traits object for any properties that are not yet present.
func applyDefaults(traitSchema map[string]any, traits map[string]any, depth int) map[string]any {
	if depth >= maxTraitsRecursionDepth {
		return traits
	}

	result := make(map[string]any)
	for k, v := range traits {
		result[k] = v
	}

	props := collectAllProperties(traitSchema, 0)
	for _, p := range props {
		if _, exists := result[p.name]; !exists {
			if def, ok := p.schema["default"]; ok {
				result[p.name] = def
			} else if p.schema["type"] == "object" {
				if _, hasProps := p.schema["properties"]; hasProps {
					// Parent absent but sub-properties may have defaults — recurse with empty map.
					sub := applyDefaults(p.schema, map[string]any{}, depth+1)
					if len(sub) > 0 {
						result[p.name] = sub
					}
				}
			}
		} else if p.schema["type"] == "object" {
			if _, hasProps := p.schema["properties"]; hasProps {
				if existing, ok := result[p.name].(map[string]any); ok {
					result[p.name] = applyDefaults(p.schema, existing, depth+1)
				}
			}
		}
	}

	return result
}

// validateTraitsAgainstSchema validates the effective traits object against the effective trait schema.
func validateTraitsAgainstSchema(traitSchema map[string]any, effectiveTraits map[string]any, checkUnresolved bool) []string {
	var errors []string

	// Use jsonschema library for standard JSON Schema validation
	compiler := jsonschema.NewCompiler()
	compiler.UseRegexpEngine(ecmaRegexpEngine)

	// Register lenient format validators
	lenientValidator := func(v any) error { return nil }
	formats := []string{
		"uuid", "date-time", "date", "time", "email", "hostname",
		"ipv4", "ipv6", "uri", "uri-reference", "iri", "iri-reference",
		"uri-template", "json-pointer", "relative-json-pointer", "regex",
	}
	for _, fmt := range formats {
		compiler.RegisterFormat(&jsonschema.Format{
			Name:     fmt,
			Validate: lenientValidator,
		})
	}

	// Remove x-gts-ref and x-gts-traits from schema before validation
	cleanSchema := removeXGtsFields(traitSchema)

	schemaID := "gts://internal/trait-schema"
	if err := compiler.AddResource(schemaID, cleanSchema); err != nil {
		errors = append(errors, fmt.Sprintf("failed to compile trait schema: %v", err))
		return errors
	}

	compiled, err := compiler.Compile(schemaID)
	if err != nil {
		errors = append(errors, fmt.Sprintf("failed to compile trait schema: %v", err))
		return errors
	}

	if verr := compiled.Validate(effectiveTraits); verr != nil {
		errors = append(errors, fmt.Sprintf("trait validation: %v", verr))
	}

	if !checkUnresolved {
		return errors
	}

	// Check that every *required* trait property is resolved.
	errors = append(errors, checkUnresolvedProps(traitSchema, effectiveTraits)...)

	return errors
}

// checkUnresolvedProps checks that every *required* trait property has either a
// value in traits or a `default` in the schema. Per ADR-0003 / README §9.7.5,
// completeness is keyed on the effective trait schema's `required` set: optional
// declared properties MAY be left unresolved. Standard JSON Schema validation
// (run by the caller) already reports missing required members against the
// materialized object; this loop adds a type-annotated, trait-specific message.
func checkUnresolvedProps(schema map[string]any, traits map[string]any) []string {
	var errors []string
	required := collectAllRequired(schema, 0)
	for _, p := range collectAllProperties(schema, 0) {
		if !required[p.name] {
			continue
		}
		_, hasValue := traits[p.name]
		_, hasDefault := p.schema["default"]
		if !hasValue && !hasDefault {
			propType, _ := p.schema["type"].(string)
			if propType == "" {
				propType = "any"
			}
			errors = append(errors, fmt.Sprintf(
				"trait property '%s' (type: %s) is not resolved: no value provided and no default defined in the trait schema",
				p.name, propType,
			))
		}
	}
	return errors
}

// removeXGtsFields removes x-gts-* extension fields from a schema recursively.
func removeXGtsFields(schema map[string]any) map[string]any {
	return walkSchema(schema, nil, func(k string) bool {
		return strings.HasPrefix(k, "x-gts-")
	})
}

// ValidateSchemaTraitsResult is the result of OP#13 schema traits validation.
type ValidateSchemaTraitsResult struct {
	TypeID string `json:"type_id"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

// ValidateSchemaTraits validates schema traits across the inheritance chain (OP#13).
// Walks the chain from base to leaf, collects x-gts-traits-schema and x-gts-traits
// from each level's raw content, then validates.
func (s *GtsStore) ValidateSchemaTraits(schemaID string) *ValidateSchemaTraitsResult {
	gid, err := NewGtsID(schemaID)
	if err != nil {
		return &ValidateSchemaTraitsResult{
			TypeID: schemaID,
			OK:     false,
			Error:  fmt.Sprintf("Invalid GTS ID: %v", err),
		}
	}

	segments := gid.Segments

	// Pass 1: walk the chain root → leaf, collecting x-gts-traits-schema
	// subschemas (composed via allOf per ADR-0002) and merging x-gts-traits
	// values per RFC 7396 JSON Merge Patch (ADR-0004). Publishers lock values
	// via standard JSON Schema `const`; the registry carries no GTS-specific
	// immutability rule.
	var traitSchemas []any
	mergedTraits := make(map[string]any)

	for i := range segments {
		segSchemaID := buildIDFromSegments(segments[:i+1])

		entity := s.Get(segSchemaID)
		if entity == nil {
			return &ValidateSchemaTraitsResult{
				TypeID: schemaID,
				OK:     false,
				Error:  fmt.Sprintf("Schema '%s' not found for trait validation", segSchemaID),
			}
		}

		content := entity.Content

		collectTraitSchemaFromValue(content, &traitSchemas, 0)

		levelTraits := make(map[string]any)
		collectTraitsFromValue(content, levelTraits, 0)
		mergeRFC7396Into(mergedTraits, levelTraits)
	}

	// Pass 2: normalize $$ref and resolve $ref in object-form trait schemas so
	// external/standalone trait shapes are inlined. Boolean subschemas pass
	// through untouched.
	for i, ts := range traitSchemas {
		tsMap, ok := ts.(map[string]any)
		if !ok {
			continue
		}
		normalized := normalizeDollarRefs(tsMap)
		resolved, err := s.resolveRefs(normalized)
		if err != nil {
			return &ValidateSchemaTraitsResult{
				TypeID: schemaID,
				OK:     false,
				Error:  fmt.Sprintf("Schema '%s' trait schema has %v", schemaID, err),
			}
		}
		traitSchemas[i] = resolved
	}

	hasTraitValues := len(mergedTraits) > 0

	// No x-gts-traits-schema anywhere in the chain: trait values are meaningless.
	if len(traitSchemas) == 0 {
		if hasTraitValues {
			return &ValidateSchemaTraitsResult{
				TypeID: schemaID,
				OK:     false,
				Error:  "x-gts-traits values provided but no x-gts-traits-schema is defined in the inheritance chain",
			}
		}
		return &ValidateSchemaTraitsResult{TypeID: schemaID, OK: true}
	}

	// Each x-gts-traits-schema is a JSON Schema subschema (ADR-0002): an object,
	// `true`, or `false`. Reject any other JSON type. x-gts-* members nested
	// inside an object subschema (e.g. a $ref-reused GTS type) are tolerated —
	// they are unknown JSON Schema keywords and inert at validation time.
	for i, ts := range traitSchemas {
		switch ts.(type) {
		case bool, map[string]any:
		default:
			return &ValidateSchemaTraitsResult{
				TypeID: schemaID,
				OK:     false,
				Error:  fmt.Sprintf("x-gts-traits-schema[%d] must be an object subschema or a boolean", i),
			}
		}
	}

	// Pairwise trait-schema compatibility: each descendant trait-schema must be
	// a valid narrowing of its ancestor. This catches type changes, AP:false
	// orphaning, and incompatible extensions — even for abstract types that skip
	// value completeness.
	for i := 1; i < len(traitSchemas); i++ {
		baseTS, baseIsMap := traitSchemas[i-1].(map[string]any)
		derivedTS, derivedIsMap := traitSchemas[i].(map[string]any)
		if !baseIsMap || !derivedIsMap {
			continue
		}
		baseEff := extractEffectiveSchema(removeXGtsFields(baseTS))
		derivedEff := extractEffectiveSchema(removeXGtsFields(derivedTS))
		compErrs := validateTraitSchemaCompatibility(baseEff, derivedEff)
		if len(compErrs) > 0 {
			return &ValidateSchemaTraitsResult{
				TypeID: schemaID,
				OK:     false,
				Error:  fmt.Sprintf("Schema '%s' trait-schema compatibility failed: %s", schemaID, strings.Join(compErrs, "; ")),
			}
		}
	}

	// Build the effective trait schema by composing the chain via allOf.
	effAny := buildEffectiveTraitSchema(traitSchemas)

	// A `false` somewhere in the chain makes the effective schema unsatisfiable:
	// traits are prohibited on this host and its whole subtree. A type carrying
	// no traits is still valid; any trait value fails.
	if b, ok := effAny.(bool); ok && !b {
		if hasTraitValues {
			return &ValidateSchemaTraitsResult{
				TypeID: schemaID,
				OK:     false,
				Error:  "x-gts-traits-schema resolves to `false` in the chain — x-gts-traits values are prohibited",
			}
		}
		return &ValidateSchemaTraitsResult{TypeID: schemaID, OK: true}
	}

	// Abstract types are "incomplete waiting for descendants" (ADR-0003): the
	// trait completeness and value validation is skipped for them, but
	// trait-schema structural compatibility has already been checked above.
	if leafEntity := s.Get(schemaID); leafEntity != nil {
		if ab, isBool := leafEntity.Content[KeyXGtsAbstract].(bool); isBool && ab {
			return &ValidateSchemaTraitsResult{TypeID: schemaID, OK: true}
		}
	}

	effectiveTraitSchema, _ := effAny.(map[string]any)

	// Materialize: apply defaults from the effective trait schema for any
	// properties not present after the chain merge.
	effectiveTraits := applyDefaults(effectiveTraitSchema, mergedTraits, 0)

	// Validate the materialized effective traits against the effective trait
	// schema, including the required-trait completeness check (this type is
	// non-abstract — abstract types returned OK above).
	errs := validateTraitsAgainstSchema(effectiveTraitSchema, effectiveTraits, true)
	if len(errs) > 0 {
		return &ValidateSchemaTraitsResult{
			TypeID: schemaID,
			OK:     false,
			Error:  fmt.Sprintf("Schema '%s' trait validation failed: %s", schemaID, strings.Join(errs, "; ")),
		}
	}

	return &ValidateSchemaTraitsResult{TypeID: schemaID, OK: true}
}

// walkSchema applies a key transform and a recursive map transform to every node in a schema.
// keyFn renames keys; valFn transforms map values (called after key rename).
func walkSchema(m map[string]any, keyFn func(string) string, skipKey func(string) bool) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		if skipKey != nil && skipKey(k) {
			continue
		}
		newKey := k
		if keyFn != nil {
			newKey = keyFn(k)
		}
		switch val := v.(type) {
		case map[string]any:
			result[newKey] = walkSchema(val, keyFn, skipKey)
		case []any:
			newArr := make([]any, len(val))
			for i, item := range val {
				if sub, ok := item.(map[string]any); ok {
					newArr[i] = walkSchema(sub, keyFn, skipKey)
				} else {
					newArr[i] = item
				}
			}
			result[newKey] = newArr
		default:
			result[newKey] = v
		}
	}
	return result
}

// normalizeDollarRefs converts $$ref → $ref throughout a schema map.
func normalizeDollarRefs(m map[string]any) map[string]any {
	return walkSchema(m, func(k string) string {
		if k == "$$ref" {
			return "$ref"
		}
		return k
	}, nil)
}

// validateEntityLevelTraits is the OP#13 entity-level check applied on the
// /validate-entity path (NOT on /validate-type-schema). For a schema to be a
// valid standalone *entity*:
//   - if any x-gts-traits-schema is declared along the chain, x-gts-traits
//     values must be provided somewhere in the chain (an open trait surface
//     with no values is an incomplete entity); and
//   - every object-form x-gts-traits-schema must be closed
//     (additionalProperties: false) — an open trait schema signals a type
//     designed to be extended, not a deployable entity.
//
// Boolean trait subschemas (true/false) carry no additionalProperties and are
// not subject to the closedness check. This mirrors the type-schema-validation
// relaxations (ADR-0002/0003) while preserving the stricter entity contract.
func (s *GtsStore) validateEntityLevelTraits(schemaID string) error {
	gid, err := NewGtsID(schemaID)
	if err != nil {
		return fmt.Errorf("invalid GTS ID: %v", err)
	}

	segments := gid.Segments
	var traitSchemas []any
	hasTraitValues := false

	for i := range segments {
		segSchemaID := buildIDFromSegments(segments[:i+1])
		entity := s.Get(segSchemaID)
		if entity == nil {
			return fmt.Errorf("schema '%s' not found", segSchemaID)
		}
		content := entity.Content
		collectTraitSchemaFromValue(content, &traitSchemas, 0)
		levelTraits := make(map[string]any)
		collectTraitsFromValue(content, levelTraits, 0)
		if len(levelTraits) > 0 {
			hasTraitValues = true
		}
	}

	if len(traitSchemas) == 0 {
		return nil
	}

	if !hasTraitValues {
		return fmt.Errorf("Entity defines x-gts-traits-schema but no x-gts-traits values are provided")
	}

	for _, ts := range traitSchemas {
		obj, ok := ts.(map[string]any)
		if !ok {
			// Boolean subschema — no additionalProperties to check.
			continue
		}
		if ap, hasAP := obj["additionalProperties"]; !hasAP {
			return fmt.Errorf("Entity trait schema must set additionalProperties: false to be a valid standalone entity")
		} else if b, isBool := ap.(bool); !isBool || b {
			return fmt.Errorf("Entity trait schema must set additionalProperties: false to be a valid standalone entity")
		}
	}

	return nil
}

// ValidateEntityResult is the result of OP#13 entity-level validation.
type ValidateEntityResult struct {
	EntityID   string `json:"entity_id"`
	EntityType string `json:"entity_type"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
}

// ValidateEntity validates an entity by running both OP#12 (schema chain) and OP#13 (traits).
// The entity_id can be either a schema ID or an instance ID.
func (s *GtsStore) ValidateEntity(entityID string) *ValidateEntityResult {
	entity := s.Get(entityID)
	if entity == nil {
		return &ValidateEntityResult{
			EntityID: entityID,
			OK:       false,
			Error:    fmt.Sprintf("Entity '%s' not found", entityID),
		}
	}

	if entity.IsTypeSchema {
		// Validate schema modifiers (x-gts-final, x-gts-abstract): type, mutual exclusion, placement.
		if err := ValidateSchemaModifiers(entity.Content); err != nil {
			return &ValidateEntityResult{
				EntityID:   entityID,
				EntityType: "schema",
				OK:         false,
				Error:      err.Error(),
			}
		}

		// Validate trait keyword placement (x-gts-traits, x-gts-traits-schema):
		// these are type-level keywords and MUST appear only at the schema top
		// level (gts-spec §9.7.1/§9.11).
		if err := ValidateTraitPlacement(entity.Content); err != nil {
			return &ValidateEntityResult{
				EntityID:   entityID,
				EntityType: "schema",
				OK:         false,
				Error:      err.Error(),
			}
		}

		// For schemas: run OP#12 chain validation + OP#13 traits validation
		chainResult := s.ValidateSchemaChain(entityID)
		if !chainResult.OK {
			return &ValidateEntityResult{
				EntityID:   entityID,
				EntityType: "schema",
				OK:         false,
				Error:      chainResult.Error,
			}
		}

		traitsResult := s.ValidateSchemaTraits(entityID)
		if !traitsResult.OK {
			return &ValidateEntityResult{
				EntityID:   entityID,
				EntityType: "schema",
				OK:         false,
				Error:      traitsResult.Error,
			}
		}

		// Entity-level trait check (stricter than type-schema validation): a
		// deployable standalone entity must provide trait values when a trait
		// schema is declared, and its trait schemas must be closed.
		if err := s.validateEntityLevelTraits(entityID); err != nil {
			return &ValidateEntityResult{
				EntityID:   entityID,
				EntityType: "schema",
				OK:         false,
				Error:      err.Error(),
			}
		}

		return &ValidateEntityResult{EntityID: entityID, EntityType: "schema", OK: true}
	}

	// Check that schema-only keywords do not appear in instance content.
	if err := ValidateInstanceModifiers(entity.Content); err != nil {
		return &ValidateEntityResult{
			EntityID:   entityID,
			EntityType: "instance",
			OK:         false,
			Error:      err.Error(),
		}
	}

	// For instances: validate against schema
	instanceResult := s.ValidateInstance(entityID)
	if !instanceResult.OK {
		return &ValidateEntityResult{
			EntityID:   entityID,
			EntityType: "instance",
			OK:         false,
			Error:      instanceResult.Error,
		}
	}

	// Also run OP#12 chain validation and OP#13 traits validation on the type-schema
	if entity.TypeID != "" {
		chainResult := s.ValidateSchemaChain(entity.TypeID)
		if !chainResult.OK {
			return &ValidateEntityResult{
				EntityID:   entityID,
				EntityType: "instance",
				OK:         false,
				Error:      chainResult.Error,
			}
		}

		traitsResult := s.ValidateSchemaTraits(entity.TypeID)
		if !traitsResult.OK {
			return &ValidateEntityResult{
				EntityID:   entityID,
				EntityType: "instance",
				OK:         false,
				Error:      traitsResult.Error,
			}
		}
	}

	return &ValidateEntityResult{EntityID: entityID, EntityType: "instance", OK: true}
}

// validateTraitSchemaCompatibility checks structural compatibility between an
// ancestor and descendant trait-schema. It delegates to the core
// validateSchemaCompatibility but filters out required-removal errors because
// trait schemas compose via allOf where required is additive (union semantics).
// A descendant only declares its OWN new required fields; ancestor required
// fields are preserved automatically through allOf composition.
func validateTraitSchemaCompatibility(base, derived *effectiveSchema) []string {
	all := validateSchemaCompatibility(base, derived, "ancestor trait-schema", "descendant trait-schema", true)
	var filtered []string
	for _, e := range all {
		if !strings.Contains(e, "removes required field") {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
