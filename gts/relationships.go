/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

// SchemaGraphNode represents a node in the schema relationship graph
type SchemaGraphNode struct {
	ID     string                      `json:"id"`
	Refs   map[string]*SchemaGraphNode `json:"refs,omitempty"`
	TypeID *SchemaGraphNode            `json:"type_id,omitempty"`
	Errors []string                    `json:"errors,omitempty"`
}

// BuildSchemaGraph recursively builds a relationship graph for a GTS entity
// This matches Python's build_schema_graph method in store.py
func (s *GtsStore) BuildSchemaGraph(gtsID string) *SchemaGraphNode {
	seen := make(map[string]bool)
	return s.buildNode(gtsID, seen)
}

// buildNode recursively builds a single node in the graph
func (s *GtsStore) buildNode(gtsID string, seen map[string]bool) *SchemaGraphNode {
	node := &SchemaGraphNode{
		ID: gtsID,
	}

	// Check for cycles
	if seen[gtsID] {
		return node
	}
	seen[gtsID] = true

	// Get the entity from store
	entity := s.Get(gtsID)
	if entity == nil {
		node.Errors = append(node.Errors, "Entity not found")
		return node
	}

	// Process GTS references found in the entity
	refs := make(map[string]*SchemaGraphNode)
	for _, ref := range entity.GtsRefs {
		// Skip self-references
		if ref.ID == gtsID {
			continue
		}
		// Skip JSON Schema meta-schema references
		if isJSONSchemaURL(ref.ID) {
			continue
		}
		// Recursively build node for this reference
		refs[ref.SourcePath] = s.buildNode(ref.ID, seen)
	}
	if len(refs) > 0 {
		node.Refs = refs
	}

	// Process type ID if present
	if entity.TypeID != "" {
		if !isJSONSchemaURL(entity.TypeID) {
			node.TypeID = s.buildNode(entity.TypeID, seen)
		}
	} else if !entity.IsTypeSchema {
		// Instance without type ID is an error
		node.Errors = append(node.Errors, "Type-schema not recognized")
	}

	return node
}

// isJSONSchemaURL checks if a string is a JSON Schema meta-schema URL
func isJSONSchemaURL(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || s[:8] == "https://") &&
		(len(s) > 22 && s[:23] == "http://json-schema.org" ||
			len(s) > 23 && s[:24] == "https://json-schema.org")
}
