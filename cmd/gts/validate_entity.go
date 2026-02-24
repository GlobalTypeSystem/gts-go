/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

var cmdValidateEntity = &Command{
	UsageLine: "validate-entity -id <gts-id>",
	Short:     "validate any entity (schema or instance) including traits",
	Long: `
ValidateEntity validates any GTS entity by its ID.

For schemas: runs OP#12 chain validation and OP#13 traits validation.
For instances: validates the instance against its schema.

The -id flag specifies the GTS entity ID to validate.
Requires -path to be set to load entities.

Example:

	gts -path ./examples validate-entity -id gts.vendor.pkg.ns.base.v1~derived.v1~
	gts -path ./examples validate-entity -id gts.vendor.pkg.ns.type.v1.0
	`,
}

var validateEntityID string

func init() {
	cmdValidateEntity.Run = runValidateEntity
	cmdValidateEntity.Flag.StringVar(&validateEntityID, "id", "", "GTS entity ID to validate")
}

func runValidateEntity(cmd *Command, args []string) {
	if validateEntityID == "" {
		cmd.Usage()
		return
	}

	store := newStore()
	result := store.ValidateEntity(validateEntityID)
	writeJSON(result)
}
