/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gtsid

import "testing"

func intp(i int) *int { return &i }

// ── markers.go ──────────────────────────────────────────────────────────────

func TestNormalizeID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"gts.x.core.ns.thing.v1~", "gts.x.core.ns.thing.v1~"},
		{"gts://gts.x.core.ns.thing.v1~", "gts.x.core.ns.thing.v1~"},
		{"gts:///gts.x.core.ns.thing.v1~", "gts.x.core.ns.thing.v1~"},
		{"  gts://gts.x.core.ns.thing.v1~  ", "gts.x.core.ns.thing.v1~"},
	}
	for _, c := range cases {
		if got := NormalizeID(c.in); got != c.want {
			t.Errorf("NormalizeID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// Idempotent.
	once := NormalizeID("gts:///gts.x.core.ns.thing.v1~")
	if NormalizeID(once) != once {
		t.Errorf("NormalizeID is not idempotent: %q", once)
	}
}

func TestToCompileURI(t *testing.T) {
	bare := "gts.x.core.ns.thing.v1~"
	want := CompileURIPrefix + bare
	for _, in := range []string{bare, URIPrefix + bare, CompileURIPrefix + bare} {
		if got := ToCompileURI(in); got != want {
			t.Errorf("ToCompileURI(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHasURIPrefix(t *testing.T) {
	if !HasURIPrefix("gts://gts.x.core.ns.thing.v1~") {
		t.Error("HasURIPrefix should be true for gts:// form")
	}
	if HasURIPrefix("gts.x.core.ns.thing.v1~") {
		t.Error("HasURIPrefix should be false for bare form")
	}
}

func TestIsTypePattern(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"gts.x.core.ns.thing.v1~", true},
		{"gts.x.core.ns.thing.v1~*", true},
		{"gts.x.core.*", false},
		{"gts.x.core.ns.thing.v1.0", false},
	}
	for _, c := range cases {
		if got := IsTypePattern(c.in); got != c.want {
			t.Errorf("IsTypePattern(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// ── id.go / match.go — error formatting ──────────────────────────────────────

func TestErrorMessagesWithAndWithoutCause(t *testing.T) {
	if got := (&InvalidIDError{GtsID: "bad"}).Error(); got != "Invalid GTS identifier: bad" {
		t.Errorf("InvalidIDError without cause = %q", got)
	}
	if got := (&InvalidIDError{GtsID: "bad", Cause: "why"}).Error(); got != "Invalid GTS identifier: bad: why" {
		t.Errorf("InvalidIDError with cause = %q", got)
	}
	if got := (&InvalidSegmentError{Num: 1, Offset: 2, Segment: "seg"}).Error(); got != "Invalid GTS segment #1 @ offset 2: 'seg'" {
		t.Errorf("InvalidSegmentError without cause = %q", got)
	}
	if got := (&InvalidSegmentError{Num: 1, Offset: 2, Segment: "seg", Cause: "why"}).Error(); got != "Invalid GTS segment #1 @ offset 2: 'seg': why" {
		t.Errorf("InvalidSegmentError with cause = %q", got)
	}
	if got := (&InvalidWildcardError{Pattern: "pat"}).Error(); got != "Invalid GTS wildcard pattern: pat" {
		t.Errorf("InvalidWildcardError without cause = %q", got)
	}
	if got := (&InvalidWildcardError{Pattern: "pat", Cause: "why"}).Error(); got != "Invalid GTS wildcard pattern: pat: why" {
		t.Errorf("InvalidWildcardError with cause = %q", got)
	}
}

// ── ops.go — IDToUUID ───────────────────────────────────────────────────────

func TestIDToUUID_Paths(t *testing.T) {
	const uuid = "7a1d2f34-5678-49ab-9012-abcdef123456"
	tail := IDToUUID("gts.x.core.events.type.v1~" + uuid)
	if tail.Error != "" || tail.UUID != uuid {
		t.Errorf("IDToUUID UUID tail = %+v, want embedded %q", tail, uuid)
	}
	normal := IDToUUID("gts.x.core.ns.thing.v1~")
	if normal.Error != "" || normal.UUID == "" {
		t.Errorf("IDToUUID normal = %+v, want a derived UUID", normal)
	}
	bad := IDToUUID("not-a-gts-id")
	if bad.Error == "" || bad.UUID != "" {
		t.Errorf("IDToUUID invalid = %+v, want an error", bad)
	}
}

// ── coverage.go — segment helpers (direct) ──────────────────────────────────

func TestSegmentHelpers(t *testing.T) {
	u := &Segment{IsUUID: true, Segment: "uuid-value"}
	if s, ok := u.uuidTail(); !ok || s != "uuid-value" {
		t.Errorf("uuidTail on UUID segment = %q,%v", s, ok)
	}
	if _, ok := (&Segment{}).uuidTail(); ok {
		t.Error("uuidTail on non-UUID segment should be false")
	}
	if m, ok := (&Segment{HasVersion: true, VerMajor: 3}).verMajorOpt(); !ok || m != 3 {
		t.Errorf("verMajorOpt with version = %d,%v", m, ok)
	}
	if _, ok := (&Segment{}).verMajorOpt(); ok {
		t.Error("verMajorOpt without version should be false")
	}
}

func TestMajorEq(t *testing.T) {
	v1 := &Segment{HasVersion: true, VerMajor: 1}
	v1b := &Segment{HasVersion: true, VerMajor: 1}
	v2 := &Segment{HasVersion: true, VerMajor: 2}
	none := &Segment{}
	cases := []struct {
		a, b *Segment
		want bool
	}{
		{v1, v1b, true},
		{v1, v2, false},
		{v1, none, false},
		{none, none, true},
	}
	for i, c := range cases {
		if got := majorEq(c.a, c.b); got != c.want {
			t.Errorf("majorEq case %d = %v, want %v", i, got, c.want)
		}
	}
}

func TestMinorEq(t *testing.T) {
	v10 := &Segment{VerMinor: intp(0)}
	v11 := &Segment{VerMinor: intp(1)}
	vnil := &Segment{}
	cases := []struct {
		a, b *Segment
		want bool
	}{
		{vnil, vnil, true},
		{vnil, v10, false},
		{v10, vnil, false},
		{v10, &Segment{VerMinor: intp(0)}, true},
		{v10, v11, false},
	}
	for i, c := range cases {
		if got := minorEq(c.a, c.b); got != c.want {
			t.Errorf("minorEq case %d = %v, want %v", i, got, c.want)
		}
	}
}

func TestSegmentCoversAndSameChain(t *testing.T) {
	a := &Segment{Vendor: "x", HasVersion: true, VerMajor: 1} // minor nil
	bMinor := &Segment{Vendor: "x", HasVersion: true, VerMajor: 1, VerMinor: intp(0)}
	aMinor := &Segment{Vendor: "x", HasVersion: true, VerMajor: 1, VerMinor: intp(0)}
	bMinor1 := &Segment{Vendor: "x", HasVersion: true, VerMajor: 1, VerMinor: intp(1)}
	other := &Segment{Vendor: "z", HasVersion: true, VerMajor: 1}

	if !segmentCovers(a, bMinor) {
		t.Error("minor-less operand should cover a concrete minor")
	}
	if !segmentCovers(aMinor, bMinor) {
		t.Error("equal minors should cover")
	}
	if segmentCovers(aMinor, bMinor1) {
		t.Error("different minors should not cover")
	}
	if segmentCovers(a, other) {
		t.Error("different vendor should not cover")
	}

	if !sameChainCovers([]*Segment{a}, []*Segment{bMinor}) {
		t.Error("same-length covering chain expected true")
	}
	if sameChainCovers([]*Segment{a}, []*Segment{a, a}) {
		t.Error("different chain lengths should not cover")
	}
	if sameChainCovers([]*Segment{a}, []*Segment{other}) {
		t.Error("non-covering segment should break chain coverage")
	}
}

func TestPartialHelpers(t *testing.T) {
	pNames := &Segment{Vendor: "x", Package: "core"} // namespace/type open
	full := &Segment{Vendor: "x", Package: "core", Namespace: "ns", Type: "t"}
	wrong := &Segment{Vendor: "y", Package: "core", Namespace: "ns", Type: "t"}

	if !partialNamesCover(pNames, full) {
		t.Error("open names should cover a concrete segment")
	}
	if partialNamesCover(pNames, wrong) {
		t.Error("mismatched vendor should not be covered")
	}

	segFull := &Segment{Vendor: "x", HasVersion: true, VerMajor: 1, VerMinor: intp(0)}
	segNoMinor := &Segment{Vendor: "x", HasVersion: true, VerMajor: 1}
	segMajor2 := &Segment{Vendor: "x", HasVersion: true, VerMajor: 2, VerMinor: intp(0)}
	pNoVer := &Segment{Vendor: "x"}
	pVer := &Segment{Vendor: "x", HasVersion: true, VerMajor: 1}

	if !partialAdmits(pNoVer, segFull) {
		t.Error("version-less partial should admit any matching segment")
	}
	if !partialAdmits(pVer, segFull) {
		t.Error("partial v1 should admit v1.0")
	}
	if partialAdmits(pVer, segMajor2) {
		t.Error("partial v1 should not admit v2.0")
	}
	if partialAdmits(pVer, segNoMinor) {
		t.Error("partial v1 requires an explicit minor")
	}
	if partialAdmits(&Segment{Vendor: "z"}, segFull) {
		t.Error("mismatched names should not be admitted")
	}

	if !partialCovers(pNoVer, segFull) {
		t.Error("version-less partial should cover")
	}
	if !partialCovers(pVer, segFull) {
		t.Error("partial v1 should cover v1.x")
	}
	if partialCovers(pVer, segMajor2) {
		t.Error("partial v1 should not cover v2")
	}
	if partialCovers(pVer, &Segment{Vendor: "x"}) {
		t.Error("partial v1 should not cover a version-less segment")
	}
	if partialCovers(&Segment{Vendor: "z"}, segFull) {
		t.Error("mismatched names should not cover")
	}
}

func TestPrefixedCoversSpecialBranch(t *testing.T) {
	only := &Segment{Vendor: "x", Package: "core", Namespace: "ns", Type: "t", HasVersion: true, VerMajor: 1}
	bNext := &Segment{Vendor: "x", Package: "core", Namespace: "ns", Type: "t", HasVersion: true, VerMajor: 1}
	if !prefixedCovers([]*Segment{only}, nil, nil, bNext) {
		t.Error("a bare type scope should cover its first-segment minor wildcard")
	}
	// len(a) > len(b) but not the special single-segment case.
	if prefixedCovers([]*Segment{only, only}, nil, []*Segment{only}, nil) {
		t.Error("a longer operand chain cannot cover a shorter value chain")
	}
}

// ── coverage.go — Covers through real patterns ──────────────────────────────

func TestCoversInstanceSelections(t *testing.T) {
	const uuid = "7a1d2f34-5678-49ab-9012-abcdef123456"
	const uuid2 = "00000000-0000-4000-8000-000000000000"
	cases := []struct {
		operand, value string
		want           bool
	}{
		// instance/instance, non-UUID tails
		{"gts.x.core.ns.t.v1~x.commerce.orders.o.v1.0", "gts.x.core.ns.t.v1~x.commerce.orders.o.v1.0", true},
		{"gts.x.core.ns.t.v1~x.commerce.orders.o.v1.0", "gts.x.core.ns.t.v1~x.commerce.orders.o.v1.1", false},
		// different chain length
		{"gts.x.core.ns.t.v1~x.commerce.orders.o.v1.0", "gts.x.core.ns.t.v1~x.c.n.m.v1~x.commerce.orders.o.v1.0", false},
		// UUID tails
		{"gts.x.core.events.type.v1~" + uuid, "gts.x.core.events.type.v1~" + uuid, true},
		{"gts.x.core.events.type.v1~" + uuid, "gts.x.core.events.type.v1~" + uuid2, false},
		// UUID operand vs non-UUID instance value
		{"gts.x.core.events.type.v1~" + uuid, "gts.x.core.events.type.v1~x.c.n.o.v1.0", false},
	}
	for _, c := range cases {
		if got := Covers(mustPat(t, c.operand), mustPat(t, c.value)); got != c.want {
			t.Errorf("Covers(%q, %q) = %v, want %v", c.operand, c.value, got, c.want)
		}
	}
}

func TestCoversPrefixedSelections(t *testing.T) {
	cases := []struct {
		operand, value string
		want           bool
	}{
		{"gts.x.core.*", "gts.x.core.ns.*", true},
		{"gts.x.core.ns.*", "gts.x.core.*", false},
		{"gts.x.core.ns.thing.*", "gts.x.core.ns.thing.v1~", true},
	}
	for _, c := range cases {
		if got := Covers(mustPat(t, c.operand), mustPat(t, c.value)); got != c.want {
			t.Errorf("Covers(%q, %q) = %v, want %v", c.operand, c.value, got, c.want)
		}
	}
}

func TestMatchesPatternWildcardFilters(t *testing.T) {
	cases := []struct {
		pattern, candidate string
		want               bool
	}{
		{"gts.x.core.ns.*", "gts.x.core.ns.thing.v1~", true},
		{"gts.x.core.ns.*", "gts.y.core.ns.thing.v1~", false},
		{"gts.x.core.ns.thing.v1~*", "gts.x.core.ns.thing.v1~", true},
		{"gts.x.core.ns.thing.v1~*", "gts.x.core.ns.thing.v1~x.c.n.i.v1.0", true},
		// A version-wildcard partial requires the candidate to carry an
		// explicit matching major and a minor.
		{"gts.x.core.ns.thing.v1.*", "gts.x.core.ns.thing.v1.0~", true},
		{"gts.x.core.ns.thing.v1.*", "gts.x.core.ns.thing.v1~", false},
		{"gts.x.core.ns.thing.v2.*", "gts.x.core.ns.thing.v1.0~", false},
		// Package-level filter rejecting a different package.
		{"gts.x.core.ns.thing.v1~", "gts.x.core.ns.thing.v2~", false},
	}
	for _, c := range cases {
		cand, err := New(c.candidate)
		if err != nil {
			t.Fatalf("New(%q): %v", c.candidate, err)
		}
		if got := MatchesPattern(cand, mustPat(t, c.pattern)); got != c.want {
			t.Errorf("MatchesPattern(%q, %q) = %v, want %v", c.candidate, c.pattern, got, c.want)
		}
	}
}

func TestCoversPrefixedAcrossChains(t *testing.T) {
	cases := []struct {
		operand, value string
		want           bool
	}{
		// prefixed operand covering a longer chained type value (partialAdmits).
		{"gts.x.core.ns.thing.v1~*", "gts.x.core.ns.thing.v1~x.c.n.leaf.v1~", true},
		// type-boundary wildcard covers its own base type.
		{"gts.x.core.ns.thing.v1~*", "gts.x.core.ns.thing.v1~", true},
		// a narrower wildcard does not cover a broader one.
		{"gts.x.core.ns.thing.v1~acme.*", "gts.x.core.ns.thing.v1~*", false},
	}
	for _, c := range cases {
		if got := Covers(mustPat(t, c.operand), mustPat(t, c.value)); got != c.want {
			t.Errorf("Covers(%q, %q) = %v, want %v", c.operand, c.value, got, c.want)
		}
	}
}
