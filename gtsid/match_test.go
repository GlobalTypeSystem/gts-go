/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gtsid

import "testing"

// TestMatchIDPattern_Positive1 tests basic wildcard matching with chained identifiers
func TestMatchIDPattern_Positive1(t *testing.T) {
	result := Match(
		"gts.x.test4.events.type.v1~abc.app._.custom_event.v1.2",
		"gts.x.test4.events.type.v1~abc.*",
	)

	if !result.Match {
		t.Errorf("Expected match=true, got match=false with error: %s", result.Error)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
}

// TestMatchIDPattern_Positive2 tests wildcard matching with derived types
func TestMatchIDPattern_Positive2(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		pattern   string
		match     bool
	}{
		{
			name:      "Type identifier with ~* matches",
			candidate: "gts.vendor.pkg.ns.type.v0~",
			pattern:   "gts.vendor.pkg.ns.type.v0~*",
			match:     true,
		},
		{
			name:      "Derived instance with ~* matches",
			candidate: "gts.vendor.pkg.ns.type.v0~a.b.c.d.v1",
			pattern:   "gts.vendor.pkg.ns.type.v0~*",
			match:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(tt.candidate, tt.pattern)

			if result.Match != tt.match {
				t.Errorf("Expected match=%v, got match=%v (error: %s)", tt.match, result.Match, result.Error)
			}
		})
	}
}

// TestMatchIDPattern_Positive3 tests wildcard matching with different minor versions
func TestMatchIDPattern_Positive3(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		pattern   string
		match     bool
	}{
		{
			name:      "Type with different minor version matches ~*",
			candidate: "gts.vendor.pkg.ns.type.v0.1~",
			pattern:   "gts.vendor.pkg.ns.type.v0~*",
			match:     true,
		},
		{
			name:      "Derived instance with different minor version matches ~*",
			candidate: "gts.vendor.pkg.ns.type.v0.1~a.b.c.d.v1",
			pattern:   "gts.vendor.pkg.ns.type.v0~*",
			match:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(tt.candidate, tt.pattern)

			if result.Match != tt.match {
				t.Errorf("Expected match=%v, got match=%v (error: %s)", tt.match, result.Match, result.Error)
			}
		})
	}
}

// TestMatchIDPattern_VersionWildcards tests version wildcard matching
func TestMatchIDPattern_VersionWildcards(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		candidate string
		match     bool
	}{
		{
			name:      "Type with major version matches any minor",
			pattern:   "gts.x.pkg.ns.type.v1~",
			candidate: "gts.x.pkg.ns.type.v1.5~",
			match:     true,
		},
		{
			name:      "Chained type with wildcard matches any minor",
			pattern:   "gts.x.pkg.ns.type.v1~a.b.c.*",
			candidate: "gts.x.pkg.ns.type.v1.5~a.b.c.d.v1",
			match:     true,
		},
		{
			name:      "Chained instance with wildcard matches any minor",
			pattern:   "gts.x.pkg.ns.type.v1~a.b.c.d.v1",
			candidate: "gts.x.pkg.ns.type.v1.5~a.b.c.d.v1.2",
			match:     true,
		},
		{
			name:      "Specific minor version matches exactly",
			pattern:   "gts.x.pkg.ns.type.v1.2~",
			candidate: "gts.x.pkg.ns.type.v1.2~",
			match:     true,
		},
		{
			name:      "Different major versions do not match",
			pattern:   "gts.x.pkg.ns.type.v1~",
			candidate: "gts.x.pkg.ns.type.v2~",
			match:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(tt.candidate, tt.pattern)

			if result.Match != tt.match {
				t.Errorf("Expected match=%v, got match=%v (error: %s)", tt.match, result.Match, result.Error)
			}
		})
	}
}

// TestMatchIDPattern_ChainedPatterns tests chained pattern matching
func TestMatchIDPattern_ChainedPatterns(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		candidate   string
		match       bool
		expectError bool
	}{
		{
			name:        "Match base with wildcard derived type",
			pattern:     "gts.x.test4.events.type.v1~abc.*",
			candidate:   "gts.x.test4.events.type.v1~abc.app._.custom.v1~",
			match:       true,
			expectError: false,
		},
		{
			name:        "Wildcard in chain middle is invalid",
			pattern:     "gts.x.*.events.type.v1~",
			candidate:   "gts.x.test4.events.type.v1~",
			match:       false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(tt.candidate, tt.pattern)

			if result.Match != tt.match {
				t.Errorf("Expected match=%v, got match=%v", tt.match, result.Match)
			}

			if tt.expectError && result.Error == "" {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && result.Error != "" {
				t.Errorf("Expected no error but got: %s", result.Error)
			}
		})
	}
}

