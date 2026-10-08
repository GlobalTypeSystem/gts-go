/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidateTransientJSON_SchemaValidationMatchesStored(t *testing.T) {
	const schemaID = "gts.x.validate.transient.host.v1~"
	traitSchema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"value": map[string]any{"type": "string"}},
		"required":   []any{"value"},
	}
	for _, tc := range []struct {
		name      string
		body      map[string]any
		wantOK    bool
		wantError string
	}{
		{"plain", nil, true, ""},
		{"valid_traits", map[string]any{KeyXGtsTraitsSchema: traitSchema, KeyXGtsTraits: map[string]any{"value": "a"}}, true, ""},
		{"invalid_trait_value", map[string]any{KeyXGtsTraitsSchema: traitSchema, KeyXGtsTraits: map[string]any{"value": 1}}, false, "trait validation failed"},
		{"missing_required_trait", map[string]any{KeyXGtsTraitsSchema: traitSchema}, false, "trait validation failed"},
		{"abstract_incomplete_traits", map[string]any{KeyXGtsAbstract: true, KeyXGtsTraitsSchema: traitSchema}, true, ""},
		{"invalid_modifier", map[string]any{KeyXGtsFinal: "true"}, false, "must be a boolean"},
		{"misplaced_traits", map[string]any{"properties": map[string]any{"p": map[string]any{KeyXGtsTraits: map[string]any{}}}}, false, "must be at the schema top level"},
		{"missing_constraint", map[string]any{"properties": map[string]any{"p": map[string]any{"type": "string", KeyXGtsRef: "gts.x.validate.transient.missing.v1~"}}}, false, "x-gts-ref"},
		{"unsupported_trait_pattern", map[string]any{KeyXGtsTraitsSchema: map[string]any{"properties": map[string]any{"value": map[string]any{"pattern": "a(?=b)"}}}}, false, "regex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := map[string]any{"$id": "gts://" + schemaID, "$schema": "http://json-schema.org/draft-07/schema#", "type": "object"}
			for k, v := range tc.body {
				content[k] = v
			}
			store := NewGtsStore(nil)
			transient := store.ValidateTransientJSON(content, "")
			if store.Get(schemaID) != nil {
				t.Fatal("transient schema was saved")
			}
			mustRegister(t, store, content)
			stored := store.ValidateEntity(schemaID)
			if stored.OK != tc.wantOK || !strings.Contains(stored.Error, tc.wantError) {
				t.Fatalf("stored validation: %+v, want OK=%v, error containing %q", stored, tc.wantOK, tc.wantError)
			}
			if transient.OK != stored.OK || transient.Error != adaptTypeMismatchMessage(stored.Error) {
				t.Fatalf("validation differs: transient=%+v, stored=%+v", transient, stored)
			}
		})
	}
}

func TestValidateTransientJSON_SchemaDependencies(t *testing.T) {
	const baseID = "gts.x.validate.transient.base.v1~"
	for _, dependency := range []string{"ancestor", "reference"} {
		t.Run(dependency, func(t *testing.T) {
			store := NewGtsStore(nil)
			mustRegister(t, store, map[string]any{
				"$id": "gts://" + baseID, "$schema": "http://json-schema.org/draft-07/schema#", "type": "object",
				KeyXGtsFinal: "invalid",
			})
			schemaID := "gts.x.validate.transient.refhost.v1~"
			if dependency == "ancestor" {
				schemaID = baseID + "x.validate.transient.child.v1~"
			}
			content := map[string]any{
				"$id": "gts://" + schemaID, "$schema": "http://json-schema.org/draft-07/schema#", "type": "object",
				"allOf": []any{map[string]any{"$ref": "gts://" + baseID}},
			}
			transient := store.ValidateTransientJSON(content, "")
			mustRegister(t, store, content)
			stored := store.ValidateEntity(schemaID)
			if stored.OK || !strings.Contains(stored.Error, "must be a boolean") {
				t.Fatalf("expected invalid dependency: %+v", stored)
			}
			if transient.OK || transient.Error != stored.Error {
				t.Fatalf("validation differs: transient=%+v, stored=%+v", transient, stored)
			}
		})
	}
}

func TestValidateTransientJSON_SchemaPreservesStoredVersion(t *testing.T) {
	const schemaID = "gts.x.validate.transient.existing.v1~"
	store := NewGtsStore(nil)
	original := map[string]any{"$id": "gts://" + schemaID, "$schema": "http://json-schema.org/draft-07/schema#", "type": "object"}
	mustRegister(t, store, original)
	for _, modifier := range []any{true, "invalid"} {
		candidate := map[string]any{"$id": "gts://" + schemaID, "$schema": original["$schema"], "type": "object", KeyXGtsFinal: modifier}
		result := store.ValidateTransientJSON(candidate, "")
		wantOK := modifier == true
		if result.OK != wantOK {
			t.Fatalf("candidate validation: %+v, want OK=%v", result, wantOK)
		}
		if got := store.Get(schemaID); got == nil || !reflect.DeepEqual(got.Content, original) {
			t.Fatalf("stored version changed: %+v", got)
		}
		if len(store.staged) != 0 || len(store.stagedByKey) != 0 {
			t.Fatal("transient validation leaked staging entries")
		}
	}
}

func TestValidateTransientJSON_MissingSchemaParent(t *testing.T) {
	result := NewGtsStore(nil).ValidateTransientJSON(map[string]any{
		"$id":     "gts://gts.x.validate.transient.missing.v1~x.validate.transient.child.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#", "type": "object",
	}, "")
	if result.OK || !strings.Contains(result.Error, "Parent GTS Type Schema not found") {
		t.Fatalf("expected missing parent error: %+v", result)
	}
}

type transientSchemaReader struct {
	mockReader
	read func(string) *JsonEntity
}

func (r *transientSchemaReader) ReadByID(id string) *JsonEntity { return r.read(id) }

func TestValidateTransientJSON_SchemaIsolatedDuringValidation(t *testing.T) {
	const schemaID = "gts.x.validate.transient.isolated.v1~"
	const dependencyID = "gts.x.validate.transient.dependency.v1~"
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "existing"}[existing], func(t *testing.T) {
			reader := &transientSchemaReader{}
			store := NewGtsStore(reader)
			original := map[string]any{"$id": "gts://" + schemaID, "$schema": "http://json-schema.org/draft-07/schema#", "title": "committed"}
			if existing {
				mustRegister(t, store, original)
			}
			observed := false
			reader.read = func(id string) *JsonEntity {
				if id != dependencyID {
					t.Fatalf("unexpected lazy read: %s", id)
				}
				observed = true
				visible := store.Items()[schemaID]
				if existing {
					if visible == nil || !reflect.DeepEqual(visible.Content, original) {
						t.Fatalf("public read saw transient candidate: %+v", visible)
					}
				} else if visible != nil {
					t.Fatal("public read saw new transient schema")
				}
				return NewJsonEntity(map[string]any{"$id": "gts://" + dependencyID, "$schema": original["$schema"], "type": "object"}, DefaultGtsConfig())
			}
			result := store.ValidateTransientJSON(map[string]any{
				"$id": "gts://" + schemaID, "$schema": original["$schema"], "title": "candidate",
				"allOf": []any{map[string]any{"$ref": "gts://" + dependencyID}},
			}, "")
			if !observed || !result.OK {
				t.Fatalf("expected successful validation with a lazy dependency read: observed=%v, result=%+v", observed, result)
			}
		})
	}
}
