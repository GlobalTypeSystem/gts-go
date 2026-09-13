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

	// PointerPrefix marks a relative JSON Pointer used by x-gts-ref (e.g. "/$id").
	PointerPrefix = "/"

	// HTTPPrefix / HTTPSPrefix are the URI schemes rejected as GTS references.
	HTTPPrefix  = "http://"
	HTTPSPrefix = "https://"

	// XGtsExtPrefix is the shared prefix of every GTS JSON Schema extension
	// keyword (x-gts-ref, x-gts-final, x-gts-traits, …).
	XGtsExtPrefix = "x-gts-"

	// KeyXGtsRef is the x-gts-ref extension keyword.
	KeyXGtsRef = "x-gts-ref"
)

// IsXGtsExtension reports whether a JSON Schema keyword is a GTS extension.
func IsXGtsExtension(key string) bool { return strings.HasPrefix(key, XGtsExtPrefix) }

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
	// RefRelPointer is a relative JSON Pointer ("/...", e.g. x-gts-ref "/$id").
	RefRelPointer
	// RefOther is anything else.
	RefOther
)

// ClassifyRef determines which reference form s takes. The order of checks is
// significant: the gts:// URI form is recognized before the bare gts. form, and
// concrete scheme prefixes before the generic relative-pointer prefix.
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
	case strings.HasPrefix(s, PointerPrefix):
		return RefRelPointer
	default:
		return RefOther
	}
}