// TestMatchIDPattern_MultiLevelWildcards tests multi-level wildcard patterns
func TestMatchIDPattern_MultiLevelWildcards(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		candidate   string
		match       bool
		expectError bool
	}{
		{
			name:        "Wildcard vendor and type is invalid",
			pattern:     "gts.*.pkg.ns.*",
			candidate:   "gts.vendor.pkg.ns.type.v1~",
			match:       false,
			expectError: true,
		},
		{
			name:        "Wildcard all except vendor",
			pattern:     "gts.myvendor.*",
			candidate:   "gts.myvendor.pkg.ns.type.v1.0~",
			match:       true,
			expectError: false,
		},
		{
			name:        "Match all types in namespace",
			pattern:     "gts.x.pkg.events.*",
			candidate:   "gts.x.pkg.events.order_placed.v1~",
			match:       true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(tt.candidate, tt.pattern)

			if result.Match != tt.match {
				t.Errorf("Expected match=%v, got match=%v", tt.match, result.Match)
			}

			if tt.expectError && result.Error == "" {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && result.Error != "" {
				t.Errorf("Expected no error but got: %s", result.Error)
			}
		})
	}
}

// TestMatchIDPattern_Negative tests cases that should not match
func TestMatchIDPattern_Negative(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		pattern   string
	}{
		{
			name:      "Different major version",
			candidate: "gts.x.test4.events.type.v1~abc.app._.custom_event.v1.3",
			pattern:   "gts.x.test4.events.type.v2~abc.*",
		},
		{
			name:      "Different major version in base",
			candidate: "gts.vendor.pkg.ns.type.v1.1~",
			pattern:   "gts.vendor.pkg.ns.type.v0~*",
		},
		{
			name:      "Pattern without wildcard does not match chained",
			candidate: "gts.x.test4.events.type.v1~abc.app._.custom_event.v1.2",
			pattern:   "gts.x.test4.events.type.v1~abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(tt.candidate, tt.pattern)

			if result.Match {
				t.Errorf("Expected match=false, got match=true")
			}
		})
	}
}

// TestMatchIDPattern_Invalid tests invalid patterns
func TestMatchIDPattern_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		pattern   string
	}{
		{
			name:      "Pattern with uppercase",
			candidate: "gts.x.test4.events.type.v1~abc.app._.custom_event.v1.2",
			pattern:   "GTS.vendor.pkg.ns.type.v0.*",
		},
		{
			name:      "Wildcard not at end",
			candidate: "gts.vendor.pkg.ns.type.v0~",
			pattern:   "gts.x.test4.events.type.v1*abc",
		},
		{
			name:      "Pattern too short",
			candidate: "gts.vendor.pkg.ns.type.v0~",
			pattern:   "gts.x.test4.events.type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(tt.candidate, tt.pattern)

			if result.Match {
				t.Errorf("Expected match=false for invalid pattern")
			}

			if result.Error == "" {
				t.Error("Expected error for invalid pattern but got none")
			}
		})
	}
}

// TestMatchIDPattern_ExactMatch tests exact matching without wildcards
func TestMatchIDPattern_ExactMatch(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		pattern   string
		match     bool
	}{
		{
			name:      "Exact match with same version",
			candidate: "gts.vendor.pkg.ns.type.v1~",
			pattern:   "gts.vendor.pkg.ns.type.v1~",
			match:     true,
		},
		{
			name:      "Exact match with full version",
			candidate: "gts.vendor.pkg.ns.type.v1.2~a.b.c.d.v1",
			pattern:   "gts.vendor.pkg.ns.type.v1.2~a.b.c.d.v1",
			match:     true,
		},
		{
			name:      "Pattern without minor accepts candidate with minor",
			candidate: "gts.vendor.pkg.ns.type.v1.5~",
			pattern:   "gts.vendor.pkg.ns.type.v1~",
			match:     true,
		},
		{
			name:      "Pattern with minor requires exact minor match",
			candidate: "gts.vendor.pkg.ns.type.v1.5~",
			pattern:   "gts.vendor.pkg.ns.type.v1.2~",
			match:     false,
		},
		{
			name:      "Different namespaces do not match",
			candidate: "gts.vendor.pkg.ns1.type.v1~",
			pattern:   "gts.vendor.pkg.ns2.type.v1~",
			match:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(tt.candidate, tt.pattern)

			if result.Match != tt.match {
				t.Errorf("Expected match=%v, got match=%v (error: %s)", tt.match, result.Match, result.Error)
			}
		})
	}
}

