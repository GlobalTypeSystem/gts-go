/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
)

// This file implements the GTS entity- and pattern-reference keywords
// (README §9.6, ADR-0007, ADR-0008):
//
//   - x-gts-type-ref     the candidate MUST be a GTS Type Identifier
//   - x-gts-instance-ref the candidate MUST be a GTS Instance Identifier
//   - x-gts-pattern-ref  the candidate MUST be a GTS identifier pattern whose
//                        selection set is covered by the operand
//
// The former kind-blind x-gts-ref keyword was removed in GTS 0.16; it is now
// handled like any other unknown x-gts-* keyword (rejected on validation).
//
// Operand (schema) validity and candidate (value) validity are two concerns:
// an invalid operand is a schema error (cannot be inverted by `not`); a value
// that does not satisfy the constraint is an ordinary validation failure that
// composition keywords may invert. Registry lookup applies only to the two
// entity-reference keywords; x-gts-pattern-ref never consults the registry.

// refKind is the identifier kind (or pattern) a reference keyword requires.
type refKind int

const (
	refKindType refKind = iota
	refKindInstance
	refKindPattern
)

func refKindOf(keyword string) (refKind, bool) {
	switch keyword {
	case KeyXGtsTypeRef:
		return refKindType, true
	case KeyXGtsInstanceRef:
		return refKindInstance, true
	case KeyXGtsPatternRef:
		return refKindPattern, true
	}
	return 0, false
}

// refKeywords are the three GTS reference keywords, in a stable order.
var refKeywords = []string{KeyXGtsTypeRef, KeyXGtsInstanceRef, KeyXGtsPatternRef}

func (k refKind) keyword() string {
	switch k {
	case refKindType:
		return KeyXGtsTypeRef
	case refKindInstance:
		return KeyXGtsInstanceRef
	default:
		return KeyXGtsPatternRef
	}
}

func (k refKind) noun() string {
	if k == refKindType {
		return "GTS Type Identifier"
	}
	return "GTS Instance Identifier"
}

func (k refKind) otherNoun() string {
	if k == refKindType {
		return "GTS Instance Identifier"
	}
	return "GTS Type Identifier"
}

// XGtsRefValidationError represents a validation error for a GTS reference
// constraint (x-gts-type-ref, x-gts-instance-ref or x-gts-pattern-ref).
type XGtsRefValidationError struct {
	Keyword    string
	FieldPath  string
	Value      any
	RefPattern string
	Reason     string
}

func (e *XGtsRefValidationError) Error() string {
	return fmt.Sprintf("GTS reference validation failed for field '%s': %s", e.FieldPath, e.Reason)
}

func invalidJSONRefErrors(path string, err error) []*XGtsRefValidationError {
	return []*XGtsRefValidationError{{
		FieldPath: path,
		Reason:    fmt.Sprintf("input must be valid JSON: %v", err),
	}}
}

// resolveRefOperand validates a keyword operand and returns the parsed pattern
// it denotes, together with the spelling actually used (with /$id resolved), or
// an error describing the schema defect. selectedTypeID is the leaf type that
// "/$id" names (bare, no gts://), or "" when there is none.
func resolveRefOperand(kind refKind, operand any, selectedTypeID string) (*gtsid.ID, string, error) {
	spelling, ok := operand.(string)
	if !ok {
		return nil, "", fmt.Errorf("%s operand must be a string, got %T", kind.keyword(), operand)
	}
	if IsXGtsRefSelf(spelling) {
		if selectedTypeID == "" {
			return nil, "", fmt.Errorf("%s operand '%s' has no selected GTS Type Schema '$id' to resolve to", kind.keyword(), XGtsRefSelf)
		}
		id, err := gtsid.New(selectedTypeID)
		if err != nil || !id.IsType() {
			return nil, "", fmt.Errorf("%s operand '%s' resolves to '%s', which is not a concrete GTS Type Identifier", kind.keyword(), XGtsRefSelf, selectedTypeID)
		}
		return id, selectedTypeID, nil
	}
	pattern, err := gtsid.ParsePattern(spelling)
	if err != nil {
		return nil, "", fmt.Errorf("%s operand '%s': %v; expected a GTS identifier, a wildcard pattern or '%s'", kind.keyword(), spelling, err, XGtsRefSelf)
	}
	if kind == refKindType && !gtsid.HasWildcard(spelling) && !gtsid.IsTypeID(spelling) {
		return nil, "", fmt.Errorf("%s operand '%s' is a GTS Instance Identifier, which selects no GTS Type Identifiers", kind.keyword(), spelling)
	}
	return pattern, spelling, nil
}

