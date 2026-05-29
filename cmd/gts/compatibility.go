/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

var cmdCompatibility = &Command{
	UsageLine: "compatibility -old <old-type-id> -new <new-type-id>",
	Short:     "check compatibility between two type-schemas",
	Long: `
Compatibility checks whether two type-schema versions are compatible.

The -old flag specifies the old type-schema GTS ID.
The -new flag specifies the new type-schema GTS ID.
Requires -path to be set to load entities.

Example:

	gts -path ./examples compatibility -old gts.vendor.pkg.ns.type.v1~ -new gts.vendor.pkg.ns.type.v2~
	`,
}

var (
	compatOld string
	compatNew string
)

func init() {
	cmdCompatibility.Run = runCompatibility
	cmdCompatibility.Flag.StringVar(&compatOld, "old", "", "old type-schema GTS ID")
	cmdCompatibility.Flag.StringVar(&compatNew, "new", "", "new type-schema GTS ID")
}

func runCompatibility(cmd *Command, args []string) {
	if compatOld == "" || compatNew == "" {
		cmd.Usage()
	}

	store := newStore()
	result := store.CheckCompatibility(compatOld, compatNew)
	writeJSON(result)
}
