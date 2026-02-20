/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const usageText = `GTS is a tool for working with Global Type System identifiers and schemas.

Usage:

	gts <command> [arguments]

The commands are:

	validate-id     validate a GTS ID format
	parse-id        parse a GTS ID into its components
	match-id-pattern match a GTS ID against a pattern
	uuid            generate UUID from a GTS ID
	validate        validate an instance against its schema
	validate-schema validate a derived schema against its chain
	validate-entity validate any entity (schema or instance) including traits
	relationships   resolve relationships for an entity
	compatibility   check compatibility between two schemas
	cast            cast an instance to a target schema
	query           query entities using an expression
	attr            get attribute value from a GTS entity
	list            list all entities
	server          start the GTS HTTP server
	openapi         generate OpenAPI specification
	version         print GTS version

Use "gts <command> -h" for more information about a command.

Additional help topics:

Use "gts help <topic>" for more information about that topic.
`

// Command represents a gts subcommand.
type Command struct {
	Run       func(cmd *Command, args []string)
	UsageLine string
	Short     string
	Long      string
	Flag      flag.FlagSet
}

// Name returns the command's name: the first word in the usage line.
func (c *Command) Name() string {
	name := c.UsageLine
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

// Usage prints the command usage and exits.
func (c *Command) Usage() {
	fmt.Fprintf(os.Stderr, "usage: %s\n", c.UsageLine)
	fmt.Fprintf(os.Stderr, "%s\n", strings.TrimSpace(c.Long))
	os.Exit(2)
}

// Runnable reports whether the command can be run; otherwise
// it is a documentation pseudo-command such as importpath.
func (c *Command) Runnable() bool {
	return c.Run != nil
}

// commands is the list of all available commands.
var commands = []*Command{
	cmdValidateID,
	cmdParseID,
	cmdMatchIDPattern,
	cmdUUID,
	cmdValidate,
	cmdValidateSchema,
	cmdValidateEntity,
	cmdRelationships,
	cmdCompatibility,
	cmdCast,
	cmdQuery,
	cmdAttr,
	cmdList,
	cmdServer,
	cmdOpenAPI,
	cmdVersion,
}

// Global flags
var (
	verbose int
	cfgPath string
	path    string
)

func init() {
	// Environment variable defaults
	if v := os.Getenv("GTS_VERBOSE"); v != "" {
		fmt.Sscanf(v, "%d", &verbose)
	}
	if p := os.Getenv("GTS_PATH"); p != "" {
		path = p
	}
	if c := os.Getenv("GTS_CONFIG"); c != "" {
		cfgPath = c
	}
}

func main() {
	flag.Usage = usage
	flag.IntVar(&verbose, "v", verbose, "enable verbose logging")
	flag.StringVar(&path, "path", path, "path to JSON and schema files or directories")
	flag.StringVar(&cfgPath, "config", cfgPath, "path to GTS config JSON file")

	log.SetPrefix("gts: ")
	log.SetFlags(0)

	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	// Configure logging based on verbosity
	if verbose == 0 {
		log.SetOutput(io.Discard)
	}

	cmdName := args[0]
	for _, cmd := range commands {
		if cmd.Name() == cmdName {
			if !cmd.Runnable() {
				continue
			}
			cmd.Flag.Usage = func() { cmd.Usage() }
			cmd.Flag.Parse(args[1:])
			cmd.Run(cmd, cmd.Flag.Args())
			return
		}
	}

	fmt.Fprintf(os.Stderr, "gts: unknown command %q\nRun 'gts help' for usage.\n", cmdName)
	os.Exit(2)
}

func usage() {
	fmt.Fprint(os.Stderr, usageText)
	os.Exit(2)
}
