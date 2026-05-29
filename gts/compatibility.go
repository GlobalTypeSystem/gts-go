/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

// CompatibilityResult represents the result of schema compatibility checking
type CompatibilityResult struct {
	FromID                 string              `json:"from"`
	ToID                   string              `json:"to"`
	OldID                  string              `json:"old"`
	NewID                  string              `json:"new"`
	Direction              string              `json:"direction"`
	AddedProperties        []string            `json:"added_properties"`
	RemovedProperties      []string            `json:"removed_properties"`
	ChangedProperties      []map[string]string `json:"changed_properties"`
	IsFullyCompatible      bool                `json:"is_fully_compatible"`
	IsBackwardCompatible   bool                `json:"is_backward_compatible"`
	IsForwardCompatible    bool                `json:"is_forward_compatible"`
	IncompatibilityReasons []string            `json:"incompatibility_reasons"`
	BackwardErrors         []string            `json:"backward_errors"`
	ForwardErrors          []string            `json:"forward_errors"`
	Error                  string              `json:"error,omitempty"`
}

// CheckCompatibility checks compatibility between two schemas
// see gts-python store.py is_minor_compatible method
func (s *GtsStore) CheckCompatibility(oldTypeID, newTypeID string) *CompatibilityResult {
	oldEntity := s.Get(oldTypeID)
	newEntity := s.Get(newTypeID)

	if oldEntity == nil || newEntity == nil {
		return &CompatibilityResult{
			FromID:                 oldTypeID,
			ToID:                   newTypeID,
			OldID:                  oldTypeID,
			NewID:                  newTypeID,
			Direction:              "unknown",
			AddedProperties:        []string{},
			RemovedProperties:      []string{},
			ChangedProperties:      []map[string]string{},
			IsFullyCompatible:      false,
			IsBackwardCompatible:   false,
			IsForwardCompatible:    false,
			IncompatibilityReasons: []string{},
			BackwardErrors:         []string{"Schema not found"},
			ForwardErrors:          []string{"Schema not found"},
		}
	}

	oldSchema, ok1 := oldEntity.Content, oldEntity.Content != nil
	newSchema, ok2 := newEntity.Content, newEntity.Content != nil
	if !ok1 || !ok2 {
		return &CompatibilityResult{
			FromID:                 oldTypeID,
			ToID:                   newTypeID,
			OldID:                  oldTypeID,
			NewID:                  newTypeID,
			Direction:              "unknown",
			AddedProperties:        []string{},
			RemovedProperties:      []string{},
			ChangedProperties:      []map[string]string{},
			IsFullyCompatible:      false,
			IsBackwardCompatible:   false,
			IsForwardCompatible:    false,
			IncompatibilityReasons: []string{},
			BackwardErrors:         []string{"Invalid schema content"},
			ForwardErrors:          []string{"Invalid schema content"},
		}
	}

	// Check compatibility
	isBackward, backwardErrors := checkBackwardCompatibility(oldSchema, newSchema)
	isForward, forwardErrors := checkForwardCompatibility(oldSchema, newSchema)

	// Determine direction
	direction := inferDirection(oldTypeID, newTypeID)

	return &CompatibilityResult{
		FromID:                 oldTypeID,
		ToID:                   newTypeID,
		OldID:                  oldTypeID,
		NewID:                  newTypeID,
		Direction:              direction,
		AddedProperties:        []string{},
		RemovedProperties:      []string{},
		ChangedProperties:      []map[string]string{},
		IsFullyCompatible:      isBackward && isForward,
		IsBackwardCompatible:   isBackward,
		IsForwardCompatible:    isForward,
		IncompatibilityReasons: []string{},
		BackwardErrors:         backwardErrors,
		ForwardErrors:          forwardErrors,
	}
}

