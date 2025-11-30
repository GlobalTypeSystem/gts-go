/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/GlobalTypeSystem/gts-go/gts"
)

// Server represents the GTS HTTP server
type Server struct {
	store   *gts.GtsStore
	host    string
	port    int
	verbose int
	mux     *http.ServeMux
}

// NewServer creates a new GTS HTTP server
func NewServer(store *gts.GtsStore, host string, port int, verbose int) *Server {
	s := &Server{
		store:   store,
		host:    host,
		port:    port,
		verbose: verbose,
		mux:     http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// registerRoutes registers all HTTP routes
func (s *Server) registerRoutes() {
	// Entity management
	s.mux.HandleFunc("GET /entities", s.handleGetEntities)
	s.mux.HandleFunc("GET /entities/{id}", s.handleGetEntity)
	s.mux.HandleFunc("POST /entities", s.handleAddEntity)
	s.mux.HandleFunc("POST /entities/bulk", s.handleAddEntities)
	s.mux.HandleFunc("POST /schemas", s.handleAddSchema)

	// OP#1 - Validate ID
	s.mux.HandleFunc("GET /validate-id", s.handleValidateID)

	// OP#2 - Extract ID
	s.mux.HandleFunc("POST /extract-id", s.handleExtractID)

	// OP#3 - Parse ID
	s.mux.HandleFunc("GET /parse-id", s.handleParseID)

	// OP#4 - Match ID Pattern
	s.mux.HandleFunc("GET /match-id-pattern", s.handleMatchIDPattern)

	// OP#5 - UUID
	s.mux.HandleFunc("GET /uuid", s.handleUUID)

	// OP#6 - Validate Instance
	s.mux.HandleFunc("POST /validate-instance", s.handleValidateInstance)

	// OP#7 - Resolve Relationships
	s.mux.HandleFunc("GET /resolve-relationships", s.handleResolveRelationships)

	// OP#8 - Compatibility
	s.mux.HandleFunc("GET /compatibility", s.handleCompatibility)

	// OP#9 - Cast
	s.mux.HandleFunc("POST /cast", s.handleCast)

	// OP#10 - Query
	s.mux.HandleFunc("GET /query", s.handleQuery)

	// OP#11 - Attribute Access
	s.mux.HandleFunc("GET /attr", s.handleAttribute)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	log.Printf("Starting GTS server on http://%s", addr)

	handler := s.withLogging(s.mux)
	return http.ListenAndServe(addr, handler)
}

// Helper methods

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (s *Server) getQueryParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

func (s *Server) getQueryParamInt(r *http.Request, key string, defaultValue int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return intVal
}

// GetOpenAPISpec returns a basic OpenAPI specification for the server
func (s *Server) GetOpenAPISpec() map[string]any {
	return map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":       "GTS Server",
			"version":     "0.1.0",
			"description": "GTS (Global Type System) HTTP API",
		},
		"servers": []map[string]any{
			{
				"url":         fmt.Sprintf("http://%s:%d", s.host, s.port),
				"description": "GTS Server",
			},
		},
		"paths": map[string]any{
			"/entities": map[string]any{
				"get": map[string]any{
					"summary":     "Get all entities in the registry",
					"operationId": "getEntities",
					"parameters": []map[string]any{
						{
							"name":        "limit",
							"in":          "query",
							"description": "Maximum number of entities to return",
							"schema":      map[string]any{"type": "integer", "default": 100},
						},
					},
				},
				"post": map[string]any{
					"summary":     "Register a single entity (object or schema)",
					"operationId": "addEntity",
				},
			},
			"/validate-id": map[string]any{
				"get": map[string]any{
					"summary":     "Validate a GTS ID format",
					"operationId": "validateID",
					"parameters": []map[string]any{
						{
							"name":        "gts_id",
							"in":          "query",
							"description": "GTS ID to validate",
							"required":    true,
							"schema":      map[string]any{"type": "string"},
						},
					},
				},
			},
			"/parse-id": map[string]any{
				"get": map[string]any{
					"summary":     "Parse a GTS ID into its components",
					"operationId": "parseID",
					"parameters": []map[string]any{
						{
							"name":        "gts_id",
							"in":          "query",
							"description": "GTS ID to parse",
							"required":    true,
							"schema":      map[string]any{"type": "string"},
						},
					},
				},
			},
			"/match-id-pattern": map[string]any{
				"get": map[string]any{
					"summary":     "Match a GTS ID against a pattern",
					"operationId": "matchIDPattern",
				},
			},
			"/uuid": map[string]any{
				"get": map[string]any{
					"summary":     "Generate UUID from a GTS ID",
					"operationId": "uuid",
				},
			},
			"/validate-instance": map[string]any{
				"post": map[string]any{
					"summary":     "Validate an instance against its schema",
					"operationId": "validateInstance",
				},
			},
			"/resolve-relationships": map[string]any{
				"get": map[string]any{
					"summary":     "Resolve relationships for an entity",
					"operationId": "resolveRelationships",
				},
			},
			"/compatibility": map[string]any{
				"get": map[string]any{
					"summary":     "Check compatibility between two schemas",
					"operationId": "compatibility",
				},
			},
			"/cast": map[string]any{
				"post": map[string]any{
					"summary":     "Cast an instance to a target schema",
					"operationId": "cast",
				},
			},
			"/query": map[string]any{
				"get": map[string]any{
					"summary":     "Query entities using an expression",
					"operationId": "query",
				},
			},
			"/attr": map[string]any{
				"get": map[string]any{
					"summary":     "Get attribute value from a GTS entity",
					"operationId": "attr",
				},
			},
		},
	}
}
