/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"testing"

	"github.com/google/uuid"
)

// TestToUUID_Types tests UUID generation for type identifiers
func TestToUUID_Types(t *testing.T) {
	tests := []struct {
		name     string
		gtsID    string
		expected string
	}{
		{
			name:     "Type with major version only",
			gtsID:    "gts.x.test5.events.type.v1~",
			expected: "de567dcc-10ef-597d-8f82-3c999ed9b979",
		},
		{
			name:     "Type with major and minor version",
			gtsID:    "gts.x.test5.events.type.v1.1~",
			expected: "b9a18e35-890b-586c-81fa-a156b9a26e2b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gts, err := NewGtsID(tt.gtsID)
			if err != nil {
				t.Fatalf("Failed to parse GTS ID: %v", err)
			}

			result := gts.ToUUID()
			if result.String() != tt.expected {
				t.Errorf("Expected UUID %s, got %s", tt.expected, result.String())
			}
		})
	}
}

// TestToUUID_Instances tests UUID generation for instance identifiers
func TestToUUID_Instances(t *testing.T) {
	tests := []struct {
		name     string
		gtsID    string
		expected string
	}{
		{
			name:     "Chained instance identifier",
			gtsID:    "gts.x.test5.events.type.v1~abc.app._.custom_event.v1.2",
			expected: "c7f8cca7-3af6-58af-b72b-3febfd93f1a8",
		},
		{
			name:     "Combined anonymous instance (UUID tail)",
			gtsID:    "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0~7a1d2f34-5678-49ab-9012-abcdef123456",
			expected: "4a31b759-722b-5bb1-a1dc-2cf40963e81b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gts, err := NewGtsID(tt.gtsID)
			if err != nil {
				t.Fatalf("Failed to parse GTS ID: %v", err)
			}

			result := gts.ToUUID()
			if result.String() != tt.expected {
				t.Errorf("Expected UUID %s, got %s", tt.expected, result.String())
			}
		})
	}
}

// TestToUUID_Deterministic tests that UUID generation is deterministic
func TestToUUID_Deterministic(t *testing.T) {
	gtsID := "gts.x.test5.events.type.v1~"

	gts1, err := NewGtsID(gtsID)
	if err != nil {
		t.Fatalf("Failed to parse GTS ID: %v", err)
	}

	gts2, err := NewGtsID(gtsID)
	if err != nil {
		t.Fatalf("Failed to parse GTS ID: %v", err)
	}

	uuid1 := gts1.ToUUID()
	uuid2 := gts2.ToUUID()

	if uuid1 != uuid2 {
		t.Errorf("UUID generation is not deterministic: %s != %s", uuid1, uuid2)
	}
}

// TestToUUID_DifferentIDs tests that different IDs produce different UUIDs
func TestToUUID_DifferentIDs(t *testing.T) {
	gts1, _ := NewGtsID("gts.x.test5.events.type.v1~")
	gts2, _ := NewGtsID("gts.x.test5.events.type.v1.1~")

	uuid1 := gts1.ToUUID()
	uuid2 := gts2.ToUUID()

	if uuid1 == uuid2 {
		t.Errorf("Different GTS IDs should produce different UUIDs")
	}
}

// TestGtsNamespace verifies the GTS namespace UUID is correctly generated
func TestGtsNamespace(t *testing.T) {
	// The GTS namespace should be uuid5(NAMESPACE_URL, "gts")
	expected := uuid.NewSHA1(uuid.NameSpaceURL, []byte("gts"))

	if GtsNamespace != expected {
		t.Errorf("GtsNamespace mismatch: expected %s, got %s", expected, GtsNamespace)
	}
}

// TestToUUID_MoreExamples tests additional UUID generation examples
func TestToUUID_MoreExamples(t *testing.T) {
	tests := []struct {
		name  string
		gtsID string
	}{
		{
			name:  "Simple type",
			gtsID: "gts.vendor.pkg.ns.type.v1~",
		},
		{
			name:  "Instance with full version",
			gtsID: "gts.vendor.pkg.ns.type.v1.0~a.b.c.d.v1",
		},
		{
			name:  "Multiple chained segments",
			gtsID: "gts.a.b.c.d.v1~e.f.g.h.v2~i.j.k.l.v3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gts, err := NewGtsID(tt.gtsID)
			if err != nil {
				t.Fatalf("Failed to parse GTS ID: %v", err)
			}

			uuid1 := gts.ToUUID()
			uuid2 := gts.ToUUID()

			// Verify determinism
			if uuid1 != uuid2 {
				t.Errorf("UUID generation is not deterministic")
			}

			// Verify it's a valid UUID
			if uuid1 == uuid.Nil {
				t.Errorf("Generated UUID should not be nil")
			}

			// Verify it's version 5 (SHA-1 based)
			if uuid1.Version() != 5 {
				t.Errorf("Expected UUID version 5, got version %d", uuid1.Version())
			}
		})
	}
}
