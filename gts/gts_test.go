/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"testing"
)

// TestGtsID_Valid tests valid GTS identifiers
func TestGtsID_Valid(t *testing.T) {
	validIDs := []string{
		"gts.vendor.package.namespace.type.v0~a.b.c.d.v1",
		"gts.vendor.package.namespace.type.v0.0~a.b.c.d.v1",
		"gts.vendor.package.namespace.type.v1~",
		"gts.vendor.package.namespace.type.v1.5~",
		"gts.vendor_name.package_name.namespace_name.type_name.v0~a.b.c.d.v1",
		"gts.vendor.package.namespace.type.v0~",
		"gts.vendor.package.namespace.type.v0.0~",
		"gts.vendor.package.namespace.type.v10.20~a.b.c.d.v1",
	}

	for _, id := range validIDs {
		t.Run(id, func(t *testing.T) {
			gtsID, err := NewGtsID(id)
			if err != nil {
				t.Errorf("Expected valid ID %q, but got error: %v", id, err)
			}
			if gtsID.ID != id {
				t.Errorf("Expected ID %q, got %q", id, gtsID.ID)
			}
		})
	}
}

// TestGtsID_IsValid tests the IsValid class method
func TestGtsID_IsValid(t *testing.T) {
	tests := []struct {
		id       string
		expected bool
	}{
		{"gts.vendor.package.namespace.type.v0~a.b.c.d.v1", true},
		{"gts.vendor.package.namespace.type.v0.0~a.b.c.d.v1", true},
		{"invalid.prefix.package.namespace.type.v0", false},
		{"GTS.vendor.package.namespace.type.v0", false},
		{"gts.vendor.package.namespace", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			result := IsValidGtsID(tt.id)
			if result != tt.expected {
				t.Errorf("IsValidGtsID(%q) = %v, want %v", tt.id, result, tt.expected)
			}
		})
	}
}

// TestGtsID_InvalidPrefix tests IDs without 'gts.' prefix
func TestGtsID_InvalidPrefix(t *testing.T) {
	invalidIDs := []string{
		"vendor.package.namespace.type.v0",
		"gt.vendor.package.namespace.type.v0",
		"gts",
		"",
	}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			_, err := NewGtsID(id)
			if err == nil {
				t.Errorf("Expected error for ID without 'gts.' prefix: %q", id)
			}
			if err != nil {
				gtsErr, ok := err.(*InvalidGtsIDError)
				if !ok {
					t.Errorf("Expected InvalidGtsIDError, got %T", err)
				}
				if gtsErr != nil && gtsErr.GtsID != id {
					t.Errorf("Error GtsID = %q, want %q", gtsErr.GtsID, id)
				}
			}
		})
	}
}

// TestGtsID_NotLowerCase tests IDs that are not lowercase
func TestGtsID_NotLowerCase(t *testing.T) {
	invalidIDs := []string{
		"GTS.vendor.package.namespace.type.v0",
		"gts.Vendor.package.namespace.type.v0",
		"gts.vendor.Package.namespace.type.v0",
	}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			_, err := NewGtsID(id)
			if err == nil {
				t.Errorf("Expected error for non-lowercase ID: %q", id)
			}
		})
	}
}

// TestGtsID_ContainsHyphen tests IDs containing hyphens
func TestGtsID_ContainsHyphen(t *testing.T) {
	invalidIDs := []string{
		"gts.vendor-name.package.namespace.type.v0",
		"gts.vendor.package-name.namespace.type.v0",
	}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			_, err := NewGtsID(id)
			if err == nil {
				t.Errorf("Expected error for ID containing hyphen: %q", id)
			}
		})
	}
}

// TestGtsID_TooLong tests IDs exceeding maximum length
func TestGtsID_TooLong(t *testing.T) {
	// Create an ID longer than 1024 characters
	longID := "gts." + string(make([]byte, 1025))

	_, err := NewGtsID(longID)
	if err == nil {
		t.Errorf("Expected error for ID longer than 1024 characters")
	}
}

// TestGtsID_TooFewTokens tests IDs with insufficient tokens
func TestGtsID_TooFewTokens(t *testing.T) {
	invalidIDs := []string{
		"gts.vendor",
		"gts.vendor.package",
		"gts.vendor.package.namespace",
		"gts.vendor.package.namespace.type",
	}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			_, err := NewGtsID(id)
			if err == nil {
				t.Errorf("Expected error for ID with too few tokens: %q", id)
			}
		})
	}
}