// TestMatchIDPattern_WildcardValidation tests wildcard pattern validation
func TestMatchIDPattern_WildcardValidation(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		expectError bool
	}{
		{
			name:        "Valid wildcard at end",
			pattern:     "gts.vendor.pkg.ns.*",
			expectError: false,
		},
		{
			name:        "Valid wildcard after tilde",
			pattern:     "gts.vendor.pkg.ns.type.v1~*",
			expectError: false,
		},
		{
			name:        "Multiple wildcards",
			pattern:     "gts.*.pkg.*.type.v1~",
			expectError: true,
		},
		{
			name:        "Wildcard in middle",
			pattern:     "gts.vendor.*.pkg.type.v1~",
			expectError: true,
		},
		{
			name:        "No gts prefix",
			pattern:     "vendor.pkg.ns.*",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match("gts.vendor.pkg.ns.type.v1~", tt.pattern)

			if tt.expectError && result.Error == "" {
				t.Error("Expected error for invalid pattern but got none")
			}

			if !tt.expectError && result.Error != "" {
				t.Errorf("Expected no error but got: %s", result.Error)
			}
		})
	}
}

// TestMatchIDPattern_ChainedIdentifiers tests matching with chained identifiers
func TestMatchIDPattern_ChainedIdentifiers(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		pattern   string
		match     bool
	}{
		{
			name:      "Single segment matches prefix wildcard",
			candidate: "gts.vendor.pkg.ns.type.v1~",
			pattern:   "gts.vendor.*",
			match:     true,
		},
		{
			name:      "Two segments match first segment wildcard",
			candidate: "gts.vendor.pkg.ns.type.v1~derived.pkg.ns.type.v1~",
			pattern:   "gts.vendor.*",
			match:     true,
		},
		{
			name:      "Three segments match two segment pattern",
			candidate: "gts.a.b.c.d.v1~e.f.g.h.v1~i.j.k.l.v1",
			pattern:   "gts.a.b.c.d.v1~e.f.g.h.v1~*",
			match:     true,
		},
		{
			name:      "Pattern longer than candidate",
			candidate: "gts.vendor.pkg.ns.type.v1~",
			pattern:   "gts.vendor.pkg.ns.type.v1~derived.pkg.ns.type.v1~*",
			match:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(tt.candidate, tt.pattern)

			if result.Match != tt.match {
				t.Errorf("Expected match=%v, got match=%v (error: %s)", tt.match, result.Match, result.Error)
			}
		})
	}
}

// TestMatchIDPattern_InvalidCandidate tests error handling for invalid candidates
func TestMatchIDPattern_InvalidCandidate(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		pattern   string
	}{
		{
			name:      "Candidate with uppercase",
			candidate: "GTS.vendor.pkg.ns.type.v1~",
			pattern:   "gts.vendor.*",
		},
		{
			name:      "Candidate too short",
			candidate: "gts.vendor.pkg",
			pattern:   "gts.vendor.*",
		},
		{
			name:      "Candidate with hyphen",
			candidate: "gts.vendor-name.pkg.ns.type.v1~",
			pattern:   "gts.vendor-name.*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Match(tt.candidate, tt.pattern)

			if result.Match {
				t.Error("Expected match=false for invalid candidate")
			}

			if result.Error == "" {
				t.Error("Expected error for invalid candidate but got none")
			}
		})
	}
}

// TestWildcardMatch_DirectCall tests the wildcardMatch function directly
// to ensure defensive validation even if ValidateWildcard is bypassed
func TestWildcardMatch_DirectCall(t *testing.T) {
	// Create a valid candidate
	candidateID, _ := New("gts.vendor.pkg.ns.type.v1~")

	tests := []struct {
		name      string
		patternID *ID
		match     bool
	}{
		{
			name: "Pattern with multiple wildcards returns false",
			patternID: &ID{
				ID:       "gts.*.pkg.*.type.v1~",
				Segments: []*Segment{{IsWildcard: true}},
			},
			match: false,
		},
		{
			name: "Pattern with wildcard not at end returns false",
			patternID: &ID{
				ID:       "gts.vendor*pkg.ns.type.v1~",
				Segments: []*Segment{{Vendor: "vendor"}},
			},
			match: false,
		},
		{
			name: "Pattern with wildcard at end matches",
			patternID: &ID{
				ID:       "gts.vendor.pkg.ns.*",
				Segments: []*Segment{{Vendor: "vendor", Package: "pkg", Namespace: "ns", IsWildcard: true}},
			},
			match: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := wildcardMatch(candidateID, tt.patternID)
			if match != tt.match {
				t.Errorf("Expected match=%v, got match=%v for pattern '%s'", tt.match, match, tt.patternID.ID)
			}
		})
	}
}
