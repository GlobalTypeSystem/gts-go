/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/GlobalTypeSystem/gts-go/gts"
	"github.com/GlobalTypeSystem/gts-go/gtsid"
)

const gtsRefValidationQueryParam = "gts-ref-validation"

func parseGtsRefValidationMode(r *http.Request) (gts.GtsRefValidationMode, error) {
	return gts.ParseGtsRefValidationMode(r.URL.Query().Get(gtsRefValidationQueryParam))
}

// isConflict reports whether err represents an entity content conflict, which is
// surfaced to clients as HTTP 409.
func isConflict(err error) bool {
	var conflict *gts.EntityConflictError
	return errors.As(err, &conflict)
}

// Entity Management Handlers

func (s *Server) handleGetEntities(w http.ResponseWriter, r *http.Request) {
	limit := s.getQueryParamInt(r, "limit", 100)
	if limit < 1 {
		limit = 1
	}
	if limit > 1000 {
		limit = 1000
	}

	result := s.store.List(limit)
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetEntity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.writeError(w, http.StatusBadRequest, "Missing entity ID")
		return
	}

	// Public reads only ever see committed entities; a staged (in-flight,
	// not-yet-validated) entity must not be observable here.
	entity := s.store.GetCommitted(id)
	if entity == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("Entity not found: %s", id),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"id":      entity.GtsID.ID,
		"content": entity.Content,
	})
}

// validationRequested reports whether the request asked for full validation
// via ?validate=true (or the ?validation=true alias).
func validationRequested(r *http.Request) bool {
	if r.URL.Query().Get("validate") == "true" {
		return true
	}
	return r.URL.Query().Get("validation") == "true"
}

func (s *Server) handleAddEntity(w http.ResponseWriter, r *http.Request) {
	var content map[string]any
	if err := s.readJSON(r, &content); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// The GTS reference validation mode is parsed for every registration
	// request, so a bogus ?gts-ref-validation value is rejected up front
	// regardless of ?validate (matching the sibling implementations).
	mode, modeErr := parseGtsRefValidationMode(r)
	if modeErr != nil {
		s.writeError(w, http.StatusUnprocessableEntity, modeErr.Error())
		return
	}
	body, status := s.addEntityResult(content, validationRequested(r), mode)
	s.writeJSON(w, status, body)
}

