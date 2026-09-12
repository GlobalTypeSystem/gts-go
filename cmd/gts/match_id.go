/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import "github.com/GlobalTypeSystem/gts-go/gtsid"

var cmdMatchIDPattern = &Command{
	UsageLine: "match-id-pattern -pattern <pattern> -candidate <gts-id>",
	Short:     "match a GTS ID against a pattern",
	Long: `
Match-id-pattern checks whether a GTS identifier matches a pattern.

The -pattern flag specifies the pattern (may contain wildcards).
The -candidate flag specifies the GTS ID to match.

Example:

	gts match-id-pattern -pattern "gts.vendor.pkg.*" -candidate gts.vendor.pkg.ns.type.v1.0
	`,
}

var (
	matchPattern   string
	matchCandidate string
)

func init() {
	cmdMatchIDPattern.Run = runMatchIDPattern
	cmdMatchIDPattern.Flag.StringVar(&matchPattern, "pattern", "", "pattern to match against")
	cmdMatchIDPattern.Flag.StringVar(&matchCandidate, "candidate", "", "candidate GTS ID")
}

func runMatchIDPattern(cmd *Command, args []string) {
	if matchPattern == "" || matchCandidate == "" {
		cmd.Usage()
	}

	result := gtsid.Match(matchCandidate, matchPattern)
	writeJSON(result)
}
