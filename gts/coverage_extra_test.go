/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"strings"
	"testing"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
)

// ── ref_scope_compat.go — refScopeRelation / relation / atomRelation ────────

func typeRef(operand string) map[string]any {
	return map[string]any{"type": "string", KeyXGtsTypeRef: operand}
}

func TestRefScopeRelation_Atoms(t *testing.T) {
	cases := []struct {
		name          string
		base, derived map[string]any
		want          refRel
	}{
		{
			"base imposes no constraint",
			map[string]any{"type": "string"},
			typeRef("gts.x.scope.ns.a.v1~"),
			relIncluded,
		},
		{
			"base covers derived",
			typeRef("gts.x.scope.*"),
			typeRef("gts.x.scope.ns.a.v1~"),
			relIncluded,
		},
		{
			"base does not cover derived",
			typeRef("gts.x.scope.ns.a.*"),
			typeRef("gts.x.scope.*"),
			relExcluded,
		},
		{
			"type vs instance kinds are disjoint",
			typeRef("gts.x.scope.*"),
			map[string]any{"type": "string", KeyXGtsInstanceRef: "gts.x.scope.*"},
			relExcluded,
		},
		{
			"pattern vs entity ref is undecided",
			map[string]any{"type": "string", KeyXGtsPatternRef: "gts.x.scope.*"},
			typeRef("gts.x.scope.ns.a.v1~"),
			relUnknown,
		},
		{
			"base top accepts everything",
			map[string]any{"type": "string", KeyXGtsTypeRef: XGtsRefSelf},
			typeRef("gts.x.scope.ns.a.v1~"),
			relIncluded,
		},
		{
			"derived top is excluded by a restricted base",
			typeRef("gts.x.scope.ns.a.v1~"),
			map[string]any{"type": "string", KeyXGtsTypeRef: XGtsRefSelf},
			relExcluded,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := refScopeRelation(c.base, c.derived); got != c.want {
				t.Errorf("refScopeRelation = %v, want %v", got, c.want)
			}
		})
	}
}

func TestRefScopeRelation_Composition(t *testing.T) {
	cases := []struct {
		name          string
		base, derived map[string]any
		want          refRel
	}{
		{
			"derived union all included",
			typeRef("gts.x.scope.*"),
			map[string]any{"anyOf": []any{typeRef("gts.x.scope.ns.a.v1~"), typeRef("gts.x.scope.ns.b.v1~")}},
			relIncluded,
		},
		{
			"derived union one excluded",
			typeRef("gts.x.scope.*"),
			map[string]any{"anyOf": []any{typeRef("gts.x.scope.ns.a.v1~"), typeRef("gts.y.other.ns.b.v1~")}},
			relExcluded,
		},
		{
			"derived intersection with one included conjunct",
			typeRef("gts.x.scope.*"),
			map[string]any{"allOf": []any{typeRef("gts.x.scope.ns.a.v1~")}},
			relIncluded,
		},
		{
			"not/not reduces to the reverse relation",
			map[string]any{"not": typeRef("gts.x.scope.ns.a.*")},
			map[string]any{"not": typeRef("gts.x.scope.*")},
			relIncluded,
		},
		{
			"base union: atom fits a branch",
			map[string]any{"anyOf": []any{typeRef("gts.x.scope.ns.a.*"), typeRef("gts.x.scope.ns.b.*")}},
			typeRef("gts.x.scope.ns.a.v1~"),
			relIncluded,
		},
		{
			"base intersection: every conjunct must include",
			map[string]any{"allOf": []any{typeRef("gts.x.scope.*"), typeRef("gts.x.scope.ns.a.*")}},
			typeRef("gts.x.scope.ns.a.v1~"),
			relIncluded,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := refScopeRelation(c.base, c.derived); got != c.want {
				t.Errorf("refScopeRelation = %v, want %v", got, c.want)
			}
		})
	}
}

