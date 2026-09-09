/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"strings"
)

// MatchIDResult represents the result of matching a GTS identifier against a pattern
type MatchIDResult struct {
	Candidate string `json:"candidate"`
	Pattern   string `json:"pattern"`
	Match     bool   `json:"match"`
	Error     string `json:"error,omitempty"`
}

// InvalidWildcardError represents an error when a wildcard pattern is invalid
type InvalidWildcardError struct {
	Pattern string
	Cause   string
}

func (e *InvalidWildcardError) Error() string {
	if e.Cause != "" {
		return fmt.Sprintf("Invalid GTS wildcard pattern: %s: %s", e.Pattern, e.Cause)
	}
	return fmt.Sprintf("Invalid GTS wildcard pattern: %s", e.Pattern)
}

// MatchIDPattern matches a candidate GTS identifier against a pattern with wildcards
// Returns a MatchIDResult with Match=true if the candidate matches the pattern,
// or Match=false with an optional Error message on failure or mismatch
func MatchIDPattern(candidate, pattern string) MatchIDResult {
	// Parse candidate - it can be either a regular GTS ID or a wildcard pattern
	var candidateID *GtsID
	var err error

	if strings.Contains(candidate, "*") {
		// Candidate contains wildcard, validate it as a wildcard pattern
		candidateID, err = validateWildcard(candidate)
	} else {
		// Candidate is a regular GTS ID
		candidateID, err = NewGtsID(candidate)
	}

	if err != nil {
		return MatchIDResult{
			Candidate: candidate,
			Pattern:   pattern,
			Match:     false,
			Error:     err.Error(),
		}
	}

	// Validate and parse pattern
	patternID, err := validateWildcard(pattern)
	if err != nil {
		return MatchIDResult{
			Candidate: candidate,
			Pattern:   pattern,
			Match:     false,
			Error:     err.Error(),
		}
	}

	// Perform matching
	match := wildcardMatch(candidateID, patternID)

	return MatchIDResult{
		Candidate: candidate,
		Pattern:   pattern,
		Match:     match,
		Error:     "",
	}
}

// validateWildcard validates a wildcard pattern and returns a parsed GtsID
func validateWildcard(pattern string) (*GtsID, error) {
	p := strings.TrimSpace(pattern)

	// Must start with gts.
	if !strings.HasPrefix(p, GtsPrefix) {
		return nil, &InvalidWildcardError{
			Pattern: pattern,
			Cause:   fmt.Sprintf("Does not start with '%s'", GtsPrefix),
		}
	}

	// Count wildcards
	wildcardCount := strings.Count(p, "*")
	if wildcardCount > 1 {
		return nil, &InvalidWildcardError{
			Pattern: pattern,
			Cause:   "The wildcard '*' token is allowed only once",
		}
	}

	// If wildcard exists, must be at the end
	if wildcardCount == 1 {
		if !strings.HasSuffix(p, ".*") && !strings.HasSuffix(p, "~*") {
			return nil, &InvalidWildcardError{
				Pattern: pattern,
				Cause:   "The wildcard '*' token is allowed only at the end of the pattern",
			}
		}
	}

	// For wildcard patterns, we need custom parsing that doesn't enforce single-segment prohibition
	// Remove the wildcard token temporarily for validation
	tempPattern := strings.ReplaceAll(p, ".*", "")
	tempPattern = strings.ReplaceAll(tempPattern, "~*", "~")

	// Try to parse the base pattern (without wildcard) using standard validation
	// but skip single-segment instance check for wildcards
	_, err := validateWildcardBase(tempPattern)
	if err != nil {
		return nil, &InvalidWildcardError{
			Pattern: pattern,
			Cause:   err.Error(),
		}
	}

	// Now parse the full wildcard pattern with relaxed validation
	id, err := parseWildcardGtsID(p)
	if err != nil {
		return nil, &InvalidWildcardError{
			Pattern: pattern,
			Cause:   err.Error(),
		}
	}

	return id, nil
}

// validateWildcardBase validates the base pattern (without wildcards) with relaxed rules
func validateWildcardBase(basePattern string) (*GtsID, error) {
	if basePattern == "" {
		return nil, fmt.Errorf("empty base pattern")
	}

	// Allow bare "gts" base for global wildcard patterns like "gts.*"
	if basePattern == strings.TrimSuffix(GtsPrefix, ".") {
		return nil, nil
	}

	// Basic prefix validation
	if !strings.HasPrefix(basePattern, GtsPrefix) {
		return nil, fmt.Errorf("does not start with '%s'", GtsPrefix)
	}

	// Length validation
	if len(basePattern) > MaxIDLength {
		return nil, fmt.Errorf("too long")
	}

	// Lowercase validation
	if basePattern != strings.ToLower(basePattern) {
		return nil, fmt.Errorf("must be lower case")
	}

	// No hyphens validation
	if strings.Contains(basePattern, "-") {
		return nil, fmt.Errorf("must not contain '-'")
	}

	// For wildcard base validation, we skip the single-segment instance prohibition
	// since wildcards can match complete patterns
	return nil, nil // We don't need to return a parsed ID, just validate
}

