/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gtsid

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Prefix, URIPrefix, MaxIDLength and the other grammar markers live in
// markers.go, the single source of truth for the GTS identifier grammar.

var (
	// Namespace is the UUID namespace for GTS identifiers
	// Generated as uuid5(NAMESPACE_URL, "gts")
	Namespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("gts"))
)

var (
	// segmentTokenRegex validates individual tokens in a segment
	// Must start with lowercase letter or underscore, followed by lowercase letters, digits, or underscores
	segmentTokenRegex = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
)

// InvalidIDError represents an error when a GTS identifier is invalid
type InvalidIDError struct {
	GtsID string
	Cause string
}

func (e *InvalidIDError) Error() string {
	if e.Cause != "" {
		return fmt.Sprintf("Invalid GTS identifier: %s: %s", e.GtsID, e.Cause)
	}
	return fmt.Sprintf("Invalid GTS identifier: %s", e.GtsID)
}

// InvalidSegmentError represents an error in a specific segment
type InvalidSegmentError struct {
	Num     int
	Offset  int
	Segment string
	Cause   string
}

func (e *InvalidSegmentError) Error() string {
	if e.Cause != "" {
		return fmt.Sprintf("Invalid GTS segment #%d @ offset %d: '%s': %s", e.Num, e.Offset, e.Segment, e.Cause)
	}
	return fmt.Sprintf("Invalid GTS segment #%d @ offset %d: '%s'", e.Num, e.Offset, e.Segment)
}

// Segment represents a parsed segment of a GTS identifier
type Segment struct {
	Num        int
	Offset     int
	Segment    string
	Vendor     string
	Package    string
	Namespace  string
	Type       string
	VerMajor   int
	VerMinor   *int
	HasVersion bool
	IsType     bool
	IsWildcard bool
	IsUUID     bool
}

// ID represents a validated GTS identifier
type ID struct {
	ID       string
	Segments []*Segment
}

// ParseOptions tunes the shared identifier parser (parseGtsID) so the strict
// (New) and relaxed (wildcard) callers can share one implementation
// instead of maintaining several near-duplicate parsers.
type ParseOptions struct {
	// AllowSingleSegment lifts the "single-segment instances are prohibited"
	// rule. Wildcard patterns need this because a pattern like "gts.x.core.*"
	// legitimately has a single (wildcard) segment.
	AllowSingleSegment bool
	// DetectUUIDTail treats a trailing canonical-UUID part as a combined
	// anonymous-instance segment (spec §3.7) rather than a normal segment.
	DetectUUIDTail bool
}

// New creates and validates a new (concrete) GTS identifier.
func New(id string) (*ID, error) {
	return parseGtsID(id, ParseOptions{DetectUUIDTail: true})
}

// parseGtsID is the single, shared GTS identifier parser. All parsing paths
// (strict ids and wildcard patterns) funnel through here; ParseOptions selects
// the behavioral differences.
func parseGtsID(id string, opts ParseOptions) (*ID, error) {
	raw := strings.TrimSpace(id)

	// Validate lowercase
	if raw != strings.ToLower(raw) {
		return nil, &InvalidIDError{GtsID: id, Cause: "Must be lower case"}
	}

	// Validate prefix
	if !HasPrefix(raw) {
		return nil, &InvalidIDError{GtsID: id, Cause: fmt.Sprintf("Does not start with '%s'", Prefix)}
	}

	// Validate length
	if len(raw) > MaxIDLength {
		return nil, &InvalidIDError{GtsID: id, Cause: "Too long"}
	}

	// Split by ~ to get segments, preserving empties to detect trailing ~
	remainder := raw[len(Prefix):]
	parts := splitPreservingTilde(remainder)

	// Detect combined anonymous instance: last part is a UUID (no trailing ~)
	hasUUIDTail := opts.DetectUUIDTail && len(parts) >= 2 && isUUIDSegment(parts[len(parts)-1])

	// Validate no hyphens in non-UUID portions (reuse already-split parts)
	gtsPartsToCheck := parts
	if hasUUIDTail {
		gtsPartsToCheck = parts[:len(parts)-1]
	}
	for _, p := range gtsPartsToCheck {
		if strings.Contains(p, "-") {
			return nil, &InvalidIDError{GtsID: id, Cause: "Must not contain '-'"}
		}
	}

	gtsID := &ID{
		ID:       raw,
		Segments: make([]*Segment, 0),
	}

	offset := len(Prefix)
	for i, part := range parts {
		if part == "" {
			return nil, &InvalidIDError{GtsID: id, Cause: fmt.Sprintf("GTS segment #%d @ offset %d is empty", i+1, offset)}
		}

		// Last part is a UUID tail — store as a special segment
		if hasUUIDTail && i == len(parts)-1 {
			gtsID.Segments = append(gtsID.Segments, &Segment{
				Num:     i + 1,
				Offset:  offset,
				Segment: part,
				IsType:  false,
				IsUUID:  true,
			})
			offset += len(part)
			continue
		}

		segment, err := parseSegment(i+1, offset, part)
		if err != nil {
			return nil, err
		}

		gtsID.Segments = append(gtsID.Segments, segment)
		offset += len(part)
	}

	// Single-segment instances are prohibited
	// Well-known instances must be chained with at least one type segment
	// This check should only apply to non-wildcard, non-type single-segment IDs
	if !opts.AllowSingleSegment && len(gtsID.Segments) == 1 && !gtsID.IsType() && !gtsID.Segments[0].IsWildcard {
		return nil, &InvalidIDError{GtsID: id, Cause: "Single-segment instances are prohibited. Well-known instances must be chained with a type segment"}
	}

	return gtsID, nil
}

