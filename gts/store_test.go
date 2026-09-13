/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"strings"
	"testing"
)

// ── Error type tests ────────────────────────────────────────────────────────

func TestStoreErrorMessages(t *testing.T) {
	e1 := &StoreGtsObjectNotFoundError{EntityID: "gts.x.test.v1~inst"}
	if !strings.Contains(e1.Error(), "gts.x.test.v1~inst") {
		t.Errorf("error should contain entity ID: %s", e1.Error())
	}
	e2 := &StoreGtsSchemaNotFoundError{EntityID: "gts.x.test.v1~"}
	if !strings.Contains(e2.Error(), "gts.x.test.v1~") {
		t.Errorf("error should contain entity ID: %s", e2.Error())
	}
	e3 := &StoreGtsSchemaForInstanceNotFoundError{EntityID: "gts.x.test.v1~inst"}
	if !strings.Contains(e3.Error(), "gts.x.test.v1~inst") {
		t.Errorf("error should contain entity ID: %s", e3.Error())
	}
	e4 := &StoreGtsCastFromSchemaNotAllowedError{FromID: "gts.x.test.v1~"}
	if !strings.Contains(e4.Error(), "gts.x.test.v1~") {
		t.Errorf("error should contain from ID: %s", e4.Error())
	}
	if !strings.Contains(e4.Error(), "instance") {
		t.Errorf("error should mention instance: %s", e4.Error())
	}
}

// ── Store CRUD ──────────────────────────────────────────────────────────────

func TestGtsStore_RegisterAndGet(t *testing.T) {
	store := NewGtsStore(nil)
	schema := map[string]any{
		"$id": "gts://gts.x.test.ns.type.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}},
	}
	entity := NewJsonEntity(schema, DefaultGtsConfig())
	if err := store.Register(entity); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got := store.Get("gts.x.test.ns.type.v1~")
	if got == nil {
		t.Fatal("expected entity, got nil")
	}
	if got.Content["type"] != "object" {
		t.Errorf("unexpected content: %v", got.Content)
	}
}

func TestGtsStore_Get_NotFound(t *testing.T) {
	store := NewGtsStore(nil)
	if store.Get("gts.x.nonexistent.v1~") != nil {
		t.Error("expected nil for non-existent entity")
	}
}

func TestGtsStore_RegisterSchema(t *testing.T) {
	store := NewGtsStore(nil)
	schema := map[string]any{
		"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}},
	}
	if err := store.RegisterSchema("gts.x.test.ns.type.v1~", schema); err != nil {
		t.Fatalf("RegisterSchema: %v", err)
	}
	got := store.Get("gts.x.test.ns.type.v1~")
	if got == nil {
		t.Fatal("expected entity")
	}
	if !got.IsTypeSchema {
		t.Error("expected IsTypeSchema=true")
	}
}

func TestGtsStore_RegisterSchema_InvalidID(t *testing.T) {
	store := NewGtsStore(nil)
	err := store.RegisterSchema("gts.x.test.ns.type.v1", map[string]any{})
	if err == nil {
		t.Fatal("expected error for ID not ending with ~")
	}
}

func TestGtsStore_GetSchemaContent(t *testing.T) {
	store := NewGtsStore(nil)
	schema := map[string]any{"type": "object"}
	_ = store.RegisterSchema("gts.x.test.ns.type.v1~", schema)

	content, err := store.GetSchemaContent("gts.x.test.ns.type.v1~")
	if err != nil {
		t.Fatalf("GetSchemaContent: %v", err)
	}
	if content["type"] != "object" {
		t.Errorf("unexpected content: %v", content)
	}
}

