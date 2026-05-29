/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import "testing"

// TestParseID_TypeOnly tests parsing a type-only identifier
func TestParseID_TypeOnly(t *testing.T) {
	result := ParseID("gts.x.test3.events.type.v1~")

	if !result.OK {
		t.Fatalf("Expected OK=true, got OK=false with error: %s", result.Error)
	}

	if result.ID != "gts.x.test3.events.type.v1~" {
		t.Errorf("Expected ID='gts.x.test3.events.type.v1~', got '%s'", result.ID)
	}

	if len(result.Segments) != 1 {
		t.Fatalf("Expected 1 segment, got %d", len(result.Segments))
	}

	seg := result.Segments[0]

	if seg.Vendor != "x" {
		t.Errorf("Expected vendor='x', got '%s'", seg.Vendor)
	}

	if seg.Package != "test3" {
		t.Errorf("Expected package='test3', got '%s'", seg.Package)
	}

	if seg.Namespace != "events" {
		t.Errorf("Expected namespace='events', got '%s'", seg.Namespace)
	}

	if seg.Type != "type" {
		t.Errorf("Expected type='type', got '%s'", seg.Type)
	}

	if seg.VerMajor != 1 {
		t.Errorf("Expected ver_major=1, got %d", seg.VerMajor)
	}

	if seg.VerMinor != nil {
		t.Errorf("Expected ver_minor=nil, got %d", *seg.VerMinor)
	}

	if !seg.IsType {
		t.Error("Expected is_type=true")
	}
}

// TestParseID_ChainToInstance tests parsing a chained identifier ending in an instance
func TestParseID_ChainToInstance(t *testing.T) {
	result := ParseID("gts.x.test3.events.type.v1~abc.app._.custom_event.v1.2")

	if !result.OK {
		t.Fatalf("Expected OK=true, got OK=false with error: %s", result.Error)
	}

	if len(result.Segments) != 2 {
		t.Fatalf("Expected 2 segments, got %d", len(result.Segments))
	}

	// Check first segment (type)
	seg0 := result.Segments[0]
	if seg0.Vendor != "x" {
		t.Errorf("Segment 0: Expected vendor='x', got '%s'", seg0.Vendor)
	}
	if !seg0.IsType {
		t.Error("Segment 0: Expected is_type=true")
	}

	// Check last segment (instance)
	seg1 := result.Segments[1]
	if seg1.Vendor != "abc" {
		t.Errorf("Segment 1: Expected vendor='abc', got '%s'", seg1.Vendor)
	}
	if seg1.Package != "app" {
		t.Errorf("Segment 1: Expected package='app', got '%s'", seg1.Package)
	}
	if seg1.Namespace != "_" {
		t.Errorf("Segment 1: Expected namespace='_', got '%s'", seg1.Namespace)
	}
	if seg1.Type != "custom_event" {
		t.Errorf("Segment 1: Expected type='custom_event', got '%s'", seg1.Type)
	}
	if seg1.VerMinor == nil || *seg1.VerMinor != 2 {
		if seg1.VerMinor == nil {
			t.Error("Segment 1: Expected ver_minor=2, got nil")
		} else {
			t.Errorf("Segment 1: Expected ver_minor=2, got %d", *seg1.VerMinor)
		}
	}
	if seg1.IsType {
		t.Error("Segment 1: Expected is_type=false")
	}
}

// TestParseID_LongChainToInstance tests parsing a long chain ending in an instance
func TestParseID_LongChainToInstance(t *testing.T) {
	result := ParseID("gts.x.test3.events.type.v1~a.b.c.d.v1~e.f.g.h.v1~i.j.k.l.v1.0")

	if !result.OK {
		t.Fatalf("Expected OK=true, got OK=false with error: %s", result.Error)
	}

	if len(result.Segments) != 4 {
		t.Fatalf("Expected 4 segments, got %d", len(result.Segments))
	}

	// Check last segment
	lastSeg := result.Segments[3]
	if lastSeg.Vendor != "i" {
		t.Errorf("Last segment: Expected vendor='i', got '%s'", lastSeg.Vendor)
	}
	if lastSeg.VerMinor == nil || *lastSeg.VerMinor != 0 {
		if lastSeg.VerMinor == nil {
			t.Error("Last segment: Expected ver_minor=0, got nil")
		} else {
			t.Errorf("Last segment: Expected ver_minor=0, got %d", *lastSeg.VerMinor)
		}
	}
	if lastSeg.IsType {
		t.Error("Last segment: Expected is_type=false")
	}
}

