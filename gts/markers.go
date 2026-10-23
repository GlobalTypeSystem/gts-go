/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"strings"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
)

// This file owns the *reference* grammar: the textual forms a $ref / x-gts-ref
// value can take, and the JSON Schema extension keywords. It builds on top of
// the identifier grammar in package gtsid — reference values may embed GTS
// identifiers, but the identifier layer knows nothing about references.

const (
	// LocalRefPrefix marks a same-document JSON Pointer in a JSON Schema $ref.
	LocalRefPrefix = "#"

	// XGtsRefSelf is the only slash-prefixed x-gts-ref operand.
	XGtsRefSelf = "/$id"

	// HTTPPrefix / HTTPSPrefix are the URI schemes rejected as GTS references.
	HTTPPrefix  = "http://"
	HTTPSPrefix = "https://"

	// XGtsExtPrefix is the shared prefix of every GTS JSON Schema extension
	// keyword (x-gts-type-ref, x-gts-final, x-gts-traits, …).
	XGtsExtPrefix = "x-gts-"

	// KeyXGtsRef is the removed, kind-blind reference keyword. As of GTS 0.16
	// (ADR-0008) it is no longer a supported GTS keyword: implementations treat
	// it like any other unknown x-gts-* keyword and reject it. The constant is
	// retained only so code can refer to its spelling.
	KeyXGtsRef = "x-gts-ref"

	// KeyXGtsTypeRef constrains a string to a GTS Type Identifier selected by
	// its operand (README §9.6.1, ADR-0008).
	KeyXGtsTypeRef = "x-gts-type-ref"
	// KeyXGtsInstanceRef constrains a string to a GTS Instance Identifier
	// selected by its operand (README §9.6.1, ADR-0008).
	KeyXGtsInstanceRef = "x-gts-instance-ref"
	// KeyXGtsPatternRef constrains a string to a GTS identifier pattern whose
	// selection set is covered by its operand (README §9.6.2, ADR-0007).
	KeyXGtsPatternRef = "x-gts-pattern-ref"
)

// IsXGtsExtension reports whether a JSON Schema keyword is a GTS extension.
func IsXGtsExtension(key string) bool { return strings.HasPrefix(key, XGtsExtPrefix) }

// IsXGtsRefSelf reports whether value is the reserved selected-type reference.
func IsXGtsRefSelf(value string) bool { return value == XGtsRefSelf }

// RefKind classifies the textual form of a reference value ($ref / x-gts-ref).
type RefKind int

const (
	// RefLocalPointer is a same-document JSON Pointer ("#...").
	RefLocalPointer RefKind = iota
	// RefGtsURI is a GTS identifier in URI form ("gts://...").
	RefGtsURI
	// RefBareGtsID is a GTS identifier in bare form ("gts....").
	RefBareGtsID
	// RefHTTP is an http:// or https:// URI.
	RefHTTP
	// RefOther is anything else.
	RefOther
)

// ClassifyRef determines which reference form s takes. The order of checks is
// significant: the gts:// URI form is recognized before the bare gts. form.
func ClassifyRef(s string) RefKind {
	switch {
	case strings.HasPrefix(s, LocalRefPrefix):
		return RefLocalPointer
	case strings.HasPrefix(s, gtsid.URIPrefix):
		return RefGtsURI
	case strings.HasPrefix(s, gtsid.Prefix):
		return RefBareGtsID
	case strings.HasPrefix(s, HTTPPrefix), strings.HasPrefix(s, HTTPSPrefix):
		return RefHTTP
	default:
		return RefOther
	}
}
