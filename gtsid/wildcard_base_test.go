/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gtsid

import (
	"errors"
	"testing"
)

// ── match.go — ValidateWildcard ─────────────────────────────────────────────

func TestValidateWildcard_Base(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		valid   bool
	}{
		{"empty", "", false},
		{"no prefix", "notgts.x.test.*", false},
		{"uppercase", "gts.X.test.*", false},
		{"hyphen", "gts.x.my-package.*", false},
		{"leading whitespace", " gts.x.core.*", false},
		{"trailing whitespace", "gts.x.core.* ", false},
		{"wildcard glued to a name", "gts.x.co*", false},
		{"namespace level", "gts.x.core.ns.*", true},
		{"global", "gts.*", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateWildcard(tt.pattern)
			if tt.valid {
				if err != nil {
					t.Errorf("ValidateWildcard(%q): unexpected error %v", tt.pattern, err)
				}
				return
			}
			var wildcardErr *InvalidWildcardError
			if !errors.As(err, &wildcardErr) {
				t.Errorf("ValidateWildcard(%q): want *InvalidWildcardError, got %T (%v)", tt.pattern, err, err)
			}
		})
	}
}
