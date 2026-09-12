/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gtsid

import "testing"

// ── match.go — validateWildcardBase branches ────────────────────────────────

func TestValidateWildcardBase_EmptyBase(t *testing.T) {
	_, err := validateWildcardBase("")
	if err == nil {
		t.Fatal("expected error for empty base")
	}
}

func TestValidateWildcardBase_NoPrefix(t *testing.T) {
	_, err := validateWildcardBase("notgts.x.test")
	if err == nil {
		t.Fatal("expected error for missing gts. prefix")
	}
}

func TestValidateWildcardBase_Uppercase(t *testing.T) {
	_, err := validateWildcardBase("gts.X.test")
	if err == nil {
		t.Fatal("expected error for uppercase")
	}
}

func TestValidateWildcardBase_Hyphen(t *testing.T) {
	_, err := validateWildcardBase("gts.x.my-package")
	if err == nil {
		t.Fatal("expected error for hyphen")
	}
}

func TestValidateWildcardBase_ValidBase(t *testing.T) {
	_, err := validateWildcardBase("gts.x.core.ns")
	if err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

func TestValidateWildcardBase_GlobalPattern(t *testing.T) {
	_, err := validateWildcardBase("gts")
	if err != nil {
		t.Errorf("expected valid for bare 'gts': %v", err)
	}
}
