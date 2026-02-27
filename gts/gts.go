/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	// GtsPrefix is the required prefix for all GTS identifiers
	GtsPrefix = "gts."
	// GtsURIPrefix is the URI-compatible prefix for GTS identifiers in JSON Schema $id field
	// (e.g., "gts://gts.x.y.z..."). This is ONLY used for JSON Schema serialization/deserialization,
	// not for GTS ID parsing.
	GtsURIPrefix = "gts://"
	// MaxIDLength is the maximum allowed length for a GTS identifier
	MaxIDLength = 1024
)

var (
	// GtsNamespace is the UUID namespace for GTS identifiers
	// Generated as uuid5(NAMESPACE_URL, "gts")
	GtsNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("gts"))
)

var (
	// segmentTokenRegex validates individual tokens in a segment
	// Must start with lowercase letter or underscore, followed by lowercase letters, digits, or underscores
	segmentTokenRegex = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
)

// InvalidGtsIDError represents an error when a GTS identifier is invalid
type InvalidGtsIDError struct {
	GtsID string
	Cause string
}

func (e *InvalidGtsIDError) Error() string {
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

// GtsIDSegment represents a parsed segment of a GTS identifier
type GtsIDSegment struct {
	Num        int
	Offset     int
	Segment    string
	Vendor     string
	Package    string
	Namespace  string
	Type       string
	VerMajor   int
	VerMinor   *int
	IsType     bool
	IsWildcard bool
	IsUUID     bool
}

// GtsID represents a validated GTS identifier
type GtsID struct {
	ID       string
	Segments []*GtsIDSegment
}

// NewGtsID creates and validates a new GTS identifier
func NewGtsID(id string) (*GtsID, error) {
	raw := strings.TrimSpace(id)

	// Validate lowercase
	if raw != strings.ToLower(raw) {
		return nil, &InvalidGtsIDError{GtsID: id, Cause: "Must be lower case"}
	}

	// Validate prefix
	if !strings.HasPrefix(raw, GtsPrefix) {
		return nil, &InvalidGtsIDError{GtsID: id, Cause: fmt.Sprintf("Does not start with '%s'", GtsPrefix)}
	}

	// Validate length
	if len(raw) > MaxIDLength {
		return nil, &InvalidGtsIDError{GtsID: id, Cause: "Too long"}
	}

	// Split by ~ to get segments, preserving empties to detect trailing ~
	remainder := raw[len(GtsPrefix):]
	parts := splitPreservingTilde(remainder)

	// Detect combined anonymous instance: last part is a UUID (no trailing ~)
	hasUUIDTail := len(parts) >= 2 && isUUIDSegment(parts[len(parts)-1])

	// Validate no hyphens in non-UUID portions (reuse already-split parts)
	gtsPartsToCheck := parts
	if hasUUIDTail {
		gtsPartsToCheck = parts[:len(parts)-1]
	}
	for _, p := range gtsPartsToCheck {
		if strings.Contains(p, "-") {
			return nil, &InvalidGtsIDError{GtsID: id, Cause: "Must not contain '-'"}
		}
	}

	gtsID := &GtsID{
		ID:       raw,
		Segments: make([]*GtsIDSegment, 0),
	}

	offset := len(GtsPrefix)
	for i, part := range parts {
		if part == "" {
			return nil, &InvalidGtsIDError{GtsID: id, Cause: fmt.Sprintf("GTS segment #%d @ offset %d is empty", i+1, offset)}
		}

		// Last part is a UUID tail — store as a special segment
		if hasUUIDTail && i == len(parts)-1 {
			gtsID.Segments = append(gtsID.Segments, &GtsIDSegment{
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
	if len(gtsID.Segments) == 1 && !gtsID.IsType() && !gtsID.Segments[0].IsWildcard {
		return nil, &InvalidGtsIDError{GtsID: id, Cause: "Single-segment instances are prohibited. Well-known instances must be chained with a type segment"}
	}

	return gtsID, nil
}

// IsValidGtsID checks if a string is a valid GTS identifier
func IsValidGtsID(s string) bool {
	if !strings.HasPrefix(s, GtsPrefix) {
		return false
	}
	_, err := NewGtsID(s)
	return err == nil
}

// IsType returns true if this identifier represents a type (ends with ~)
func (g *GtsID) IsType() bool {
	return strings.HasSuffix(g.ID, "~")
}

// IsWildcard returns true if this identifier contains wildcard patterns
func (g *GtsID) IsWildcard() bool {
	for _, segment := range g.Segments {
		if segment.IsWildcard {
			return true
		}
	}
	return false
}

// ToUUID generates a deterministic UUID (v5) from the GTS identifier
// The UUID is generated using uuid5(GTS_NAMESPACE, gts_id)
func (g *GtsID) ToUUID() uuid.UUID {
	return uuid.NewSHA1(GtsNamespace, []byte(g.ID))
}

// splitPreservingTilde splits a string by ~ while preserving the ~ at the end of each part
func splitPreservingTilde(s string) []string {
	_parts := strings.Split(s, "~")
	parts := make([]string, 0, len(_parts))

	for i := 0; i < len(_parts); i++ {
		if i < len(_parts)-1 {
			parts = append(parts, _parts[i]+"~")
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
func parseSegment(num, offset int, segment string) (*GtsIDSegment, error) {
	seg := &GtsIDSegment{
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
	if strings.Count(workingSegment, "~") > 0 {
		if strings.Count(workingSegment, "~") > 1 {
			return nil, &InvalidSegmentError{
				Num:     num,
				Offset:  offset,
				Segment: segment,
				Cause:   "Too many '~' characters",
			}
		}
		if strings.HasSuffix(workingSegment, "~") {
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
	tokens := strings.Split(workingSegment, ".")

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
	if !strings.HasSuffix(workingSegment, "*") {
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
		if tokens[0] == "*" {
			seg.IsWildcard = true
			return seg, nil
		}
		seg.Vendor = tokens[0]
	}

	if len(tokens) > 1 {
		if tokens[1] == "*" {
			seg.IsWildcard = true
			return seg, nil
		}
		seg.Package = tokens[1]
	}

	if len(tokens) > 2 {
		if tokens[2] == "*" {
			seg.IsWildcard = true
			return seg, nil
		}
		seg.Namespace = tokens[2]
	}

	if len(tokens) > 3 {
		if tokens[3] == "*" {
			seg.IsWildcard = true
			return seg, nil
		}
		seg.Type = tokens[3]
	}

	// Parse major version
	if len(tokens) > 4 {
		if tokens[4] == "*" {
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
	}

	// Parse minor version
	if len(tokens) > 5 {
		if tokens[5] == "*" {
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
