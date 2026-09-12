/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// CastCompatResult holds the structural-diff compatibility info returned by OP#9 (Cast).
// This is separate from OP#8's CompatibilityResult, which uses accepted-instance-set
// inclusion and tri-state verdicts (v0.13).
type CastCompatResult struct {
	FromID                 string              `json:"from"`
	ToID                   string              `json:"to"`
	OldID                  string              `json:"old"`
	NewID                  string              `json:"new"`
	Direction              string              `json:"direction"`
	AddedProperties        []string            `json:"added_properties"`
	RemovedProperties      []string            `json:"removed_properties"`
	ChangedProperties      []map[string]string `json:"changed_properties"`
	IsFullyCompatible      *bool               `json:"is_fully_compatible"`
	IsBackwardCompatible   *bool               `json:"is_backward_compatible"`
	IsForwardCompatible    *bool               `json:"is_forward_compatible"`
	IncompatibilityReasons []string            `json:"incompatibility_reasons"`
	BackwardErrors         []string            `json:"backward_errors"`
	ForwardErrors          []string            `json:"forward_errors"`
	BackwardCompatibility  string              `json:"backward_compatibility"`
	ForwardCompatibility   string              `json:"forward_compatibility"`
	FullCompatibility      string              `json:"full_compatibility"`
	Error                  string              `json:"error,omitempty"`
}

// CastResult represents the result of casting an instance to a new schema version
type CastResult struct {
	*CastCompatResult
	CastedEntity map[string]any `json:"casted_entity,omitempty"`
}

// Cast transforms an instance to conform to a target schema version
// see gts-python store.py cast method
func (s *GtsStore) Cast(instanceID, toTypeID string) (*CastResult, error) {
	// Get instance entity
	instanceEntity := s.Get(instanceID)
	if instanceEntity == nil {
		return nil, &StoreGtsObjectNotFoundError{EntityID: instanceID}
	}

	// Get target schema
	toSchema := s.Get(toTypeID)
	if toSchema == nil {
		return nil, &StoreGtsSchemaNotFoundError{EntityID: toTypeID}
	}

	// Determine source type-schema
	var fromTypeID string
	var fromSchema *JsonEntity
	if instanceEntity.IsTypeSchema {
		// Not allowed to cast directly from a type-schema
		return nil, &StoreGtsCastFromSchemaNotAllowedError{FromID: instanceID}
	} else {
		// Casting an instance - need to find its type-schema
		fromTypeID = instanceEntity.TypeID
		if fromTypeID == "" {
			return nil, &StoreGtsSchemaForInstanceNotFoundError{EntityID: instanceID}
		}
		fromSchema = s.Get(fromTypeID)
		if fromSchema == nil {
			return nil, &StoreGtsSchemaNotFoundError{EntityID: fromTypeID}
		}
	}

	// Get content as maps
	instanceContent := instanceEntity.Content
	fromSchemaContent := fromSchema.Content
	toSchemaContent := toSchema.Content

	// Perform the cast
	result, err := castInstance(instanceID, toTypeID, instanceContent, fromSchemaContent, toSchemaContent, s)
	if err != nil {
		return nil, err
	}
	compatibility := s.CheckCompatibility(fromTypeID, toTypeID)
	result.BackwardCompatibility = compatibility.BackwardCompatibility
	result.ForwardCompatibility = compatibility.ForwardCompatibility
	result.FullCompatibility = compatibility.FullCompatibility
	// When the schemas declare distinct JSON Schema dialects the structural
	// verdicts are undecidable; surface them as null alongside the unknown
	// accepted-instance-set verdicts.
	if dialectsDiffer(fromSchemaContent, toSchemaContent) {
		result.IsBackwardCompatible = nil
		result.IsForwardCompatible = nil
		result.IsFullyCompatible = nil
	}
	return result, nil
}

