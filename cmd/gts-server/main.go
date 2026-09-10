/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import (
	"flag"
	"log"

	"github.com/GlobalTypeSystem/gts-go/gts"
	"github.com/GlobalTypeSystem/gts-go/server"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Host to bind to")
	port := flag.Int("port", 8000, "Port to listen on")
	verbose := flag.Int("verbose", 1, "Verbosity level (0=silent, 1=info, 2=debug)")
	flag.Parse()

	// Create store; store logging follows the -verbose flag
	store := gts.NewGtsStoreWithConfig(nil, &gts.RegistryConfig{Verbose: *verbose >= 1})

	// Create and start server
	srv := server.NewServer(store, *host, *port, *verbose)
	log.Fatal(srv.Start())
}
