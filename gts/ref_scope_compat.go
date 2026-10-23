/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
)

// This file implements the derivation/evolution comparison of the GTS reference
// keywords (README §9.6.1 "Derivation and evolution", ADR-0007/0008). A property
// that constrains a GTS reference accepts a set of strings; a derived property
// is admissible when the strings it accepts are a subset of those the base
// accepts. The two entity-reference keywords compare kind-filtered scopes;
// x-gts-pattern-ref compares selection-set coverage. Composition (allOf, anyOf,
// not) combines scopes by intersection, union and complement.
//
// The comparison is a sound three-valued decision (refRel): it returns included
// or excluded only when it can prove the relation, and unknown otherwise. OP#12
// admission fails closed (unknown is rejected); OP#8 reports unknown.

// maxRefScopeDepth bounds the structural recursion of the reference-scope
// walkers so a pathologically nested schema cannot exhaust the stack
// (matches maxCompatRecursionDepth).
const maxRefScopeDepth = 64

// refRel is the three-valued result of a subset comparison.
type refRel int

const (
	// relIncluded: every string the derived scope accepts the base accepts too.
	relIncluded refRel = iota
	// relExcluded: the derived scope provably accepts a string the base rejects.
	relExcluded
	// relUnknown: inclusion could neither be proved nor disproved.
	relUnknown
)

type refScopeKind int

const (
	// refScopeTop accepts every string: the property declares no reference
	// constraint (or one that resolved to no concrete operand).
	refScopeTop refScopeKind = iota
	refScopeAtom
	refScopeUnion
	refScopeInter
	refScopeNot
	// refScopeUndecidable stands in for a scope the builder could not fully
	// analyse (e.g. nesting past maxRefScopeDepth); any relation touching it is
	// relUnknown.
	refScopeUndecidable
)

// refScope is the set of identifier strings a property accepts through the GTS
// reference keywords.
type refScope struct {
	kind refScopeKind
	// atom fields
	refKind refKind
	pattern *gtsid.ID
	empty   bool
	// union / inter
	children []refScope
	// not
	child *refScope
}

const minInstanceSegmentLen = 10 // "a.a.a.a.v0"

// buildRefScope computes the reference scope a property declares, and reports
// whether the property mentions any reference keyword at all (directly or
// through composition). Operands are expected to be already resolved (no /$id).
func buildRefScope(prop map[string]any) (refScope, bool) {
	return buildRefScopeAt(prop, 0)
}

func buildRefScopeAt(prop map[string]any, depth int) (refScope, bool) {
	if depth >= maxRefScopeDepth {
		return refScope{kind: refScopeUndecidable}, true
	}
	var parts []refScope
	has := false

	for _, keyword := range refKeywords {
		operand, ok := prop[keyword]
		if !ok {
			continue
		}
		has = true
		kind, _ := refKindOf(keyword)
		if atom, ok := refAtom(kind, operand); ok {
			parts = append(parts, atom)
		}
	}

	if notValue, ok := prop["not"].(map[string]any); ok {
		if childScope, childHas := buildRefScopeAt(notValue, depth+1); childHas {
			has = true
			c := childScope
			parts = append(parts, refScope{kind: refScopeNot, child: &c})
		}
	}

	for _, key := range []string{"anyOf", "oneOf"} {
		branches, ok := prop[key].([]any)
		if !ok {
			continue
		}
		var union []refScope
		branchHas := false
		for _, b := range branches {
			branchMap, ok := b.(map[string]any)
			if !ok {
				union = append(union, refScope{kind: refScopeTop})
				continue
			}
			bs, bh := buildRefScopeAt(branchMap, depth+1)
			if bh {
				branchHas = true
			}
			union = append(union, bs)
		}
		if branchHas {
			has = true
			parts = append(parts, refScope{kind: refScopeUnion, children: union})
		}
	}

	if branches, ok := prop["allOf"].([]any); ok {
		for _, b := range branches {
			branchMap, ok := b.(map[string]any)
			if !ok {
				continue
			}
			if bs, bh := buildRefScopeAt(branchMap, depth+1); bh {
				has = true
				parts = append(parts, bs)
			}
		}
	}

	switch len(parts) {
	case 0:
		return refScope{kind: refScopeTop}, has
	case 1:
		return parts[0], has
	default:
		return refScope{kind: refScopeInter, children: parts}, has
	}
}