// castInstance performs the actual casting logic
// see gts-python schema_cast.py cast method
func castInstance(
	fromInstanceID, toTypeID string,
	fromInstanceContent, fromSchemaContent, toSchemaContent map[string]any,
	store *GtsStore,
) (*CastResult, error) {
	// Flatten target schema to merge allOf
	targetSchema := flattenSchema(toSchemaContent)

	// Determine direction
	direction := inferDirection(fromInstanceID, toTypeID)

	// Determine which is old/new based on direction
	var oldSchema, newSchema map[string]any
	switch direction {
	case "up":
		oldSchema = fromSchemaContent
		newSchema = toSchemaContent
	case "down":
		oldSchema = toSchemaContent
		newSchema = fromSchemaContent
	default:
		oldSchema = fromSchemaContent
		newSchema = toSchemaContent
	}

	// Check compatibility
	isBackward, backwardErrors := checkBackwardCompatibility(oldSchema, newSchema)
	isForward, forwardErrors := checkForwardCompatibility(oldSchema, newSchema)

	// Apply casting rules to transform the instance
	casted, added, removed, incompatibilityReasons := castInstanceToSchema(
		copyMap(fromInstanceContent),
		targetSchema,
		"",
	)

	// Validate the casted instance against the full target schema
	var isFullyCompatible bool
	if casted != nil {
		err := validateWithGtsIDTolerance(casted, toSchemaContent, store)
		if err != nil {
			incompatibilityReasons = append(incompatibilityReasons, err.Error())
			isFullyCompatible = false
		} else {
			isFullyCompatible = true
		}
	} else {
		isFullyCompatible = false
	}

	return &CastResult{
		CastCompatResult: &CastCompatResult{
			FromID:                 fromInstanceID,
			ToID:                   toTypeID,
			OldID:                  fromInstanceID,
			NewID:                  toTypeID,
			Direction:              direction,
			AddedProperties:        deduplicate(added),
			RemovedProperties:      deduplicate(removed),
			ChangedProperties:      []map[string]string{},
			IsFullyCompatible:      boolPtr(isFullyCompatible),
			IsBackwardCompatible:   boolPtr(isBackward),
			IsForwardCompatible:    boolPtr(isForward),
			IncompatibilityReasons: incompatibilityReasons,
			BackwardErrors:         backwardErrors,
			ForwardErrors:          forwardErrors,
		},
		CastedEntity: casted,
	}, nil
}

// castInstanceToSchema transforms instance to conform to target schema
// see gts-python schema_cast.py _cast_instance_to_schema method
func castInstanceToSchema(
	instance map[string]any,
	schema map[string]any,
	basePath string,
) (map[string]any, []string, []string, []string) {
	added := []string{}
	removed := []string{}
	incompatibilityReasons := []string{}

	if instance == nil {
		incompatibilityReasons = append(incompatibilityReasons, "Instance must be an object for casting")
		return nil, added, removed, incompatibilityReasons
	}

	targetProps := getPropertiesMap(schema)
	required := getRequiredSet(schema)
	additional := getAdditionalProperties(schema)

	// Start from current values
	result := copyMap(instance)

	// 1) Ensure required properties exist (fill defaults if provided)
	for reqProp := range required {
		if _, exists := result[reqProp]; !exists {
			propSchema := getMap(targetProps, reqProp)
			if propSchema != nil {
				if defaultVal, hasDefault := propSchema["default"]; hasDefault {
					result[reqProp] = copyValue(defaultVal)
					path := buildPath(basePath, reqProp)
					added = append(added, path)
				} else {
					path := buildPath(basePath, reqProp)
					incompatibilityReasons = append(incompatibilityReasons,
						fmt.Sprintf("Missing required property '%s' and no default is defined", path))
				}
			}
		}
	}

	// 2) For optional properties with defaults, set if missing
	for prop, propSchemaAny := range targetProps {
		if required[prop] {
			continue
		}
		propSchema, ok := propSchemaAny.(map[string]any)
		if !ok {
			continue
		}
		if _, exists := result[prop]; !exists {
			if defaultVal, hasDefault := propSchema["default"]; hasDefault {
				result[prop] = copyValue(defaultVal)
				path := buildPath(basePath, prop)
				added = append(added, path)
			}
		}
	}

	// 2.5) Update const values to match target schema (for GTS ID fields)
	for prop, propSchemaAny := range targetProps {
		propSchema, ok := propSchemaAny.(map[string]any)
		if !ok {
			continue
		}
		if constVal, hasConst := propSchema["const"]; hasConst {
			if existingVal, exists := result[prop]; exists {
				constStr, constIsStr := constVal.(string)
				existingStr, existingIsStr := existingVal.(string)
				if constIsStr && existingIsStr {
					// Only update if both are GTS IDs and they differ
					if IsValidGtsID(constStr) && IsValidGtsID(existingStr) {
						if existingStr != constStr {
							result[prop] = constStr
						}
					}
				}
			}
		}
	}

	// 3) Remove properties not in target schema when additionalProperties is false.
	// Reserved GTS identity fields (id, type) are preserved at the instance root.
	if !additional {
		for prop := range result {
			if _, inTarget := targetProps[prop]; inTarget {
				continue
			}
			if basePath == "" && (prop == "id" || prop == "type") {
				continue
			}
			delete(result, prop)
			path := buildPath(basePath, prop)
			removed = append(removed, path)
		}
	}

	// 4) Recurse into nested object properties
	for prop, propSchemaAny := range targetProps {
		val, exists := result[prop]
		if !exists {
			continue
		}
		propSchema, ok := propSchemaAny.(map[string]any)
		if !ok {
			continue
		}
		propType := getString(propSchema, "type")

		// Handle nested objects
		if propType == "object" {
			if valMap, isMap := val.(map[string]any); isMap {
				nestedSchema := effectiveObjectSchema(propSchema)
				newObj, addSub, remSub, incompatSub := castInstanceToSchema(
					valMap,
					nestedSchema,
					buildPath(basePath, prop),
				)
				result[prop] = newObj
				added = append(added, addSub...)
				removed = append(removed, remSub...)
				incompatibilityReasons = append(incompatibilityReasons, incompatSub...)
			}
		}

		// Handle arrays of objects
		if propType == "array" {
			if valArray, isArray := val.([]any); isArray {
				itemsSchema := getMap(propSchema, "items")
				if itemsSchema != nil && getString(itemsSchema, "type") == "object" {
					nestedSchema := effectiveObjectSchema(itemsSchema)
					newList := []any{}
					for idx, item := range valArray {
						if itemMap, isMap := item.(map[string]any); isMap {
							newItem, addSub, remSub, incompatSub := castInstanceToSchema(
								itemMap,
								nestedSchema,
								buildPath(basePath, fmt.Sprintf("%s[%d]", prop, idx)),
							)
							newList = append(newList, newItem)
							added = append(added, addSub...)
							removed = append(removed, remSub...)
							incompatibilityReasons = append(incompatibilityReasons, incompatSub...)
						} else {
							newList = append(newList, item)
						}
					}
					result[prop] = newList
				}
			}
		}
	}

	return result, added, removed, incompatibilityReasons
}

