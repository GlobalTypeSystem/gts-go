/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gtsid

import "fmt"

// MatchResult represents the result of matching a GTS identifier against a pattern
type MatchResult struct {
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

// Match matches a candidate GTS identifier against a pattern (README §10, OP#4).
//
// The pattern side is always a pattern; a concrete id is a zero-wildcard
// pattern. A concrete candidate matches when it is selected by the pattern
// (M(pattern) membership); a wildcard candidate matches when the pattern covers
// its whole selection set. An invalid candidate or pattern yields Match=false
// with a non-empty Error.
func Match(candidate, pattern string) MatchResult {
	result := MatchResult{Candidate: candidate, Pattern: pattern}

	patternID, err := ParsePattern(pattern)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	if HasWildcard(candidate) {
		candidateID, err := ParsePattern(candidate)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Match = Covers(patternID, candidateID)
		return result
	}

	// New validates the candidate as spelled (no trimming, §2/§10).
	candidateID, err := New(candidate)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Match = MatchesPattern(candidateID, patternID)
	return result
}

// ValidateWildcard validates a match pattern (see ParsePattern) and returns
// it parsed. Every failure is reported as an *InvalidWildcardError.
func ValidateWildcard(pattern string) (*ID, error) {
	id, err := ParsePattern(pattern)
	if err != nil {
		if _, ok := err.(*InvalidWildcardError); !ok {
			err = &InvalidWildcardError{Pattern: pattern, Cause: err.Error()}
		}
		return nil, err
	}
	return id, nil
}
