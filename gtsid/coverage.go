/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gtsid

import "strings"

// This file implements GTS identifier pattern coverage and kind-aware matching
// (README §10), ported from the gts-rust reference (gts-id/src/gts_id_pattern.rs).
//
// A pattern is a validated GTS identifier that may end in a single trailing `*`
// wildcard token, but may also be fully concrete. A zero-wildcard pattern is
// not exact-match: a base type id used as a pattern also matches everything
// derived from it down the chain, with minor-version flexibility.
//
//   - Covers reports whether every identifier selected by one pattern is also
//     selected by another (the coverage relation used by x-gts-pattern-ref and
//     the derivation/evolution relations).
//   - MatchesPattern reports whether a concrete identifier is selected by a
//     pattern (the candidate matching used by x-gts-type-ref / x-gts-instance-ref).

// ParsePattern parses a GTS identifier match pattern: a concrete GTS identifier
// or one ending in a single trailing `*` wildcard token. It is the one pattern
// parser; ValidateWildcard and Match build on it.
func ParsePattern(pattern string) (*ID, error) {
	// Checked first so surrounding whitespace is reported as such rather than
	// as a misplaced wildcard or missing prefix.
	if hasSurroundingWhitespace(pattern) {
		return nil, &InvalidWildcardError{Pattern: pattern, Cause: "must not contain leading or trailing whitespace"}
	}
	if !HasPrefix(pattern) {
		return nil, &InvalidWildcardError{Pattern: pattern, Cause: "Does not start with '" + Prefix + "'"}
	}
	wildcardCount := strings.Count(pattern, WildcardMarker)
	if wildcardCount > 1 {
		return nil, &InvalidWildcardError{Pattern: pattern, Cause: "The wildcard '*' token is allowed only once"}
	}
	if wildcardCount == 1 && !strings.HasSuffix(pattern, WildcardMarker) {
		return nil, &InvalidWildcardError{Pattern: pattern, Cause: "The wildcard '*' token is allowed only at the end of the pattern"}
	}
	// A single segment is allowed only when it is a wildcard or a type: a bare
	// single-segment *instance* is prohibited (issue #37). parseGtsID's
	// single-segment check already exempts wildcard and type segments, which is
	// exactly the pattern rule.
	return parseGtsID(pattern)
}

// verMajorOpt reports the segment's major version, present only when the
// segment carries one (concrete segments and partial wildcard segments like
// `v1.*`); it is absent for name-only wildcard partials and UUID tails.
func (s *Segment) verMajorOpt() (int, bool) { return s.VerMajor, s.HasVersion }

func (s *Segment) uuidTail() (string, bool) {
	if s.IsUUID {
		return s.Segment, true
	}
	return "", false
}

func majorEq(a, b *Segment) bool {
	am, aok := a.verMajorOpt()
	bm, bok := b.verMajorOpt()
	return aok == bok && (!aok || am == bm)
}

func minorEq(a, b *Segment) bool {
	if a.VerMinor == nil {
		return b.VerMinor == nil
	}
	return b.VerMinor != nil && *a.VerMinor == *b.VerMinor
}

// MatchesPattern reports whether the concrete identifier candidate is selected
// by pattern (README §10). A non-wildcard pattern segment must match exactly
// (with minor-version flexibility); a wildcard segment accepts anything from
// that point on.
func MatchesPattern(candidate, pattern *ID) bool {
	return matchesViews(pattern.Segments, candidate.Segments)
}

