/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/message"
)

// regexp2RE adapts dlclark/regexp2 (PCRE-compatible) to the jsonschema.Regexp interface.
type regexp2RE struct {
	re *regexp2.Regexp
}

func (r *regexp2RE) MatchString(s string) bool {
	ok, err := r.re.MatchString(s)
	return err == nil && ok
}

func (r *regexp2RE) String() string {
	return r.re.String()
}

// ecmaRegexpEngine compiles patterns using ECMA-262 compatible regexp2.
// JSON Schema specifies ECMA-262 regex, which supports lookaheads ((?!, (?=)
// that Go's stdlib regexp (RE2) does not.
func ecmaRegexpEngine(s string) (jsonschema.Regexp, error) {
	re, err := regexp2.Compile(s, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = time.Second
	return &regexp2RE{re}, nil
}

// gtsURLLoader implements jsonschema.URLLoader for GTS ID reference resolution
type gtsURLLoader struct {
	store *GtsStore
}

// Load resolves GTS ID references to their schema content
// This matches Python's resolve_gts_ref handler
func (l *gtsURLLoader) Load(url string) (any, error) {
	// Strip the gts:// URI prefix if present (JSON Schema $id may have it)
	normalizedURL := strings.TrimPrefix(url, GtsURIPrefix)

	// Check if this is a GTS ID reference
	if IsValidGtsID(normalizedURL) {
		entity := l.store.Get(normalizedURL)
		if entity == nil {
			return nil, fmt.Errorf("unresolvable GTS reference: %s", url)
		}
		if !entity.IsTypeSchema {
			return nil, fmt.Errorf("GTS reference is not a type-schema: %s", url)
		}
		return entity.Content, nil
	}
	// For non-GTS URLs, return error to let default handling occur
	return nil, fmt.Errorf("unsupported URL: %s", url)
}

// ValidationResult represents the result of validating an instance
type ValidationResult struct {
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// ValidateInstance validates an object instance against its schema.
// Accepts either a well-known GTS instance ID or an anonymous instance id
// (e.g. a UUID paired with a separate "type" field on the stored entity,
// spec §3.7). Returns ValidationResult with ok=true if validation succeeds.
func (s *GtsStore) ValidateInstance(instanceID string) *ValidationResult {
	// Well-known GTS id first; fall back to a raw store lookup by the
	// passed string so anonymous instances (keyed by UUID) resolve too.
	lookupID := instanceID
	if IsValidGtsID(instanceID) {
		gid, err := NewGtsID(instanceID)
		if err != nil {
			return &ValidationResult{
				ID:    instanceID,
				OK:    false,
				Error: fmt.Sprintf("Invalid GTS ID: %v", err),
			}
		}
		lookupID = gid.ID
	}

	// Get the instance from store
	obj := s.Get(lookupID)
	if obj == nil {
		return &ValidationResult{
			ID:    instanceID,
			OK:    false,
			Error: (&StoreGtsObjectNotFoundError{EntityID: instanceID}).Error(),
		}
	}

	// Schema-only keywords must not appear in instance content.
	if err := ValidateInstanceModifiers(obj.Content); err != nil {
		return &ValidationResult{
			ID:    instanceID,
			OK:    false,
			Error: err.Error(),
		}
	}

	// Check if instance has a type ID
	if obj.TypeID == "" {
		return &ValidationResult{
			ID:    instanceID,
			OK:    false,
			Error: (&StoreGtsSchemaForInstanceNotFoundError{EntityID: lookupID}).Error(),
		}
	}

	// Get the type-schema from store
	schemaEntity := s.Get(obj.TypeID)
	if schemaEntity == nil {
		return &ValidationResult{
			ID:    instanceID,
			OK:    false,
			Error: (&StoreGtsSchemaNotFoundError{EntityID: obj.TypeID}).Error(),
		}
	}

	if !schemaEntity.IsTypeSchema {
		return &ValidationResult{
			ID:    instanceID,
			OK:    false,
			Error: fmt.Sprintf("entity '%s' is not a type-schema", obj.TypeID),
		}
	}

	// Check x-gts-abstract: abstract types cannot have direct instances.
	if isAbstract, ok := schemaEntity.Content[KeyXGtsAbstract]; ok {
		if abstract, isBool := isAbstract.(bool); isBool && abstract {
			return &ValidationResult{
				ID:    instanceID,
				OK:    false,
				Error: fmt.Sprintf("type '%s' is abstract and cannot have direct instances", obj.TypeID),
			}
		}
	}

	// Validate the instance against the schema
	if err := s.validateWithSchema(obj.Content, schemaEntity.Content); err != nil {
		return &ValidationResult{
			ID:    instanceID,
			OK:    false,
			Error: err.Error(),
		}
	}

	// Validate x-gts-ref constraints via XGtsRefValidator (separate pass with full
	// instance path context for JSON pointer resolution and prefix/self-ref semantics)
	xGtsRefValidator := NewXGtsRefValidator(s)
	xGtsRefErrors := xGtsRefValidator.ValidateInstance(obj.Content, schemaEntity.Content, "")
	if len(xGtsRefErrors) > 0 {
		var errorMsgs []string
		for _, e := range xGtsRefErrors {
			errorMsgs = append(errorMsgs, e.Error())
		}
		return &ValidationResult{
			ID:    instanceID,
			OK:    false,
			Error: fmt.Sprintf("x-gts-ref validation failed: %s", strings.Join(errorMsgs, "; ")),
		}
	}

	return &ValidationResult{
		ID:    instanceID,
		OK:    true,
		Error: "",
	}
}

// xGtsRefExt is the compiled form of an x-gts-ref keyword for a single schema node.
// Validate enforces the GTS pattern constraint so that oneOf/anyOf/allOf branches
// correctly pass or fail based on whether the value matches the pattern.
// The separate XGtsRefValidator pass handles JSON pointer resolution and other
// schema-level semantics that require full instance path context.
type xGtsRefExt struct {
	pattern    string
	rootSchema map[string]any
	store      *GtsStore
}

func (e *xGtsRefExt) Validate(ctx *jsonschema.ValidatorContext, v any) {
	str, ok := v.(string)
	if !ok {
		return
	}
	// Relative pointer patterns (starting with "/") require the full root schema
	// context for resolution — defer those entirely to XGtsRefValidator's separate pass.
	if strings.HasPrefix(e.pattern, "/") {
		return
	}
	validator := NewXGtsRefValidator(e.store)
	if err := validator.validateRefValue(str, e.pattern, "", e.rootSchema); err != nil {
		ctx.AddError(&xGtsRefErrorKind{err.Reason})
	}
}

// xGtsRefErrorKind implements jsonschema.ErrorKind for x-gts-ref validation errors.
type xGtsRefErrorKind struct{ reason string }

func (k *xGtsRefErrorKind) KeywordPath() []string                     { return []string{"x-gts-ref"} }
func (k *xGtsRefErrorKind) LocalizedString(_ *message.Printer) string { return k.reason }

// newXGtsRefVocabulary registers x-gts-ref as a proper vocabulary with the JSON schema
// compiler. This is the correct fix for the oneOf/anyOf/allOf problem: branches like
// {"x-gts-ref": "gts.x.foo~"} are no longer empty match-all schemas — they carry a
// real constraint that the library evaluates during combinator resolution.
func newXGtsRefVocabulary(store *GtsStore) *jsonschema.Vocabulary {
	return &jsonschema.Vocabulary{
		URL: "https://globaltypesystem.io/vocab/x-gts-ref",
		Compile: func(_ *jsonschema.CompilerContext, obj map[string]any) (jsonschema.SchemaExt, error) {
			raw, ok := obj["x-gts-ref"]
			if !ok {
				return nil, nil
			}
			pattern, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("x-gts-ref must be a string")
			}
			return &xGtsRefExt{pattern: pattern, rootSchema: obj, store: store}, nil
		},
	}
}

// normalizeSchemaForCompile returns a shallow copy of a schema with the gts://
// URI prefix stripped from $id. This ensures the embedded $id agrees with the
// (already normalized) resource URL used by the JSON Schema compiler, so
// relative $ref values resolve correctly.
func normalizeSchemaForCompile(schema map[string]any) map[string]any {
	normalized := make(map[string]any, len(schema))
	for k, v := range schema {
		normalized[k] = v
	}
	if id, ok := normalized["$id"].(string); ok {
		normalized["$id"] = strings.TrimPrefix(id, GtsURIPrefix)
	}
	return normalized
}

type JSONValidationResult struct {
	OK           bool   `json:"ok"`
	IsTypeSchema bool   `json:"is_type_schema"`
	TypeID       string `json:"type_id,omitempty"`
	Error        string `json:"error,omitempty"`
}

func (s *GtsStore) validateJSON(instance, schema map[string]any) error {
	return s.validateWithSchema(instance, schema)
}

func (s *GtsStore) validateJSONSchema(schema map[string]any) error {
	normalizedSchema := normalizeSchemaForCompile(schema)
	schemaID, ok := normalizedSchema["$id"].(string)
	if !ok || schemaID == "" {
		schemaID = "gts.validation.schema"
		normalizedSchema["$id"] = schemaID
	}
	schemaID = strings.TrimPrefix(schemaID, GtsURIPrefix)
	normalizedSchema["$id"] = schemaID

	compiler := jsonschema.NewCompiler()
	compiler.UseRegexpEngine(ecmaRegexpEngine)
	compiler.UseLoader(&gtsURLLoader{store: s})
	if err := compiler.AddResource(schemaID, normalizedSchema); err != nil {
		return fmt.Errorf("JSON Schema validation failed: %v", err)
	}
	for id, entity := range s.byID {
		if entity.IsTypeSchema && id != schemaID {
			_ = compiler.AddResource(id, normalizeSchemaForCompile(entity.Content))
		}
	}
	if _, err := compiler.Compile(schemaID); err != nil {
		return fmt.Errorf("JSON Schema validation failed: %v", err)
	}
	return nil
}

func (s *GtsStore) ValidateTransientJSON(content map[string]any, typeID string) *JSONValidationResult {
	entity := NewJsonEntity(content, DefaultGtsConfig())
	result := &JSONValidationResult{IsTypeSchema: entity.IsTypeSchema}
	fail := func(message string) *JSONValidationResult {
		if strings.Contains(message, "got number, want string") {
			message += ": is not of type 'string'"
		}
		result.Error = message
		return result
	}
	if typeID != "" {
		if entity.IsTypeSchema {
			return fail("validate-json with an explicit type only accepts instance JSON")
		}
		if !strings.HasSuffix(typeID, "~") {
			if strings.HasPrefix(typeID, GtsPrefix) {
				return fail("explicit type must be GTS Type schema")
			}
			return fail("Invalid GTS Type Schema ID")
		}
		if !IsValidGtsID(typeID) {
			return fail("Invalid GTS Type Schema ID")
		}
		if entity.TypeID != "" && entity.TypeID != typeID {
			return fail("instance type does not match path type")
		}
		entity.TypeID = typeID
	}
	if entity.IsTypeSchema {
		if err := s.validateJSONSchema(content); err != nil {
			return fail(err.Error())
		}
		if entity.GtsID == nil {
			return fail("Unable to detect GTS ID in schema")
		}
		s.mu.Lock()
		previous, existed := s.byID[entity.GtsID.ID]
		s.byID[entity.GtsID.ID] = entity
		validation := s.ValidateSchemaChain(entity.GtsID.ID)
		if existed {
			s.byID[entity.GtsID.ID] = previous
		} else {
			delete(s.byID, entity.GtsID.ID)
		}
		s.mu.Unlock()
		if !validation.OK {
			if strings.Contains(validation.Error, "has schema") && strings.Contains(validation.Error, "not found") {
				return fail("Parent GTS Type Schema not found")
			}
			return fail(validation.Error)
		}
		result.OK = true
		return result
	}
	if entity.TypeID == "" {
		return fail("Unable to determine instance type")
	}
	schema := s.Get(entity.TypeID)
	if schema == nil {
		return fail("GTS Type Schema not found")
	}
	if !schema.IsTypeSchema {
		return fail("explicit type must be GTS Type schema")
	}
	schemaContent := normalizeSchemaForCompile(schema.Content)
	if _, ok := schemaContent["$id"]; !ok {
		schemaContent["$id"] = entity.TypeID
	}
	if err := s.validateJSON(content, schemaContent); err != nil {
		return fail(err.Error())
	}
	result.OK = true
	result.TypeID = entity.TypeID
	return result
}

// validateWithSchema performs the actual JSON Schema validation
func (s *GtsStore) validateWithSchema(instance map[string]any, schema map[string]any) error {
	// Normalize schema by stripping the gts:// prefix from $id for JSON Schema validation
	normalizedSchema := normalizeSchemaForCompile(schema)

	// Create a custom compiler with GTS reference resolution
	compiler := jsonschema.NewCompiler()

	// Use ECMA-262 compatible regexp engine for pattern validation.
	// Go's stdlib regexp uses RE2 which rejects lookaheads ((?!, (?=)
	// that are valid in JSON Schema patterns.
	compiler.UseRegexpEngine(ecmaRegexpEngine)

	// Register x-gts-ref as a proper vocabulary so the library treats it as a real
	// keyword with validation semantics. This prevents oneOf/anyOf/allOf branches
	// containing only x-gts-ref from being treated as empty match-all schemas.
	compiler.RegisterVocabulary(newXGtsRefVocabulary(s))

	// Register lenient format validators to match Python's jsonschema behavior
	// Python's jsonschema library does NOT validate formats by default
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

	// Set up custom loader for GTS ID references (matches Python's resolve_gts_ref handler)
	compiler.UseLoader(&gtsURLLoader{store: s})

	// Get schema ID for compilation (now from normalized schema)
	schemaID, ok := normalizedSchema["$id"].(string)
	if !ok || schemaID == "" {
		return fmt.Errorf("schema must have a valid $id field")
	}

	// Normalize schema ID by stripping gts:// prefix if present
	normalizedSchemaID := strings.TrimPrefix(schemaID, GtsURIPrefix)

	// Update the $id in the normalized schema to use the normalized ID
	normalizedSchema["$id"] = normalizedSchemaID

	// Add the main schema to the compiler (use normalized schema with normalized ID)
	if err := compiler.AddResource(normalizedSchemaID, normalizedSchema); err != nil {
		return fmt.Errorf("add schema resource: %v", err)
	}

	// Pre-load all schemas from the store (matches Python's store dict pre-population)
	// Note: Store IDs are already normalized (without gts:// prefix). The schema
	// content, however, may still carry a gts:// $id which would override the
	// resource URL and make relative $ref resolution produce malformed URLs
	// (e.g. "gts://base/ref"). Normalize each pre-loaded schema's $id so the
	// resource URL and embedded $id agree.
	for id, entity := range s.byID {
		if entity.IsTypeSchema && id != normalizedSchemaID {
			resource := normalizeSchemaForCompile(entity.Content)
			if err := compiler.AddResource(id, resource); err != nil {
				// Ignore errors - gtsURLLoader will handle dynamic resolution
				continue
			}
		}
	}

	// Compile the schema using the normalized ID
	compiledSchema, err := compiler.Compile(normalizedSchemaID)
	if err != nil {
		return fmt.Errorf("compile schema: %v", err)
	}

	// Validate the instance
	if err := compiledSchema.Validate(instance); err != nil {
		return fmt.Errorf("validation error: %v", err)
	}

	return nil
}