func TestRefScopeRelation_EnumeratedDerived(t *testing.T) {
	base := typeRef("gts.x.scope.*")
	cases := []struct {
		name    string
		derived map[string]any
		want    refRel
	}{
		{"const inside scope", map[string]any{"const": "gts.x.scope.ns.a.v1~"}, relIncluded},
		{"const outside scope", map[string]any{"const": "gts.y.other.ns.a.v1~"}, relExcluded},
		{"enum with non-string", map[string]any{"enum": []any{"gts.x.scope.ns.a.v1~", 123}}, relExcluded},
		{"plain string, no enumeration", map[string]any{"type": "string"}, relExcluded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := refScopeRelation(base, c.derived); got != c.want {
				t.Errorf("refScopeRelation = %v, want %v", got, c.want)
			}
		})
	}
}

func TestRefScopeAccepts_Composition(t *testing.T) {
	cases := []struct {
		name  string
		base  map[string]any
		value string
		want  refRel // relIncluded means accepted
	}{
		{
			"union accepts via second branch",
			map[string]any{"anyOf": []any{typeRef("gts.x.a.*"), typeRef("gts.x.b.*")}},
			"gts.x.b.ns.t.v1~",
			relIncluded,
		},
		{
			"complement accepts a value outside the inner scope",
			map[string]any{"not": typeRef("gts.x.a.*")},
			"gts.x.b.ns.t.v1~",
			relIncluded,
		},
		{
			"intersection rejects a value missing from one conjunct",
			map[string]any{"allOf": []any{typeRef("gts.x.a.*"), typeRef("gts.x.b.*")}},
			"gts.x.a.ns.t.v1~",
			relExcluded,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := refScopeRelation(c.base, map[string]any{"const": c.value})
			if got != c.want {
				t.Errorf("refScopeRelation via const = %v, want %v", got, c.want)
			}
		})
	}
}

func TestCheckRefScopeCompatibility(t *testing.T) {
	ok := checkRefScopeCompatibility(typeRef("gts.x.scope.*"), typeRef("gts.x.scope.ns.a.v1~"), "p")
	if len(ok) != 0 {
		t.Errorf("included scope should yield no errors, got %v", ok)
	}
	bad := checkRefScopeCompatibility(typeRef("gts.x.scope.ns.a.*"), typeRef("gts.x.scope.*"), "p")
	if len(bad) == 0 || !strings.Contains(bad[0], "not included") {
		t.Errorf("excluded scope should report an error, got %v", bad)
	}
}

func TestNarrowerRefOperand_Comparable(t *testing.T) {
	if n, ok := narrowerRefOperand(KeyXGtsTypeRef, "gts.x.scope.*", "gts.x.scope.ns.a.v1~"); !ok || n != "gts.x.scope.ns.a.v1~" {
		t.Errorf("narrower should be the covered operand b, got %v,%v", n, ok)
	}
	if n, ok := narrowerRefOperand(KeyXGtsTypeRef, "gts.x.scope.ns.a.v1~", "gts.x.scope.*"); !ok || n != "gts.x.scope.ns.a.v1~" {
		t.Errorf("narrower should be a when a ⊆ b, got %v,%v", n, ok)
	}
	if _, ok := narrowerRefOperand(KeyXGtsTypeRef, "gts.x.scope.ns.a.*", "gts.x.scope.ns.b.*"); ok {
		t.Error("incomparable operands should report ok=false")
	}
}

// ── x_gts_ref.go — refKind strings and candidate violations ─────────────────

func TestRefKindStrings(t *testing.T) {
	if refKindType.keyword() != KeyXGtsTypeRef || refKindType.noun() != "GTS Type Identifier" ||
		refKindType.otherNoun() != "GTS Instance Identifier" {
		t.Error("refKindType strings are wrong")
	}
	if refKindInstance.keyword() != KeyXGtsInstanceRef || refKindInstance.noun() != "GTS Instance Identifier" ||
		refKindInstance.otherNoun() != "GTS Type Identifier" {
		t.Error("refKindInstance strings are wrong")
	}
	if refKindPattern.keyword() != KeyXGtsPatternRef {
		t.Error("refKindPattern keyword is wrong")
	}
}