// refAtom builds a single-keyword scope from an operand, or reports !ok when the
// operand is not a usable pattern (an invalid operand is a schema error caught
// separately, so it is simply not compared here).
func refAtom(kind refKind, operand any) (refScope, bool) {
	spelling, ok := operand.(string)
	if !ok || IsXGtsRefSelf(spelling) {
		return refScope{}, false
	}
	pattern, err := gtsid.ParsePattern(spelling)
	if err != nil {
		return refScope{}, false
	}
	atom := refScope{kind: refScopeAtom, refKind: kind, pattern: pattern}
	// An instance reference to a concrete Type ID too long for any instance
	// tail selects nothing (README §9.6.1 "Derivation and evolution").
	if kind == refKindInstance && !gtsid.HasWildcard(spelling) && gtsid.IsTypeID(spelling) &&
		len(spelling)+minInstanceSegmentLen > gtsid.MaxIDLength {
		atom.empty = true
	}
	return atom, true
}

// relation reports whether Valid(derived) ⊆ Valid(base) for the reference
// dimension, as a sound three-valued decision. Derived composition is
// decomposed before base composition so that, e.g., a union derived requires
// every branch to be included (which a single-branch base test would miss).
func relation(base, derived refScope) refRel {
	if base.kind == refScopeUndecidable || derived.kind == refScopeUndecidable {
		return relUnknown
	}
	if base.kind == refScopeTop {
		return relIncluded // base accepts every string
	}
	if derived.kind == refScopeTop {
		return relExcluded // derived accepts strings the restricted base rejects
	}

	// Decompose the derived side first.
	switch derived.kind {
	case refScopeUnion:
		// A union is included only if every branch is; one excluded branch
		// excludes the union.
		result := relIncluded
		for i := range derived.children {
			switch relation(base, derived.children[i]) {
			case relExcluded:
				return relExcluded
			case relUnknown:
				result = relUnknown
			}
		}
		return result
	case refScopeInter:
		// An intersection is included if any one conjunct is already included;
		// otherwise it cannot be proved either way.
		for i := range derived.children {
			if relation(base, derived.children[i]) == relIncluded {
				return relIncluded
			}
		}
		return relUnknown
	case refScopeNot:
		if base.kind == refScopeNot {
			// complement(D) ⊆ complement(B) ⟺ B ⊆ D.
			return relation(*derived.child, *base.child)
		}
		return relUnknown
	}

	// Derived is an atom; dispatch on the base side.
	switch base.kind {
	case refScopeUnion:
		// Included if the atom fits some branch; straddling branches cannot be
		// disproved, so fall back to unknown rather than a wrong "excluded".
		for i := range base.children {
			if relation(base.children[i], derived) == relIncluded {
				return relIncluded
			}
		}
		return relUnknown
	case refScopeInter:
		result := relIncluded
		for i := range base.children {
			switch relation(base.children[i], derived) {
			case relExcluded:
				return relExcluded
			case relUnknown:
				result = relUnknown
			}
		}
		return result
	case refScopeNot:
		// atom ⊆ complement(B): provable only via disjointness, which this
		// checker does not compute.
		return relUnknown
	case refScopeAtom:
		return atomRelation(base, derived)
	}
	return relUnknown
}

// atomRelation decides inclusion between two single-keyword scopes. Same kind
// reduces to operand coverage, which is complete for GTS pattern sets (README
// §10), so a non-covering operand is a definite exclusion. Different entity
// kinds select disjoint nonempty sets; mixing a pattern reference with an entity
// reference is left undecided.
func atomRelation(base, derived refScope) refRel {
	if derived.empty {
		return relIncluded
	}
	basePattern := base.refKind == refKindPattern
	derivedPattern := derived.refKind == refKindPattern
	if basePattern != derivedPattern {
		return relUnknown
	}
	if base.refKind != derived.refKind {
		return relExcluded // type vs instance: disjoint nonempty scopes
	}
	if gtsid.Covers(base.pattern, derived.pattern) {
		return relIncluded
	}
	return relExcluded
}

// refScopeRelation is the single entry point for both OP#12 and OP#8: it reports
// whether the derived property's reference scope is included in the base's. It
// also admits a derived property that narrows with ordinary keywords: a finite
// const/enum is included when every value lies in the base scope.
func refScopeRelation(baseProp, derivedProp map[string]any) refRel {
	baseScope, baseHas := buildRefScope(baseProp)
	if !baseHas {
		return relIncluded // base imposes no reference restriction
	}
	if derivedScope, derivedHas := buildRefScope(derivedProp); derivedHas {
		return relation(baseScope, derivedScope)
	}
	values, enumerates := collectDerivedEnumeratedValues(derivedProp)
	if !enumerates {
		// A plain string or regex accepts values outside the base scope.
		return relExcluded
	}
	for _, v := range values {
		s, ok := v.(string)
		if !ok || !refScopeAccepts(baseScope, s) {
			return relExcluded
		}
	}
	return relIncluded
}

