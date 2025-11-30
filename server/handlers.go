/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/GlobalTypeSystem/gts-go/gts"
)

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

	entity := s.store.Get(id)
	if entity == nil {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("Entity not found: %s", id))
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":      entity.GtsID.ID,
		"content": entity.Content,
	})
}

func (s *Server) handleAddEntity(w http.ResponseWriter, r *http.Request) {
	var content map[string]any
	if err := s.readJSON(r, &content); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	entity := gts.NewJsonEntity(content, gts.DefaultGtsConfig())
	if entity.GtsID == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "Unable to extract GTS ID from entity",
		})
		return
	}

	// Always validate x-gts-ref constraints for schemas
	if entity.IsSchema {
		// Create a validator to validate x-gts-ref patterns in schema definition
		xGtsRefValidator := gts.NewXGtsRefValidator(s.store)
		xGtsRefErrors := xGtsRefValidator.ValidateSchema(entity.Content, "", nil)
		if len(xGtsRefErrors) > 0 {
			var errorMsgs []string
			for _, err := range xGtsRefErrors {
				errorMsgs = append(errorMsgs, err.Error())
			}
			s.writeJSON(w, http.StatusOK, map[string]any{
				"ok":    false,
				"error": fmt.Sprintf("Validation failed: %s", strings.Join(errorMsgs, "; ")),
			})
			return
		}
	}

	// Check if instance validation is requested via query parameter
	validation := r.URL.Query().Get("validation")
	if validation == "true" && !entity.IsSchema {
		// For non-schema entities with validation=true, register first then validate
		err := s.store.Register(entity)
		if err != nil {
			s.writeJSON(w, http.StatusOK, map[string]any{
				"ok":    false,
				"error": err.Error(),
			})
			return
		}

		// Validate the instance
		result := s.store.ValidateInstance(entity.GtsID.ID)
		if !result.OK {
			s.writeJSON(w, http.StatusOK, map[string]any{
				"ok":    false,
				"error": result.Error,
			})
			return
		}

		s.writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"gts_id": entity.GtsID.ID,
		})
		return
	}

	err := s.store.Register(entity)
	if err != nil {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"gts_id": entity.GtsID.ID,
	})
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
		if entity.GtsID == nil {
			result[i] = map[string]any{
				"ok":    false,
				"error": "Unable to extract GTS ID from entity",
			}
			continue
		}

		err := s.store.Register(entity)
		if err != nil {
			result[i] = map[string]any{
				"ok":    false,
				"error": err.Error(),
			}
			continue
		}

		result[i] = map[string]any{
			"ok":     true,
			"gts_id": entity.GtsID.ID,
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

func (s *Server) handleAddSchema(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TypeID string         `json:"type_id"`
		Schema map[string]any `json:"schema"`
	}
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	err := s.store.RegisterSchema(req.TypeID, req.Schema)
	if err != nil {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"type_id": req.TypeID,
			"error":   err.Error(),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"type_id": req.TypeID,
	})
}

// Operation Handlers

// OP#1 - Validate ID
func (s *Server) handleValidateID(w http.ResponseWriter, r *http.Request) {
	gtsID := s.getQueryParam(r, "gts_id")
	if gtsID == "" {
		s.writeError(w, http.StatusBadRequest, "Missing gts_id parameter")
		return
	}

	result := gts.ValidateGtsID(gtsID)
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

	result := gts.ParseGtsID(gtsID)
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

	result := gts.MatchIDPattern(candidate, pattern)
	s.writeJSON(w, http.StatusOK, result)
}

// OP#5 - UUID
func (s *Server) handleUUID(w http.ResponseWriter, r *http.Request) {
	gtsID := s.getQueryParam(r, "gts_id")
	if gtsID == "" {
		s.writeError(w, http.StatusBadRequest, "Missing gts_id parameter")
		return
	}

	result := gts.IDToUUID(gtsID)
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

	result := s.store.ValidateInstance(req.InstanceID)
	s.writeJSON(w, http.StatusOK, result)
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
	oldSchemaID := s.getQueryParam(r, "old_schema_id")
	newSchemaID := s.getQueryParam(r, "new_schema_id")

	if oldSchemaID == "" || newSchemaID == "" {
		s.writeError(w, http.StatusBadRequest, "Missing old_schema_id or new_schema_id parameter")
		return
	}

	result := s.store.CheckCompatibility(oldSchemaID, newSchemaID)
	s.writeJSON(w, http.StatusOK, result)
}

// OP#9 - Cast
func (s *Server) handleCast(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InstanceID string `json:"instance_id"`
		ToSchemaID string `json:"to_schema_id"`
	}
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	result, err := s.store.Cast(req.InstanceID, req.ToSchemaID)
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
