/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import (
	"fmt"
	"os"

	"github.com/GlobalTypeSystem/gts-go/gts"
)

var cmdValidateJson = &Command{
	UsageLine: "validate-json -path <path>",
	Short:     "validate all JSON documents in a file or directory",
	Long: `
ValidateJson validates all JSON documents in a file or directory.

It scans for .json files, validates JSON syntax, registers GTS entities,
and validates schemas and instances.

The -path flag specifies a JSON file or directory to scan. If not provided,
the global -path flag value is used.

Example:
  gts validate-json -path ./schemas
  gts validate-json -path ./my-schema.json
	`,
}

var validateJsonPath string

func init() {
	cmdValidateJson.Run = runValidateJson
	cmdValidateJson.Flag.StringVar(&validateJsonPath, "path", "", "JSON file or directory to scan")
}

func runValidateJson(cmd *Command, args []string) {
	scanPath := validateJsonPath
	if scanPath == "" {
		scanPath = path // fall back to global -path flag
	}
	if scanPath == "" {
		cmd.Usage()
		return
	}

	var cfg *gts.GtsConfig
	if cfgPath != "" {
		cfg = loadConfig(cfgPath)
	}

	validator := NewGtsJsonValidator(scanPath, cfg)
	result := validator.Validate()

	for _, issue := range result.Issues {
		suffix := ""
		if issue.Index != nil {
			suffix = fmt.Sprintf("#%d", *issue.Index)
		}
		fmt.Fprintf(os.Stderr, "%s%s: %s: %s\n", issue.File, suffix, issue.Stage, issue.Message)
	}

	writeJSON(result)
}
