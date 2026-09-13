/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import "testing"

// ── compatibility_helpers.go ────────────────────────────────────────────────

func TestGetNumber_AllTypes(t *testing.T) {
	m := map[string]any{
		"f64": float64(3.14),
		"i":   int(42),
		"i64": int64(99),
		"str": "not-a-number",
	}
	if v := getNumber(m, "f64"); v == nil || *v != 3.14 {
		t.Errorf("f64: %v", v)
	}
	if v := getNumber(m, "i"); v == nil || *v != 42 {
		t.Errorf("int: %v", v)
	}
	if v := getNumber(m, "i64"); v == nil || *v != 99 {
		t.Errorf("int64: %v", v)
	}
	if v := getNumber(m, "str"); v != nil {
		t.Errorf("string should return nil: %v", v)
	}
	if v := getNumber(m, "missing"); v != nil {
		t.Error("missing key should return nil")
	}
}

func TestGetStringSlice(t *testing.T) {
	m := map[string]any{
		"enum": []any{"a", "b", "c"},
		"mix":  []any{"a", 42, "c"},
		"num":  []any{1, 2},
		"str":  "not-a-slice",
	}
	r := getStringSlice(m, "enum")
	if len(r) != 3 || r[0] != "a" || r[2] != "c" {
		t.Errorf("enum: %v", r)
	}
	r2 := getStringSlice(m, "mix")
	if len(r2) != 2 { // skips non-string 42
		t.Errorf("mix: %v", r2)
	}
	r3 := getStringSlice(m, "num")
	if len(r3) != 0 {
		t.Errorf("num: should have 0 strings: %v", r3)
	}
	r4 := getStringSlice(m, "str")
	if len(r4) != 0 {
		t.Errorf("str: should return empty: %v", r4)
	}
	r5 := getStringSlice(m, "missing")
	if len(r5) != 0 {
		t.Error("missing: should return empty")
	}
}

func TestGetMap(t *testing.T) {
	inner := map[string]any{"x": 1}
	m := map[string]any{"obj": inner, "str": "nope"}
	if getMap(m, "obj") == nil {
		t.Error("expected map")
	}
	if getMap(m, "str") != nil {
		t.Error("string should return nil")
	}
	if getMap(m, "missing") != nil {
		t.Error("missing should return nil")
	}
}

func TestStringSliceToSet(t *testing.T) {
	s := stringSliceToSet([]string{"a", "b", "a"})
	if len(s) != 2 || !s["a"] || !s["b"] {
		t.Errorf("expected {a, b}, got %v", s)
	}
}

func TestSetToString(t *testing.T) {
	s := map[string]bool{"b": true, "a": true}
	r := setToString(s)
	if r != "a, b" {
		t.Errorf("expected 'a, b', got %q", r)
	}
}

func TestFloatToString(t *testing.T) {
	tests := []struct {
		in  float64
		out string
	}{
		{0, "0"},
		{1.5, "1.5"},
		{42, "42"},
		{3.14, "3.14"},
		{100.0, "100"},
	}
	for _, tt := range tests {
		r := floatToString(tt.in)
		if r != tt.out {
			t.Errorf("floatToString(%v): want %q, got %q", tt.in, tt.out, r)
		}
	}
}

// ── compatibility.go internal helpers ───────────────────────────────────────

func TestVerdict(t *testing.T) {
	tr, fa := true, false
	if verdict(&tr) != VerdictCompatible {
		t.Error("true should be compatible")
	}
	if verdict(&fa) != VerdictIncompatible {
		t.Error("false should be incompatible")
	}
	if verdict(nil) != VerdictUnknown {
		t.Error("nil should be unknown")
	}
}

func TestFullVerdict(t *testing.T) {
	tests := []struct {
		bw, fw, want string
	}{
		{VerdictCompatible, VerdictCompatible, VerdictCompatible},
		{VerdictIncompatible, VerdictCompatible, VerdictIncompatible},
		{VerdictCompatible, VerdictIncompatible, VerdictIncompatible},
		{VerdictIncompatible, VerdictIncompatible, VerdictIncompatible},
		{VerdictUnknown, VerdictCompatible, VerdictUnknown},
		{VerdictCompatible, VerdictUnknown, VerdictUnknown},
		{VerdictUnknown, VerdictUnknown, VerdictUnknown},
		{VerdictUnknown, VerdictIncompatible, VerdictIncompatible},
	}
	for _, tt := range tests {
		got := fullVerdict(tt.bw, tt.fw)
		if got != tt.want {
			t.Errorf("fullVerdict(%s, %s): want %s, got %s", tt.bw, tt.fw, tt.want, got)
		}
	}
}

