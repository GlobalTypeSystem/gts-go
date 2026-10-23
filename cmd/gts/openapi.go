/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import (
	"github.com/GlobalTypeSystem/gts-go/internal/server"
)

var cmdOpenAPI = &Command{
	UsageLine: "openapi -out <file> [-host address] [-port number]",
	Short:     "generate OpenAPI specification",
	Long: `
OpenAPI generates an OpenAPI specification file for the GTS server.

The -out flag specifies the output file path.
The -host flag specifies the server host (default: 127.0.0.1).
The -port flag specifies the server port (default: 8000).

Example:

	gts openapi -out openapi.json
	`,
}

var (
	openAPIOut  string
	openAPIHost string
	openAPIPort int
)

func init() {
	cmdOpenAPI.Run = runOpenAPI
	cmdOpenAPI.Flag.StringVar(&openAPIOut, "out", "", "output file path")
	cmdOpenAPI.Flag.StringVar(&openAPIHost, "host", "127.0.0.1", "server host")
	cmdOpenAPI.Flag.IntVar(&openAPIPort, "port", 8000, "server port")
}

func runOpenAPI(cmd *Command, args []string) {
	if openAPIOut == "" {
		cmd.Usage()
	}

	store := newStore()
	srv := server.NewServer(store, openAPIHost, openAPIPort, verbose)
	spec := srv.GetOpenAPISpec()

	if err := writeJSONFile(openAPIOut, spec); err != nil {
		fatalf("failed to write OpenAPI spec: %v", err)
	}

	writeJSON(map[string]any{
		"ok":  true,
		"out": openAPIOut,
	})
}