// IsValid checks if a string is a valid GTS identifier
func IsValid(s string) bool {
	if !HasPrefix(s) {
		return false
	}
	_, err := New(s)
	return err == nil
}

// IsType returns true if this identifier represents a type (ends with ~)
func (g *ID) IsType() bool {
	return IsTypeID(g.ID)
}

// IsWildcard returns true if this identifier contains wildcard patterns
func (g *ID) IsWildcard() bool {
	for _, segment := range g.Segments {
		if segment.IsWildcard {
			return true
		}
	}
	return false
}

// ToUUID generates a deterministic UUID (v5) from the GTS identifier
// The UUID is generated using uuid5(GTS_NAMESPACE, gts_id)
func (g *ID) ToUUID() uuid.UUID {
	return uuid.NewSHA1(Namespace, []byte(g.ID))
}

// splitPreservingTilde splits a string by ~ while preserving the ~ at the end of each part
func splitPreservingTilde(s string) []string {
	_parts := strings.Split(s, SegmentSep)
	parts := make([]string, 0, len(_parts))

	for i := 0; i < len(_parts); i++ {
		if i < len(_parts)-1 {
			parts = append(parts, _parts[i]+SegmentSep)
			// If next part is empty and this is second to last, we're done
			if i == len(_parts)-2 && _parts[i+1] == "" {
				break
			}
		} else {
			parts = append(parts, _parts[i])
		}
	}

	return parts
}

// parseSegment parses a single segment of a GTS identifier
func parseSegment(num, offset int, segment string) (*Segment, error) {
	seg := &Segment{
		Num:        num,
		Offset:     offset,
		Segment:    strings.TrimSpace(segment),
		VerMajor:   0,
		VerMinor:   nil,
		IsType:     false,
		IsWildcard: false,
	}

	workingSegment := seg.Segment

	// Check for type marker (~)
	if strings.Count(workingSegment, TypeMarker) > 0 {
		if strings.Count(workingSegment, TypeMarker) > 1 {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   "Too many '~' characters",
			}
		}
		if strings.HasSuffix(workingSegment, TypeMarker) {
			seg.IsType = true
			workingSegment = workingSegment[:len(workingSegment)-1]
		} else {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   " '~' must be at the end",
			}
		}
	}

	// Split into tokens
	tokens := strings.Split(workingSegment, TokenSep)

	// Validate token count
	if len(tokens) > 6 {
		return nil, &InvalidSegmentError{
			Num:     num,
			Offset:  offset,
			Segment: segment,
			Cause:   "Too many tokens",
		}
	}

	// If not ending with wildcard, must have at least 5 tokens
	if !strings.HasSuffix(workingSegment, WildcardMarker) {
		if len(tokens) < 5 {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   "Too few tokens",
			}
		}

		// Validate first 4 tokens
		for t := 0; t < 4; t++ {
			if !segmentTokenRegex.MatchString(tokens[t]) {
				return nil, &InvalidSegmentError{
					Num:     num,
					Offset:  offset,
					Segment: segment,
					Cause:   "Invalid segment token: " + tokens[t],
				}
			}
		}
	}

	// Parse tokens
	if len(tokens) > 0 {
		if tokens[0] == WildcardMarker {
			seg.IsWildcard = true
			return seg, nil
		}
		seg.Vendor = tokens[0]
	}

	if len(tokens) > 1 {
		if tokens[1] == WildcardMarker {
			seg.IsWildcard = true
			return seg, nil
		}
		seg.Package = tokens[1]
	}

	if len(tokens) > 2 {
		if tokens[2] == WildcardMarker {
			seg.IsWildcard = true
			return seg, nil
		}
		seg.Namespace = tokens[2]
	}

	if len(tokens) > 3 {
		if tokens[3] == WildcardMarker {
			seg.IsWildcard = true
			return seg, nil
		}
		seg.Type = tokens[3]
	}

	// Parse major version
	if len(tokens) > 4 {
		if tokens[4] == WildcardMarker {
			seg.IsWildcard = true
			return seg, nil
		}

		if !strings.HasPrefix(tokens[4], "v") {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   "Major version must start with 'v'",
			}
		}

		majorStr := tokens[4][1:]
		major, err := strconv.Atoi(majorStr)
		if err != nil {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   "Major version must be an integer",
			}
		}

		if major < 0 {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   "Major version must be >= 0",
			}
		}

		// Verify no leading zeros
		if strconv.Itoa(major) != majorStr {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   "Major version must be an integer",
			}
		}

		seg.VerMajor = major
		seg.HasVersion = true
	}

	// Parse minor version
	if len(tokens) > 5 {
		if tokens[5] == WildcardMarker {
			seg.IsWildcard = true
			return seg, nil
		}

		minor, err := strconv.Atoi(tokens[5])
		if err != nil {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   "Minor version must be an integer",
			}
		}

		if minor < 0 {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   "Minor version must be >= 0",
			}
		}

		// Verify no leading zeros
		if strconv.Itoa(minor) != tokens[5] {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   "Minor version must be an integer",
			}
		}

		seg.VerMinor = &minor
	}

	return seg, nil
}

// isUUIDSegment returns true if s is a valid RFC 4122 UUID (lowercase hex with hyphens)
func isUUIDSegment(s string) bool {
	u, err := uuid.Parse(s)
	if err != nil {
		return false
	}
	if u.Variant() != uuid.RFC4122 {
		return false
	}
	// uuid.Parse accepts URN, braced, and uppercase forms; enforce canonical lowercase hyphenated form
	return s == strings.ToLower(u.String())
}
