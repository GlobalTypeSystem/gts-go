/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"strings"
	"time"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
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
	// Reduce any URI form (gts://, gts:///) to the bare GTS identifier.
	normalizedURL := gtsid.NormalizeID(url)

	// Check if this is a GTS ID reference
	if gtsid.IsValid(normalizedURL) {
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
func (s *GtsStore) ValidateInstance(instanceID string, modes ...GtsRefValidationMode) *ValidationResult {
	mode := GtsRefValidationAnyValid
	if len(modes) > 0 {
		mode = modes[0]
	}
	if err := s.validateInstanceTransitive(instanceID, mode); err != nil {
		return &ValidationResult{ID: instanceID, OK: false, Error: err.Error()}
	}
	return &ValidationResult{ID: instanceID, OK: true, Error: ""}
}

func (s *GtsStore) validateInstanceLocal(instanceID string, mode GtsRefValidationMode) *ValidationResult {
	// Well-known GTS id first; fall back to a raw store lookup by the
	// passed string so anonymous instances (keyed by UUID) resolve too.
	lookupID := instanceID
	if gtsid.IsValid(instanceID) {
		gid, err := gtsid.New(instanceID)
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

	// Resolve composed schemas before x-gts-ref traversal while keeping the
	// selected leaf identifier as the /$id root.
	xGtsRefSchema, err := s.resolveSchemaRefsChecked(obj.TypeID)
	if err != nil {
		return &ValidationResult{ID: instanceID, OK: false, Error: err.Error()}
	}
	xGtsRefValidator := NewXGtsRefValidator(s, mode)
	xGtsRefErrors := xGtsRefValidator.ValidateInstance(obj.Content, xGtsRefSchema, "", obj.TypeID)
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
// The separate XGtsRefValidator pass handles /$id and registry semantics that
// require the selected type and full instance path context.
type xGtsRefExt struct {
	pattern string
}

func (e *xGtsRefExt) Validate(ctx *jsonschema.ValidatorContext, v any) {
	str, ok := v.(string)
	if !ok {
		return
	}
	// /$id needs the selected type and is enforced by the separate store pass.
	if IsXGtsRefSelf(e.pattern) {
		return
	}
	validator := NewXGtsRefValidator(nil, GtsRefValidationNone)
	if err := validator.validateRefValue(str, e.pattern, "", ""); err != nil {
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
func newXGtsRefVocabulary(_ *GtsStore) *jsonschema.Vocabulary {
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
			return &xGtsRefExt{pattern: pattern}, nil
		},
	}
}

// normalizeSchemaForCompile returns a shallow copy of a schema with $id rewritten
// to the absolute compile-URI form (see gtsid.CompileURIPrefix). This ensures the
// embedded $id agrees with the resource URL used by the JSON Schema compiler, so
// relative $ref values resolve correctly and error messages reference a portable
// GTS URI instead of a local file:// path.
func normalizeSchemaForCompile(schema map[string]any) map[string]any {
	normalized := make(map[string]any, len(schema))
	for k, v := range schema {
		normalized[k] = v
	}
	if id, ok := normalized["$id"].(string); ok {
		normalized["$id"] = gtsid.ToCompileURI(id)
	}
	return normalized
}

func collectExternalSchemaRefs(node any, refs map[string]struct{}) {
	switch value := node.(type) {
	case map[string]any:
		if raw, ok := value["$ref"].(string); ok && !strings.HasPrefix(raw, "#") {
			target, _, _ := strings.Cut(raw, "#")
			id := gtsid.NormalizeID(target)
			if gtsid.IsValid(id) {
				refs[id] = struct{}{}
			}
		}
		for _, child := range value {
			collectExternalSchemaRefs(child, refs)
		}
	case []any:
		for _, child := range value {
			collectExternalSchemaRefs(child, refs)
		}
	}
}

func (s *GtsStore) addSchemaDependencyResources(compiler *jsonschema.Compiler, schema map[string]any, rootID string) {
	refs := make(map[string]struct{})
	collectExternalSchemaRefs(schema, refs)
	loaded := map[string]struct{}{gtsid.NormalizeID(rootID): {}}
	for len(refs) > 0 {
		var id string
		for candidate := range refs {
			id = candidate
			delete(refs, candidate)
			break
		}
		if _, exists := loaded[id]; exists {
			continue
		}
		loaded[id] = struct{}{}
		entity := s.Get(id)
		if entity == nil || !entity.IsTypeSchema {
			continue
		}
		resource := normalizeSchemaForCompile(entity.Content)
		if compiler.AddResource(gtsid.ToCompileURI(id), resource) == nil {
			collectExternalSchemaRefs(entity.Content, refs)
		}
	}
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
	if _, err := schemaDialect(schema); err != nil {
		return fmt.Errorf("JSON Schema validation failed: %v", err)
	}
	normalizedSchema := normalizeSchemaForCompile(schema)
	schemaID, ok := normalizedSchema["$id"].(string)
	if !ok || schemaID == "" {
		schemaID = gtsid.ToCompileURI("gts.validation.schema")
		normalizedSchema["$id"] = schemaID
	}

	compiler := jsonschema.NewCompiler()
	compiler.UseRegexpEngine(ecmaRegexpEngine)
	compiler.UseLoader(&gtsURLLoader{store: s})
	if err := compiler.AddResource(schemaID, normalizedSchema); err != nil {
		return fmt.Errorf("JSON Schema validation failed: %v", err)
	}
	s.forEachEntity(func(id string, entity *JsonEntity) bool {
		if resourceID := gtsid.ToCompileURI(id); entity.IsTypeSchema && resourceID != schemaID {
			_ = compiler.AddResource(resourceID, normalizeSchemaForCompile(entity.Content))
		}
		return true
	})
	if _, err := compiler.Compile(schemaID); err != nil {
		return fmt.Errorf("JSON Schema validation failed: %v", err)
	}
	return nil
}

// adaptTypeMismatchMessage augments a raw JSON Schema validation message with the
// spec-mandated "is not of type 'string'" phrasing (gts-spec OP#6). This is an
// intentional, isolated adaptation of the underlying jsonschema library's
// wording: the library does not expose a typed error kind we can match on for
// this case, so a library upgrade that rewords "got number, want string" would
// require updating this single helper. Keeping it in one place makes that
// coupling explicit rather than scattering substring checks through the flow.
func adaptTypeMismatchMessage(message string) string {
	if strings.Contains(message, "got number, want string") {
		return message + ": is not of type 'string'"
	}
	return message
}

func (s *GtsStore) ValidateTransientJSON(content map[string]any, typeID string) *JSONValidationResult {
	entity := NewJsonEntity(content, DefaultGtsConfig())
	result := &JSONValidationResult{IsTypeSchema: entity.IsTypeSchema}
	fail := func(message string) *JSONValidationResult {
		result.Error = adaptTypeMismatchMessage(message)
		return result
	}
	if typeID != "" {
		if entity.IsTypeSchema {
			return fail("validate-json with an explicit type only accepts instance JSON")
		}
		if !gtsid.IsTypeID(typeID) {
			if gtsid.HasPrefix(typeID) {
				return fail("explicit type must be GTS Type schema")
			}
			return fail("Invalid GTS Type Schema ID")
		}
		if !gtsid.IsValid(typeID) {
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
		s.writerMu.Lock()
		s.mu.Lock()
		previous, existed := s.byID[entity.GtsID.ID]
		s.byID[entity.GtsID.ID] = cloneJsonEntity(entity)
		s.mu.Unlock()
		s.invalidateSchemaCache()
		validation := s.ValidateSchemaChain(entity.GtsID.ID)
		s.mu.Lock()
		if existed {
			s.byID[entity.GtsID.ID] = previous
		} else {
			delete(s.byID, entity.GtsID.ID)
		}
		s.mu.Unlock()
		s.invalidateSchemaCache()
		s.writerMu.Unlock()
		if !validation.OK {
			if validation.MissingSchema {
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
	if validation := s.ValidateSchemaChain(entity.TypeID); !validation.OK {
		return fail(validation.Error)
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

// compiledSchemaEntry is a memoized compilation result. A failed compile is
// cached too so repeated validation against a broken schema doesn't recompile.
// gen is the schemaCacheGen observed when this compile began; a reader trusts
// the entry only while gen is still current (see compileSchema).
type compiledSchemaEntry struct {
	schema *jsonschema.Schema
	err    error
	gen    uint64
}

// compileSchema returns a compiled JSON Schema for schemaID, reusing a cached
// result when available. The cache is cleared on every store mutation
// (invalidateSchemaCache), so a cached entry is only reused while the schema and
// every schema it can resolve are unchanged. The key additionally includes a
// content hash of the (normalized) schema so callers that compile different
// content under the same $id (e.g. transient validation) never collide.
//
// A compile can run concurrently with a mutation (validation does not hold
// writerMu). To keep such a compile from publishing a result built from
// now-stale dependencies, the generation is captured before the compile begins:
// a cached entry is reused only when its generation still matches, and the entry
// is published only when no invalidation intervened. An entry that loses either
// race is simply recomputed on the next call.
//
// rawSchema is the pre-normalization schema; it is walked to discover external
// gts:// $ref dependencies to preload as compiler resources.
func (s *GtsStore) compileSchema(schemaID string, normalizedSchema, rawSchema map[string]any) (*jsonschema.Schema, error) {
	gen := s.schemaCacheGen.Load()
	key := schemaID + "\x00" + contentHash(normalizedSchema)
	if cached, ok := s.schemaCache.Load(key); ok {
		if entry := cached.(*compiledSchemaEntry); entry.gen == gen {
			return entry.schema, entry.err
		}
	}

	// Create a custom compiler with GTS reference resolution.
	compiler := jsonschema.NewCompiler()
	// Use ECMA-262 compatible regexp engine for pattern validation. Go's stdlib
	// regexp uses RE2 which rejects lookaheads ((?!, (?=) valid in JSON Schema.
	compiler.UseRegexpEngine(ecmaRegexpEngine)
	// Register x-gts-ref as a proper vocabulary so the library treats it as a real
	// keyword with validation semantics. This prevents oneOf/anyOf/allOf branches
	// containing only x-gts-ref from being treated as empty match-all schemas.
	compiler.RegisterVocabulary(newXGtsRefVocabulary(s))
	// Assert JSON Schema format keywords (uuid, email, date-time, …) so format
	// violations are reported as validation errors (gts-spec OP#6).
	compiler.AssertFormat()
	// Set up custom loader for GTS ID references (matches Python's resolve_gts_ref handler).
	compiler.UseLoader(&gtsURLLoader{store: s})

	entry := &compiledSchemaEntry{gen: gen}
	if err := compiler.AddResource(schemaID, normalizedSchema); err != nil {
		entry.err = fmt.Errorf("add schema resource: %v", err)
	} else {
		s.addSchemaDependencyResources(compiler, rawSchema, schemaID)
		compiled, err := compiler.Compile(schemaID)
		if err != nil {
			entry.err = fmt.Errorf("compile schema: %v", err)
		} else {
			entry.schema = compiled
		}
	}
	// Publish only if no invalidation intervened during the compile. If one did,
	// this entry reflects a stale view; skip storing it (a later call recompiles)
	// rather than caching a result the generation guard would reject anyway.
	if s.schemaCacheGen.Load() == gen {
		s.schemaCache.Store(key, entry)
	}
	return entry.schema, entry.err
}

// validateWithSchema performs the actual JSON Schema validation
func (s *GtsStore) validateWithSchema(instance map[string]any, schema map[string]any) error {
	if _, err := schemaDialect(schema); err != nil {
		return err
	}
	// Rewrite $id to the absolute compile-URI form for JSON Schema validation
	normalizedSchema := normalizeSchemaForCompile(schema)

	// Get schema ID for compilation (already in compile-URI form from normalization)
	schemaID, ok := normalizedSchema["$id"].(string)
	if !ok || schemaID == "" {
		return fmt.Errorf("schema must have a valid $id field")
	}

	compiledSchema, err := s.compileSchema(schemaID, normalizedSchema, schema)
	if err != nil {
		return err
	}

	// Validate the instance
	if err := compiledSchema.Validate(instance); err != nil {
		return fmt.Errorf("validation error: %v", err)
	}

	return nil
}
