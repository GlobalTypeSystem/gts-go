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
	result := deepCopyMap(schema)
	delete(result, "allOf")

	if allOf, ok := schema["allOf"].([]any); ok {
		for _, value := range allOf {
			if subschema, ok := value.(map[string]any); ok {
				result = mergeFlattenedSchemas(result, flattenSchema(subschema))
			}
		}
	}
	return result
}

func mergeFlattenedSchemas(left, right map[string]any) map[string]any {
	result := deepCopyMap(left)
	for key, value := range right {
		switch key {
		case "properties":
			rightProperties, ok := value.(map[string]any)
			if !ok {
				continue
			}
			leftProperties, _ := result[key].(map[string]any)
			if leftProperties == nil {
				leftProperties = make(map[string]any)
			}
			for property, rightSchema := range rightProperties {
				if leftSchema, exists := leftProperties[property].(map[string]any); exists {
					if rightSchema, ok := rightSchema.(map[string]any); ok {
						leftProperties[property] = mergeFlattenedSchemas(leftSchema, rightSchema)
						continue
					}
				}
				leftProperties[property] = deepCopyValue(rightSchema)
			}
			result[key] = leftProperties
		case "required":
			existing, _ := result[key].([]any)
			for _, required := range value.([]any) {
				if !anySliceContains(existing, required) {
					existing = append(existing, required)
				}
			}
			result[key] = existing
		case "additionalProperties":
			if left, ok := result[key].(bool); ok && !left {
				continue
			}
			if right, ok := value.(bool); ok && !right {
				result[key] = false
			} else if _, exists := result[key]; !exists {
				result[key] = deepCopyValue(value)
			}
		case "items":
			if left, ok := result[key].(map[string]any); ok {
				if right, ok := value.(map[string]any); ok {
					result[key] = mergeFlattenedSchemas(left, right)
					continue
				}
			}
			if _, exists := result[key]; !exists {
				result[key] = deepCopyValue(value)
			}
		default:
			if _, exists := result[key]; !exists {
				result[key] = deepCopyValue(value)
			}
		}
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
		oldEnum, oldHasEnum := oldPropSchema["enum"].([]any)
		newEnum, newHasEnum := newPropSchema["enum"].([]any)
		if checkBackward && !oldHasEnum && newHasEnum {
			errors = append(errors, "Property '"+prop+"' added enum constraint")
		} else if !checkBackward && oldHasEnum && !newHasEnum {
			errors = append(errors, "Property '"+prop+"' removed enum constraint")
		} else if oldHasEnum && newHasEnum {
			if checkBackward {
				for _, value := range newEnum {
					if !anySliceContains(oldEnum, value) {
						errors = append(errors, "Property '"+prop+"' added enum values")
						break
					}
				}
			} else {
				for _, value := range oldEnum {
					if !anySliceContains(newEnum, value) {
						errors = append(errors, "Property '"+prop+"' removed enum values")
						break
					}
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