func TestSanitizeSchema(t *testing.T) {
	schema := map[string]any{
		"$id": "x", "$schema": "y", "description": "ignored",
		"x-gts-ref": "gts.x~", "type": "string",
		"properties": map[string]any{
			"f": map[string]any{"$comment": "gone", "type": "integer"},
		},
		"allOf": []any{
			map[string]any{"title": "stripped", "minimum": float64(0)},
		},
	}
	r := sanitizeSchema(schema).(map[string]any)
	if _, ok := r["$id"]; ok {
		t.Error("$id should be stripped")
	}
	if _, ok := r["description"]; ok {
		t.Error("description should be stripped")
	}
	if _, ok := r["x-gts-ref"]; ok {
		t.Error("x-gts-ref should be stripped")
	}
	if r["type"] != "string" {
		t.Error("type should be preserved")
	}
	// Check nested
	props := r["properties"].(map[string]any)
	f := props["f"].(map[string]any)
	if _, ok := f["$comment"]; ok {
		t.Error("nested $comment should be stripped")
	}
}

func TestSanitizeSchema_DropsRedundantType(t *testing.T) {
	schema := map[string]any{
		"type": "string",
		"enum": []any{"a", "b"},
	}
	r := sanitizeSchema(schema).(map[string]any)
	if _, ok := r["type"]; ok {
		t.Error("type should be dropped when redundant with enum")
	}
}

func TestSanitizeSchema_KeepsTypeWithMixedEnum(t *testing.T) {
	schema := map[string]any{
		"type": "string",
		"enum": []any{"a", float64(1)},
	}
	r := sanitizeSchema(schema).(map[string]any)
	if _, ok := r["type"]; !ok {
		t.Error("type should be kept when enum has mixed types")
	}
}

func TestSanitizeSchema_Passthrough(t *testing.T) {
	if sanitizeSchema("abc") != "abc" {
		t.Error("string should pass through")
	}
	if sanitizeSchema(float64(5)) != float64(5) {
		t.Error("number should pass through")
	}
}

func TestValueHasType(t *testing.T) {
	tests := []struct {
		val  any
		typ  string
		want bool
	}{
		{"hello", "string", true},
		{float64(1), "number", true},
		{float64(1), "integer", true},
		{float64(1.5), "integer", false},
		{true, "boolean", true},
		{nil, "null", true},
		{[]any{1}, "array", true},
		{map[string]any{}, "object", true},
		{"hello", "number", false},
		{float64(1), "string", false},
		{true, "string", false},
		{nil, "string", false},
	}
	for _, tt := range tests {
		got := valueHasType(tt.val, tt.typ)
		if got != tt.want {
			t.Errorf("valueHasType(%v, %q): want %v, got %v", tt.val, tt.typ, tt.want, got)
		}
	}
}