func TestRefCandidateViolation(t *testing.T) {
	patternOperand, err := gtsid.ParsePattern("gts.x.scope.*")
	if err != nil {
		t.Fatal(err)
	}
	typeOperand, err := gtsid.ParsePattern("gts.x.scope.*")
	if err != nil {
		t.Fatal(err)
	}

	// Pattern kind.
	if v := refCandidateViolation(refKindPattern, "gts.x.scope.ns.a.*", patternOperand, "gts.x.scope.*"); v != "" {
		t.Errorf("covered pattern should pass, got %q", v)
	}
	if v := refCandidateViolation(refKindPattern, "not a pattern", patternOperand, "gts.x.scope.*"); !strings.Contains(v, "not a valid GTS identifier pattern") {
		t.Errorf("invalid pattern message = %q", v)
	}
	if v := refCandidateViolation(refKindPattern, "gts.y.other.*", patternOperand, "gts.x.scope.*"); !strings.Contains(v, "outside") {
		t.Errorf("out-of-scope pattern message = %q", v)
	}

	// Type kind.
	if v := refCandidateViolation(refKindType, "gts.x.scope.ns.thing.v1~", typeOperand, "gts.x.scope.*"); v != "" {
		t.Errorf("matching type should pass, got %q", v)
	}
	if v := refCandidateViolation(refKindType, "gts.x.scope.*", typeOperand, "gts.x.scope.*"); !strings.Contains(v, "not a valid GTS identifier") {
		t.Errorf("wildcard value message = %q", v)
	}
	if v := refCandidateViolation(refKindType, "@bad", typeOperand, "gts.x.scope.*"); !strings.Contains(v, "not a valid GTS identifier") {
		t.Errorf("invalid value message = %q", v)
	}
	if v := refCandidateViolation(refKindType, "gts.x.scope.ns.thing.v1~x.c.n.i.v1", typeOperand, "gts.x.scope.*"); !strings.Contains(v, "requires a") {
		t.Errorf("instance-where-type-required message = %q", v)
	}
	narrow, _ := gtsid.ParsePattern("gts.x.scope.ns.only.*")
	if v := refCandidateViolation(refKindType, "gts.x.scope.ns.thing.v1~", narrow, "gts.x.scope.ns.only.*"); !strings.Contains(v, "does not match pattern") {
		t.Errorf("non-matching type message = %q", v)
	}
}

// ── regex_profile.go — exported helpers and trait-schema walk ───────────────

func TestRegexProfileSupportedAndError(t *testing.T) {
	if !RegexProfileSupported("^[a-z]+$") {
		t.Error("a simple anchored class should be supported")
	}
	if RegexProfileSupported("a(?=b)") {
		t.Error("lookahead is outside the profile")
	}
	if (&RegexProfileError{Reason: "boom"}).Error() != "boom" {
		t.Error("RegexProfileError.Error should return the reason")
	}
}

func TestCheckTraitSchemaRegexProfile_NoTraits(t *testing.T) {
	if err := checkTraitSchemaRegexProfile(map[string]any{"type": "object"}, "draft-07"); err != nil {
		t.Errorf("a doc without a traits schema should pass, got %v", err)
	}
}