// refCandidateViolation checks a string candidate against a resolved operand and
// returns a human-readable reason when the structural constraint is violated, or
// "" when it is satisfied. It performs no registry lookup. For the entity
// reference keywords the candidate must be a concrete identifier of the required
// kind; for x-gts-pattern-ref it must be a pattern the operand covers.
func refCandidateViolation(kind refKind, value string, operand *gtsid.ID, spelling string) string {
	if kind == refKindPattern {
		valuePattern, err := gtsid.ParsePattern(value)
		if err != nil {
			return fmt.Sprintf("'%s' is not a valid GTS identifier pattern: %v", value, err)
		}
		if !gtsid.Covers(operand, valuePattern) {
			return fmt.Sprintf("'%s' selects GTS identifiers outside '%s'", value, spelling)
		}
		return ""
	}

	if gtsid.HasWildcard(value) {
		return fmt.Sprintf("Value '%s' is not a valid GTS identifier", value)
	}
	id, err := gtsid.New(value)
	if err != nil {
		return fmt.Sprintf("Value '%s' is not a valid GTS identifier", value)
	}
	if (kind == refKindType) != id.IsType() {
		return fmt.Sprintf("Value '%s' is a %s, but %s requires a %s", value, kind.otherNoun(), kind.keyword(), kind.noun())
	}
	if !gtsid.MatchesPattern(id, operand) {
		return fmt.Sprintf("Value '%s' does not match pattern '%s'", value, spelling)
	}
	return ""
}

// XGtsRefValidator validates GTS reference constraints in GTS schemas.
type XGtsRefValidator struct {
	store                      *GtsStore
	mode                       GtsRefValidationMode
	session                    string
	referencedIDs              map[string]struct{}
	referencedWildcardPatterns map[string]struct{}
}

func NewXGtsRefValidator(store *GtsStore, modes ...GtsRefValidationMode) *XGtsRefValidator {
	mode := GtsRefValidationAnyValid
	if len(modes) > 0 {
		mode = modes[0]
	}
	return &XGtsRefValidator{
		store:                      store,
		mode:                       mode,
		referencedIDs:              make(map[string]struct{}),
		referencedWildcardPatterns: make(map[string]struct{}),
	}
}

// NewXGtsRefValidatorScoped is like NewXGtsRefValidator but resolves referenced
// entities against the given staging session's overlay (plus the committed
// store), so a schema validated as part of a batch sees its own staged siblings
// - and only its own. An empty session is committed-only.
func NewXGtsRefValidatorScoped(store *GtsStore, mode GtsRefValidationMode, session string) *XGtsRefValidator {
	v := NewXGtsRefValidator(store, mode)
	v.session = session
	return v
}

func (v *XGtsRefValidator) ReferencedIDs() []string {
	ids := make([]string, 0, len(v.referencedIDs))
	for id := range v.referencedIDs {
		ids = append(ids, id)
	}
	return ids
}

func (v *XGtsRefValidator) ReferencedWildcardPatterns() []string {
	patterns := make([]string, 0, len(v.referencedWildcardPatterns))
	for pattern := range v.referencedWildcardPatterns {
		patterns = append(patterns, pattern)
	}
	return patterns
}

// selectedTypeIDFor returns the leaf type "/$id" names for the given schema.
func selectedTypeIDFor(schema map[string]any, override []string) string {
	if len(override) > 0 {
		return gtsid.NormalizeID(override[0])
	}
	if schemaID, ok := schema["$id"].(string); ok {
		return gtsid.NormalizeID(schemaID)
	}
	return ""
}

