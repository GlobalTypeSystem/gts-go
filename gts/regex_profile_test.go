/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

// Profile membership cases follow the conformance suite's support categories.
func TestCompileRegexProfile_ProfileMembership(t *testing.T) {
	supported := []string{
		"", "abc", "é😀", "^[A-Za-z0-9]+$", "[^abc]", "(foo|bar)+", "(?:ab)+",
		"[a-z]{1,3}", "a{3}", "a{2,}", "a{0}", "a{0,0}", "a{1000}", "(a(b)?c)*", "a.*?b",
		"a*?b+?c??d{1,2}?e{2,}?f{2}?", `\(x\)\.\*`, `\\C`, `\n\r\t\f\v`, `\x41`,
		`\d\D\w\W\s\S`, "^a.c$", "(a*)*b", "(a+)+$", "^(a|aa)+$", `^(?:(a|aa)+$|a+!$)`,
		`\^\$\\\.\*\+\?\(\)\[\]\{\}\|\/`, "a|", "|", "()", "(?:)*", "a/b", "#&~ ",
		// Classes: '-' first or last, escaped '-' and ']', shorthands, '^' not first.
		"[-a]", "[a-]", "[^-a]", `[\-\]]`, `[\d\s\w\D\S\W]`, `[\w-]`, "[a^]", `[\x00-\x7F]`,
		"[😀é]", `[\/\[]`, "[&]", "[a&b~c]",
		// At the common support bounds.
		"a{1000,}", "(?:a{10}){100}", "(?:a{0}){500}", "(?:a*){500}",
		strings.Repeat("a{1000}", 4) + strings.Repeat("a", 72), // expanded length 4096
		strings.Repeat("a", 4096),
		strings.Repeat("é", 4096), // code points, not bytes
		strings.Repeat("(", maxRegexGroupDepth) + "a" + strings.Repeat(")", maxRegexGroupDepth),
		strings.Repeat("(?:", maxRegexGroupDepth) + "a" + strings.Repeat(")", maxRegexGroupDepth),
	}
	for _, pattern := range supported {
		if _, err := compileRegexProfile(pattern); err != nil {
			t.Errorf("%q: expected the profile to support it: %v", pattern, err)
		}
	}

	unsupported := []string{
		// Malformed in ECMA-262 `u` and RE2.
		"[", "[unclosed", "(unclosed", "a)", "a{3,2}", `\`, "*abc", "a**",
		// ECMA-262 `u` only.
		"a(?=b)", "a(?!b)", "(?<=a)b", "(?<!a)b", `^(a+)\1$`, `^(?<w>a)\k<w>$`, `\` + "u0041",
		`\cA`, `\0`, `[\b]`,
		// RE2 only.
		`\Qx.y\E`, "(?U)a+", "(?i)abc", "(?s).", "(?m)^a", `abc\z`, `\Aabc`, `\x{41}`, "[[:alpha:]]",
		`\C`, `(?P<n>a)`, `\pL`, "a{01}", "a{,2}", "a{", "a{2", "}", "]", "a{2}{3}", "a???",
		// Neither.
		"(?>a+)b", "a++b", `\e`, `\x4`, `\xZZ`, `\-`,
		// Accepted by both but excluded by the draft.
		"(?i:a)b", "(?<w>a)", `^\p{L}+$`, `\ba\b`, `\B`,
		// Assertions are not quantifiable in ECMA-262 `u`.
		"^*", "$+", "a^{2}",
		// Ambiguous class spellings and nested classes.
		"[]", "[^]", "[]a]", "[a[]", "[a&&b]", "[a--b]", "[a~~b]", "[a-b-c]", `[\d-z]`,
		`[a-\d]`, `[\s-a]`, `[a-\S]`, "[z-a]", "[--]",
		// Beyond the common support bounds.
		"a{1001}", "(?:a{1000}){2}", "(?:a{1000,}){2}", "(?:a{2}){501}", "a{99999999999999999999}",
		strings.Repeat("a{1000}", 4) + strings.Repeat("a", 73), // expanded length 4097
		strings.Repeat("a", 4097),
		"(?:ab){1000}", "(?:a*){1000}", "(?:a{0}){1000}", // expanded length 6006, 6006, 8006
		`\w{1000}\w{1000}\w{100}`,
		strings.Repeat("(", maxRegexGroupDepth+1) + "a" + strings.Repeat(")", maxRegexGroupDepth+1),
		strings.Repeat("(", 100000) + strings.Repeat(")", 100000),
		// Not UTF-8.
		"\xff",
	}
	for _, pattern := range unsupported {
		if _, err := compileRegexProfile(pattern); err == nil {
			t.Errorf("%q: expected the profile to reject it", pattern)
		}
	}
}

// Reference RE2 semantics: `.` excludes only LF; `\s` is [\t\n\f\r ].
func TestCompileRegexProfile_Matching(t *testing.T) {
	cases := []struct {
		pattern    string
		matches    []string
		nonMatches []string
	}{
		{"abc", []string{"abc", "xabcx"}, []string{"ab", "a-b-c"}},
		{"^abc$", []string{"abc"}, []string{"ABC", "x\nabc\ny", "abc\n", "x\nabc"}},
		{"", []string{"", "abc"}, nil},
		{"^[^0-9]+$", []string{"abc", "a\nb"}, []string{"a1", ""}},
		{"^a+?b??c{1,2}?$", []string{"ac", "abcc", "aabc"}, []string{"a", "bc", "accc"}},
		{`^\(a\.b\)\*$`, []string{"(a.b)*"}, []string{"(axb)*"}},
		{`^\t\n\r\f\v$`, []string{"\t\n\r\f\v"}, []string{"\t\n\r\f"}},
		{`^\x41$`, []string{"A"}, []string{"a", "x41"}},
		{`^\d$`, []string{"0", "9"}, []string{"a", "\u0661"}},
		{`^\D$`, []string{"a", "\u0661"}, []string{"0"}},
		{`^\w$`, []string{"a", "Z", "0", "_"}, []string{"-", "é"}},
		{`^\W$`, []string{"-", "é"}, []string{"a", "_"}},
		// [\t\n\f\r ], not ECMA-262 WhiteSpace and LineTerminator.
		{`^\s$`, []string{" ", "\t", "\n", "\f", "\r"}, []string{"", "a", "\v", "\u00a0", "\u2028", "\ufeff", "\u0085"}},
		{`^\S$`, []string{"a", "\v", "\u00a0", "\ufeff", "😀"}, []string{" ", "\t", "\n"}},
		{`^[\s]$`, []string{" ", "\r"}, []string{"a", "\v"}},
		{`^[^\s]$`, []string{"a", "\v"}, []string{" ", "\r"}},
		{`^[\S]$`, []string{"a", "\u00a0"}, []string{" "}},
		{`^[^\S]$`, []string{" "}, []string{"a", "\u00a0"}},
		{`^[\Sa]$`, []string{"a", "b"}, []string{" "}},
		{`^[\s\S]$`, []string{"a", "\n", "\r", "\u2028", "\u2029", "😀"}, []string{""}},
		{`^[^\s\S]$`, nil, []string{"a", "\n", " "}},
		{`^[a\S]$`, []string{"a", "\u00a0"}, []string{" "}},
		{`^[^a\S]$`, []string{" "}, []string{"a", "\u00a0"}},
		{`^[\d\D]$`, []string{"a", "1", "\n"}, nil},
		{`^[^\w\W]$`, nil, []string{"a", "-"}},
		{`^[.]$`, []string{"."}, []string{"a"}},
		{`^\.$`, []string{"."}, []string{"a"}},
		{"(^)*a", []string{"a", "ba"}, nil},
		{`^[a\x26\x26b]$`, []string{"&", "a"}, []string{"c"}},
		{`^[\-\-]$`, []string{"-"}, []string{"a"}},
		// Unlike ECMA-262, `.` matches CR, U+2028 and U+2029.
		{"^.$", []string{"a", " ", "\t", "\r", "\u2028", "\u2029", "\u0085", "😀", "é"}, []string{"", "\n", "ab", "😀😀"}},
		{"^.{2}$", []string{"ab", "😀a"}, []string{"😀"}},
		{"^[^a]$", []string{"😀", "\n"}, []string{"a", "😀😀"}},
		{"^[😀é]$", []string{"😀", "é"}, []string{"😀é"}},
		// Class literals that are metacharacters elsewhere.
		{`^[\-\]\[\\^]+$`, []string{"-]", `[\^`}, []string{"a"}},
		{"^[-a]$", []string{"-", "a"}, []string{"b"}},
		{"^#&~ /$", []string{"#&~ /"}, []string{"#&~/"}},
		{`^[\x00-\x1F]$`, []string{"\x00", "\x1f"}, []string{" "}},
		{"^a{1000}$", []string{strings.Repeat("a", 1000)}, []string{strings.Repeat("a", 1001)}},
	}
	for _, tc := range cases {
		re, err := compileRegexProfile(tc.pattern)
		if err != nil {
			t.Fatalf("%q: unexpected compile error: %v", tc.pattern, err)
		}
		if re.String() != tc.pattern {
			t.Errorf("String() = %q, want the source %q", re.String(), tc.pattern)
		}
		for _, s := range tc.matches {
			if !re.MatchString(s) {
				t.Errorf("%q should match %q", tc.pattern, s)
			}
		}
		for _, s := range tc.nonMatches {
			if re.MatchString(s) {
				t.Errorf("%q should not match %q", tc.pattern, s)
			}
		}
	}
}

// Classic ReDoS shapes must finish on long inputs.
func TestCompileRegexProfile_LinearMatching(t *testing.T) {
	input := strings.Repeat("a", 50_000) + "!"
	for _, pattern := range []string{"^(a+)+$", "^(a|aa)+$", "^a*a*a*b$", "(a*)*b"} {
		re, err := compileRegexProfile(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if re.MatchString(input) {
			t.Errorf("%q should not match the attack input", pattern)
		}
	}
}

// Casting asserts format "regex" in every dialect, like instance validation.
func TestValidateWithGtsIDTolerance_RegexFormat(t *testing.T) {
	for _, dialect := range []string{
		"http://json-schema.org/draft-07/schema#",
		"https://json-schema.org/draft/2019-09/schema",
		"https://json-schema.org/draft/2020-12/schema",
	} {
		value := map[string]any{"type": "string", "format": "regex"}
		schema := func(valueSchema map[string]any) map[string]any {
			return map[string]any{
				"$id":        "gts://gts.x.test.cast.regexformat.v1~",
				"$schema":    dialect,
				"type":       "object",
				"properties": map[string]any{"value": valueSchema},
			}
		}
		positive := schema(value)
		negated := schema(map[string]any{"not": value})
		store := NewGtsStore(nil)
		for _, tc := range []struct {
			value     string
			supported bool
		}{{"a+", true}, {"a(?=b)", false}, {"[", false}} {
			instance := map[string]any{"value": tc.value}
			if err := validateWithGtsIDTolerance(instance, positive, store); (err == nil) != tc.supported {
				t.Errorf("%s: format regex %q: err = %v, want supported=%v", dialect, tc.value, err, tc.supported)
			}
			if err := validateWithGtsIDTolerance(instance, negated, store); (err == nil) == tc.supported {
				t.Errorf("%s: not format regex %q: err = %v, want supported=%v", dialect, tc.value, err, tc.supported)
			}
		}
	}
}

func TestCompileRegexProfile_EngineFailure(t *testing.T) {
	engineErr := errors.New("injected engine failure")
	previous := compileRegexEngine
	compileRegexEngine = func(string) (*regexp.Regexp, error) {
		return nil, engineErr
	}
	t.Cleanup(func() { compileRegexEngine = previous })

	re, err := compileRegexProfile("^a$")
	var failure regexEngineFailure
	if re != nil || !errors.As(err, &failure) || !errors.Is(err, engineErr) {
		t.Fatalf("expected a wrapped engine failure and no regexp, got: %v, %v", re, err)
	}
}

func TestValidateSchemaChain_RegexEngineFailure(t *testing.T) {
	store := NewGtsStore(nil)
	const base = "gts.x.regex.engine.base.v1~"
	const derived = base + "x.regex.engine.child.v1~"
	registerSchema(t, store, map[string]any{
		"$id":     "gts://" + base,
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]any{
			"value": map[string]any{"type": "string", "pattern": "^a$"},
		},
	})
	registerSchema(t, store, map[string]any{
		"$id":     "gts://" + derived,
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]any{
			"value": map[string]any{"type": "string", "const": "a"},
		},
	})

	previous := compileRegexEngine
	compileRegexEngine = func(string) (*regexp.Regexp, error) {
		return nil, errors.New("injected engine failure")
	}
	t.Cleanup(func() { compileRegexEngine = previous })

	result := store.ValidateSchemaChain(derived)
	if result.OK || !strings.Contains(result.Error, "engine failed") ||
		strings.Contains(result.Error, "unsupported") {
		t.Fatalf("expected an engine failure, got: %+v", result)
	}
}

// An engine failure on an in-profile expression fails the whole validation:
// it is neither an unsupported expression nor a format violation `not` inverts.
func TestRegexEngineFailureFailsWholeValidation(t *testing.T) {
	compileRegexEngine = func(string) (*regexp.Regexp, error) {
		return nil, errors.New("injected engine failure")
	}
	t.Cleanup(func() { compileRegexEngine = regexp.Compile })

	store := NewGtsStore(nil)
	const formatType = "gts.x.regex.engine.format.v1~"
	mustRegister(t, store, map[string]any{
		"$id":     "gts://" + formatType,
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]any{
			"value": map[string]any{"not": map[string]any{"format": "regex"}},
		},
	})
	result := store.ValidateTransientJSON(map[string]any{"value": "abc"}, formatType)
	if result.OK || !strings.Contains(result.Error, "engine failed") {
		t.Fatalf("format: regex under not must fail the validation, got: %+v", result)
	}

	// Schema validation compiles every pattern, including inactive ones.
	err := store.validateJSONSchema(map[string]any{
		"$id":     "gts://gts.x.regex.engine.pattern.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"anyOf": []any{true, map[string]any{
			"properties": map[string]any{"value": map[string]any{"pattern": "^a$"}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "engine failed") ||
		strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected an engine failure, got: %v", err)
	}
}
