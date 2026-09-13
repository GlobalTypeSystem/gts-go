/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import "github.com/GlobalTypeSystem/gts-go/gtsid"

var cmdParseID = &Command{
	UsageLine: "parse-id -id <gts-id>",
	Short:     "parse a GTS ID into its components",
	Long: `
Parse-id parses a GTS identifier into its component parts.

The -id flag specifies the GTS ID to parse.

Example:

	gts parse-id -id gts.vendor.pkg.ns.type.v1.0
	`,
}

var (
	parseIDFlag string
)

func init() {
	cmdParseID.Run = runParseID
	cmdParseID.Flag.StringVar(&parseIDFlag, "id", "", "GTS ID to parse")
}

func runParseID(cmd *Command, args []string) {
	if parseIDFlag == "" {
		cmd.Usage()
	}

	result := gtsid.Parse(parseIDFlag)
	writeJSON(result)
}