func TestCheckTraitSchemaRegexProfile_AllKeywords(t *testing.T) {
	traits := map[string]any{
		"not":                   map[string]any{"pattern": "^a$"},
		"allOf":                 []any{map[string]any{"pattern": "^b$"}},
		"anyOf":                 []any{map[string]any{"pattern": "^c$"}},
		"oneOf":                 []any{map[string]any{"pattern": "^d$"}},
		"properties":            map[string]any{"p": map[string]any{"pattern": "^e$"}},
		"patternProperties":     map[string]any{"^k$": map[string]any{"type": "string"}},
		"items":                 map[string]any{"pattern": "^f$"},
		"additionalItems":       map[string]any{"pattern": "^g$"},
		"prefixItems":           []any{map[string]any{"pattern": "^h$"}},
		"contains":              map[string]any{"pattern": "^i$"},
		"if":                    map[string]any{"pattern": "^j$"},
		"then":                  map[string]any{"pattern": "^l$"},
		"else":                  map[string]any{"pattern": "^m$"},
		"propertyNames":         map[string]any{"pattern": "^n$"},
		"additionalProperties":  map[string]any{"pattern": "^o$"},
		"definitions":           map[string]any{"d": map[string]any{"pattern": "^p1$"}},
		"$defs":                 map[string]any{"e": map[string]any{"pattern": "^q$"}},
		"dependencies":          map[string]any{"x": map[string]any{"pattern": "^r$"}, "y": []any{"a", "b"}},
		"dependentSchemas":      map[string]any{"z": map[string]any{"pattern": "^s$"}},
		"unevaluatedItems":      map[string]any{"pattern": "^t$"},
		"unevaluatedProperties": map[string]any{"pattern": "^u$"},
		"contentSchema":         map[string]any{"pattern": "^v$"},
	}
	for _, dialect := range []string{"draft-07", "2019-09", "2020-12"} {
		doc := map[string]any{KeyXGtsTraitsSchema: traits}
		if err := checkTraitSchemaRegexProfile(doc, dialect); err != nil {
			t.Errorf("%s: all-valid traits should pass, got %v", dialect, err)
		}
	}
}

func TestCheckTraitSchemaRegexProfile_Rejections(t *testing.T) {
	// Unsupported pattern nested in properties.
	doc := map[string]any{KeyXGtsTraitsSchema: map[string]any{
		"properties": map[string]any{"p": map[string]any{"pattern": "a(?=b)"}},
	}}
	if err := checkTraitSchemaRegexProfile(doc, "draft-07"); err == nil {
		t.Error("an unsupported nested pattern should be rejected")
	}
	// Unsupported patternProperties key.
	doc = map[string]any{KeyXGtsTraitsSchema: map[string]any{
		"patternProperties": map[string]any{"a(?=b)": map[string]any{}},
	}}
	if err := checkTraitSchemaRegexProfile(doc, "2020-12"); err == nil {
		t.Error("an unsupported patternProperties key should be rejected")
	}
	// Local $ref target reached from the traits schema.
	doc = map[string]any{
		KeyXGtsTraitsSchema: map[string]any{"$ref": "#/$defs/shared"},
		"$defs":             map[string]any{"shared": map[string]any{"pattern": "a(?=b)"}},
	}
	if err := checkTraitSchemaRegexProfile(doc, "2020-12"); err == nil {
		t.Error("an unsupported pattern behind a local $ref should be rejected")
	}
}

func TestResolveLocalPointer(t *testing.T) {
	doc := map[string]any{
		"a": map[string]any{"b": map[string]any{"c": "leaf"}},
	}
	if got := resolveLocalPointer(doc, "#"); got == nil {
		t.Error("# should resolve to the document root")
	}
	if got := resolveLocalPointer(doc, "#/a"); got == nil {
		t.Error("#/a should resolve to an object")
	}
	if got := resolveLocalPointer(doc, "#/a/b/c"); got != nil {
		t.Error("a pointer to a non-object should resolve to nil")
	}
	if got := resolveLocalPointer(doc, "#/missing"); got != nil {
		t.Error("a missing pointer should resolve to nil")
	}
	if got := resolveLocalPointer(doc, "relative"); got != nil {
		t.Error("a non-anchored pointer should resolve to nil")
	}
}

// ── gts_ref_validation.go ───────────────────────────────────────────────────

func TestParseGtsRefValidationMode(t *testing.T) {
	cases := []struct {
		in      string
		want    GtsRefValidationMode
		wantErr bool
	}{
		{"", GtsRefValidationAnyValid, false},
		{"none", GtsRefValidationNone, false},
		{"any-present", GtsRefValidationAnyPresent, false},
		{"any-valid", GtsRefValidationAnyValid, false},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := ParseGtsRefValidationMode(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseGtsRefValidationMode(%q): want error", c.in)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("ParseGtsRefValidationMode(%q) = %q,%v", c.in, got, err)
		}
	}
}

