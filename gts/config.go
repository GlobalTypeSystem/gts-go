/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

// GtsConfig holds configuration for extracting GTS IDs from JSON content
type GtsConfig struct {
	EntityIDFields []string
	SchemaIDFields []string
}

// DefaultGtsConfig returns the default configuration for ID extraction
func DefaultGtsConfig() *GtsConfig {
	return &GtsConfig{
		EntityIDFields: []string{
			"$id",
			"$$id",
			"gtsId",
			"gtsIid",
			"gtsOid",
			"gtsI",
			"gts_id",
			"gts_oid",
			"gts_iid",
			"id",
		},
		SchemaIDFields: []string{
			"$schema",
			"$$schema",
			"gtsTid",
			"gtsT",
			"gts_t",
			"gts_tid",
			"type",
			"schema",
		},
	}
}
