/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

// Structural-diff compatibility helpers used by OP#9 (Cast).
//
// These functions perform a property-by-property structural comparison,
// which is appropriate for OP#9's instance-transformation use-case but
// NOT for OP#8's accepted-instance-set inclusion semantics (see
// compatibility.go for that).

// flattenSchema merges allOf schemas into a single schema.
func flattenSchema(schema map[string]any) map[string]any {
	result := map[string]any{
		"properties": make(map[string]any),
		"required":   []any{},
	}

	if allOfVal, ok := schema["allOf"]; ok {
		if allOfList, ok := allOfVal.([]any); ok {
			for _, subSchemaAny := range allOfList {
				if subSchema, ok := subSchemaAny.(map[string]any); ok {
					flattened := flattenSchema(subSchema)
					if props, ok := flattened["properties"].(map[string]any); ok {
						if resultProps, ok := result["properties"].(map[string]any); ok {
							for k, v := range props {
								resultProps[k] = v
							}
						}
					}
					if req, ok := flattened["required"].([]any); ok {
						if resultReq, ok := result["required"].([]any); ok {
							result["required"] = append(resultReq, req...)
						}
					}
					if addProps, ok := flattened["additionalProperties"]; ok {
						result["additionalProperties"] = addProps
					}
				}
			}
		}
	}

	if props, ok := schema["properties"].(map[string]any); ok {
		if resultProps, ok := result["properties"].(map[string]any); ok {
			for k, v := range props {
				resultProps[k] = v
			}
		}
	}
	if req, ok := schema["required"].([]any); ok {
		if resultReq, ok := result["required"].([]any); ok {
			result["required"] = append(resultReq, req...)
		}
	}
	if addProps, ok := schema["additionalProperties"]; ok {
		result["additionalProperties"] = addProps
	}

	return result
}

// checkBackwardCompatibility checks if new schema is backward compatible with old (structural diff).
func checkBackwardCompatibility(oldSchema, newSchema map[string]any) (bool, []string) {
	return checkStructuralCompatibility(oldSchema, newSchema, true)
}

// checkForwardCompatibility checks if new schema is forward compatible with old (structural diff).
func checkForwardCompatibility(oldSchema, newSchema map[string]any) (bool, []string) {
	return checkStructuralCompatibility(oldSchema, newSchema, false)
}

