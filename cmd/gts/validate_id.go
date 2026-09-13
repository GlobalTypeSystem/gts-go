/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import "github.com/GlobalTypeSystem/gts-go/gtsid"

var cmdValidateID = &Command{
	UsageLine: "validate-id -id <gts-id>",
	Short:     "validate a GTS ID format",
	Long: `
Validate-id validates the format of a GTS identifier.

The -id flag specifies the GTS ID to validate.

Example:

	gts validate-id -id gts.vendor.pkg.ns.type.v1~
	`,
}

var (
	validateIDFlag string
)

func init() {
	cmdValidateID.Run = runValidateID
	cmdValidateID.Flag.StringVar(&validateIDFlag, "id", "", "GTS ID to validate")
}

func runValidateID(cmd *Command, args []string) {
	if validateIDFlag == "" {
		cmd.Usage()
	}

	result := gtsid.Validate(validateIDFlag)
	writeJSON(result)
}
