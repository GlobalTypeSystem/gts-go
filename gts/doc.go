/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

// Package gts is the public library API of the gts-go reference
// implementation of the Global Type System (GTS) specification.
//
// It builds on the identifier grammar in the sibling gtsid package and adds
// everything that operates on JSON/JSON Schema artifacts: entity extraction,
// an in-memory registry (GtsStore), instance and schema validation,
// compatibility checking, version casting, queries, and attribute access.
//
// The GTS specification defines a set of numbered operations ("OP#n"). The
// entry points for each are:
//
//   - OP#1  ID Validation      — gtsid.Validate / gtsid.IsValid
//   - OP#2  ID Extraction      — ExtractID / ExtractGtsID
//   - OP#3  ID Parsing         — gtsid.Parse
//   - OP#4  Pattern Matching   — gtsid.Match
//   - OP#5  ID to UUID         — gtsid.IDToUUID
//   - OP#6  Instance Validation — GtsStore.ValidateInstance
//   - OP#7  Relationships      — GtsStore.BuildSchemaGraph
//   - OP#8  Compatibility      — GtsStore.CheckCompatibility
//   - OP#9  Version Casting    — GtsStore.Cast
//   - OP#10 Query Execution    — GtsStore.Query
//   - OP#11 Attribute Access   — GtsStore.GetAttribute
//   - OP#12 Schema Validation  — GtsStore.ValidateSchemaChain
//   - OP#13 Entity/Traits      — GtsStore.ValidateEntity
//
// Most consumers start by constructing a GtsStore with a GtsReader (for
// example NewGtsFileReader) and calling the operation methods on it. Stateless
// operations that do not need a registry (OP#1–OP#5) are available directly
// from this package and from gtsid.
//
// Regular-expression handling follows the GTS safe regular-expression profile
// (spec §11.0.1); see CheckRegexProfile and RegexProfileSupported.
package gts
