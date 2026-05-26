/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"strings"
)

// IDValidationResult represents the result of GTS ID validation
type IDValidationResult struct {
	ID         string `json:"id"`
	Valid      bool   `json:"valid"`
	IsType     bool   `json:"is_type"`
	IsWildcard bool   `json:"is_wildcard"`
	Error      string `json:"error,omitempty"`
}

// ValidateGtsID validates a GTS identifier and returns a result
func ValidateGtsID(gtsID string) *IDValidationResult {
	// Check if it contains wildcards first
	isWildcard := strings.Contains(gtsID, "*")
	result := &IDValidationResult{
		ID:         gtsID,
		IsWildcard: isWildcard,
	}

	if isWildcard {
		// Validate as wildcard pattern
		_, err := validateWildcard(gtsID)
		if err != nil {
			result.Valid = false
			result.IsType = false
			result.Error = formatValidateError(gtsID, err)
			return result
		}

		result.Valid = true
		result.IsType = strings.HasSuffix(gtsID, "~*") || strings.HasSuffix(gtsID, ".*")
		return result
	}

	// Validate as regular GTS ID
	id, err := NewGtsID(gtsID)
	if err != nil {
		result.Valid = false
		result.IsType = false
		result.Error = formatValidateError(gtsID, err)
		return result
	}

	result.Valid = true
	result.IsType = id.IsType()
	return result
}

func formatValidateError(gtsID string, err error) string {
	return fmt.Sprintf("Unable to validate GTS ID '%s': %s", gtsID, err.Error())
}

// ExtractGtsID extracts GTS ID from JSON content
func ExtractGtsID(content map[string]any, cfg *GtsConfig) *ExtractIDResult {
	return ExtractID(content, cfg)
}

// ParseGtsID parses a GTS identifier into its components
func ParseGtsID(gtsID string) ParseIDResult {
	return ParseID(gtsID)
}

// UUIDResult represents the result of GTS ID to UUID conversion
type UUIDResult struct {
	ID    string `json:"id"`
	UUID  string `json:"uuid"`
	Error string `json:"error"`
}

// IDToUUID converts a GTS ID to a UUID
func IDToUUID(gtsID string) *UUIDResult {
	id, err := NewGtsID(gtsID)
	if err != nil {
		return &UUIDResult{
			ID:    gtsID,
			UUID:  "",
			Error: err.Error(),
		}
	}

	uuid := id.ToUUID()
	return &UUIDResult{
		ID:    gtsID,
		UUID:  uuid.String(),
		Error: "",
	}
}