// parseWildcardGtsID parses a wildcard GTS ID with relaxed validation rules
func parseWildcardGtsID(id string) (*GtsID, error) {
	raw := strings.TrimSpace(id)

	// Basic validation (same as NewGtsID but skip single-segment check)
	if raw != strings.ToLower(raw) {
		return nil, &InvalidGtsIDError{GtsID: id, Cause: "Must be lower case"}
	}

	if strings.Contains(raw, "-") {
		return nil, &InvalidGtsIDError{GtsID: id, Cause: "Must not contain '-'"}
	}

	if !strings.HasPrefix(raw, GtsPrefix) {
		return nil, &InvalidGtsIDError{GtsID: id, Cause: fmt.Sprintf("Does not start with '%s'", GtsPrefix)}
	}

	if len(raw) > MaxIDLength {
		return nil, &InvalidGtsIDError{GtsID: id, Cause: "Too long"}
	}

	gtsID := &GtsID{
		ID:       raw,
		Segments: make([]*GtsIDSegment, 0),
	}

	// Split by ~ to get segments, preserving empties to detect trailing ~
	remainder := raw[len(GtsPrefix):]
	parts := splitPreservingTilde(remainder)

	offset := len(GtsPrefix)
	for i, part := range parts {
		if part == "" {
			return nil, &InvalidGtsIDError{GtsID: id, Cause: fmt.Sprintf("GTS segment #%d @ offset %d is empty", i+1, offset)}
		}

		segment, err := parseSegment(i+1, offset, part)
		if err != nil {
			return nil, err
		}

		gtsID.Segments = append(gtsID.Segments, segment)
		offset += len(part)
	}

	// Skip single-segment instance prohibition for wildcard patterns
	// Wildcards are allowed to match patterns that would otherwise be invalid

	return gtsID, nil
}

// wildcardMatch performs the actual matching between candidate and pattern
func wildcardMatch(candidate, pattern *GtsID) bool {
	if candidate == nil || pattern == nil {
		return false
	}

	// If no wildcard in pattern, perform exact match with version flexibility
	if !strings.Contains(pattern.ID, "*") {
		return matchSegments(pattern.Segments, candidate.Segments)
	}

	// Wildcard case
	if strings.Count(pattern.ID, "*") > 1 || !strings.HasSuffix(pattern.ID, "*") {
		return false
	}

	// Use segment matching for wildcard patterns too
	return matchSegments(pattern.Segments, candidate.Segments)
}

// isBareWildcard returns true if the segment is a bare wildcard (*) with no
// other fields set — i.e. the segment parsed from a lone "*" token after ~.
func isBareWildcard(seg *GtsIDSegment) bool {
	return seg.IsWildcard && seg.Vendor == "" && seg.Package == "" && seg.Namespace == "" && seg.Type == ""
}

// matchSegments matches pattern segments against candidate segments
func matchSegments(patternSegs, candidateSegs []*GtsIDSegment) bool {
	// If pattern is longer than candidate, allow the last pattern segment to be
	// a bare wildcard that matches zero additional segments (e.g. ~* matching
	// the type itself with no instance segments).
	if len(patternSegs) > len(candidateSegs) {
		if len(patternSegs) == len(candidateSegs)+1 && isBareWildcard(patternSegs[len(patternSegs)-1]) {
			patternSegs = patternSegs[:len(patternSegs)-1]
		} else {
			return false
		}
	}

	for i, pSeg := range patternSegs {
		cSeg := candidateSegs[i]

		// If pattern segment is a wildcard, check non-wildcard fields first
		if pSeg.IsWildcard {
			// Check the fields that are set (non-empty) in the wildcard pattern
			if pSeg.Vendor != "" && pSeg.Vendor != cSeg.Vendor {
				return false
			}
			if pSeg.Package != "" && pSeg.Package != cSeg.Package {
				return false
			}
			if pSeg.Namespace != "" && pSeg.Namespace != cSeg.Namespace {
				return false
			}
			if pSeg.Type != "" && pSeg.Type != cSeg.Type {
				return false
			}
			// Check version fields if they are set in the pattern
			if pSeg.HasVersion && pSeg.VerMajor != cSeg.VerMajor {
				return false
			}
			if pSeg.VerMinor != nil && (cSeg.VerMinor == nil || *pSeg.VerMinor != *cSeg.VerMinor) {
				return false
			}
			// Check is_type flag if set
			if pSeg.IsType && pSeg.IsType != cSeg.IsType {
				return false
			}
			// Wildcard matches - accept anything after this point
			return true
		}

		// Non-wildcard segment - all fields must match
		if pSeg.Vendor != cSeg.Vendor {
			return false
		}
		if pSeg.Package != cSeg.Package {
			return false
		}
		if pSeg.Namespace != cSeg.Namespace {
			return false
		}
		if pSeg.Type != cSeg.Type {
			return false
		}

		// Check version matching
		// Major version must match
		if pSeg.VerMajor != cSeg.VerMajor {
			return false
		}

		// Minor version: if pattern has no minor version, accept any minor in candidate
		// If pattern has minor version, it must match exactly
		if pSeg.VerMinor != nil {
			if cSeg.VerMinor == nil || *pSeg.VerMinor != *cSeg.VerMinor {
				return false
			}
		}
		// else: pattern has no minor version, so any minor version in candidate is OK

		// Check is_type flag matches
		if pSeg.IsType != cSeg.IsType {
			return false
		}
	}

	// If we've matched all pattern segments, it's a match
	return true
}