func TestSchemaAcceptsValue(t *testing.T) {
	// Type check
	schema := map[string]any{"type": "string"}
	if schemaAcceptsValue(schema, float64(1)) {
		t.Error("string schema should reject number")
	}
	if !schemaAcceptsValue(schema, "hello") {
		t.Error("string schema should accept string")
	}

	// Const check
	constSchema := map[string]any{"const": "exact"}
	if schemaAcceptsValue(constSchema, "wrong") {
		t.Error("const should reject wrong value")
	}
	if !schemaAcceptsValue(constSchema, "exact") {
		t.Error("const should accept exact value")
	}

	// Enum check
	enumSchema := map[string]any{"enum": []any{"a", "b"}}
	if schemaAcceptsValue(enumSchema, "c") {
		t.Error("enum should reject 'c'")
	}
	if !schemaAcceptsValue(enumSchema, "a") {
		t.Error("enum should accept 'a'")
	}

	// Numeric bounds
	numSchema := map[string]any{"type": "number", "minimum": float64(0), "maximum": float64(100)}
	if schemaAcceptsValue(numSchema, float64(-1)) {
		t.Error("should reject below minimum")
	}
	if schemaAcceptsValue(numSchema, float64(101)) {
		t.Error("should reject above maximum")
	}
	if !schemaAcceptsValue(numSchema, float64(50)) {
		t.Error("should accept within bounds")
	}

	// Exclusive bounds
	exSchema := map[string]any{"type": "number", "exclusiveMinimum": float64(0), "exclusiveMaximum": float64(10)}
	if schemaAcceptsValue(exSchema, float64(0)) {
		t.Error("should reject exclusive minimum")
	}
	if schemaAcceptsValue(exSchema, float64(10)) {
		t.Error("should reject exclusive maximum")
	}
	if !schemaAcceptsValue(exSchema, float64(5)) {
		t.Error("should accept between exclusive bounds")
	}

	// String length
	strSchema := map[string]any{"type": "string", "minLength": float64(2), "maxLength": float64(5)}
	if schemaAcceptsValue(strSchema, "a") {
		t.Error("should reject short string")
	}
	if schemaAcceptsValue(strSchema, "toolongstring") {
		t.Error("should reject long string")
	}
	if !schemaAcceptsValue(strSchema, "abc") {
		t.Error("should accept valid length string")
	}
}

func TestIsOpenModel(t *testing.T) {
	if !isOpenModel(map[string]any{}) {
		t.Error("no AP = open")
	}
	if !isOpenModel(map[string]any{"additionalProperties": true}) {
		t.Error("AP:true = open")
	}
	if isOpenModel(map[string]any{"additionalProperties": false}) {
		t.Error("AP:false = closed")
	}
	// Schema-valued AP = partially open
	if !isOpenModel(map[string]any{"additionalProperties": map[string]any{"type": "string"}}) {
		t.Error("AP:schema = partially open")
	}
}

func TestIsAcceptAllSchema(t *testing.T) {
	if !isAcceptAllSchema(true) {
		t.Error("boolean true is accept-all")
	}
	if isAcceptAllSchema(false) {
		t.Error("boolean false is not accept-all")
	}
	if !isAcceptAllSchema(map[string]any{}) {
		t.Error("empty object is accept-all")
	}
	if !isAcceptAllSchema(map[string]any{"description": "ignored"}) {
		t.Error("annotation-only object is accept-all")
	}
	if isAcceptAllSchema(map[string]any{"type": "string"}) {
		t.Error("typed schema is not accept-all")
	}
	if isAcceptAllSchema("nope") {
		t.Error("string is not accept-all")
	}
}

func TestCheckArrayInclusion(t *testing.T) {
	// Both have items, subset items ⊆ superset items
	sub := map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}
	sup := map[string]any{"type": "array", "items": map[string]any{"type": "number"}}
	r := checkArrayInclusion(sub, sup)
	if r == nil || !*r {
		t.Error("integer items should be subset of number items")
	}

	// Superset has items constraint, subset doesn't
	subOpen := map[string]any{"type": "array"}
	supConstrained := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	r2 := checkArrayInclusion(subOpen, supConstrained)
	if r2 == nil || *r2 {
		t.Error("open items should not be subset of constrained items")
	}

	// Superset has no items constraint → accepts any
	subConstrained := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	supOpen := map[string]any{"type": "array"}
	r3 := checkArrayInclusion(subConstrained, supOpen)
	if r3 == nil || !*r3 {
		t.Error("constrained items should be subset of open items")
	}

	// Both have no items → equivalent
	r4 := checkArrayInclusion(map[string]any{"type": "array"}, map[string]any{"type": "array"})
	if r4 == nil || !*r4 {
		t.Error("both open should be subset")
	}
}