// TestParseID_ChainedTypes tests parsing chained type identifiers
func TestParseID_ChainedTypes(t *testing.T) {
	result := ParseID("gts.x.test3.events.type.v1~abc.app._.custom.v1~")

	if !result.OK {
		t.Fatalf("Expected OK=true, got OK=false with error: %s", result.Error)
	}

	if len(result.Segments) != 2 {
		t.Fatalf("Expected 2 segments, got %d", len(result.Segments))
	}

	if result.Segments[0].Vendor != "x" {
		t.Errorf("Segment 0: Expected vendor='x', got '%s'", result.Segments[0].Vendor)
	}

	if result.Segments[1].Vendor != "abc" {
		t.Errorf("Segment 1: Expected vendor='abc', got '%s'", result.Segments[1].Vendor)
	}

	if !result.Segments[0].IsType {
		t.Error("Segment 0: Expected is_type=true")
	}

	if !result.Segments[1].IsType {
		t.Error("Segment 1: Expected is_type=true")
	}
}

// TestParseID_ChainedWithInstance tests parsing chained types with final instance
func TestParseID_ChainedWithInstance(t *testing.T) {
	result := ParseID("gts.x.test3.events.type.v1~abc.app._.custom.v1~abc.app._.instance.v1.0")

	if !result.OK {
		t.Fatalf("Expected OK=true, got OK=false with error: %s", result.Error)
	}

	if len(result.Segments) != 3 {
		t.Fatalf("Expected 3 segments, got %d", len(result.Segments))
	}

	// Check first segment
	if result.Segments[0].Vendor != "x" {
		t.Errorf("Segment 0: Expected vendor='x', got '%s'", result.Segments[0].Vendor)
	}

	// Check second segment
	if result.Segments[1].Namespace != "_" {
		t.Errorf("Segment 1: Expected namespace='_', got '%s'", result.Segments[1].Namespace)
	}
	if result.Segments[1].VerMinor != nil {
		t.Errorf("Segment 1: Expected ver_minor=nil, got %d", *result.Segments[1].VerMinor)
	}
	if !result.Segments[1].IsType {
		t.Error("Segment 1: Expected is_type=true")
	}

	// Check third segment
	if result.Segments[2].IsType {
		t.Error("Segment 2: Expected is_type=false")
	}
	if result.Segments[2].VerMinor == nil || *result.Segments[2].VerMinor != 0 {
		if result.Segments[2].VerMinor == nil {
			t.Error("Segment 2: Expected ver_minor=0, got nil")
		} else {
			t.Errorf("Segment 2: Expected ver_minor=0, got %d", *result.Segments[2].VerMinor)
		}
	}
}

// TestParseID_VersionComponents tests parsing various version formats
func TestParseID_VersionComponents(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		verMajor int
		verMinor *int
		isType   bool
	}{
		{
			name:     "Major version only",
			id:       "gts.x.pkg.ns.type.v1~",
			verMajor: 1,
			verMinor: nil,
			isType:   true,
		},
		{
			name:     "Major and minor version (type)",
			id:       "gts.x.pkg.ns.type.v2.5~",
			verMajor: 2,
			verMinor: intPtr(5),
			isType:   true,
		},
		{
			name:     "Major and minor version (instance)",
			id:       "gts.x.pkg.ns.type.v2.5~a.b.c.d.v1.0",
			verMajor: 2,
			verMinor: intPtr(5),
			isType:   false,
		},
		{
			name:     "Version zero",
			id:       "gts.x.pkg.ns.type.v0~",
			verMajor: 0,
			verMinor: nil,
			isType:   true,
		},
		{
			name:     "Version zero with minor",
			id:       "gts.x.pkg.ns.type.v0.0~a.b.c.d.v1.0",
			verMajor: 0,
			verMinor: intPtr(0),
			isType:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseID(tt.id)

			if !result.OK {
				t.Fatalf("Expected OK=true, got OK=false with error: %s", result.Error)
			}

			// For type IDs, expect 1 segment; for instance IDs, expect 2 segments
			if tt.isType && len(result.Segments) != 1 {
				t.Fatalf("Expected 1 segment for type ID, got %d", len(result.Segments))
			}
			if !tt.isType && len(result.Segments) != 2 {
				t.Fatalf("Expected 2 segments for instance ID, got %d", len(result.Segments))
			}

			// Version info is always in the first segment
			seg := result.Segments[0]

			if seg.VerMajor != tt.verMajor {
				t.Errorf("Expected ver_major=%d, got %d", tt.verMajor, seg.VerMajor)
			}

			if tt.verMinor == nil && seg.VerMinor != nil {
				t.Errorf("Expected ver_minor=nil, got %d", *seg.VerMinor)
			} else if tt.verMinor != nil && seg.VerMinor == nil {
				t.Errorf("Expected ver_minor=%d, got nil", *tt.verMinor)
			} else if tt.verMinor != nil && seg.VerMinor != nil && *seg.VerMinor != *tt.verMinor {
				t.Errorf("Expected ver_minor=%d, got %d", *tt.verMinor, *seg.VerMinor)
			}

			// Check overall ID type (IsType), not individual segment's IsType
			if result.IsType != tt.isType {
				t.Errorf("Expected is_type=%v, got %v", tt.isType, result.IsType)
			}
		})
	}
}