// TestGtsID_InvalidTokens tests IDs with invalid token format
func TestGtsID_InvalidTokens(t *testing.T) {
	invalidIDs := []string{
		"gts.123vendor.package.namespace.type.v0",
		"gts.vendor.9package.namespace.type.v0",
		"gts.vendor.package.name-space.type.v0",
	}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			_, err := NewGtsID(id)
			if err == nil {
				t.Errorf("Expected error for ID with invalid token: %q", id)
			}
		})
	}
}

// TestGtsID_InvalidVersion tests IDs with invalid version format
func TestGtsID_InvalidVersion(t *testing.T) {
	invalidIDs := []string{
		"gts.vendor.package.namespace.type.0",
		"gts.vendor.package.namespace.type.v",
		"gts.vendor.package.namespace.type.v-1",
		"gts.vendor.package.namespace.type.v1.-1",
		"gts.vendor.package.namespace.type.v1.2.3",
	}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			_, err := NewGtsID(id)
			if err == nil {
				t.Errorf("Expected error for ID with invalid version: %q", id)
			}
		})
	}
}

// TestGtsID_IsType tests the IsType property
func TestGtsID_IsType(t *testing.T) {
	tests := []struct {
		id       string
		expected bool
	}{
		{"gts.vendor.package.namespace.type.v0~", true},
		{"gts.vendor.package.namespace.type.v0.0~", true},
		{"gts.vendor.package.namespace.type.v0~a.b.c.d.v1", false},
		{"gts.vendor.package.namespace.type.v0.0~a.b.c.d.v1", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			gtsID, err := NewGtsID(tt.id)
			if err != nil {
				t.Fatalf("Unexpected error for valid ID %q: %v", tt.id, err)
			}
			if gtsID.IsType() != tt.expected {
				t.Errorf("IsType() = %v, want %v", gtsID.IsType(), tt.expected)
			}
		})
	}
}

// TestGtsID_EmptySegment tests IDs with empty segments
func TestGtsID_EmptySegment(t *testing.T) {
	invalidIDs := []string{
		"gts.vendor.package.namespace.type.v0~~",
		"gts.vendor.package.namespace.type.v0~type2.v1~",
	}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			_, err := NewGtsID(id)
			if err == nil {
				t.Errorf("Expected error for ID with empty segment: %q", id)
			}
		})
	}
}

// TestGtsID_MultipleTildeInSegment tests segments with multiple tildes
func TestGtsID_MultipleTildeInSegment(t *testing.T) {
	invalidID := "gts.vendor.package.namespace.ty~~pe.v0"

	_, err := NewGtsID(invalidID)
	if err == nil {
		t.Errorf("Expected error for segment with multiple tildes: %q", invalidID)
	}
}

// TestGtsID_TildeNotAtEnd tests segments with tilde not at the end
func TestGtsID_TildeNotAtEnd(t *testing.T) {
	invalidID := "gts.vendor.package.namespace.ty~pe.v0"

	_, err := NewGtsID(invalidID)
	if err == nil {
		t.Errorf("Expected error for tilde not at end: %q", invalidID)
	}
}

// TestGtsID_CombinedAnonymousInstance tests combined anonymous instance IDs (UUID tail)
func TestGtsID_CombinedAnonymousInstance(t *testing.T) {
	validIDs := []string{
		"gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~7a1d2f34-5678-49ab-9012-abcdef123456",
		"gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~7a1d2f34-5678-49ab-9012-000000000000",
	}

	for _, id := range validIDs {
		t.Run(id, func(t *testing.T) {
			gtsID, err := NewGtsID(id)
			if err != nil {
				t.Fatalf("Expected valid combined anonymous instance ID %q, got error: %v", id, err)
			}
			lastSeg := gtsID.Segments[len(gtsID.Segments)-1]
			if !lastSeg.IsUUID {
				t.Errorf("Expected last segment IsUUID=true for %q", id)
			}
			if lastSeg.IsType {
				t.Errorf("Expected last segment IsType=false for %q", id)
			}
			if gtsID.IsType() {
				t.Errorf("Expected IsType()=false for combined anonymous instance %q", id)
			}
		})
	}
}

// TestGtsID_CombinedAnonymousInstance_Invalid tests that invalid UUID tails are rejected
func TestGtsID_CombinedAnonymousInstance_Invalid(t *testing.T) {
	invalidIDs := []struct {
		id   string
		desc string
	}{
		{
			"gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~not-a-uuid",
			"non-UUID tail with hyphens",
		},
		{
			"gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~7A1D2F34-5678-49AB-9012-ABCDEF123456",
			"uppercase UUID tail",
		},
	}

	for _, tt := range invalidIDs {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := NewGtsID(tt.id)
			if err == nil {
				t.Errorf("Expected error for %s: %q", tt.desc, tt.id)
			}
		})
	}
}
