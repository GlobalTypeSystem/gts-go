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
func collectTraitSchemaFromValue(value map[string]any, out *[]map[string]any, depth int) {
	walkAllOf(value, depth, func(node map[string]any) {
		if ts, ok := node["x-gts-traits-schema"]; ok {
			if tsMap, ok := ts.(map[string]any); ok {
				*out = append(*out, tsMap)
			} else {
				// Non-object trait schema — still collect it as a sentinel (nil) to signal presence
				*out = append(*out, nil)
			}
		}
	})
}

// collectTraitsFromValue recursively searches a schema value for x-gts-traits entries and merges them.
func collectTraitsFromValue(value map[string]any, merged map[string]any, depth int) {
	walkAllOf(value, depth, func(node map[string]any) {
		if traits, ok := node["x-gts-traits"].(map[string]any); ok {
			for k, v := range traits {
				merged[k] = v
			}
		}
	})
}

// buildEffectiveTraitSchema composes all collected trait schemas using allOf.
func buildEffectiveTraitSchema(schemas []map[string]any) map[string]any {
	switch len(schemas) {
	case 0:
		return map[string]any{}
	case 1:
		if schemas[0] == nil {
			return map[string]any{}
		}
		return schemas[0]
	default:
		allOf := make([]any, 0, len(schemas))
		for _, s := range schemas {
			if s != nil {
				allOf = append(allOf, s)
			}
		}
		return map[string]any{
			"type":  "object",
			"allOf": allOf,
		}
	}
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

	// Check for unresolved (missing) trait properties that have no default,
	// recursing into nested object sub-properties.
	errors = append(errors, checkUnresolvedProps(traitSchema, effectiveTraits, "")...)

	return errors
}

// checkUnresolvedProps recursively checks that all trait properties have either a value
// in traits or a default in the schema. Per spec §9.7.5: "if a trait is required by the
// effective trait schema (i.e., not covered by a default) but is not provided by any
// x-gts-traits in the chain, schema validation MUST fail". A property without a default
// is implicitly required regardless of the JSON Schema 'required' array.
func checkUnresolvedProps(schema map[string]any, traits map[string]any, prefix string) []string {
	var errors []string
	for _, p := range collectAllProperties(schema, 0) {
		fullName := p.name
		if prefix != "" {
			fullName = prefix + "." + p.name
		}
		val, hasValue := traits[p.name]
		_, hasDefault := p.schema["default"]
		if !hasValue && !hasDefault {
			propType, _ := p.schema["type"].(string)
			if propType == "" {
				propType = "any"
			}
			errors = append(errors, fmt.Sprintf(
				"trait property '%s' (type: %s) is not resolved: no value provided and no default defined in the trait schema",
				fullName, propType,
			))
		} else if p.schema["type"] == "object" {
			if _, hasProps := p.schema["properties"]; hasProps {
				var subTraits map[string]any
				if valMap, ok := val.(map[string]any); ok {
					subTraits = valMap
				} else {
					subTraits = map[string]any{}
				}
				errors = append(errors, checkUnresolvedProps(p.schema, subTraits, fullName)...)
			}
		}
	}
	return errors
}

// containsXGtsTraits reports whether a schema map contains an 'x-gts-traits' key
// at its top level or nested inside any allOf items (recursively).
func containsXGtsTraits(schema map[string]any) bool {
	found := false
	walkAllOf(schema, 0, func(node map[string]any) {
		if _, ok := node["x-gts-traits"]; ok {
			found = true
		}
	})
	return found
}

// removeXGtsFields removes x-gts-* extension fields from a schema recursively.
func removeXGtsFields(schema map[string]any) map[string]any {
	return walkSchema(schema, nil, func(k string) bool {
		return strings.HasPrefix(k, "x-gts-")
	})
}

