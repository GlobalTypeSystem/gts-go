/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"regexp"
	"strconv"
	"unicode/utf8"
)

// GTS regex profile (gts-spec §11.0.1, ADR-0006): the shared ECMA-262 `u`/RE2
// syntax subset. Accepted expressions compile unchanged with Go's regexp,
// giving linear-time matching with the reference semantics and no deviations.

// Common support bounds of the profile (ADR-0006 "Common support bounds").
const (
	// Length in code points after expanding counted repetitions.
	maxRegexExpandedLength = 4096

	maxRegexGroupDepth = 32
	// Maximum count and product of counts along a nesting path.
	maxRegexRepeat = 1000
)

// compileRegexEngine compiles an in-profile expression; tests replace it.
var compileRegexEngine = regexp.Compile

// regexEngineFailure distinguishes an engine failure on an in-profile
// expression from an unsupported expression.
type regexEngineFailure struct{ err error }

func (f regexEngineFailure) Error() string { return f.err.Error() }
func (f regexEngineFailure) Unwrap() error { return f.err }

// compileRegexProfile checks profile membership and compiles source unchanged.
// An engine failure is returned separately from a profile membership error.
func compileRegexProfile(source string) (*regexp.Regexp, error) {
	if err := checkRegexProfile(source); err != nil {
		return nil, err
	}
	re, err := compileRegexEngine(source)
	if err != nil {
		// Not expected within the support bounds.
		return nil, regexEngineFailure{fmt.Errorf("regular expression engine failed on %s: %w", regexExcerpt(source), err)}
	}
	return re, nil
}

// checkRegexProfile rejects source outside the profile or its support bounds.
func checkRegexProfile(source string) error {
	if err := checkRegexSyntax(source); err != nil {
		return fmt.Errorf("unsupported regular expression %s: %v", regexExcerpt(source), err)
	}
	return nil
}

func checkRegexSyntax(source string) error {
	if !utf8.ValidString(source) {
		return fmt.Errorf("invalid UTF-8")
	}
	// Expanded length is at least the source length.
	if utf8.RuneCountInString(source) > maxRegexExpandedLength {
		return fmt.Errorf("expanded length above %d", maxRegexExpandedLength)
	}
	p := &regexProfileParser{src: []rune(source)}
	length, _ := p.parseDisjunction()
	if p.err == nil && !p.eof() {
		// parseDisjunction stops only at the end or at an unmatched ')'.
		p.fail("unmatched ')'")
	}
	if p.err != nil {
		return p.err
	}
	if length > maxRegexExpandedLength {
		return fmt.Errorf("expanded length above %d", maxRegexExpandedLength)
	}
	return nil
}

// regexExcerpt quotes source for an error message, shortened if long.
func regexExcerpt(source string) string {
	const maxRunes = 64
	runes := 0
	for i := range source {
		if runes == maxRunes {
			return strconv.Quote(source[:i]) + "..."
		}
		runes++
	}
	return strconv.Quote(source)
}

// satAdd and satMul saturate just above the largest bound, so sizes of deeply
// repeated expressions cannot overflow.
func satAdd(a, b int) int { return min(a+b, maxRegexExpandedLength+1) }

func satMul(a, b int) int {
	if a != 0 && b > (maxRegexExpandedLength+1)/a {
		return maxRegexExpandedLength + 1
	}
	return min(a*b, maxRegexExpandedLength+1)
}

type regexProfileParser struct {
	src   []rune
	pos   int
	depth int
	err   error
}

func (p *regexProfileParser) eof() bool { return p.pos >= len(p.src) }

// peek returns the code point at offset from the current position, or -1.
func (p *regexProfileParser) peek(offset int) rune {
	if i := p.pos + offset; i < len(p.src) {
		return p.src[i]
	}
	return -1
}

func (p *regexProfileParser) fail(format string, args ...any) {
	if p.err == nil {
		p.err = fmt.Errorf("%s at offset %d", fmt.Sprintf(format, args...), p.pos)
	}
}

func isRegexQuantifierStart(r rune) bool {
	return r == '*' || r == '+' || r == '?' || r == '{'
}

// The parse functions return the expanded length of what they parsed and the
// largest product of counted repetitions along a nesting path within it.