// ── compatibility.go — small helpers ────────────────────────────────────────

func TestHasObjectAndArrayKeywords(t *testing.T) {
	if !hasObjectKeywords(map[string]any{"properties": map[string]any{}}) {
		t.Error("properties should be recognised as an object keyword")
	}
	if hasObjectKeywords(map[string]any{"type": "string"}) {
		t.Error("a plain string schema has no object keywords")
	}
	if !hasArrayKeywords(map[string]any{"items": map[string]any{}}) {
		t.Error("items should be recognised as an array keyword")
	}
	if hasArrayKeywords(map[string]any{"type": "string"}) {
		t.Error("a plain string schema has no array keywords")
	}
}

func TestLowerUnevaluatedProperties(t *testing.T) {
	// No unevaluatedProperties: schema is returned unchanged.
	in := map[string]any{"type": "object"}
	if out, ok := lowerUnevaluatedProperties(in); !ok || out["type"] != "object" {
		t.Errorf("schema without unevaluatedProperties should pass through, got %v,%v", out, ok)
	}
	// Lowered into additionalProperties.
	in = map[string]any{"type": "object", "unevaluatedProperties": false}
	out, ok := lowerUnevaluatedProperties(in)
	if !ok {
		t.Fatal("simple unevaluatedProperties should lower")
	}
	if _, exists := out["unevaluatedProperties"]; exists {
		t.Error("unevaluatedProperties should be removed after lowering")
	}
	if out["additionalProperties"] != false {
		t.Error("unevaluatedProperties value should become additionalProperties")
	}
	// Applicator sibling blocks lowering.
	if _, ok := lowerUnevaluatedProperties(map[string]any{"unevaluatedProperties": false, "allOf": []any{}}); ok {
		t.Error("an allOf sibling must block lowering")
	}
	// Conflicting additionalProperties blocks lowering.
	if _, ok := lowerUnevaluatedProperties(map[string]any{"unevaluatedProperties": false, "additionalProperties": true}); ok {
		t.Error("a conflicting additionalProperties must block lowering")
	}
}

// ── schema_compat.go — small helpers ────────────────────────────────────────

func TestCanonicalSchemaDialectURI(t *testing.T) {
	cases := map[string]string{
		"draft-07": "http://json-schema.org/draft-07/schema#",
		"2019-09":  "https://json-schema.org/draft/2019-09/schema",
		"2020-12":  "https://json-schema.org/draft/2020-12/schema",
	}
	for dialect, want := range cases {
		got, err := canonicalSchemaDialectURI(dialect)
		if err != nil || got != want {
			t.Errorf("canonicalSchemaDialectURI(%q) = %q,%v", dialect, got, err)
		}
	}
	if _, err := canonicalSchemaDialectURI("nope"); err == nil {
		t.Error("an unknown dialect should error")
	}
}

func TestResolveLocalSchemaRef(t *testing.T) {
	root := map[string]any{
		"$defs": map[string]any{"a": map[string]any{"type": "string"}},
		"arr":   []any{map[string]any{"x": 1}},
	}
	if resolveLocalSchemaRef(root, "#") == nil {
		t.Error("# should resolve to the root")
	}
	if resolveLocalSchemaRef(root, "#/$defs/a") == nil {
		t.Error("#/$defs/a should resolve")
	}
	if resolveLocalSchemaRef(root, "#/arr/0") == nil {
		t.Error("#/arr/0 should resolve an array element")
	}
	if resolveLocalSchemaRef(root, "#/arr/9") != nil {
		t.Error("an out-of-range array index should resolve to nil")
	}
	if resolveLocalSchemaRef(root, "external") != nil {
		t.Error("a non-anchored ref should resolve to nil")
	}
	if resolveLocalSchemaRef(root, "#/missing/x") != nil {
		t.Error("a missing path should resolve to nil")
	}
}