func TestGtsStore_GetSchemaContent_NotFound(t *testing.T) {
	store := NewGtsStore(nil)
	_, err := store.GetSchemaContent("gts.x.missing.v1~")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGtsStore_Items(t *testing.T) {
	store := NewGtsStore(nil)
	_ = store.RegisterSchema("gts.x.test.ns.a.v1~", map[string]any{"type": "object"})
	_ = store.RegisterSchema("gts.x.test.ns.b.v1~", map[string]any{"type": "object"})
	items := store.Items()
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestGtsStore_Count(t *testing.T) {
	store := NewGtsStore(nil)
	if store.Count() != 0 {
		t.Errorf("expected 0, got %d", store.Count())
	}
	_ = store.RegisterSchema("gts.x.test.ns.a.v1~", map[string]any{})
	if store.Count() != 1 {
		t.Errorf("expected 1, got %d", store.Count())
	}
}

func TestGtsStore_Unregister(t *testing.T) {
	store := NewGtsStore(nil)
	_ = store.RegisterSchema("gts.x.test.ns.a.v1~", map[string]any{})
	store.Unregister("gts.x.test.ns.a.v1~")
	if store.Get("gts.x.test.ns.a.v1~") != nil {
		t.Error("expected nil after unregister")
	}
}

func TestGtsStore_List(t *testing.T) {
	store := NewGtsStore(nil)
	_ = store.RegisterSchema("gts.x.test.ns.a.v1~", map[string]any{})
	_ = store.RegisterSchema("gts.x.test.ns.b.v1~", map[string]any{})
	_ = store.RegisterSchema("gts.x.test.ns.c.v1~", map[string]any{})

	r := store.List(2)
	if r.Count != 2 {
		t.Errorf("expected count=2, got %d", r.Count)
	}
	if r.Total != 3 {
		t.Errorf("expected total=3, got %d", r.Total)
	}

	r2 := store.List(100)
	if r2.Count != 3 {
		t.Errorf("expected count=3, got %d", r2.Count)
	}
}

// ── RegisterWithValidation ──────────────────────────────────────────────────

func TestGtsStore_RegisterWithValidation_Success(t *testing.T) {
	store := NewGtsStore(nil)
	schema := map[string]any{
		"$id": "gts://gts.x.test.ns.type.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
	}
	entity := NewJsonEntity(schema, DefaultGtsConfig())
	err := store.RegisterWithValidation(entity, func(id string) error {
		return nil // validation passes
	})
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if store.Get("gts.x.test.ns.type.v1~") == nil {
		t.Error("entity should be registered")
	}
}

func TestGtsStore_RegisterWithValidation_Rollback(t *testing.T) {
	store := NewGtsStore(nil)
	schema := map[string]any{
		"$id": "gts://gts.x.test.ns.type.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
	}
	entity := NewJsonEntity(schema, DefaultGtsConfig())
	err := store.RegisterWithValidation(entity, func(id string) error {
		return &StoreGtsSchemaNotFoundError{EntityID: "fake"}
	})
	if err == nil {
		t.Fatal("expected error from validation callback")
	}
	if store.Get("gts.x.test.ns.type.v1~") != nil {
		t.Error("entity should have been rolled back")
	}
}

func TestGtsStore_RegisterWithValidation_RollbackRestoresPrevious(t *testing.T) {
	store := NewGtsStore(nil)
	schema1 := map[string]any{
		"$id": "gts://gts.x.test.ns.type.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "description": "original",
	}
	entity1 := NewJsonEntity(schema1, DefaultGtsConfig())
	_ = store.Register(entity1)

	schema2 := map[string]any{
		"$id": "gts://gts.x.test.ns.type.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object", "description": "replacement",
	}
	entity2 := NewJsonEntity(schema2, DefaultGtsConfig())
	_ = store.RegisterWithValidation(entity2, func(id string) error {
		return &StoreGtsSchemaNotFoundError{EntityID: "trigger-rollback"}
	})

	got := store.Get("gts.x.test.ns.type.v1~")
	if got == nil {
		t.Fatal("expected entity after rollback")
	}
	if got.Content["description"] != "original" {
		t.Errorf("expected original entity restored, got: %v", got.Content["description"])
	}
}

// ── PopulateFromReader ──────────────────────────────────────────────────────

type mockReader struct {
	entities []*JsonEntity
	idx      int
}

func (m *mockReader) Next() *JsonEntity {
	if m.idx >= len(m.entities) {
		return nil
	}
	e := m.entities[m.idx]
	m.idx++
	return e
}

func (m *mockReader) ReadByID(id string) *JsonEntity {
	for _, e := range m.entities {
		if e.EffectiveID() == id {
			return e
		}
	}
	return nil
}

func (m *mockReader) Reset() { m.idx = 0 }

func TestGtsStore_PopulateFromReader(t *testing.T) {
	schema := map[string]any{
		"$id": "gts://gts.x.test.ns.type.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
	}
	entity := NewJsonEntity(schema, DefaultGtsConfig())
	reader := &mockReader{entities: []*JsonEntity{entity}}

	store := NewGtsStore(reader)
	if store.Count() != 1 {
		t.Errorf("expected 1, got %d", store.Count())
	}
	if store.Get("gts.x.test.ns.type.v1~") == nil {
		t.Error("expected entity from reader")
	}
}

func TestGtsStore_Get_FallsBackToReader(t *testing.T) {
	schema := map[string]any{
		"$id": "gts://gts.x.test.ns.lazy.v1~", "$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
	}
	entity := NewJsonEntity(schema, DefaultGtsConfig())
	// Reader with entity NOT pre-populated (empty entities slice for iteration)
	reader := &mockReader{entities: []*JsonEntity{entity}}

	store := NewGtsStoreWithConfig(nil, DefaultRegistryConfig())
	store.reader = reader
	// Direct Get should fall back to reader
	got := store.Get("gts.x.test.ns.lazy.v1~")
	if got == nil {
		t.Error("expected entity from reader fallback")
	}
}

// ── ValidateSchema (store-level) ────────────────────────────────────────────

func TestGtsStore_ValidateSchema_NotTypeID(t *testing.T) {
	store := NewGtsStore(nil)
	err := store.ValidateSchema("gts.x.test.ns.type.v1")
	if err == nil {
		t.Fatal("expected error for non-type ID")
	}
}

func TestGtsStore_ValidateSchema_NotFound(t *testing.T) {
	store := NewGtsStore(nil)
	err := store.ValidateSchema("gts.x.test.ns.type.v1~")
	if err == nil {
		t.Fatal("expected error for missing schema")
	}
}