func (p *regexProfileParser) parseDisjunction() (length, product int) {
	length, product = p.parseAlternative()
	for p.err == nil && p.peek(0) == '|' {
		p.pos++
		l, n := p.parseAlternative()
		length, product = satAdd(length, l+1), max(product, n)
	}
	return length, product
}

func (p *regexProfileParser) parseAlternative() (length, product int) {
	product = 1
	for p.err == nil && !p.eof() && p.peek(0) != '|' && p.peek(0) != ')' {
		l, n := p.parseTerm()
		length, product = satAdd(length, l), max(product, n)
	}
	return length, product
}

func (p *regexProfileParser) parseTerm() (length, product int) {
	start := p.pos
	switch c := p.peek(0); c {
	case '^', '$':
		p.pos++
		if isRegexQuantifierStart(p.peek(0)) {
			p.fail("quantifier after an assertion")
		}
		return 1, 1
	case '(':
		groupLength, groupProduct := p.parseGroup()
		if p.err != nil {
			return 0, 0
		}
		return p.parseQuantifier(groupLength, groupProduct)
	case '[':
		p.parseClass()
	case '\\':
		p.parseEscape(false)
	case '*', '+', '?', '{':
		p.fail("nothing to repeat")
	case '}', ']':
		p.fail("unescaped %q", c)
	default: // a literal or '.'
		p.pos++
	}
	if p.err != nil {
		return 0, 0
	}
	// A literal, escape or class counts by its spelling.
	return p.parseQuantifier(p.pos-start, 1)
}

func (p *regexProfileParser) parseGroup() (length, product int) {
	start := p.pos
	p.pos++ // '('
	if p.peek(0) == '?' {
		if p.peek(1) != ':' {
			p.fail("unsupported group construct")
			return 0, 0
		}
		p.pos += 2
	}
	if p.depth++; p.depth > maxRegexGroupDepth {
		p.fail("groups nested deeper than %d", maxRegexGroupDepth)
		return 0, 0
	}
	opening := p.pos - start
	length, product = p.parseDisjunction()
	p.depth--
	if p.err != nil {
		return 0, 0
	}
	if p.peek(0) != ')' {
		p.fail("unterminated group")
		return 0, 0
	}
	p.pos++
	return satAdd(length, opening+1), product
}

// parseQuantifier parses an optional quantifier after an atom of the given
// expanded length and repetition product.
func (p *regexProfileParser) parseQuantifier(atomLength, atomProduct int) (length, product int) {
	start := p.pos
	copies, factor := 1, 1
	switch p.peek(0) {
	case '*', '+', '?':
		p.pos++
	case '{':
		low, high, ok := p.parseBraceQuantifier()
		if !ok {
			return 0, 0
		}
		if high == -1 { // {n,}
			copies, factor = low+1, max(low, 1)
		} else {
			copies, factor = max(high, 1), max(high, 1)
		}
	default:
		return atomLength, atomProduct
	}
	if p.peek(0) == '?' { // lazy
		p.pos++
	}
	if isRegexQuantifierStart(p.peek(0)) {
		p.fail("nested quantifier")
		return 0, 0
	}
	if product = factor * atomProduct; product > maxRegexRepeat {
		p.fail("nested repetition above %d", maxRegexRepeat)
		return 0, 0
	}
	return satAdd(p.pos-start, satMul(copies, atomLength)), product
}

// parseBraceQuantifier returns n and m for {n}, {n,} or {n,m}; m = -1 for {n,}.
// Other uses of '{' are invalid in ECMA-262 `u`, even if RE2 accepts them.
func (p *regexProfileParser) parseBraceQuantifier() (low, high int, ok bool) {
	p.pos++ // '{'
	if low, ok = p.parseRepeatCount(); !ok {
		return 0, 0, false
	}
	high = low
	if p.peek(0) == ',' {
		p.pos++
		high = -1
		if p.peek(0) != '}' {
			if high, ok = p.parseRepeatCount(); !ok {
				return 0, 0, false
			}
		}
	}
	if p.peek(0) != '}' {
		p.fail("malformed repetition")
		return 0, 0, false
	}
	p.pos++
	if high != -1 && high < low {
		p.fail("repetition range {%d,%d} out of order", low, high)
		return 0, 0, false
	}
	return low, high, true
}