// ValidateInstance validates an instance against the reference constraints in
// schema.
func (v *XGtsRefValidator) ValidateInstance(instance map[string]any, schema map[string]any, instancePath string, selectedTypeIDs ...string) []*XGtsRefValidationError {
	if err := validateJSONContent(instance); err != nil {
		return invalidJSONRefErrors(instancePath, err)
	}
	if err := validateJSONContent(schema); err != nil {
		return invalidJSONRefErrors(instancePath, err)
	}
	selectedTypeID := selectedTypeIDFor(schema, selectedTypeIDs)
	var errors []*XGtsRefValidationError
	v.visitInstance(instance, schema, instancePath, selectedTypeID, &errors)
	return errors
}

// ValidateSchema validates the reference-keyword operands in a schema
// definition (schema-level validity, ADR-0007/0008).
func (v *XGtsRefValidator) ValidateSchema(schema map[string]any, schemaPath string, selectedTypeIDs ...string) []*XGtsRefValidationError {
	if err := validateJSONContent(schema); err != nil {
		return invalidJSONRefErrors(schemaPath, err)
	}
	selectedTypeID := selectedTypeIDFor(schema, selectedTypeIDs)
	var errors []*XGtsRefValidationError
	v.visitSchema(schema, schemaPath, selectedTypeID, &errors)
	return errors
}

// visitInstance recursively visits instance nodes and validates reference
// constraints found in the corresponding schema node.
func (v *XGtsRefValidator) visitInstance(instance any, schema map[string]any, path, selectedTypeID string, errors *[]*XGtsRefValidationError) {
	if schema == nil {
		return
	}

	for _, keyword := range refKeywords {
		operand, has := schema[keyword]
		if !has {
			continue
		}
		strInstance, ok := instance.(string)
		if !ok {
			continue
		}
		if err := v.validateRefValue(keyword, strInstance, operand, path, selectedTypeID); err != nil {
			*errors = append(*errors, err)
		}
	}

	// Recurse into object properties
	if schemaType, ok := schema["type"].(string); ok && schemaType == "object" {
		if properties, hasProps := schema["properties"].(map[string]any); hasProps {
			if instanceMap, ok := instance.(map[string]any); ok {
				for propName, propSchema := range properties {
					if propValue, hasProp := instanceMap[propName]; hasProp {
						propPath := propName
						if path != "" {
							propPath = path + "." + propName
						}
						if propSchemaMap, ok := propSchema.(map[string]any); ok {
							v.visitInstance(propValue, propSchemaMap, propPath, selectedTypeID, errors)
						}
					}
				}
			}
		}
	}

	// Recurse into array items
	if schemaType, ok := schema["type"].(string); ok && schemaType == "array" {
		if instanceArray, ok := instance.([]any); ok {
			if prefixItems, ok := schema["prefixItems"].([]any); ok {
				for idx, itemSchema := range prefixItems {
					if idx >= len(instanceArray) {
						break
					}
					if schemaMap, ok := itemSchema.(map[string]any); ok {
						itemPath := fmt.Sprintf("%s[%d]", path, idx)
						v.visitInstance(instanceArray[idx], schemaMap, itemPath, selectedTypeID, errors)
					}
				}
				if items, ok := schema["items"].(map[string]any); ok {
					for idx := len(prefixItems); idx < len(instanceArray); idx++ {
						itemPath := fmt.Sprintf("%s[%d]", path, idx)
						v.visitInstance(instanceArray[idx], items, itemPath, selectedTypeID, errors)
					}
				}
				return
			}
			switch items := schema["items"].(type) {
			case map[string]any:
				for idx, item := range instanceArray {
					itemPath := fmt.Sprintf("%s[%d]", path, idx)
					v.visitInstance(item, items, itemPath, selectedTypeID, errors)
				}
			case []any:
				for idx, itemSchema := range items {
					if idx >= len(instanceArray) {
						break
					}
					if schemaMap, ok := itemSchema.(map[string]any); ok {
						itemPath := fmt.Sprintf("%s[%d]", path, idx)
						v.visitInstance(instanceArray[idx], schemaMap, itemPath, selectedTypeID, errors)
					}
				}
				if additionalItems, ok := schema["additionalItems"].(map[string]any); ok {
					for idx := len(items); idx < len(instanceArray); idx++ {
						itemPath := fmt.Sprintf("%s[%d]", path, idx)
						v.visitInstance(instanceArray[idx], additionalItems, itemPath, selectedTypeID, errors)
					}
				}
			}
		}
	}
}

