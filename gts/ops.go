/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

// ExtractGtsID extracts GTS ID from JSON content.
// This lives in the gts (model) layer rather than gtsid because it operates on
// JSON entities and configuration, not on identifiers in isolation.
func ExtractGtsID(content map[string]any, cfg *GtsConfig) *ExtractIDResult {
	return ExtractID(content, cfg)
}