// checkRefScopeCompatibility reports an OP#12 error when the derived property's
// GTS reference scope is not provably included in the base property's scope.
// Admission fails closed: an unknown relation is rejected.
func checkRefScopeCompatibility(baseProp, derivedProp map[string]any, propName string) []string {
	if refScopeRelation(baseProp, derivedProp) == relIncluded {
		return nil
	}
	return []string{fmt.Sprintf(
		"property '%s': derived GTS reference constraint is not included in the base constraint", propName)}
}

// isRefKeyword reports whether key is one of the GTS reference keywords.
func isRefKeyword(key string) bool {
	_, ok := refKindOf(key)
	return ok
}

// refScopeAccepts reports whether the scope accepts the concrete string value.
func refScopeAccepts(scope refScope, value string) bool {
	switch scope.kind {
	case refScopeTop:
		return true
	case refScopeNot:
		return !refScopeAccepts(*scope.child, value)
	case refScopeUnion:
		for _, c := range scope.children {
			if refScopeAccepts(c, value) {
				return true
			}
		}
		return false
	case refScopeInter:
		for _, c := range scope.children {
			if !refScopeAccepts(c, value) {
				return false
			}
		}
		return true
	case refScopeAtom:
		if scope.empty {
			return false
		}
		return refCandidateViolation(scope.refKind, value, scope.pattern, scope.pattern.ID) == ""
	}
	return false
}

// mergeEffectiveProperty folds a later allOf-branch declaration of a property
// (next) onto an earlier one (prev). Ordinary keywords follow innermost-wins
// (next replaces prev), but a GTS reference keyword conjoins like allOf: the
// narrower operand survives, and when neither operand covers the other both are
// kept as an allOf conjunction so a wider overlay never widens the inherited
// scope (README §9.6.1).
func mergeEffectiveProperty(prev, next any) any {
	prevMap, prevOK := prev.(map[string]any)
	nextMap, nextOK := next.(map[string]any)
	if prev == nil || !prevOK || !nextOK {
		return next
	}
	merged := make(map[string]any, len(nextMap))
	for k, v := range nextMap {
		merged[k] = v
	}
	for _, keyword := range refKeywords {
		prevOperand, hasPrev := prevMap[keyword]
		if !hasPrev {
			continue
		}
		nextOperand, hasNext := merged[keyword]
		if !hasNext {
			merged[keyword] = prevOperand
			continue
		}
		if narrower, ok := narrowerRefOperand(keyword, prevOperand, nextOperand); ok {
			merged[keyword] = narrower
			continue
		}
		// Incomparable operands: keep both as a conjunction rather than
		// dropping one, which would widen the effective scope.
		delete(merged, keyword)
		existing, _ := merged["allOf"].([]any)
		both := append([]any{}, existing...)
		both = append(both,
			map[string]any{keyword: prevOperand},
			map[string]any{keyword: nextOperand})
		merged["allOf"] = both
	}
	return merged
}

// narrowerRefOperand returns the operand (of two for the same reference keyword)
// that selects the smaller set, with ok=true. When neither covers the other, or
// either cannot be compared (an unresolved "/$id", or an invalid operand that is
// reported elsewhere), it returns ok=false so the caller keeps both as a
// conjunction: dropping one would widen the effective scope.
func narrowerRefOperand(keyword string, a, b any) (any, bool) {
	kind, _ := refKindOf(keyword)
	atomA, okA := refAtom(kind, a)
	atomB, okB := refAtom(kind, b)
	if !okA || !okB {
		return nil, false
	}
	switch {
	case gtsid.Covers(atomA.pattern, atomB.pattern):
		return b, true // b ⊆ a: b is the narrower (or equal)
	case gtsid.Covers(atomB.pattern, atomA.pattern):
		return a, true // a ⊆ b: a is the narrower
	default:
		return nil, false // incomparable
	}
}

// resolveSelfRefOperands replaces every "/$id" reference-keyword operand under
// node with selfID (the bare top-level identifier of the schema being
// compared), so that each schema's self-reference resolves to its own type
// before scopes are compared (README §9.6.1).
func resolveSelfRefOperands(node any, selfID string) {
	resolveSelfRefOperandsAt(node, selfID, 0)
}

func resolveSelfRefOperandsAt(node any, selfID string, depth int) {
	if depth >= maxRefScopeDepth {
		return
	}
	switch n := node.(type) {
	case map[string]any:
		for _, keyword := range refKeywords {
			if v, ok := n[keyword].(string); ok && IsXGtsRefSelf(v) {
				n[keyword] = selfID
			}
		}
		for _, v := range n {
			resolveSelfRefOperandsAt(v, selfID, depth+1)
		}
	case []any:
		for _, v := range n {
			resolveSelfRefOperandsAt(v, selfID, depth+1)
		}
	}
}