// checkStructuralCompatibility is the unified structural-diff checker for backward and forward.
func checkStructuralCompatibility(oldSchema, newSchema map[string]any, checkBackward bool) (bool, []string) {
	errors := []string{}

	oldFlat := flattenSchema(oldSchema)
	newFlat := flattenSchema(newSchema)

	oldProps := getPropertiesMap(oldFlat)
	newProps := getPropertiesMap(newFlat)
	oldRequired := getRequiredSet(oldFlat)
	newRequired := getRequiredSet(newFlat)

	if checkBackward {
		newlyRequired := setDifference(newRequired, oldRequired)
		if len(newlyRequired) > 0 {
			errors = append(errors, "Added required properties: "+joinStrings(newlyRequired))
		}
	} else {
		removedRequired := setDifference(oldRequired, newRequired)
		if len(removedRequired) > 0 {
			errors = append(errors, "Removed required properties: "+joinStrings(removedRequired))
		}
	}

	commonProps := setIntersection(getKeys(oldProps), getKeys(newProps))
	for _, prop := range commonProps {
		oldPropSchema, ok1 := oldProps[prop].(map[string]any)
		newPropSchema, ok2 := newProps[prop].(map[string]any)
		if !ok1 || !ok2 {
			continue
		}
		oldType := getString(oldPropSchema, "type")
		newType := getString(newPropSchema, "type")
		if oldType != "" && newType != "" && oldType != newType {
			errors = append(errors, "Property '"+prop+"' type changed from "+oldType+" to "+newType)
		}
		oldEnum := getStringSlice(oldPropSchema, "enum")
		newEnum := getStringSlice(newPropSchema, "enum")
		if len(oldEnum) > 0 && len(newEnum) > 0 {
			oldEnumSet := stringSliceToSet(oldEnum)
			newEnumSet := stringSliceToSet(newEnum)
			if checkBackward {
				addedEnumValues := setDifference(newEnumSet, oldEnumSet)
				if len(addedEnumValues) > 0 {
					errors = append(errors, "Property '"+prop+"' added enum values: "+joinStrings(addedEnumValues))
				}
			} else {
				removedEnumValues := setDifference(oldEnumSet, newEnumSet)
				if len(removedEnumValues) > 0 {
					errors = append(errors, "Property '"+prop+"' removed enum values: "+joinStrings(removedEnumValues))
				}
			}
		}
		constraintErrors := checkConstraintCompatibility(prop, oldPropSchema, newPropSchema, checkBackward)
		errors = append(errors, constraintErrors...)
		if oldType == "object" && newType == "object" {
			nestedCompat, nestedErrors := checkStructuralCompatibility(oldPropSchema, newPropSchema, checkBackward)
			if !nestedCompat {
				for _, err := range nestedErrors {
					errors = append(errors, "Property '"+prop+"': "+err)
				}
			}
		}
		if oldType == "array" && newType == "array" {
			oldItems := getMap(oldPropSchema, "items")
			newItems := getMap(newPropSchema, "items")
			if oldItems != nil && newItems != nil {
				itemsCompat, itemsErrors := checkStructuralCompatibility(oldItems, newItems, checkBackward)
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

// checkConstraintCompatibility checks if constraints are compatible.
func checkConstraintCompatibility(prop string, oldPropSchema, newPropSchema map[string]any, checkTightening bool) []string {
	errors := []string{}
	propType := getString(oldPropSchema, "type")

	if propType == "number" || propType == "integer" {
		errors = append(errors, checkMinMaxConstraint(prop, oldPropSchema, newPropSchema, "minimum", "maximum", checkTightening)...)
	}
	if propType == "string" {
		errors = append(errors, checkMinMaxConstraint(prop, oldPropSchema, newPropSchema, "minLength", "maxLength", checkTightening)...)
	}
	if propType == "array" {
		errors = append(errors, checkMinMaxConstraint(prop, oldPropSchema, newPropSchema, "minItems", "maxItems", checkTightening)...)
	}
	return errors
}

// checkMinMaxConstraint checks min/max constraint compatibility.
func checkMinMaxConstraint(prop string, oldSchema, newSchema map[string]any, minKey, maxKey string, checkTightening bool) []string {
	errors := []string{}

	oldMin := getNumber(oldSchema, minKey)
	newMin := getNumber(newSchema, minKey)
	oldMax := getNumber(oldSchema, maxKey)
	newMax := getNumber(newSchema, maxKey)

	if checkTightening {
		if oldMin != nil && newMin != nil && *newMin > *oldMin {
			errors = append(errors, "Property '"+prop+"' "+minKey+" increased from "+floatToString(*oldMin)+" to "+floatToString(*newMin))
		} else if oldMin == nil && newMin != nil {
			errors = append(errors, "Property '"+prop+"' added "+minKey+" constraint: "+floatToString(*newMin))
		}
	} else {
		if oldMin != nil && newMin != nil && *newMin < *oldMin {
			errors = append(errors, "Property '"+prop+"' "+minKey+" decreased from "+floatToString(*oldMin)+" to "+floatToString(*newMin))
		} else if oldMin != nil && newMin == nil {
			errors = append(errors, "Property '"+prop+"' removed "+minKey+" constraint")
		}
	}

	if checkTightening {
		if oldMax != nil && newMax != nil && *newMax < *oldMax {
			errors = append(errors, "Property '"+prop+"' "+maxKey+" decreased from "+floatToString(*oldMax)+" to "+floatToString(*newMax))
		} else if oldMax == nil && newMax != nil {
			errors = append(errors, "Property '"+prop+"' added "+maxKey+" constraint: "+floatToString(*newMax))
		}
	} else {
		if oldMax != nil && newMax != nil && *newMax > *oldMax {
			errors = append(errors, "Property '"+prop+"' "+maxKey+" increased from "+floatToString(*oldMax)+" to "+floatToString(*newMax))
		} else if oldMax != nil && newMax == nil {
			errors = append(errors, "Property '"+prop+"' removed "+maxKey+" constraint")
		}
	}

	return errors
}
