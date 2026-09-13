/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import "testing"

// ── OP#2: ExtractGtsID ──────────────────────────────────────────────────────

func TestExtractGtsID_Schema(t *testing.T) {
	content := map[string]any{
		"$id":     "gts://gts.x.core.ns.type.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
	}
	r := ExtractGtsID(content, DefaultGtsConfig())
	if r.ID != "gts.x.core.ns.type.v1~" {
		t.Errorf("expected gts.x.core.ns.type.v1~, got %s", r.ID)
	}
	if !r.IsTypeSchema {
		t.Error("expected IsTypeSchema=true")
	}
}

func TestExtractGtsID_Instance(t *testing.T) {
	content := map[string]any{
		"id":      "gts.x.core.ns.type.v1~x.vendor.ns.inst.v1",
		"type_id": "gts.x.core.ns.type.v1~",
	}
	r := ExtractGtsID(content, DefaultGtsConfig())
	if r.ID != "gts.x.core.ns.type.v1~x.vendor.ns.inst.v1" {
		t.Errorf("expected instance ID, got %s", r.ID)
	}
	if r.IsTypeSchema {
		t.Error("expected IsTypeSchema=false for instance")
	}
}

func TestExtractGtsID_NoID(t *testing.T) {
	content := map[string]any{"foo": "bar"}
	r := ExtractGtsID(content, DefaultGtsConfig())
	if r.ID != "" {
		t.Errorf("expected empty ID for content without GTS ID, got %s", r.ID)
	}
}