// inferDirection determines if going up/down based on minor version
// see gts-python schema_cast.py _infer_direction method
func inferDirection(fromID, toID string) string {
	fromGtsID, err1 := NewGtsID(fromID)
	toGtsID, err2 := NewGtsID(toID)

	if err1 != nil || err2 != nil {
		return "unknown"
	}

	// Get last segment (the one with version info)
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

// flattenSchema merges allOf schemas into a single schema
// see gts-python schema_cast.py _flatten_schema method
func flattenSchema(schema map[string]any) map[string]any {
	result := map[string]any{
		"properties": make(map[string]any),
		"required":   []any{},
	}

	// Merge allOf schemas
	if allOfVal, ok := schema["allOf"]; ok {
		if allOfList, ok := allOfVal.([]any); ok {
			for _, subSchemaAny := range allOfList {
				if subSchema, ok := subSchemaAny.(map[string]any); ok {
					flattened := flattenSchema(subSchema)

					// Merge properties
					if props, ok := flattened["properties"].(map[string]any); ok {
						if resultProps, ok := result["properties"].(map[string]any); ok {
							for k, v := range props {
								resultProps[k] = v
							}
						}
					}

					// Merge required
					if req, ok := flattened["required"].([]any); ok {
						if resultReq, ok := result["required"].([]any); ok {
							result["required"] = append(resultReq, req...)
						}
					}

					// Preserve additionalProperties (last one wins)
					if addProps, ok := flattened["additionalProperties"]; ok {
						result["additionalProperties"] = addProps
					}
				}
			}
		}
	}

	// Add direct properties
	if props, ok := schema["properties"].(map[string]any); ok {
		if resultProps, ok := result["properties"].(map[string]any); ok {
			for k, v := range props {
				resultProps[k] = v
			}
		}
	}

	// Add direct required
	if req, ok := schema["required"].([]any); ok {
		if resultReq, ok := result["required"].([]any); ok {
			result["required"] = append(resultReq, req...)
		}
	}

	// Top level additionalProperties overrides
	if addProps, ok := schema["additionalProperties"]; ok {
		result["additionalProperties"] = addProps
	}

	return result
}

// checkBackwardCompatibility checks if new schema is backward compatible with old
// Backward compatibility: new consumers can read old data
// see gts-python schema_cast.py _check_backward_compatibility method
func checkBackwardCompatibility(oldSchema, newSchema map[string]any) (bool, []string) {
	return checkSchemaCompatibility(oldSchema, newSchema, true)
}

// checkForwardCompatibility checks if new schema is forward compatible with old
// Forward compatibility: old consumers can read new data
// see gts-python schema_cast.py _check_forward_compatibility method
func checkForwardCompatibility(oldSchema, newSchema map[string]any) (bool, []string) {
	return checkSchemaCompatibility(oldSchema, newSchema, false)
}

// checkSchemaCompatibility unified checker for backward and forward compatibility
// see gts-python schema_cast.py _check_schema_compatibility method
func checkSchemaCompatibility(oldSchema, newSchema map[string]any, checkBackward bool) (bool, []string) {
	errors := []string{}

	// Flatten schemas to handle allOf
	oldFlat := flattenSchema(oldSchema)
	newFlat := flattenSchema(newSchema)

	oldProps := getPropertiesMap(oldFlat)
	newProps := getPropertiesMap(newFlat)
	oldRequired := getRequiredSet(oldFlat)
	newRequired := getRequiredSet(newFlat)

	// Check required properties changes
	if checkBackward {
		// Backward: cannot add required properties
		newlyRequired := setDifference(newRequired, oldRequired)
		if len(newlyRequired) > 0 {
			errors = append(errors, "Added required properties: "+joinStrings(newlyRequired))
		}
	} else {
		// Forward: cannot remove required properties
		removedRequired := setDifference(oldRequired, newRequired)
		if len(removedRequired) > 0 {
			errors = append(errors, "Removed required properties: "+joinStrings(removedRequired))
		}
	}

	// Check properties that exist in both schemas
	commonProps := setIntersection(getKeys(oldProps), getKeys(newProps))
	for _, prop := range commonProps {
		oldPropSchema := oldProps[prop].(map[string]any)
		newPropSchema := newProps[prop].(map[string]any)

		// Check if type changed
		oldType := getString(oldPropSchema, "type")
		newType := getString(newPropSchema, "type")
		if oldType != "" && newType != "" && oldType != newType {
			errors = append(errors, "Property '"+prop+"' type changed from "+oldType+" to "+newType)
		}

		// Check enum constraints
		oldEnum := getStringSlice(oldPropSchema, "enum")
		newEnum := getStringSlice(newPropSchema, "enum")
		if len(oldEnum) > 0 && len(newEnum) > 0 {
			oldEnumSet := stringSliceToSet(oldEnum)
			newEnumSet := stringSliceToSet(newEnum)
			if checkBackward {
				// Backward: cannot add enum values
				addedEnumValues := setDifference(newEnumSet, oldEnumSet)
				if len(addedEnumValues) > 0 {
					errors = append(errors, "Property '"+prop+"' added enum values: "+joinStrings(addedEnumValues))
				}
			} else {
				// Forward: cannot remove enum values
				removedEnumValues := setDifference(oldEnumSet, newEnumSet)
				if len(removedEnumValues) > 0 {
					errors = append(errors, "Property '"+prop+"' removed enum values: "+joinStrings(removedEnumValues))
				}
			}
		}

		// Check constraint compatibility
		constraintErrors := checkConstraintCompatibility(prop, oldPropSchema, newPropSchema, checkBackward)
		errors = append(errors, constraintErrors...)

		// Recursively check nested object properties
		if oldType == "object" && newType == "object" {
			nestedCompat, nestedErrors := checkSchemaCompatibility(oldPropSchema, newPropSchema, checkBackward)
			if !nestedCompat {
				for _, err := range nestedErrors {
					errors = append(errors, "Property '"+prop+"': "+err)
				}
			}
		}

		// Recursively check array item schemas
		if oldType == "array" && newType == "array" {
			oldItems := getMap(oldPropSchema, "items")
			newItems := getMap(newPropSchema, "items")
			if oldItems != nil && newItems != nil {
				itemsCompat, itemsErrors := checkSchemaCompatibility(oldItems, newItems, checkBackward)
				if !itemsCompat {
					for _, err := range itemsErrors {
						errors = append(errors, "Property '"+prop+"' array items: "+err)
					}
				}
			}
		}
	}

	return len(errors) == 0, errors
}

// checkConstraintCompatibility checks if constraints are compatible
// see gts-python schema_cast.py _check_constraint_compatibility method
func checkConstraintCompatibility(prop string, oldPropSchema, newPropSchema map[string]any, checkTightening bool) []string {
	errors := []string{}
	propType := getString(oldPropSchema, "type")

	// Numeric constraints (for number/integer types)
	if propType == "number" || propType == "integer" {
		errors = append(errors, checkMinMaxConstraint(prop, oldPropSchema, newPropSchema, "minimum", "maximum", checkTightening)...)
	}

	// String constraints
	if propType == "string" {
		errors = append(errors, checkMinMaxConstraint(prop, oldPropSchema, newPropSchema, "minLength", "maxLength", checkTightening)...)
	}

	// Array constraints
	if propType == "array" {
		errors = append(errors, checkMinMaxConstraint(prop, oldPropSchema, newPropSchema, "minItems", "maxItems", checkTightening)...)
	}

	return errors
}

// checkMinMaxConstraint checks min/max constraint compatibility
// see gts-python schema_cast.py _check_min_max_constraint method
func checkMinMaxConstraint(prop string, oldSchema, newSchema map[string]any, minKey, maxKey string, checkTightening bool) []string {
	errors := []string{}

	oldMin := getNumber(oldSchema, minKey)
	newMin := getNumber(newSchema, minKey)
	oldMax := getNumber(oldSchema, maxKey)
	newMax := getNumber(newSchema, maxKey)

	// Check minimum constraint
	if checkTightening {
		// Backward: cannot increase minimum (tighten)
		if oldMin != nil && newMin != nil && *newMin > *oldMin {
			errors = append(errors, "Property '"+prop+"' "+minKey+" increased from "+floatToString(*oldMin)+" to "+floatToString(*newMin))
		} else if oldMin == nil && newMin != nil {
			errors = append(errors, "Property '"+prop+"' added "+minKey+" constraint: "+floatToString(*newMin))
		}
	} else {
		// Forward: cannot decrease minimum (relax)
		if oldMin != nil && newMin != nil && *newMin < *oldMin {
			errors = append(errors, "Property '"+prop+"' "+minKey+" decreased from "+floatToString(*oldMin)+" to "+floatToString(*newMin))
		} else if oldMin != nil && newMin == nil {
			errors = append(errors, "Property '"+prop+"' removed "+minKey+" constraint")
		}
	}

	// Check maximum constraint
	if checkTightening {
		// Backward: cannot decrease maximum (tighten)
		if oldMax != nil && newMax != nil && *newMax < *oldMax {
			errors = append(errors, "Property '"+prop+"' "+maxKey+" decreased from "+floatToString(*oldMax)+" to "+floatToString(*newMax))
		} else if oldMax == nil && newMax != nil {
			errors = append(errors, "Property '"+prop+"' added "+maxKey+" constraint: "+floatToString(*newMax))
		}
	} else {
		// Forward: cannot increase maximum (relax)
		if oldMax != nil && newMax != nil && *newMax > *oldMax {
			errors = append(errors, "Property '"+prop+"' "+maxKey+" increased from "+floatToString(*oldMax)+" to "+floatToString(*newMax))
		} else if oldMax != nil && newMax == nil {
			errors = append(errors, "Property '"+prop+"' removed "+maxKey+" constraint")
		}
	}

	return errors
}
