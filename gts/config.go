/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

// GtsConfig holds configuration for extracting GTS IDs from JSON content
type GtsConfig struct {
	EntityIDFields []string
	TypeIDFields   []string
}

// DefaultGtsConfig returns the default configuration for ID extraction
func DefaultGtsConfig() *GtsConfig {
	return &GtsConfig{
		EntityIDFields: []string{
			"$id",
			"gtsId",
			"gtsIid",
			"gtsOid",
			"gtsI",
			"gts_id",
			"gts_oid",
			"gts_iid",
			"id",
		},
		TypeIDFields: []string{
			"gtsTid",
			"gtsType",
			"gtsT",
			"gts_t",
			"gts_tid",
			"gts_type",
			"type",
			"schema",
		},
	}
}
