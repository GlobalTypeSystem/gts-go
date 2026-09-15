/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

// Package gtsid is the foundational layer of the reference implementation: it
// owns the GTS *identifier* grammar and nothing else. It parses and validates
// identifiers, segments and wildcard patterns, matches candidates against
// patterns, and derives UUIDs. It deliberately has zero dependencies on the
// rest of gts-go (store, entities, schema validation), so the compiler enforces
// that the grammar can never reach "upward" into higher layers.
package gtsid

import "strings"

// This file is the single source of truth for the textual markers that make up
// the GTS identifier grammar. Reference-form markers ("#", "/", "http://") and
// JSON Schema extension keywords live in the reference layer (package gts),
// which builds on top of these.

const (
	// Prefix is the required prefix for all GTS identifiers in their bare
	// (canonical) form, e.g. "gts.x.example._.user.v1~".
	Prefix = "gts."

	// URIPrefix is the URI-compatible prefix used ONLY for GTS identifiers
	// carried in JSON Schema serialization ($id / $ref), e.g.
	// "gts://gts.x.example._.user.v1~". It is never part of a parsed GTS ID —
	// use NormalizeID to strip it before parsing.
	URIPrefix = "gts://"

	// CompileURIPrefix is the absolute-URI form used for schema $id / $ref
	// values handed to a JSON Schema compiler, e.g.
	// "gts:///gts.x.example._.user.v1~". The empty authority (three slashes)
	// keeps the URI absolute — so a compiler never resolves a bare id against
	// the process working directory into a "file://<cwd>/id" URL, which would
	// leak the local filesystem layout into validation errors — while the
	// path-based form still lets relative $ref values resolve correctly (unlike
	// the authority form URIPrefix, which mis-merges "a~" + "b~" into "a~/b~").
	// Like URIPrefix it is never part of a parsed GTS ID; use NormalizeID to
	// strip it, or ToCompileURI to produce it.
	CompileURIPrefix = URIPrefix + "/"

	// TypeMarker terminates a GTS *type* identifier (as opposed to an instance).
	TypeMarker = "~"

	// WildcardMarker is the single wildcard token permitted in access-control
	// patterns.
	WildcardMarker = "*"

	// SegmentWildcardSuffix and TypeWildcardSuffix are the only two legal
	// endings for a wildcard pattern: ".*" widens on a segment boundary, "~*"
	// widens on a type boundary.
	SegmentWildcardSuffix = "." + WildcardMarker
	TypeWildcardSuffix    = TypeMarker + WildcardMarker

	// TokenSep separates the tokens (vendor.package.namespace.type.version…)
	// within a single GTS segment.
	TokenSep = "."

	// SegmentSep separates chained segments within a GTS identifier; it is the
	// same character as the type marker.
	SegmentSep = TypeMarker

	// MaxIDLength is the maximum allowed length for a GTS identifier.
	MaxIDLength = 1024
)

// NormalizeID returns the bare GTS-identifier form of s: it trims surrounding
// whitespace and strips any GTS URI prefix (the "gts://" serialization form or
// the "gts:///" compile form) if present. It is the single canonicalization
// step every caller should use before parsing or storing an id sourced from a
// JSON Schema $id / $ref, and is idempotent.
func NormalizeID(s string) string {
	s = strings.TrimSpace(s)
	// CompileURIPrefix extends URIPrefix, so try the longer prefix first.
	if stripped, ok := strings.CutPrefix(s, CompileURIPrefix); ok {
		return stripped
	}
	return strings.TrimPrefix(s, URIPrefix)
}

// ToCompileURI returns s in the absolute compile-URI form (see CompileURIPrefix).
// It first normalizes s to its bare form, so it accepts bare, "gts://" and
// "gts:///" inputs alike and is idempotent.
func ToCompileURI(s string) string {
	return CompileURIPrefix + NormalizeID(s)
}

// HasPrefix reports whether s is written in the bare "gts." identifier form.
func HasPrefix(s string) bool { return strings.HasPrefix(s, Prefix) }

// HasURIPrefix reports whether s is written in the "gts://" URI form.
func HasURIPrefix(s string) bool { return strings.HasPrefix(s, URIPrefix) }

// IsTypeID reports whether a concrete (non-wildcard) id denotes a GTS *type*,
// i.e. it ends with the type marker '~'.
func IsTypeID(s string) bool { return strings.HasSuffix(s, TypeMarker) }

// HasWildcard reports whether s contains the wildcard token '*'.
func HasWildcard(s string) bool { return strings.Contains(s, WildcardMarker) }

// EndsWithWildcardSuffix reports whether s ends with one of the two legal
// wildcard endings (".*" or "~*"). For an already-validated wildcard pattern
// this is also the test for "is this a type pattern".
func EndsWithWildcardSuffix(s string) bool {
	return strings.HasSuffix(s, SegmentWildcardSuffix) || strings.HasSuffix(s, TypeWildcardSuffix)
}

// IsTypePattern reports whether a possibly-wildcarded pattern denotes a type:
// it ends with '~' (a concrete type id) or '~*' (a type wildcard).
func IsTypePattern(s string) bool {
	return strings.HasSuffix(s, TypeMarker) || strings.HasSuffix(s, TypeWildcardSuffix)
}