// effectiveObjectSchema extracts the object schema from allOf if needed
// see gts-python schema_cast.py _effective_object_schema method
func effectiveObjectSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return make(map[string]any)
	}

	// If it has properties or required directly, use it
	if _, hasProps := schema["properties"]; hasProps {
		return schema
	}
	if _, hasReq := schema["required"]; hasReq {
		return schema
	}

	// Check allOf for object schemas
	if allOfVal, ok := schema["allOf"]; ok {
		if allOfList, ok := allOfVal.([]any); ok {
			for _, partAny := range allOfList {
				if part, ok := partAny.(map[string]any); ok {
					if _, hasProps := part["properties"]; hasProps {
						return part
					}
					if _, hasReq := part["required"]; hasReq {
						return part
					}
				}
			}
		}
	}

	return schema
}

// validateWithGtsIDTolerance validates instance against schema, allowing GTS ID const differences
// see gts-python schema_cast.py _validate_with_gts_id_tolerance method
func validateWithGtsIDTolerance(instance, schema map[string]any, store *GtsStore) error {
	// Create modified schema that removes const constraints for GTS IDs
	modifiedSchema := removeGtsConstConstraints(schema)

	// Compile and validate
	compiler := jsonschema.NewCompiler()
	compiler.UseRegexpEngine(ecmaRegexpEngine)

	// Set up custom loader for GTS ID references
	compiler.UseLoader(&gtsURLLoader{store: store})

	// Pre-load all schemas from the store
	for id, entity := range store.byID {
		if entity.IsTypeSchema {
			_ = compiler.AddResource(id, entity.Content)
		}
	}

	// Add the modified schema as a resource
	schemaID := "_cast_validation"
	_ = compiler.AddResource(schemaID, modifiedSchema)

	// Compile the modified schema
	schemaObj, err := compiler.Compile(schemaID)
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	// Validate instance
	err = schemaObj.Validate(instance)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// removeGtsConstConstraints recursively removes const constraints where value is a GTS ID
// see gts-python schema_cast.py _remove_gts_const_constraints method
func removeGtsConstConstraints(schema any) any {
	switch v := schema.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			if key == "const" {
				if strVal, ok := value.(string); ok && IsValidGtsID(strVal) {
					// Replace const with type constraint instead
					result["type"] = "string"
					continue
				}
			}
			result[key] = removeGtsConstConstraints(value)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = removeGtsConstConstraints(item)
		}
		return result
	default:
		return v
	}
}

// Helper functions

// getAdditionalProperties safely extracts additionalProperties (defaults to true)
func getAdditionalProperties(schema map[string]any) bool {
	if val, ok := schema["additionalProperties"]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return true // Default is true if not specified
}

// buildPath constructs a property path for error messages
func buildPath(base, prop string) string {
	if base == "" {
		return prop
	}
	// Handle array indices that already have brackets
	if strings.HasPrefix(prop, "[") {
		return base + prop
	}
	return base + "." + prop
}

// copyMap creates a deep copy of a map
func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any)
	for k, v := range m {
		result[k] = copyValue(v)
	}
	return result
}

// copyValue creates a deep copy of any value
func copyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return copyMap(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = copyValue(item)
		}
		return result
	default:
		return v
	}
}

// deduplicate removes duplicates from string slice and sorts
func deduplicate(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	// Sort for consistent output
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}
