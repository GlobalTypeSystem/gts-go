/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"errors"
	"testing"
)

func instanceContent(name string) map[string]any {
	return map[string]any{
		"id":   "gts.x.test._.foo.v1.0~alice",
		"name": name,
	}
}

func TestRegisterIdenticalEntityIsIdempotent(t *testing.T) {
	store := NewGtsStore(nil)
	if err := store.Register(NewJsonEntity(instanceContent("Alice"), DefaultGtsConfig())); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if err := store.Register(NewJsonEntity(instanceContent("Alice"), DefaultGtsConfig())); err != nil {
		t.Fatalf("identical re-register should be idempotent, got: %v", err)
	}
}

func TestRegisterChangedEntityIsConflict(t *testing.T) {
	store := NewGtsStore(nil)
	if err := store.Register(NewJsonEntity(instanceContent("Alice"), DefaultGtsConfig())); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	err := store.Register(NewJsonEntity(instanceContent("changed"), DefaultGtsConfig()))
	var conflict *EntityConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected EntityConflictError, got: %v", err)
	}

	// The previous content must be preserved.
	if got := store.Get("gts.x.test._.foo.v1.0~alice"); got == nil || got.Content["name"] != "Alice" {
		t.Fatalf("previous content should be preserved, got: %+v", got)
	}
}

func TestAllowEntityUpdatesReplacesChangedEntity(t *testing.T) {
	store := NewGtsStoreWithConfig(nil, &RegistryConfig{AllowEntityUpdates: true})
	if err := store.Register(NewJsonEntity(instanceContent("Alice"), DefaultGtsConfig())); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if err := store.Register(NewJsonEntity(instanceContent("changed"), DefaultGtsConfig())); err != nil {
		t.Fatalf("update should be allowed, got: %v", err)
	}
	if got := store.Get("gts.x.test._.foo.v1.0~alice"); got == nil || got.Content["name"] != "changed" {
		t.Fatalf("content should be replaced, got: %+v", got)
	}
}

func TestRegisterSchemaChangedContentIsConflict(t *testing.T) {
	store := NewGtsStore(nil)
	typeID := "gts.x.test._.legacy.v1~"
	if err := store.RegisterSchema(typeID, map[string]any{"type": "object"}); err != nil {
		t.Fatalf("first RegisterSchema failed: %v", err)
	}
	// Identical re-registration is idempotent.
	if err := store.RegisterSchema(typeID, map[string]any{"type": "object"}); err != nil {
		t.Fatalf("identical RegisterSchema should be idempotent, got: %v", err)
	}

	err := store.RegisterSchema(typeID, map[string]any{"type": "string"})
	var conflict *EntityConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected EntityConflictError, got: %v", err)
	}
}

func TestRegisterSchemaAllowEntityUpdates(t *testing.T) {
	store := NewGtsStoreWithConfig(nil, &RegistryConfig{AllowEntityUpdates: true})
	typeID := "gts.x.test._.legacy.v1~"
	if err := store.RegisterSchema(typeID, map[string]any{"type": "object"}); err != nil {
		t.Fatalf("first RegisterSchema failed: %v", err)
	}
	if err := store.RegisterSchema(typeID, map[string]any{"type": "string"}); err != nil {
		t.Fatalf("update should be allowed, got: %v", err)
	}
}
