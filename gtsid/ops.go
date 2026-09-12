/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gtsid

import (
	"fmt"
)

// ValidationResult represents the result of GTS ID validation
type ValidationResult struct {
	ID         string `json:"id"`
	Valid      bool   `json:"valid"`
	IsType     bool   `json:"is_type"`
	IsWildcard bool   `json:"is_wildcard"`
	Error      string `json:"error,omitempty"`
}

// Validate validates a GTS identifier and returns a result
func Validate(gtsID string) *ValidationResult {
	// Check if it contains wildcards first
	isWildcard := HasWildcard(gtsID)
	result := &ValidationResult{
		ID:         gtsID,
		IsWildcard: isWildcard,
	}

	if isWildcard {
		// Validate as wildcard pattern
		_, err := ValidateWildcard(gtsID)
		if err != nil {
			result.Valid = false
			result.IsType = false
			result.Error = formatValidateError(gtsID, err)
			return result
		}

		result.Valid = true
		result.IsType = EndsWithWildcardSuffix(gtsID)
		return result
	}

	// Validate as regular GTS ID
	id, err := New(gtsID)
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

// UUIDResult represents the result of GTS ID to UUID conversion
type UUIDResult struct {
	ID    string `json:"id"`
	UUID  string `json:"uuid"`
	Error string `json:"error"`
}

// IDToUUID converts a GTS ID to a UUID.
// For combined anonymous instances (IDs ending with a UUID tail segment),
// the embedded UUID is returned directly. For all other IDs, a deterministic
// UUID5 is computed from the GTS namespace.
func IDToUUID(gtsID string) *UUIDResult {
	id, err := New(gtsID)
	if err != nil {
		return &UUIDResult{
			ID:    gtsID,
			UUID:  "",
			Error: err.Error(),
		}
	}

	// Combined anonymous instances carry their UUID in the last segment.
	if segs := id.Segments; len(segs) > 0 && segs[len(segs)-1].IsUUID {
		return &UUIDResult{
			ID:   gtsID,
			UUID: segs[len(segs)-1].Segment,
		}
	}

	uuid := id.ToUUID()
	return &UUIDResult{
		ID:    gtsID,
		UUID:  uuid.String(),
		Error: "",
	}
}