func visitSchemaChildren(schema map[string]any, path string, visit func(map[string]any, string)) {
	for key, value := range schema {
		nestedPath := key
		if path != "" {
			nestedPath = path + "/" + key
		}
		switch key {
		case "properties", "patternProperties", "definitions", "$defs", "dependentSchemas", "dependencies":
			if named, ok := value.(map[string]any); ok {
				for name, child := range named {
					if childSchema, ok := child.(map[string]any); ok {
						visit(childSchema, nestedPath+"/"+name)
					}
				}
			}
		case "allOf", "anyOf", "oneOf", "prefixItems":
			if children, ok := value.([]any); ok {
				for index, child := range children {
					if childSchema, ok := child.(map[string]any); ok {
						visit(childSchema, fmt.Sprintf("%s[%d]", nestedPath, index))
					}
				}
			}
		case "items":
			switch children := value.(type) {
			case map[string]any:
				visit(children, nestedPath)
			case []any:
				for index, child := range children {
					if childSchema, ok := child.(map[string]any); ok {
						visit(childSchema, fmt.Sprintf("%s[%d]", nestedPath, index))
					}
				}
			}
		case "not", "if", "then", "else", "contains", "additionalProperties", "additionalItems", "propertyNames", "unevaluatedProperties", "unevaluatedItems", KeyXGtsTraitsSchema:
			if childSchema, ok := value.(map[string]any); ok {
				visit(childSchema, nestedPath)
			}
		}
	}
}

// visitSchema recursively visits schema nodes, validating reference operands.
func (v *XGtsRefValidator) visitSchema(schema map[string]any, path, selectedTypeID string, errors *[]*XGtsRefValidationError) {
	if schema == nil {
		return
	}

	for _, keyword := range refKeywords {
		operand, has := schema[keyword]
		if !has {
			continue
		}
		refPath := keyword
		if path != "" {
			refPath = path + "/" + keyword
		}
		if err := v.validateRefOperand(keyword, operand, refPath, selectedTypeID); err != nil {
			*errors = append(*errors, err)
		}
	}

	visitSchemaChildren(schema, path, func(child map[string]any, childPath string) {
		v.visitSchema(child, childPath, selectedTypeID, errors)
	})
}

// validateRefValue validates an instance value against a reference constraint.
func (v *XGtsRefValidator) validateRefValue(keyword string, value string, operand any, fieldPath, selectedTypeID string) *XGtsRefValidationError {
	kind, _ := refKindOf(keyword)
	operandID, spelling, err := resolveRefOperand(kind, operand, selectedTypeID)
	if err != nil {
		return &XGtsRefValidationError{Keyword: keyword, FieldPath: fieldPath, Value: value, RefPattern: fmt.Sprintf("%v", operand), Reason: err.Error()}
	}
	if reason := refCandidateViolation(kind, value, operandID, spelling); reason != "" {
		return &XGtsRefValidationError{Keyword: keyword, FieldPath: fieldPath, Value: value, RefPattern: spelling, Reason: reason}
	}
	// Entity references may additionally require the candidate to be registered
	// (and, under any-valid, itself valid). Pattern references never consult the
	// registry.
	if kind != refKindPattern && v.store != nil && v.mode != GtsRefValidationNone {
		if v.store.getScoped(value, v.session) == nil {
			return &XGtsRefValidationError{Keyword: keyword, FieldPath: fieldPath, Value: value, RefPattern: spelling, Reason: fmt.Sprintf("Referenced entity '%s' not found in registry", value)}
		}
		v.referencedIDs[value] = struct{}{}
	}
	return nil
}

