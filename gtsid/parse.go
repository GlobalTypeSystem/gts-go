/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gtsid

// ParseSegment represents a parsed segment component from a GTS identifier
type ParseSegment struct {
	Vendor    string `json:"vendor"`
	Package   string `json:"package"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	VerMajor  int    `json:"ver_major"`
	VerMinor  *int   `json:"ver_minor"`
	IsType    bool   `json:"is_type"`
	IsUUID    bool   `json:"is_uuid"`
}

// ParseResult represents the result of parsing a GTS identifier
type ParseResult struct {
	ID         string         `json:"id"`
	OK         bool           `json:"ok"`
	IsWildcard bool           `json:"is_wildcard"`
	IsType     bool           `json:"is_type"`
	Segments   []ParseSegment `json:"segments"`
	Error      string         `json:"error,omitempty"`
}

// Parse decomposes a GTS identifier into its constituent parts
// Returns a ParseResult with OK=true and populated Segments on success,
// or OK=false with an Error message on failure
func Parse(gtsID string) ParseResult {
	isWildcard := HasWildcard(gtsID)

	if isWildcard {
		// Handle wildcard patterns separately
		id, err := ValidateWildcard(gtsID)
		if err != nil {
			return ParseResult{
				ID:         gtsID,
				OK:         false,
				IsWildcard: true,
				IsType:     false,
				Segments:   nil,
				Error:      err.Error(),
			}
		}

		segments := make([]ParseSegment, len(id.Segments))
		for i, seg := range id.Segments {
			segments[i] = ParseSegment{
				Vendor:    seg.Vendor,
				Package:   seg.Package,
				Namespace: seg.Namespace,
				Type:      seg.Type,
				VerMajor:  seg.VerMajor,
				VerMinor:  seg.VerMinor,
				IsType:    seg.IsType,
				IsUUID:    seg.IsUUID,
			}
		}

		// Wildcard patterns ending with .* or ~* are type patterns
		isType := EndsWithWildcardSuffix(gtsID)

		return ParseResult{
			ID:         gtsID,
			OK:         true,
			IsWildcard: true,
			IsType:     isType,
			Segments:   segments,
			Error:      "",
		}
	}

	// Handle regular GTS IDs
	id, err := New(gtsID)
	if err != nil {
		return ParseResult{
			ID:         gtsID,
			OK:         false,
			IsWildcard: false,
			IsType:     false,
			Segments:   nil,
			Error:      err.Error(),
		}
	}

	segments := make([]ParseSegment, len(id.Segments))
	for i, seg := range id.Segments {
		segments[i] = ParseSegment{
			Vendor:    seg.Vendor,
			Package:   seg.Package,
			Namespace: seg.Namespace,
			Type:      seg.Type,
			VerMajor:  seg.VerMajor,
			VerMinor:  seg.VerMinor,
			IsType:    seg.IsType,
			IsUUID:    seg.IsUUID,
		}
	}

	return ParseResult{
		ID:         gtsID,
		OK:         true,
		IsWildcard: false,
		IsType:     id.IsType(),
		Segments:   segments,
		Error:      "",
	}
}
