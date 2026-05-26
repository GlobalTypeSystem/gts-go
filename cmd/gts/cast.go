/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

var cmdCast = &Command{
	UsageLine: "cast -from <from-id> -to <to-type-id>",
	Short:     "cast an instance to a target type-schema",
	Long: `
Cast transforms an instance to conform to a target type-schema version.

The -from flag specifies the source instance GTS ID.
The -to flag specifies the target type-schema GTS ID.
Requires -path to be set to load entities.

Example:

	gts -path ./examples cast -from gts.vendor.pkg.ns.type.v1.0 -to gts.vendor.pkg.ns.type.v2~
	`,
}

var (
	castFrom string
	castTo   string
)

func init() {
	cmdCast.Run = runCast
	cmdCast.Flag.StringVar(&castFrom, "from", "", "source instance GTS ID")
	cmdCast.Flag.StringVar(&castTo, "to", "", "target type-schema GTS ID")
}

func runCast(cmd *Command, args []string) {
	if castFrom == "" || castTo == "" {
		cmd.Usage()
	}

	store := newStore()
	result, err := store.Cast(castFrom, castTo)
	if err != nil {
		fatalf("cast failed: %v", err)
	}
	writeJSON(result)
}