func matchesViews(patternSegs, candidate []*Segment) bool {
	if len(patternSegs) == 0 {
		return false
	}
	// A final bare `~*` may match an empty chain suffix. A wildcard that already
	// specifies part of the next segment (for example `~abc.*`) still requires
	// that segment to exist.
	last := patternSegs[len(patternSegs)-1]
	matchesEmptySuffix := last.IsWildcard && last.Segment == WildcardMarker
	requiredLen := len(patternSegs)
	if matchesEmptySuffix {
		requiredLen--
	}
	if requiredLen > len(candidate) {
		return false
	}

	for i, pSeg := range patternSegs {
		if i == len(candidate) {
			return matchesEmptySuffix && i == len(patternSegs)-1
		}
		cSeg := candidate[i]

		if pSeg.IsWildcard {
			if pSeg.Vendor != "" && pSeg.Vendor != cSeg.Vendor {
				return false
			}
			if pSeg.Package != "" && pSeg.Package != cSeg.Package {
				return false
			}
			if pSeg.Namespace != "" && pSeg.Namespace != cSeg.Namespace {
				return false
			}
			if pSeg.Type != "" && pSeg.Type != cSeg.Type {
				return false
			}
			if pMajor, ok := pSeg.verMajorOpt(); ok {
				if cMajor, cok := cSeg.verMajorOpt(); !cok || pMajor != cMajor {
					return false
				}
				// `v1.*` requires an explicit minor in the concrete ID.
				if cSeg.VerMinor == nil {
					return false
				}
			}
			if pSeg.VerMinor != nil && (cSeg.VerMinor == nil || *pSeg.VerMinor != *cSeg.VerMinor) {
				return false
			}
			return true
		}

		// Non-wildcard UUID tail - compare the actual UUID.
		if pUUID, ok := pSeg.uuidTail(); ok {
			if cUUID, cok := cSeg.uuidTail(); !cok || pUUID != cUUID {
				return false
			}
		}

		if pSeg.Vendor != cSeg.Vendor || pSeg.Package != cSeg.Package ||
			pSeg.Namespace != cSeg.Namespace || pSeg.Type != cSeg.Type {
			return false
		}
		if !majorEq(pSeg, cSeg) {
			return false
		}
		if pSeg.VerMinor != nil && (cSeg.VerMinor == nil || *pSeg.VerMinor != *cSeg.VerMinor) {
			return false
		}
		if pSeg.IsType != cSeg.IsType {
			return false
		}
	}
	return true
}

// Covers reports whether operand covers value: every GTS ID matched by the
// value pattern is also matched by the operand pattern (README §10). operand is
// the broader (less specific) pattern; value is contained within it.
func Covers(operand, value *ID) bool {
	v := selectionOf(value)
	shortest := v.shortestSelected()
	return shortest > MaxIDLength ||
		coversNonempty(selectionOf(operand), v, shortest == MaxIDLength)
}

type selectionKind int

const (
	selInstance selectionKind = iota
	selPrefixed
)

// selection is a borrowed view of a parsed pattern (README §10 coverage).
type selection struct {
	kind selectionKind
	// types is the type-segment chain.
	types []*Segment
	// tail and length describe an Instance selection.
	tail   *Segment
	length int
	// next is an optional partial segment for a Prefixed selection.
	next *Segment
}

func selectionOf(pattern *ID) selection {
	segs := pattern.Segments
	end := len(segs) - 1
	last := segs[end]
	switch {
	case last.IsWildcard:
		var next *Segment
		if last.Segment != WildcardMarker {
			next = last
		}
		return selection{kind: selPrefixed, types: segs[:end], next: next}
	case last.IsType:
		return selection{kind: selPrefixed, types: segs, next: nil}
	default:
		return selection{kind: selInstance, types: segs[:end], tail: last, length: len(pattern.ID)}
	}
}

func (s selection) shortestSelected() int {
	if s.kind == selInstance {
		return s.length
	}
	chain := len(Prefix)
	for _, segment := range s.types {
		chain += len(segment.Segment)
	}
	if s.next == nil && len(s.types) > 0 {
		return chain
	}
	// Four one-letter names, four dots, and `v0` need ten characters.
	segment := 10
	if s.next != nil {
		segment = shortestPartial(s.next)
	}
	// A first segment must end in the type marker.
	if len(s.types) == 0 {
		return chain + segment + 1
	}
	return chain + segment
}

func shortestPartial(partial *Segment) int {
	if _, ok := partial.verMajorOpt(); ok {
		// In `v1.*`, replacing `*` with the shortest minor (`0`) keeps the length.
		return len(partial.Segment)
	}
	// Missing names are one character; `.v0` and the name separators add six.
	total := 6
	for _, name := range []string{partial.Vendor, partial.Package, partial.Namespace, partial.Type} {
		if len(name) == 0 {
			total++
		} else {
			total += len(name)
		}
	}
	return total
}