func TestCheckPrimitiveInclusion(t *testing.T) {
	// Superset has const, subset has matching const
	sub := map[string]any{"const": "hello"}
	sup := map[string]any{"const": "hello"}
	r := checkPrimitiveInclusion(sub, sup, "string")
	if r == nil || !*r {
		t.Error("matching consts should be subset")
	}

	// Superset has const, subset has different const
	sub2 := map[string]any{"const": "hello"}
	sup2 := map[string]any{"const": "world"}
	r2 := checkPrimitiveInclusion(sub2, sup2, "string")
	if r2 == nil || *r2 {
		t.Error("different consts should not be subset")
	}

	// Superset has const, subset has no const
	sub3 := map[string]any{"type": "string"}
	sup3 := map[string]any{"const": "only"}
	r3 := checkPrimitiveInclusion(sub3, sup3, "string")
	if r3 == nil || *r3 {
		t.Error("unconstrained should not be subset of const")
	}

	// Superset has enum, subset has matching enum (subset)
	sub4 := map[string]any{"enum": []any{"a"}}
	sup4 := map[string]any{"enum": []any{"a", "b"}}
	r4 := checkPrimitiveInclusion(sub4, sup4, "string")
	if r4 == nil || !*r4 {
		t.Error("subset enum should be included")
	}

	// Superset has enum, subset has no enum → not subset
	sub5 := map[string]any{"type": "string"}
	sup5 := map[string]any{"enum": []any{"a", "b"}}
	r5 := checkPrimitiveInclusion(sub5, sup5, "string")
	if r5 == nil || *r5 {
		t.Error("unconstrained should not be subset of enum")
	}

	// No constraints on either side (same type) → equivalent
	r6 := checkPrimitiveInclusion(map[string]any{}, map[string]any{}, "boolean")
	if r6 == nil || !*r6 {
		t.Error("same unconstrained type should be subset")
	}
}

func TestCheckStringInclusion(t *testing.T) {
	// Superset has wider length range
	sub := map[string]any{"type": "string", "minLength": float64(3), "maxLength": float64(10)}
	sup := map[string]any{"type": "string", "minLength": float64(1), "maxLength": float64(20)}
	r := checkStringInclusion(sub, sup)
	if r != nil && !*r {
		t.Error("narrower range should be subset of wider range")
	}

	// Subset has wider range than superset
	sub2 := map[string]any{"type": "string", "minLength": float64(1), "maxLength": float64(20)}
	sup2 := map[string]any{"type": "string", "minLength": float64(3), "maxLength": float64(10)}
	r2 := checkStringInclusion(sub2, sup2)
	if r2 == nil || *r2 {
		t.Error("wider range should not be subset of narrower range")
	}
}

func TestCheckNumericInclusion_ExclusiveBounds(t *testing.T) {
	sub := map[string]any{"type": "number", "exclusiveMinimum": float64(0), "exclusiveMaximum": float64(100)}
	sup := map[string]any{"type": "number", "exclusiveMinimum": float64(0), "exclusiveMaximum": float64(100)}
	r := checkNumericInclusion(sub, sup)
	if r != nil && !*r {
		t.Error("same exclusive bounds should be subset")
	}
}

func TestCheckBoundsInclusion(t *testing.T) {
	sub := map[string]any{"minimum": float64(5)}
	sup := map[string]any{"minimum": float64(0)}

	// Sub min (5) >= sup min (0) → OK for lower bound
	r := checkBoundsInclusion(sub, sup, "minimum", false)
	if r != nil && !*r {
		t.Error("subset floor 5 >= superset floor 0")
	}

	// Sub has no bound, sup does → subset is wider
	subNone := map[string]any{}
	r2 := checkBoundsInclusion(subNone, sup, "minimum", false)
	if r2 == nil || *r2 {
		t.Error("missing bound in subset means wider → not subschema")
	}

	// Superset has no bound → OK
	r3 := checkBoundsInclusion(sub, map[string]any{}, "minimum", false)
	if r3 != nil {
		t.Error("superset with no bound → nil (no constraint to check)")
	}
}

func TestDeepCopyMap(t *testing.T) {
	orig := map[string]any{
		"a": "val",
		"b": map[string]any{"nested": "deep"},
		"c": []any{1, map[string]any{"x": "y"}},
	}
	cp := deepCopyMap(orig)
	// Modify copy
	cp["a"] = "changed"
	nested := cp["b"].(map[string]any)
	nested["nested"] = "modified"
	// Original should be unchanged
	if orig["a"] != "val" {
		t.Error("original should be unchanged")
	}
	origNested := orig["b"].(map[string]any)
	if origNested["nested"] != "deep" {
		t.Error("original nested should be unchanged")
	}
}
