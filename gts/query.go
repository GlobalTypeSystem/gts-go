/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"errors"
	"fmt"
	"strings"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
)

// QueryResult represents the result of a GTS query execution
type QueryResult struct {
	Error   string           `json:"error"`
	Count   int              `json:"count"`
	Limit   int              `json:"limit"`
	Results []map[string]any `json:"results"`
}

// Query filters entities by a GTS query expression
// Supports:
// - Exact match: "gts.x.core.events.event.v1~"
// - Wildcard match: "gts.x.core.events.*"
// - With filters: "gts.x.core.events.event.v1~[status=active]"
// - Wildcard with filters: "gts.x.core.*[status=active]"
// - Wildcard filter values: "gts.x.core.*[status=active, category=*]"
// see gts-python store.py query method
func (s *GtsStore) Query(expr string, limit int) *QueryResult {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	result := &QueryResult{
		Error:   "",
		Count:   0,
		Limit:   limit,
		Results: make([]map[string]any, 0),
	}

	// Parse the query expression to extract base pattern and filters
	basePattern, filters, err := s.parseQueryExpression(expr)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Determine if pattern is wildcard
	isWildcard := gtsid.HasWildcard(basePattern)

	// Validate the pattern
	if err := s.validateQueryPattern(basePattern, isWildcard); err != nil {
		result.Error = err.Error()
		return result
	}

	// Filter entities
	for _, entity := range s.byID {
		if len(result.Results) >= limit {
			break
		}

		// Skip entities without valid content or GTS ID
		if len(entity.Content) == 0 || entity.GtsID == nil {
			continue
		}

		// Check if ID matches the pattern
		if !s.matchesIDPattern(entity.GtsID, basePattern, isWildcard) {
			continue
		}

		// Check filters
		if !s.matchesFilters(entity.Content, filters) {
			continue
		}

		result.Results = append(result.Results, entity.Content)
	}

	result.Count = len(result.Results)
	return result
}

// parseQueryExpression parses the query expression into base pattern and filters
// see gts-python store.py query method
func (s *GtsStore) parseQueryExpression(expr string) (string, map[string]string, error) {
	// Split by '[' to separate base pattern from filters
	parts := strings.SplitN(expr, "[", 2)
	basePattern := strings.TrimSpace(parts[0])

	filters := make(map[string]string)
	if len(parts) == 2 {
		// Extract filter string (remove trailing ])
		filterStr := strings.TrimSpace(parts[1])
		if !strings.HasSuffix(filterStr, "]") {
			return "", nil, errors.New("Invalid query: missing closing bracket ']'")
		}
		filterStr = strings.TrimSuffix(filterStr, "]")

		// Check if base pattern ends with ~ or ~* (type ID/pattern) - filters not allowed on type queries
		if gtsid.IsTypePattern(basePattern) {
			return "", nil, errors.New("Invalid query: filters cannot be used with type patterns (ending with ~ or ~*)")
		}

		// Parse filters
		filters = s.parseQueryFilters(filterStr)
	}

	return basePattern, filters, nil
}

// parseQueryFilters parses filter expressions from query string
// see gts-python store.py _parse_query_filters method
func (s *GtsStore) parseQueryFilters(filterStr string) map[string]string {
	filters := make(map[string]string)
	if filterStr == "" {
		return filters
	}

	// Split by comma to handle multiple filters
	parts := strings.Split(filterStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			// Remove quotes from value if present
			value = strings.Trim(value, `"'`)

			filters[key] = value
		}
	}

	return filters
}

// validateQueryPattern validates the query pattern
// see gts-python store.py _validate_query_pattern method
func (s *GtsStore) validateQueryPattern(basePattern string, isWildcard bool) error {
	if isWildcard {
		// Wildcard pattern must end with .* or ~*
		if !gtsid.EndsWithWildcardSuffix(basePattern) {
			return errors.New("Invalid query: wildcard patterns must end with .* or ~*")
		}

		// Validate as wildcard pattern
		_, err := gtsid.ValidateWildcard(basePattern)
		if err != nil {
			return fmt.Errorf("Invalid query: %w", err)
		}
	} else {
		// Non-wildcard pattern must be a complete valid GTS ID
		gtsID, err := gtsid.New(basePattern)
		if err != nil {
			return fmt.Errorf("Invalid query: %w", err)
		}

		// Must have at least one valid segment
		if len(gtsID.Segments) == 0 {
			return errors.New("Invalid query: GTS ID has no valid segments")
		}

		// Check if pattern is incomplete (missing version or type)
		// A complete GTS ID must end with a version (v1, v1.2) or ~ for types
		lastSeg := gtsID.Segments[len(gtsID.Segments)-1]
		if !lastSeg.IsType && !lastSeg.HasVersion {
			return errors.New("Invalid query: incomplete GTS ID pattern")
		}
	}

	return nil
}

// matchesIDPattern checks if entity ID matches the query pattern
// see gts-python store.py _matches_id_pattern method
func (s *GtsStore) matchesIDPattern(entityID *gtsid.ID, basePattern string, isWildcard bool) bool {
	if entityID == nil {
		return false
	}

	// Use the existing gtsid.Match function
	matchResult := gtsid.Match(entityID.ID, basePattern)
	if !matchResult.Match {
		return false
	}

	// For ~* query patterns, exclude entities that are themselves base types
	// (same segment count as the pattern minus the wildcard). The ~* query
	// semantically finds derived/instance entities, not the type itself.
	if strings.HasSuffix(basePattern, gtsid.TypeWildcardSuffix) {
		patternID, err := gtsid.ValidateWildcard(basePattern)
		if err == nil {
			// The pattern has N segments where the last is the bare wildcard.
			// Only include entities that have more segments than N-1 (the base type).
			baseSegCount := len(patternID.Segments) - 1 // exclude the wildcard segment
			if len(entityID.Segments) <= baseSegCount {
				return false
			}
		}
	}

	return true
}

// matchesFilters checks if entity content matches all filter criteria
// see gts-python store.py _matches_filters method
func (s *GtsStore) matchesFilters(entityContent map[string]any, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}

	for key, value := range filters {
		entityValue := fmt.Sprintf("%v", entityContent[key])

		// Support wildcard in filter values
		if value == "*" {
			// Wildcard matches any non-empty value
			if entityValue == "" || entityValue == "<nil>" {
				return false
			}
		} else if entityValue != value {
			return false
		}
	}

	return true
}