// prepareEntity runs every structural check that does NOT require touching the
// store (canonical $schema/$id shape, schema $ref / x-gts-ref structure,
// modifier and trait-keyword placement) and builds the JsonEntity. On success
// it returns the entity and its effective id with a nil error body; on failure
// it returns the error body and HTTP status to send. It never registers or
// stages anything, so callers are free to register directly, or stage-then-
// validate, as the request semantics require.
func (s *Server) prepareEntity(
	content map[string]any,
	validate bool,
) (*gts.JsonEntity, string, map[string]any, int) {
	// Only the canonical JSON Schema keywords $schema/$id are recognized.
	hasSchemaField := false
	if schemaVal, ok := content["$schema"]; ok && schemaVal != nil {
		hasSchemaField = true
	}

	if hasSchemaField {
		idField, exists := content["$id"]
		if !exists || idField == nil {
			return nil, "", map[string]any{
				"ok":             false,
				"error":          "Unable to detect GTS ID in schema",
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
		idStr, ok := idField.(string)
		if !ok {
			return nil, "", map[string]any{
				"ok":             false,
				"error":          "JSON Schema $id field must be a string",
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			return nil, "", map[string]any{
				"ok":             false,
				"error":          "JSON Schema $id field cannot be empty",
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
		if !gtsid.HasURIPrefix(idStr) && !gtsid.HasPrefix(idStr) {
			return nil, "", map[string]any{
				"ok":             false,
				"error":          "JSON Schema $id must be a valid GTS identifier (optionally using gts:// prefix)",
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
		normalizedID := gtsid.NormalizeID(idStr)
		if gtsid.HasWildcard(normalizedID) {
			return nil, "", map[string]any{
				"ok":             false,
				"error":          "Wildcards are not allowed in schema IDs, only in patterns for access control",
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
		isBaseSchemaID := strings.Count(normalizedID, gtsid.TypeMarker) == 1 && gtsid.IsTypeID(normalizedID)
		if isBaseSchemaID && !gtsid.HasURIPrefix(idStr) {
			return nil, "", map[string]any{
				"ok":             false,
				"error":          "JSON Schema $id field must use gts:// URI prefix for base schemas",
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
		if !gtsid.IsValid(normalizedID) {
			return nil, "", map[string]any{
				"ok":             false,
				"error":          "JSON Schema $id must be a well-formed GTS identifier",
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
	}

	entity := gts.NewJsonEntity(content, gts.DefaultGtsConfig())

	// Determine the effective id used both to key the entity in the store
	// and to echo back to the client. Well-known instances and schemas use
	// their GTS id; anonymous instances (spec §3.7) use their raw id field.
	responseID := entity.EffectiveID()
	if responseID == "" {
		status := http.StatusOK
		if validate {
			status = http.StatusUnprocessableEntity
		}
		return nil, "", map[string]any{
			"ok":             false,
			"error":          "Unable to detect GTS ID in instance entity",
			"is_type_schema": entity.IsTypeSchema,
		}, status
	}

	// Always validate schema constraints for type-schemas
	if entity.IsTypeSchema {
		// Validate $id field for GTS schemas - check for specific invalid patterns
		if idField, exists := entity.Content["$id"]; exists {
			if idStr, ok := idField.(string); ok {
				// Reject plain gts. prefix only for base schemas (single segment ending with ~)
				// Derived schemas (multiple ~ segments) are allowed to use plain gts. format
				if gts.ClassifyRef(idStr) == gts.RefBareGtsID {
					// Count ~ segments to determine if it's a base or derived schema
					tildeParts := strings.Split(idStr, gtsid.TypeMarker)
					// If it's a base schema (only 2 parts: prefix and empty after ~), require gts://
					if len(tildeParts) == 2 && tildeParts[1] == "" {
						return nil, "", map[string]any{
							"ok":             false,
							"error":          "JSON Schema $id field must use gts:// URI prefix for GTS identifiers, not plain gts. prefix",
							"is_type_schema": true,
						}, http.StatusUnprocessableEntity
					}
				}
				// Check for wildcards in any GTS schema IDs
				if (gtsid.HasURIPrefix(idStr) || gtsid.HasPrefix(idStr)) && gtsid.HasWildcard(idStr) {
					return nil, "", map[string]any{
						"ok":             false,
						"error":          "Wildcards are not allowed in schema IDs, only in patterns for access control",
						"is_type_schema": true,
					}, http.StatusUnprocessableEntity
				}
			}
		}

		// Validate $ref constraints in the schema
		refValidator := gts.NewRefValidator()
		refErrors := refValidator.ValidateSchemaRefs(entity.Content, "")
		if len(refErrors) > 0 {
			var errorMsgs []string
			for _, err := range refErrors {
				errorMsgs = append(errorMsgs, err.Error())
			}
			return nil, "", map[string]any{
				"ok":             false,
				"error":          fmt.Sprintf("$ref validation failed: %s", strings.Join(errorMsgs, "; ")),
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}

		// Create a validator to validate x-gts-ref patterns in schema definition
		xGtsRefValidator := gts.NewXGtsRefValidator(s.store)
		xGtsRefErrors := xGtsRefValidator.ValidateSchema(entity.Content, "")
		if len(xGtsRefErrors) > 0 {
			var errorMsgs []string
			for _, err := range xGtsRefErrors {
				errorMsgs = append(errorMsgs, err.Error())
			}
			return nil, "", map[string]any{
				"ok":             false,
				"error":          fmt.Sprintf("x-gts-ref validation failed: %s", strings.Join(errorMsgs, "; ")),
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
	}

	// Validate schema modifiers (x-gts-final, x-gts-abstract) and trait keyword
	// placement (x-gts-traits, x-gts-traits-schema) for type-schemas. These are
	// type-level keywords and MUST appear only at the schema top level
	// (gts-spec §9.7.1/§9.11).
	if entity.IsTypeSchema {
		if err := gts.ValidateSchemaExtensions(entity.Content); err != nil {
			return nil, "", map[string]any{
				"ok": false, "error": err.Error(), "is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
		if err := gts.ValidateSchemaModifiers(entity.Content); err != nil {
			return nil, "", map[string]any{
				"ok":             false,
				"error":          err.Error(),
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
		if err := gts.ValidateTraitPlacement(entity.Content); err != nil {
			return nil, "", map[string]any{
				"ok":             false,
				"error":          err.Error(),
				"is_type_schema": true,
			}, http.StatusUnprocessableEntity
		}
	}

	return entity, responseID, nil, 0
}

// addEntityResult runs the full single-entity registration + validation
// pipeline and returns the JSON response body and HTTP status. Both
// POST /entities and each POST /type-schemas batch entry go through the same
// prepare/stage/validate/commit steps, so batch registration honors ?validate
// / ?gts-ref-validation exactly as the single-entity endpoint does.
func (s *Server) addEntityResult(
	content map[string]any,
	validate bool,
	mode gts.GtsRefValidationMode,
) (map[string]any, int) {
	entity, responseID, errBody, errStatus := s.prepareEntity(content, validate)
	if errBody != nil {
		return errBody, errStatus
	}

	ok := map[string]any{
		"ok":             true,
		"gts_id":         responseID,
		"id":             responseID,
		"type_id":        entity.TypeID,
		"is_type_schema": entity.IsTypeSchema,
	}

	if validate {
		// Stage the entity (invisible to public reads) and validate it before
		// publishing. A failure discards the staged entity, so a reader never
		// observes an entity that has not passed validation - and no store-wide
		// write lock is held across validation, so concurrent reads are not
		// blocked.
		token, err := s.store.Stage(entity)
		if err != nil {
			status := http.StatusUnprocessableEntity
			if isConflict(err) {
				status = http.StatusConflict
			}
			return map[string]any{
				"ok":             false,
				"error":          err.Error(),
				"is_type_schema": entity.IsTypeSchema,
			}, status
		}
		if result := s.store.ValidateEntity(responseID, mode); !result.OK {
			s.store.Discard(token)
			return map[string]any{
				"ok":             false,
				"error":          result.Error,
				"is_type_schema": entity.IsTypeSchema,
			}, http.StatusUnprocessableEntity
		}
		if err := s.store.Commit(token); err != nil {
			status := http.StatusUnprocessableEntity
			if isConflict(err) {
				status = http.StatusConflict
			}
			return map[string]any{
				"ok":             false,
				"error":          err.Error(),
				"is_type_schema": entity.IsTypeSchema,
			}, status
		}
		return ok, http.StatusOK
	}

	if err := s.store.Register(entity); err != nil {
		status := http.StatusOK
		if isConflict(err) {
			status = http.StatusConflict
		}
		return map[string]any{
			"ok":             false,
			"error":          err.Error(),
			"is_type_schema": entity.IsTypeSchema,
		}, status
	}
	return ok, http.StatusOK
}

func (s *Server) handleAddEntities(w http.ResponseWriter, r *http.Request) {
	var contents []map[string]any
	if err := s.readJSON(r, &contents); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON array")
		return
	}

	result := make([]map[string]any, len(contents))
	successCount := 0

	for i, content := range contents {
		entity := gts.NewJsonEntity(content, gts.DefaultGtsConfig())
		responseID := entity.EffectiveID()
		if responseID == "" {
			result[i] = map[string]any{
				"ok":             false,
				"error":          "Unable to extract GTS ID from entity",
				"is_type_schema": entity.IsTypeSchema,
			}
			continue
		}

		err := s.store.Register(entity)
		if err != nil {
			result[i] = map[string]any{
				"ok":             false,
				"error":          err.Error(),
				"is_type_schema": entity.IsTypeSchema,
			}
			continue
		}

		result[i] = map[string]any{
			"ok":             true,
			"gts_id":         responseID,
			"is_type_schema": entity.IsTypeSchema,
		}
		successCount++
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":      successCount == len(contents),
		"count":   successCount,
		"total":   len(contents),
		"results": result,
	})
}

// handleAddSchemas registers a batch of GTS Type Schemas. The request body is
// a JSON array of GTS Type Schema objects; each entry's GTS Type Identifier is
// derived from its embedded $id (there is no external type_id field). The
// response is an aggregate {ok, results:[...]} body where the top-level ok is
// true only when every entry registered successfully.
func (s *Server) handleAddSchemas(w http.ResponseWriter, r *http.Request) {
	var schemas []map[string]any
	if err := s.readJSON(r, &schemas); err != nil {
		s.writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"error": "Request body must be a JSON array of GTS Type Schemas",
		})
		return
	}

	// ?validate / ?gts-ref-validation apply to every batch entry exactly as on
	// POST /entities; a bogus gts-ref-validation is rejected before any entry
	// is registered.
	mode, modeErr := parseGtsRefValidationMode(r)
	if modeErr != nil {
		s.writeError(w, http.StatusUnprocessableEntity, modeErr.Error())
		return
	}
	validate := validationRequested(r)

	results := make([]map[string]any, len(schemas))
	if validate {
		s.stageAndValidateBatch(schemas, mode, results)
	} else {
		for i, schema := range schemas {
			results[i] = s.registerTypeSchema(schema, false, mode)
		}
	}

	allOK := true
	for _, result := range results {
		if ok, _ := result["ok"].(bool); !ok {
			allOK = false
		}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":      allOK,
		"results": results,
	})
}

// stageAndValidateBatch runs a validate=true batch in two phases so the outcome
// is order-independent and nothing invalid is ever published:
//
//  1. Stage every structurally-valid entry (invisible to public reads).
//  2. Validate every staged entry against the fully-staged set, so an entry can
//     resolve intra-batch references/ancestors regardless of position.
//  3. Commit the entries that passed and discard the ones that failed.
//
// Because staged entries are never visible to public reads, a concurrent reader
// never observes an entry that has not (yet) passed validation, and no
// store-wide write lock is held across validation.
func (s *Server) stageAndValidateBatch(
	schemas []map[string]any,
	mode gts.GtsRefValidationMode,
	results []map[string]any,
) {
	type stagedEntry struct {
		index    int
		typeID   string
		entityID string
		token    string
	}
	var survivors []stagedEntry
	// pending tracks every still-staged token so a panic anywhere below discards
	// the leftovers instead of leaking unvalidated entries into the staging
	// overlay. Tokens are removed as they are committed or discarded.
	pending := map[string]struct{}{}
	defer func() {
		for token := range pending {
			s.store.Discard(token)
		}
	}()

	// Phase 1: stage.
	for i, schema := range schemas {
		typeID, idErr := typeSchemaIdentity(schema)
		if idErr != nil {
			results[i] = idErr
			continue
		}
		entity, responseID, errBody, _ := s.prepareEntity(schema, true)
		if errBody != nil {
			results[i] = map[string]any{"ok": false, "type_id": typeID, "error": errBody["error"]}
			continue
		}
		token, err := s.store.Stage(entity)
		if err != nil {
			results[i] = map[string]any{"ok": false, "type_id": typeID, "error": err.Error()}
			continue
		}
		pending[token] = struct{}{}
		survivors = append(survivors, stagedEntry{index: i, typeID: typeID, entityID: responseID, token: token})
	}

	// Phase 2: validate the staged entries, discarding failures and RE-validating
	// the survivors against the now-smaller staged set until a round produces no
	// new failures. This stops an entry that only validated because a sibling was
	// staged (e.g. its parent or $ref target) from being committed after that
	// sibling has itself been discarded.
	for {
		var stillGood []stagedEntry
		var failed []stagedEntry
		failErr := map[int]string{}
		for _, entry := range survivors {
			if res := s.store.ValidateEntity(entry.entityID, mode); res.OK {
				stillGood = append(stillGood, entry)
			} else {
				failed = append(failed, entry)
				failErr[entry.index] = res.Error
			}
		}
		if len(failed) == 0 {
			break
		}
		for _, entry := range failed {
			s.store.Discard(entry.token)
			delete(pending, entry.token)
			results[entry.index] = map[string]any{"ok": false, "type_id": entry.typeID, "error": failErr[entry.index]}
		}
		survivors = stillGood
	}

	// Phase 3: publish the entries that passed. A commit can still report a
	// conflict if a concurrent batch committed the same id with different content.
	for _, entry := range survivors {
		if err := s.store.Commit(entry.token); err != nil {
			results[entry.index] = map[string]any{"ok": false, "type_id": entry.typeID, "error": err.Error()}
		} else {
			results[entry.index] = map[string]any{"ok": true, "type_id": entry.typeID}
		}
		delete(pending, entry.token)
	}
}

// typeSchemaIdentity validates the batch-specific requirement that every entry
// is an object carrying a canonical $schema and a gts:// $id, and returns the
// derived GTS Type Identifier. On failure it returns the per-item error result.
func typeSchemaIdentity(schema map[string]any) (string, map[string]any) {
	if dialect, ok := schema["$schema"].(string); !ok || strings.TrimSpace(dialect) == "" {
		return "", map[string]any{
			"ok":      false,
			"type_id": nil,
			"error":   "GTS Type Schema must contain a top-level $schema field",
		}
	}
	embeddedID, ok := schema["$id"].(string)
	if !ok || !strings.HasPrefix(embeddedID, gtsid.URIPrefix+gtsid.Prefix) {
		return "", map[string]any{
			"ok":      false,
			"type_id": nil,
			"error":   "GTS Type Schema must contain a top-level $id in gts:// form",
		}
	}
	typeID := gtsid.NormalizeID(embeddedID)
	if !gtsid.IsValid(typeID) || !gtsid.IsTypeID(typeID) {
		return "", map[string]any{
			"ok":      false,
			"type_id": typeID,
			"error":   fmt.Sprintf("invalid GTS Type Schema $id: %q", embeddedID),
		}
	}
	return typeID, nil
}

// registerTypeSchema registers a single GTS Type Schema WITHOUT full validation
// (the ?validate=false batch path), deriving its GTS Type Identifier from the
// embedded $id and returning a per-item result map.
func (s *Server) registerTypeSchema(
	schema map[string]any,
	validate bool,
	mode gts.GtsRefValidationMode,
) map[string]any {
	typeID, idErr := typeSchemaIdentity(schema)
	if idErr != nil {
		return idErr
	}

	body, _ := s.addEntityResult(schema, validate, mode)
	if ok, _ := body["ok"].(bool); ok {
		return map[string]any{
			"ok":      true,
			"type_id": typeID,
		}
	}
	return map[string]any{
		"ok":      false,
		"type_id": typeID,
		"error":   body["error"],
	}
}

// Operation Handlers

// OP#1 - Validate ID
func (s *Server) handleValidateID(w http.ResponseWriter, r *http.Request) {
	gtsID := s.getQueryParam(r, "gts_id")
	if gtsID == "" {
		s.writeError(w, http.StatusBadRequest, "Missing gts_id parameter")
		return
	}

	result := gtsid.Validate(gtsID)
	s.writeJSON(w, http.StatusOK, result)
}

// OP#2 - Extract ID
func (s *Server) handleExtractID(w http.ResponseWriter, r *http.Request) {
	var content map[string]any
	if err := s.readJSON(r, &content); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	result := gts.ExtractGtsID(content, gts.DefaultGtsConfig())
	s.writeJSON(w, http.StatusOK, result)
}

// OP#3 - Parse ID
func (s *Server) handleParseID(w http.ResponseWriter, r *http.Request) {
	gtsID := s.getQueryParam(r, "gts_id")
	if gtsID == "" {
		s.writeError(w, http.StatusBadRequest, "Missing gts_id parameter")
		return
	}

	result := gtsid.Parse(gtsID)
	s.writeJSON(w, http.StatusOK, result)
}

// OP#4 - Match ID Pattern
func (s *Server) handleMatchIDPattern(w http.ResponseWriter, r *http.Request) {
	candidate := s.getQueryParam(r, "candidate")
	pattern := s.getQueryParam(r, "pattern")

	if candidate == "" || pattern == "" {
		s.writeError(w, http.StatusBadRequest, "Missing candidate or pattern parameter")
		return
	}

	result := gtsid.Match(candidate, pattern)
	s.writeJSON(w, http.StatusOK, result)
}

// OP#5 - UUID
func (s *Server) handleUUID(w http.ResponseWriter, r *http.Request) {
	gtsID := s.getQueryParam(r, "gts_id")
	if gtsID == "" {
		s.writeError(w, http.StatusBadRequest, "Missing gts_id parameter")
		return
	}

	result := gtsid.IDToUUID(gtsID)
	s.writeJSON(w, http.StatusOK, result)
}

// OP#6 - Validate Instance
func (s *Server) handleValidateInstance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	mode, err := parseGtsRefValidationMode(r)
	if err != nil {
		s.writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	result := s.store.ValidateInstance(req.InstanceID, mode)
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleValidateJSON(w http.ResponseWriter, r *http.Request) {
	var value any
	if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	content, ok := value.(map[string]any)
	if !ok {
		s.writeError(w, http.StatusUnprocessableEntity, "JSON validation body must be an object")
		return
	}
	s.writeJSON(w, http.StatusOK, s.store.ValidateTransientJSON(content, r.PathValue("typeID")))
}

// OP#7 - Resolve Relationships
func (s *Server) handleResolveRelationships(w http.ResponseWriter, r *http.Request) {
	gtsID := s.getQueryParam(r, "gts_id")
	if gtsID == "" {
		s.writeError(w, http.StatusBadRequest, "Missing gts_id parameter")
		return
	}

	result := s.store.BuildSchemaGraph(gtsID)
	s.writeJSON(w, http.StatusOK, result)
}

// OP#8 - Compatibility
func (s *Server) handleCompatibility(w http.ResponseWriter, r *http.Request) {
	oldTypeID := s.getQueryParam(r, "old_type_id")
	newTypeID := s.getQueryParam(r, "new_type_id")

	if oldTypeID == "" || newTypeID == "" {
		s.writeError(w, http.StatusBadRequest, "Missing old_type_id or new_type_id parameter")
		return
	}

	result := s.store.CheckCompatibility(oldTypeID, newTypeID)
	s.writeJSON(w, http.StatusOK, result)
}

// OP#9 - Cast
func (s *Server) handleCast(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InstanceID string `json:"instance_id"`
		ToTypeID   string `json:"to_type_id"`
	}
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	result, err := s.store.Cast(req.InstanceID, req.ToTypeID)
	if err != nil {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"error": err.Error(),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

// OP#10 - Query
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	expr := s.getQueryParam(r, "expr")
	if expr == "" {
		s.writeError(w, http.StatusBadRequest, "Missing expr parameter")
		return
	}

	limit := s.getQueryParamInt(r, "limit", 100)
	if limit < 1 {
		limit = 1
	}
	if limit > 1000 {
		limit = 1000
	}

	result := s.store.Query(expr, limit)
	s.writeJSON(w, http.StatusOK, result)
}

// OP#12 - Validate Type-Schema (schema-vs-schema chain validation)
func (s *Server) handleValidateSchema(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TypeID string `json:"type_id"`
	}
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if req.TypeID == "" {
		s.writeError(w, http.StatusBadRequest, "Missing type_id")
		return
	}

	mode, err := parseGtsRefValidationMode(r)
	if err != nil {
		s.writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	entity := s.store.Get(req.TypeID)
	if entity == nil || !entity.IsTypeSchema {
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "type_id does not identify a type schema"})
		return
	}
	result := s.store.ValidateEntity(req.TypeID, mode)
	if result.OK {
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	} else {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": result.Error,
		})
	}
}

// OP#13 - Validate Entity (schema chain + traits validation)
func (s *Server) handleValidateEntity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntityID string `json:"entity_id"`
		GtsID    string `json:"gts_id"`
	}
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	// Accept either entity_id or gts_id
	id := req.EntityID
	if id == "" {
		id = req.GtsID
	}
	if id == "" {
		s.writeError(w, http.StatusBadRequest, "Missing entity_id or gts_id")
		return
	}

	mode, err := parseGtsRefValidationMode(r)
	if err != nil {
		s.writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	result := s.store.ValidateEntity(id, mode)
	resp := map[string]any{
		"ok":          result.OK,
		"entity_type": result.EntityType,
	}
	if !result.OK {
		resp["error"] = result.Error
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// OP#11 - Attribute Access
func (s *Server) handleAttribute(w http.ResponseWriter, r *http.Request) {
	gtsWithPath := s.getQueryParam(r, "gts_with_path")
	if gtsWithPath == "" {
		s.writeError(w, http.StatusBadRequest, "Missing gts_with_path parameter")
		return
	}

	result := s.store.GetAttribute(gtsWithPath)
	s.writeJSON(w, http.StatusOK, result)
}
