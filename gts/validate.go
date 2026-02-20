/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/message"
)

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
		if !entity.IsSchema {
			return nil, fmt.Errorf("GTS reference is not a schema: %s", url)
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

// ValidateInstance validates an object instance against its schema
// Returns ValidationResult with ok=true if validation succeeds
func (s *GtsStore) ValidateInstance(gtsID string) *ValidationResult {
	// Parse and validate GTS ID
	gid, err := NewGtsID(gtsID)
	if err != nil {
		return &ValidationResult{
			ID:    gtsID,
			OK:    false,
			Error: fmt.Sprintf("Invalid GTS ID: %v", err),
		}
	}

	// Get the instance from store
	obj := s.Get(gid.ID)
	if obj == nil {
		return &ValidationResult{
			ID:    gtsID,
			OK:    false,
			Error: (&StoreGtsObjectNotFoundError{EntityID: gtsID}).Error(),
		}
	}

	// Check if instance has a schema ID
	if obj.SchemaID == "" {
		return &ValidationResult{
			ID:    gtsID,
			OK:    false,
			Error: (&StoreGtsSchemaForInstanceNotFoundError{EntityID: gid.ID}).Error(),
		}
	}

	// Get the schema from store
	schemaEntity := s.Get(obj.SchemaID)
	if schemaEntity == nil {
		return &ValidationResult{
			ID:    gtsID,
			OK:    false,
			Error: (&StoreGtsSchemaNotFoundError{EntityID: obj.SchemaID}).Error(),
		}
	}

	if !schemaEntity.IsSchema {
		return &ValidationResult{
			ID:    gtsID,
			OK:    false,
			Error: fmt.Sprintf("entity '%s' is not a schema", obj.SchemaID),
		}
	}

	// Validate the instance against the schema
	err = s.validateWithSchema(obj.Content, schemaEntity.Content)
	if err != nil {
		return &ValidationResult{
			ID:    gtsID,
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
			ID:    gtsID,
			OK:    false,
			Error: fmt.Sprintf("x-gts-ref validation failed: %s", strings.Join(errorMsgs, "; ")),
		}
	}

	return &ValidationResult{
		ID:    gtsID,
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

// validateWithSchema performs the actual JSON Schema validation
func (s *GtsStore) validateWithSchema(instance map[string]any, schema map[string]any) error {
	// Normalize schema to convert $$id to $id and $$schema to $schema for JSON Schema validation
	normalizedSchema := make(map[string]any)
	for k, v := range schema {
		switch k {
		case "$$id":
			normalizedSchema["$id"] = v
		case "$$schema":
			normalizedSchema["$schema"] = v
		default:
			normalizedSchema[k] = v
		}
	}

	// Create a custom compiler with GTS reference resolution
	compiler := jsonschema.NewCompiler()

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
	// Note: Store IDs are already normalized (without gts:// prefix)
	for id, entity := range s.byID {
		if entity.IsSchema && id != normalizedSchemaID {
			if err := compiler.AddResource(id, entity.Content); err != nil {
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
