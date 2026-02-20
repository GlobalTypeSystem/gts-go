/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

var cmdValidateSchema = &Command{
	UsageLine: "validate-schema -id <gts-id>",
	Short:     "validate a derived schema against its chain",
	Long: `
ValidateSchema checks that a derived schema only tightens, never loosens,
the constraints defined by its base schema(s).

The -id flag specifies the chained GTS schema ID to validate.
Requires -path to be set to load entities.

Example:

	gts -path ./examples validate-schema -id gts.vendor.pkg.ns.base.v1~derived.v1~
	`,
}

var validateSchemaID string

func init() {
	cmdValidateSchema.Run = runValidateSchema
	cmdValidateSchema.Flag.StringVar(&validateSchemaID, "id", "", "chained GTS schema ID to validate")
}

func runValidateSchema(cmd *Command, args []string) {
	if validateSchemaID == "" {
		cmd.Usage()
	}

	store := newStore()
	result := store.ValidateSchemaChain(validateSchemaID)
	writeJSON(result)
}