// validateRefOperand validates a reference-keyword operand in a schema.
func (v *XGtsRefValidator) validateRefOperand(keyword string, operand any, fieldPath, selectedTypeID string) *XGtsRefValidationError {
	kind, _ := refKindOf(keyword)
	if _, _, err := resolveRefOperand(kind, operand, selectedTypeID); err != nil {
		return &XGtsRefValidationError{Keyword: keyword, FieldPath: fieldPath, Value: operand, RefPattern: fmt.Sprintf("%v", operand), Reason: err.Error()}
	}
	return nil
}

// ValidateSchemaRefExistence checks that each entity-reference operand resolves
// to a registered (and, under any-valid, valid) entity when the mode requires
// it. x-gts-pattern-ref operands are never looked up (ADR-0007).
func (v *XGtsRefValidator) ValidateSchemaRefExistence(schema map[string]any, schemaPath string, selectedTypeIDs ...string) []*XGtsRefValidationError {
	if err := validateJSONContent(schema); err != nil {
		return invalidJSONRefErrors(schemaPath, err)
	}
	var errors []*XGtsRefValidationError
	if v.store == nil || v.mode == GtsRefValidationNone {
		return errors
	}
	selectedTypeID := selectedTypeIDFor(schema, selectedTypeIDs)
	v.visitSchemaRefExistence(schema, schemaPath, selectedTypeID, &errors)
	return errors
}

// visitSchemaRefExistence recursively checks concrete entity-reference operands.
func (v *XGtsRefValidator) visitSchemaRefExistence(schema map[string]any, path, selectedTypeID string, errors *[]*XGtsRefValidationError) {
	if schema == nil {
		return
	}

	for _, keyword := range []string{KeyXGtsTypeRef, KeyXGtsInstanceRef} {
		operand, has := schema[keyword]
		if !has {
			continue
		}
		operandStr, ok := operand.(string)
		if !ok {
			continue
		}
		targetID := operandStr
		if IsXGtsRefSelf(operandStr) {
			targetID = selectedTypeID
		}
		if !gtsid.HasPrefix(targetID) {
			continue
		}
		refPath := keyword
		if path != "" {
			refPath = path + "/" + keyword
		}
		if gtsid.HasWildcard(targetID) {
			// Wildcard operand lookup is kind-agnostic: a registered entity of
			// either kind satisfies it (ADR-0008 "regardless of kind"). The
			// candidate's kind is enforced separately, when the field value is
			// validated. A `~*` pattern therefore counts its type root as a
			// match for x-gts-instance-ref operand presence too.
			matched := false
			if pattern, err := gtsid.ParsePattern(targetID); err == nil {
				v.store.forEachEntity(func(entityID string, entity *JsonEntity) bool {
					id := entity.GtsID
					if id == nil {
						var err error
						if id, err = gtsid.New(entityID); err != nil {
							return true
						}
					}
					if gtsid.MatchesPattern(id, pattern) {
						matched = true
						return false
					}
					return true
				})
			}
			if !matched {
				*errors = append(*errors, &XGtsRefValidationError{
					Keyword: keyword, FieldPath: refPath, Value: operandStr, RefPattern: targetID,
					Reason: fmt.Sprintf("%s wildcard constraint '%s' has no registered match", keyword, targetID),
				})
			} else {
				v.referencedWildcardPatterns[targetID] = struct{}{}
			}
		} else if v.store.getScoped(targetID, v.session) == nil {
			*errors = append(*errors, &XGtsRefValidationError{
				Keyword: keyword, FieldPath: refPath, Value: operandStr, RefPattern: targetID,
				Reason: fmt.Sprintf("%s constraint '%s' is not registered", keyword, targetID),
			})
		} else {
			v.referencedIDs[targetID] = struct{}{}
		}
	}

	visitSchemaChildren(schema, path, func(child map[string]any, childPath string) {
		v.visitSchemaRefExistence(child, childPath, selectedTypeID, errors)
	})
}