func coversNonempty(operand, value selection, atLimit bool) bool {
	switch {
	case operand.kind == selInstance && value.kind == selInstance:
		if !sameChainCovers(operand.types, value.types) {
			return false
		}
		aUUID, aok := operand.tail.uuidTail()
		bUUID, bok := value.tail.uuidTail()
		switch {
		case aok && bok:
			return aUUID == bUUID
		case !aok && !bok:
			return segmentCovers(operand.tail, value.tail)
		default:
			return false
		}
	case operand.kind == selInstance && value.kind == selPrefixed:
		if !atLimit || value.next == nil {
			return false
		}
		// Only one-digit instance minors fit; appending a type marker or another
		// chain segment would exceed the length limit.
		partial := value.next
		_, tailHasMajor := operand.tail.verMajorOpt()
		_, partialHasMajor := partial.verMajorOpt()
		_, tailIsUUID := operand.tail.uuidTail()
		return sameChainCovers(operand.types, value.types) &&
			!tailIsUUID &&
			operand.tail.VerMinor == nil &&
			sameNames(operand.tail, partial) &&
			tailHasMajor == partialHasMajor &&
			(!tailHasMajor || operand.tail.VerMajor == partial.VerMajor)
	case operand.kind == selPrefixed && value.kind == selInstance:
		types := operand.types
		ids := value.types
		if len(ids) < len(types) {
			return false
		}
		for i := range types {
			if !segmentCovers(types[i], ids[i]) {
				return false
			}
		}
		if operand.next == nil {
			return true
		}
		target := value.tail
		if len(ids) > len(types) {
			target = ids[len(types)]
		}
		return partialAdmits(operand.next, target)
	default: // both Prefixed
		return prefixedCovers(operand.types, operand.next, value.types, value.next)
	}
}

func sameNames(a, b *Segment) bool {
	return a.Vendor == b.Vendor && a.Package == b.Package &&
		a.Namespace == b.Namespace && a.Type == b.Type
}

func segmentCovers(a, b *Segment) bool {
	return sameNames(a, b) && majorEq(a, b) && (a.VerMinor == nil || minorEq(a, b))
}

func sameChainCovers(a, b []*Segment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !segmentCovers(a[i], b[i]) {
			return false
		}
	}
	return true
}

func partialNamesCover(a, b *Segment) bool {
	return (a.Vendor == "" || a.Vendor == b.Vendor) &&
		(a.Package == "" || a.Package == b.Package) &&
		(a.Namespace == "" || a.Namespace == b.Namespace) &&
		(a.Type == "" || a.Type == b.Type)
}

func partialAdmits(partial, segment *Segment) bool {
	if !partialNamesCover(partial, segment) {
		return false
	}
	pMajor, ok := partial.verMajorOpt()
	if !ok {
		return true
	}
	sMajor, sok := segment.verMajorOpt()
	return sok && sMajor == pMajor && segment.VerMinor != nil
}

func partialCovers(a, b *Segment) bool {
	if !partialNamesCover(a, b) {
		return false
	}
	aMajor, ok := a.verMajorOpt()
	if !ok {
		return true
	}
	bMajor, bok := b.verMajorOpt()
	return bok && bMajor == aMajor
}

func prefixedCovers(a []*Segment, aNext *Segment, b []*Segment, bNext *Segment) bool {
	if len(a) > len(b) {
		// A first-segment minor wildcard selects types contained in a bare type
		// scope with the same names and major and an omitted minor.
		if len(a) == 1 && aNext == nil && len(b) == 0 && bNext != nil {
			only := a[0]
			_, onlyHasMajor := only.verMajorOpt()
			_, partialHasMajor := bNext.verMajorOpt()
			return only.VerMinor == nil &&
				sameNames(only, bNext) &&
				onlyHasMajor == partialHasMajor &&
				(!onlyHasMajor || only.VerMajor == bNext.VerMajor)
		}
		return false
	}
	for i := range a {
		if !segmentCovers(a[i], b[i]) {
			return false
		}
	}
	if aNext == nil {
		return true
	}
	if len(b) > len(a) {
		return partialAdmits(aNext, b[len(a)])
	}
	return bNext != nil && partialCovers(aNext, bNext)
}
