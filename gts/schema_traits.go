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

// collectTraitSchemaFromValue recursively searches a schema value for x-gts-traits-schema entries.
// Handles both top-level and allOf-nested occurrences.
func collectTraitSchemaFromValue(value map[string]any, out *[]map[string]any, depth int) {
	if depth >= maxTraitsRecursionDepth {
		return
	}

	if ts, ok := value["x-gts-traits-schema"]; ok {
		if tsMap, ok := ts.(map[string]any); ok {
			*out = append(*out, tsMap)
		} else {
			// Non-object trait schema — still collect it as a sentinel (nil) to signal presence
			*out = append(*out, nil)
		}
	}

	if allOf, ok := value["allOf"].([]any); ok {
		for _, item := range allOf {
			if sub, ok := item.(map[string]any); ok {
				collectTraitSchemaFromValue(sub, out, depth+1)
			}
		}
	}
}

// collectTraitsFromValue recursively searches a schema value for x-gts-traits entries and merges them.
func collectTraitsFromValue(value map[string]any, merged map[string]any, depth int) {
	if depth >= maxTraitsRecursionDepth {
		return
	}

	if traits, ok := value["x-gts-traits"].(map[string]any); ok {
		for k, v := range traits {
			merged[k] = v
		}
	}

	if allOf, ok := value["allOf"].([]any); ok {
		for _, item := range allOf {
			if sub, ok := item.(map[string]any); ok {
				collectTraitsFromValue(sub, merged, depth+1)
			}
		}
	}
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

	// Check for unresolved (missing) trait properties that have no default
	for _, p := range collectAllProperties(traitSchema, 0) {
		_, hasValue := effectiveTraits[p.name]
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

	var traitSchemas []map[string]any
	mergedTraits := make(map[string]any)
	lockedTraits := make(map[string]bool)
	knownDefaults := make(map[string]any)

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

		// Collect x-gts-traits-schema from raw content
		prevCount := len(traitSchemas)
		collectTraitSchemaFromValue(content, &traitSchemas, 0)

		// Track which properties this level's trait schema introduces
		levelSchemaProps := make(map[string]bool)
		for _, ts := range traitSchemas[prevCount:] {
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
									schemaID, p.name, segSchemaID,
								),
							}
						}
					} else {
						knownDefaults[p.name] = newDefault
					}
				}
			}
		}

		// Collect x-gts-traits from raw content
		levelTraits := make(map[string]any)
		collectTraitsFromValue(content, levelTraits, 0)

		// Check for locked trait overrides
		for k, v := range levelTraits {
			if existing, exists := mergedTraits[k]; exists {
				if !jsonEqual(existing, v) && lockedTraits[k] {
					return &ValidateSchemaTraitsResult{
						SchemaID: schemaID,
						OK:       false,
						Error: fmt.Sprintf(
							"Schema '%s' trait validation failed: trait '%s' in '%s' overrides value set by ancestor",
							schemaID, k, segSchemaID,
						),
					}
				}
			}
		}

		// Mark trait values as locked or unlocked
		for k := range levelTraits {
			if levelSchemaProps[k] {
				delete(lockedTraits, k)
			} else {
				lockedTraits[k] = true
			}
		}

		// Merge level traits (rightmost wins)
		for k, v := range levelTraits {
			mergedTraits[k] = v
		}
	}

	// Normalize $$ref → $ref in collected trait schemas, then resolve $ref references
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

	// Check for x-gts-traits-schema integrity: must not contain x-gts-traits
	for i, ts := range traitSchemas {
		if ts == nil {
			// Non-object trait schema — will fail validation below
			continue
		}
		if _, hasTraits := ts["x-gts-traits"]; hasTraits {
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

	// Check for nil (non-object) trait schemas
	for i, ts := range traitSchemas {
		if ts == nil {
			return &ValidateSchemaTraitsResult{
				SchemaID: schemaID,
				OK:       false,
				Error:    fmt.Sprintf("x-gts-traits-schema[%d] is not a valid JSON Schema object", i),
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
	var traitSchemas []map[string]any
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
		return fmt.Errorf("entity defines x-gts-traits-schema but no x-gts-traits values are provided")
	}

	for _, ts := range traitSchemas {
		if ts == nil {
			continue
		}
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

	// Also run traits validation on the schema chain
	if entity.SchemaID != "" {
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