// TestParseID_NamespaceExtraction tests namespace extraction scenarios
func TestParseID_NamespaceExtraction(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		namespace string
	}{
		{
			name:      "Underscore placeholder",
			id:        "gts.vendor.pkg._.type.v1~",
			namespace: "_",
		},
		{
			name:      "Actual namespace",
			id:        "gts.vendor.pkg.events.type.v1~",
			namespace: "events",
		},
		{
			name:      "Namespace with underscore",
			id:        "gts.vendor.pkg.some_ns.type.v1~",
			namespace: "some_ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseID(tt.id)

			if !result.OK {
				t.Fatalf("Expected OK=true, got OK=false with error: %s", result.Error)
			}

			if len(result.Segments) != 1 {
				t.Fatalf("Expected 1 segment, got %d", len(result.Segments))
			}

			seg := result.Segments[0]

			if seg.Namespace != tt.namespace {
				t.Errorf("Expected namespace='%s', got '%s'", tt.namespace, seg.Namespace)
			}
		})
	}
}

// TestParseID_InvalidIDs tests that invalid IDs return OK=false
func TestParseID_InvalidIDs(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{
			name: "Missing prefix",
			id:   "vendor.pkg.ns.type.v1~",
		},
		{
			name: "Too few tokens",
			id:   "gts.vendor.pkg.v1~",
		},
		{
			name: "Invalid version format",
			id:   "gts.vendor.pkg.ns.type.1~",
		},
		{
			name: "Contains hyphen",
			id:   "gts.vendor.pkg-name.ns.type.v1~",
		},
		{
			name: "Uppercase",
			id:   "gts.Vendor.pkg.ns.type.v1~",
		},
		{
			name: "Empty segment",
			id:   "gts.vendor.pkg.ns.type.v1~~",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseID(tt.id)

			if result.OK {
				t.Errorf("Expected OK=false for invalid ID '%s', but got OK=true", tt.id)
			}

			if result.Error == "" {
				t.Error("Expected non-empty error message")
			}

			if result.Segments != nil {
				t.Errorf("Expected nil segments for invalid ID, got %d segments", len(result.Segments))
			}
		})
	}
}

// TestParseID_AllComponents tests that all component fields are extracted
func TestParseID_AllComponents(t *testing.T) {
	result := ParseID("gts.myvendor.mypackage.mynamespace.mytype.v3.7~a.b.c.d.v1.0")

	if !result.OK {
		t.Fatalf("Expected OK=true, got OK=false with error: %s", result.Error)
	}

	if len(result.Segments) != 2 {
		t.Fatalf("Expected 2 segments for instance ID, got %d", len(result.Segments))
	}

	// Component fields are in the first (type) segment
	seg := result.Segments[0]

	if seg.Vendor != "myvendor" {
		t.Errorf("Expected vendor='myvendor', got '%s'", seg.Vendor)
	}

	if seg.Package != "mypackage" {
		t.Errorf("Expected package='mypackage', got '%s'", seg.Package)
	}

	if seg.Namespace != "mynamespace" {
		t.Errorf("Expected namespace='mynamespace', got '%s'", seg.Namespace)
	}

	if seg.Type != "mytype" {
		t.Errorf("Expected type='mytype', got '%s'", seg.Type)
	}

	if seg.VerMajor != 3 {
		t.Errorf("Expected ver_major=3, got %d", seg.VerMajor)
	}

	if seg.VerMinor == nil || *seg.VerMinor != 7 {
		if seg.VerMinor == nil {
			t.Error("Expected ver_minor=7, got nil")
		} else {
			t.Errorf("Expected ver_minor=7, got %d", *seg.VerMinor)
		}
	}

	// Check overall ID type (IsType), not individual segment's IsType
	// The first segment is a type segment (ends with ~), so seg.IsType is true
	// But the overall ID is an instance (doesn't end with ~), so result.IsType is false
	if result.IsType {
		t.Error("Expected is_type=false for instance ID")
	}
}

// intPtr is a helper function to create a pointer to an int
func intPtr(i int) *int {
	return &i
}