func TestValueTypeCompatible(t *testing.T) {
	base := map[string]bool{"integer": true}
	if !valueTypeCompatible("integer", base) {
		t.Error("integer should be compatible with an integer base")
	}
	if !valueTypeCompatible("number", base) {
		t.Error("number should be accepted when the base allows integer")
	}
	if !valueTypeCompatible("integer", map[string]bool{"number": true}) {
		t.Error("integer should be accepted when the base allows number")
	}
	if valueTypeCompatible("string", base) {
		t.Error("string is not compatible with an integer-only base")
	}
}

func TestJSONValueType(t *testing.T) {
	cases := []struct {
		v    any
		want string
	}{
		{"x", "string"},
		{true, "boolean"},
		{float64(1), "number"},
		{1, "number"},
		{[]any{}, "array"},
		{map[string]any{}, "object"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := jsonValueType(c.v); got != c.want {
			t.Errorf("jsonValueType(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestCheckMultipleOf(t *testing.T) {
	base := map[string]any{"multipleOf": float64(2)}
	if errs := checkMultipleOf(base, map[string]any{"multipleOf": float64(4)}, "p"); len(errs) != 0 {
		t.Errorf("4 is a multiple of 2, want no error, got %v", errs)
	}
	if errs := checkMultipleOf(base, map[string]any{"multipleOf": float64(3)}, "p"); len(errs) == 0 {
		t.Error("3 is not a multiple of 2, want an error")
	}
	if errs := checkMultipleOf(base, map[string]any{}, "p"); len(errs) == 0 {
		t.Error("derived omitting multipleOf should error")
	}
	if errs := checkMultipleOf(map[string]any{}, map[string]any{"multipleOf": float64(3)}, "p"); len(errs) != 0 {
		t.Errorf("no base multipleOf, want no error, got %v", errs)
	}
}

// ── store.go — conflict error, ids, staging lifecycle ───────────────────────

func TestEntityConflictError(t *testing.T) {
	err := &EntityConflictError{EntityID: "gts.x.c.n.t.v1~"}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("unexpected message: %q", err.Error())
	}
}

func TestStoreEntityIDs(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{"$id": "gts://gts.x.ids.ns.a.v1~", "type": "object"})
	mustRegister(t, store, map[string]any{"$id": "gts://gts.x.ids.ns.b.v1~", "type": "object"})
	ids := store.entityIDs()
	if len(ids) != 2 {
		t.Fatalf("entityIDs = %v, want 2 entries", ids)
	}
}

func TestStagingSessionCommit(t *testing.T) {
	store := NewGtsStore(nil)
	session := store.NewStagingSession()
	if !strings.HasPrefix(session, "ses-") {
		t.Fatalf("NewStagingSession = %q, want a ses- token", session)
	}
	entity := newSchemaEntity(map[string]any{
		"$id":     "gts://gts.x.stage.ns.a.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
	})
	token, err := store.Stage(entity, session)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if store.Get("gts.x.stage.ns.a.v1~") != nil {
		t.Error("a staged entity must not be visible to public reads")
	}
	if err := store.Commit(token); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if store.Get("gts.x.stage.ns.a.v1~") == nil {
		t.Error("the entity should be visible after commit")
	}
	// Committing an unknown token is a conflict.
	if err := store.Commit("stg-does-not-exist"); err == nil {
		t.Error("committing an unknown token should fail")
	}
}

func TestValidateEntityAndInSession(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":  "gts://gts.x.ve.ns.base.v1~",
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	})
	if res := store.ValidateEntity("gts.x.ve.ns.base.v1~"); !res.OK {
		t.Fatalf("ValidateEntity: %+v", res)
	}
	if res := store.ValidateEntityInSession("gts.x.ve.ns.base.v1~", ""); !res.OK {
		t.Fatalf("ValidateEntityInSession: %+v", res)
	}
	if res := store.ValidateEntity("gts.x.ve.ns.missing.v1~"); res.OK {
		t.Error("a missing entity should not validate")
	}
}

