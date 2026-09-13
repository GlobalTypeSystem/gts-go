/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import "github.com/GlobalTypeSystem/gts-go/gtsid"

var cmdUUID = &Command{
	UsageLine: "uuid -id <gts-id>",
	Short:     "generate UUID from a GTS ID",
	Long: `
UUID generates a deterministic UUID from a GTS identifier.

The -id flag specifies the GTS ID.

Example:

	gts uuid -id gts.vendor.pkg.ns.type.v1~
	`,
}

var (
	uuidIDFlag string
)

func init() {
	cmdUUID.Run = runUUID
	cmdUUID.Flag.StringVar(&uuidIDFlag, "id", "", "GTS ID")
}

func runUUID(cmd *Command, args []string) {
	if uuidIDFlag == "" {
		cmd.Usage()
	}

	result := gtsid.IDToUUID(uuidIDFlag)
	writeJSON(result)
}