// ValidateSchemaTraitsResult is the result of OP#13 schema traits validation.
type ValidateSchemaTraitsResult struct {
	SchemaID string `json:"schema_id"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// ValidateSchemaTraits validates schema traits across the inheritance chain (OP#13).
// Walks the chain from base to leaf, collects x-gts-traits-schema and x-gts-traits
// from each level's raw content, then validates.
func (s *GtsStore) ValidateSchemaTraits(schemaID string) *ValidateSchemaTraitsResult {
	gid, err := NewGtsID(schemaID)
	if err != nil {
		return &ValidateSchemaTraitsResult{
			SchemaID: schemaID,
			OK:       false,
			Error:    fmt.Sprintf("Invalid GTS ID: %v", err),
		}
	}

	segments := gid.Segments

	// levelInfo holds per-level data collected during the chain walk.
	type levelInfo struct {
		segSchemaID string
		rawSchemas  []map[string]any // raw (unresolved) trait schemas from this level
		traits      map[string]any   // x-gts-traits collected from this level
	}

	// Pass 1: walk the chain and collect raw trait schemas and trait values per level.
	var allLevels []levelInfo
	var traitSchemas []map[string]any

	for i := range segments {
		segSchemaID := buildIDFromSegments(segments[:i+1])

		entity := s.Get(segSchemaID)
		if entity == nil {
			return &ValidateSchemaTraitsResult{
				SchemaID: schemaID,
				OK:       false,
				Error:    fmt.Sprintf("Schema '%s' not found for trait validation", segSchemaID),
			}
		}

		content := entity.Content

		var rawSchemas []map[string]any
		collectTraitSchemaFromValue(content, &rawSchemas, 0)
		traitSchemas = append(traitSchemas, rawSchemas...)

		levelTraits := make(map[string]any)
		collectTraitsFromValue(content, levelTraits, 0)

		allLevels = append(allLevels, levelInfo{
			segSchemaID: segSchemaID,
			rawSchemas:  rawSchemas,
			traits:      levelTraits,
		})
	}

	// Pass 2: normalize $$ref and resolve $ref in all collected trait schemas.
	for i, ts := range traitSchemas {
		if ts == nil {
			continue
		}
		normalized := normalizeDollarRefs(ts)
		resolved, err := s.resolveRefs(normalized)
		if err != nil {
			return &ValidateSchemaTraitsResult{
				SchemaID: schemaID,
				OK:       false,
				Error:    fmt.Sprintf("Schema '%s' trait schema has %v", schemaID, err),
			}
		}
		traitSchemas[i] = resolved
	}

	// Build a per-level slice of resolved schemas (parallel to allLevels).
	resolvedSchemasByLevel := make([][]map[string]any, len(allLevels))
	idx := 0
	for li, lv := range allLevels {
		resolvedSchemasByLevel[li] = traitSchemas[idx : idx+len(lv.rawSchemas)]
		idx += len(lv.rawSchemas)
	}

	// Pass 3: run cross-level checks against resolved schemas, then merge traits.
	mergedTraits := make(map[string]any)
	lockedTraits := make(map[string]bool)
	knownDefaults := make(map[string]any)

	for li, lv := range allLevels {
		// Track which properties this level's resolved trait schemas introduce.
		levelSchemaProps := make(map[string]bool)
		for _, ts := range resolvedSchemasByLevel[li] {
			if ts == nil {
				continue
			}
			for _, p := range collectAllProperties(ts, 0) {
				levelSchemaProps[p.name] = true
				if newDefault, ok := p.schema["default"]; ok {
					if oldDefault, exists := knownDefaults[p.name]; exists {
						if !jsonEqual(oldDefault, newDefault) {
							return &ValidateSchemaTraitsResult{
								SchemaID: schemaID,
								OK:       false,
								Error: fmt.Sprintf(
									"Schema '%s' trait validation failed: trait schema default for '%s' in '%s' overrides default set by ancestor",
									schemaID, p.name, lv.segSchemaID,
								),
							}
						}
					} else {
						knownDefaults[p.name] = newDefault
					}
				}
			}
		}

		// Check for locked trait overrides BEFORE merging this level's values.
		for k, v := range lv.traits {
			if existing, exists := mergedTraits[k]; exists {
				if lockedTraits[k] && !jsonEqual(existing, v) {
					return &ValidateSchemaTraitsResult{
						SchemaID: schemaID,
						OK:       false,
						Error: fmt.Sprintf(
							"Schema '%s' trait validation failed: trait '%s' in '%s' overrides value set by ancestor",
							schemaID, k, lv.segSchemaID,
						),
					}
				}
			}
		}

		// Merge level traits (rightmost wins).
		for k, v := range lv.traits {
			mergedTraits[k] = v
		}

		// Lock trait values set at this level only when this level does NOT introduce
		// a schema property for the key (via the resolved schema).
		for k := range lv.traits {
			if !levelSchemaProps[k] {
				lockedTraits[k] = true
			}
		}
	}

	// Check for x-gts-traits-schema integrity: must not contain x-gts-traits anywhere
	// (including nested inside allOf items).
	for i, ts := range traitSchemas {
		if ts == nil {
			// Non-object trait schema — will fail validation below
			continue
		}
		if containsXGtsTraits(ts) {
			return &ValidateSchemaTraitsResult{
				SchemaID: schemaID,
				OK:       false,
				Error: fmt.Sprintf(
					"x-gts-traits-schema[%d] contains 'x-gts-traits' — trait values must not appear inside a trait schema definition",
					i,
				),
			}
		}
	}

	// Check: if no trait schemas, but trait values exist → error
	hasTraitValues := len(mergedTraits) > 0
	if len(traitSchemas) == 0 {
		if hasTraitValues {
			return &ValidateSchemaTraitsResult{
				SchemaID: schemaID,
				OK:       false,
				Error:    "x-gts-traits values provided but no x-gts-traits-schema is defined in the inheritance chain",
			}
		}
		return &ValidateSchemaTraitsResult{SchemaID: schemaID, OK: true}
	}

	// Check for nil (non-object) trait schemas and enforce type:object
	for i, ts := range traitSchemas {
		if ts == nil {
			return &ValidateSchemaTraitsResult{
				SchemaID: schemaID,
				OK:       false,
				Error:    fmt.Sprintf("x-gts-traits-schema[%d] is not a valid JSON Schema object", i),
			}
		}
		if t, _ := ts["type"].(string); t != "object" {
			return &ValidateSchemaTraitsResult{
				SchemaID: schemaID,
				OK:       false,
				Error:    fmt.Sprintf("x-gts-traits-schema[%d] must have \"type\": \"object\"", i),
			}
		}
	}

	// Build effective trait schema
	effectiveTraitSchema := buildEffectiveTraitSchema(traitSchemas)

	// Apply defaults
	effectiveTraits := applyDefaults(effectiveTraitSchema, mergedTraits, 0)

	// Validate
	errs := validateTraitsAgainstSchema(effectiveTraitSchema, effectiveTraits, true)
	if len(errs) > 0 {
		return &ValidateSchemaTraitsResult{
			SchemaID: schemaID,
			OK:       false,
			Error:    fmt.Sprintf("Schema '%s' trait validation failed: %s", schemaID, strings.Join(errs, "; ")),
		}
	}

	return &ValidateSchemaTraitsResult{SchemaID: schemaID, OK: true}
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

// validateEntityLevelTraits checks entity-level trait constraints:
// - If a trait schema is defined, trait values must be provided.
// - Each trait schema must be closed (additionalProperties: false).
func (s *GtsStore) validateEntityLevelTraits(schemaID string) error {
	gid, err := NewGtsID(schemaID)
	if err != nil {
		return fmt.Errorf("invalid GTS ID: %v", err)
	}

	segments := gid.Segments
	var rawTraitSchemas []map[string]any
	hasTraitValues := false

	for i := range segments {
		segSchemaID := buildIDFromSegments(segments[:i+1])
		entity := s.Get(segSchemaID)
		if entity == nil {
			return fmt.Errorf("schema '%s' not found", segSchemaID)
		}
		content := entity.Content
		collectTraitSchemaFromValue(content, &rawTraitSchemas, 0)
		levelTraits := make(map[string]any)
		collectTraitsFromValue(content, levelTraits, 0)
		if len(levelTraits) > 0 {
			hasTraitValues = true
		}
	}

	if len(rawTraitSchemas) == 0 {
		return nil
	}

	if !hasTraitValues {
		return fmt.Errorf("entity defines x-gts-traits-schema but no x-gts-traits values are provided")
	}

	// Resolve $refs before checking additionalProperties
	traitSchemas := make([]map[string]any, 0, len(rawTraitSchemas))
	for _, ts := range rawTraitSchemas {
		if ts == nil {
			continue
		}
		normalized := normalizeDollarRefs(ts)
		resolved, err := s.resolveRefs(normalized)
		if err != nil {
			return fmt.Errorf("entity trait schema has %v", err)
		}
		traitSchemas = append(traitSchemas, resolved)
	}

	for _, ts := range traitSchemas {
		ap, hasAP := ts["additionalProperties"]
		if !hasAP {
			return fmt.Errorf("entity trait schema must set additionalProperties: false to be a valid standalone entity")
		}
		if b, ok := ap.(bool); !ok || b {
			return fmt.Errorf("entity trait schema must set additionalProperties: false to be a valid standalone entity")
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

	if entity.IsSchema {
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

		// Entity-level trait check: schema must have trait values if it defines a trait schema,
		// and all trait schemas must be closed (additionalProperties: false).
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

	// Also run OP#12 chain validation and OP#13 traits validation on the schema
	if entity.SchemaID != "" {
		chainResult := s.ValidateSchemaChain(entity.SchemaID)
		if !chainResult.OK {
			return &ValidateEntityResult{
				EntityID:   entityID,
				EntityType: "instance",
				OK:         false,
				Error:      chainResult.Error,
			}
		}

		traitsResult := s.ValidateSchemaTraits(entity.SchemaID)
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