// parseRepeatCount parses a decimal count without leading zeros (RE2 reads
// `a{01}` as a literal) and at most maxRegexRepeat.
func (p *regexProfileParser) parseRepeatCount() (int, bool) {
	start := p.pos
	for p.peek(0) >= '0' && p.peek(0) <= '9' {
		p.pos++
	}
	digits := string(p.src[start:p.pos])
	switch {
	case digits == "":
		p.fail("malformed repetition")
		return 0, false
	case len(digits) > 1 && digits[0] == '0':
		p.fail("repetition count with a leading zero")
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n > maxRegexRepeat {
		p.fail("repetition count above %d", maxRegexRepeat)
		return 0, false
	}
	return n, true
}

// parseEscape parses the escape at p.pos. It returns the escaped code point,
// or isSet for a shorthand class.
func (p *regexProfileParser) parseEscape(inClass bool) (r rune, isSet, ok bool) {
	p.pos++ // '\'
	if p.eof() {
		p.fail("trailing backslash")
		return 0, false, false
	}
	c := p.peek(0)
	p.pos++
	switch c {
	case '^', '$', '\\', '.', '*', '+', '?', '(', ')', '[', ']', '{', '}', '|', '/':
		return c, false, true
	case '-':
		if inClass {
			return c, false, true
		}
	case 'n':
		return '\n', false, true
	case 'r':
		return '\r', false, true
	case 't':
		return '\t', false, true
	case 'f':
		return '\f', false, true
	case 'v':
		return '\v', false, true
	case 'x':
		h1, h2 := hexDigitValue(p.peek(0)), hexDigitValue(p.peek(1))
		if h1 < 0 || h2 < 0 {
			p.fail(`\x requires two hexadecimal digits`)
			return 0, false, false
		}
		p.pos += 2
		return rune(h1<<4 | h2), false, true
	case 'd', 'D', 'w', 'W', 's', 'S':
		return 0, true, true
	}
	p.pos--
	p.fail(`unsupported escape \%c`, c)
	return 0, false, false
}

func hexDigitValue(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10
	}
	return -1
}

// parseClass rejects empty, nested and ambiguous classes (GTS §11.0.1).
// Doubled '&&', '--' and '~~' are set operations in Rust and ECMA-262 `v`.
func (p *regexProfileParser) parseClass() {
	p.pos++ // '['
	if p.peek(0) == '^' {
		p.pos++
	}
	first := true
	for p.err == nil {
		c := p.peek(0)
		switch {
		case c == -1:
			p.fail("unterminated class")
		case c == ']':
			if first {
				p.fail("empty class")
				return
			}
			p.pos++
			return
		case (c == '&' || c == '-' || c == '~') && p.peek(1) == c:
			p.fail("ambiguous class spelling %q", string([]rune{c, c}))
		case c == '-' && !first && p.peek(1) != ']':
			p.fail("'-' must start or end a class or form a range")
		default:
			p.parseClassRange()
		}
		first = false
	}
}

// parseClassRange parses one class atom, or a range of two single code
// points.
func (p *regexProfileParser) parseClassRange() {
	low, isSet, ok := p.parseClassAtom()
	if !ok || p.peek(0) != '-' || p.peek(1) == ']' || p.peek(1) == -1 {
		return
	}
	if p.peek(1) == '-' {
		p.fail("ambiguous class spelling \"--\"")
		return
	}
	p.pos++ // '-'
	if isSet {
		p.fail("class range with a shorthand class")
		return
	}
	high, isSet, ok := p.parseClassAtom()
	switch {
	case !ok:
	case isSet:
		p.fail("class range with a shorthand class")
	case high < low:
		p.fail("class range out of order")
	}
}

func (p *regexProfileParser) parseClassAtom() (r rune, isSet, ok bool) {
	switch c := p.peek(0); c {
	case '\\':
		return p.parseEscape(true)
	case '[':
		p.fail("unescaped '[' in a class")
		return 0, false, false
	case '&', '-', '~':
		if p.peek(1) == c {
			p.fail("ambiguous class spelling %q", string([]rune{c, c}))
			return 0, false, false
		}
		p.pos++
		return c, false, true
	default:
		p.pos++
		return c, false, true
	}
}