func TestValidateSchema_Paths(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":        "gts://gts.x.vs.ns.base.v1~",
		"type":       "object",
		"properties": map[string]any{"id": map[string]any{"type": "string"}},
	})
	if err := store.ValidateSchema("gts.x.vs.ns.base.v1~"); err != nil {
		t.Fatalf("valid schema: %v", err)
	}
	// Not a type ID (no trailing ~).
	if err := store.ValidateSchema("gts.x.vs.ns.base.v1"); err == nil {
		t.Error("a non-type ID should be rejected")
	}
	// Missing schema.
	if err := store.ValidateSchema("gts.x.vs.ns.missing.v1~"); err == nil {
		t.Error("a missing schema should be rejected")
	}
}

func TestValidateInstanceWithXGtsRef_Paths(t *testing.T) {
	store := NewGtsStore(nil)
	// A type-schema is not an instance.
	mustRegister(t, store, map[string]any{
		"$id":  "gts://gts.x.vi.ns.base.v1~",
		"type": "object",
	})
	if err := store.ValidateInstanceWithXGtsRef("gts.x.vi.ns.base.v1~"); err == nil {
		t.Error("validating a type-schema as an instance should fail")
	}
	// A missing entity.
	if err := store.ValidateInstanceWithXGtsRef("gts.x.vi.ns.ghost.v1~x.c.n.i.v1"); err == nil {
		t.Error("a missing instance should fail")
	}
}

func TestValidateInstance_EndToEnd(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":      "gts://gts.x.inst.ns.thing.v1~",
		"type":     "object",
		"required": []any{"name"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	})
	mustRegisterInstance(t, store, map[string]any{
		"$id":  "gts://gts.x.inst.ns.thing.v1~x.inst._.one.v1",
		"name": "ok",
	})
	if res := store.ValidateInstance("gts.x.inst.ns.thing.v1~x.inst._.one.v1"); !res.OK {
		t.Fatalf("a conforming instance should validate: %+v", res)
	}

	// An instance missing its required field must fail.
	mustRegisterInstance(t, store, map[string]any{
		"$id": "gts://gts.x.inst.ns.thing.v1~x.inst._.two.v1",
	})
	if res := store.ValidateInstance("gts.x.inst.ns.thing.v1~x.inst._.two.v1"); res.OK {
		t.Error("an instance missing a required field should not validate")
	}
}

// Exercises the any-valid dependency walk, including concrete ReferencedIDs
// (validateEntity) and wildcard ReferencedWildcardPatterns (hasValidWildcardMatch).
func TestValidateEntity_AnyValidResolvesReferences(t *testing.T) {
	store := NewGtsStore(nil)
	mustRegister(t, store, map[string]any{
		"$id":  "gts://gts.x.wild.ns.target.v1~",
		"type": "object",
	})
	mustRegister(t, store, map[string]any{
		"$id":  "gts://gts.x.wild.ns.host.v1~",
		"type": "object",
		"properties": map[string]any{
			"c": map[string]any{"type": "string", KeyXGtsTypeRef: "gts.x.wild.ns.target.v1~"},
			"w": map[string]any{"type": "string", KeyXGtsTypeRef: "gts.x.wild.ns.*"},
		},
	})
	res := store.ValidateEntity("gts.x.wild.ns.host.v1~", GtsRefValidationAnyValid)
	if !res.OK {
		t.Fatalf("a host whose references all resolve should validate: %+v", res)
	}
}

// ── file_reader.go ──────────────────────────────────────────────────────────

func TestNewGtsFileReaderAndWithExclude(t *testing.T) {
	r := NewGtsFileReader([]string{"~/some/path", "/abs"}, nil)
	if r == nil {
		t.Fatal("NewGtsFileReader returned nil")
	}
	// WithExclude overrides the default list; an empty list is ignored.
	custom := r.WithExclude([]string{"vendor", "node_modules"})
	if custom == nil {
		t.Fatal("WithExclude returned nil")
	}
	before := append([]string(nil), custom.exclude...)
	custom.WithExclude(nil)
	if len(custom.exclude) != len(before) {
		t.Error("WithExclude(nil) should keep the existing exclusions")
	}
}
