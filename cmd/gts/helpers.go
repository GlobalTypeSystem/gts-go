/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/GlobalTypeSystem/gts-go/gts"
)

// newStore creates a new GTS store with optional file reader
func newStore() *gts.GtsStore {
	var reader gts.GtsReader

	if path != "" {
		paths := parsePaths(path)
		var cfg *gts.GtsConfig
		if cfgPath != "" {
			cfg = loadConfig(cfgPath)
		}
		reader = gts.NewGtsFileReader(paths, cfg)
		if verbose > 0 {
			log.Printf("loaded entities from: %s", strings.Join(paths, ", "))
		}
	}

	store := gts.NewGtsStore(reader)
	if verbose > 0 && path != "" {
		log.Printf("entity count: %d", store.Count())
	}
	return store
}

// parsePaths splits a comma-separated path specification into individual paths
func parsePaths(pathSpec string) []string {
	parts := strings.Split(pathSpec, ",")
	paths := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			// Expand ~ to home directory
			if strings.HasPrefix(p, "~/") {
				home, err := os.UserHomeDir()
				if err == nil {
					p = filepath.Join(home, p[2:])
				}
			}
			paths = append(paths, p)
		}
	}
	return paths
}

// loadConfig loads a GTS config from a file
func loadConfig(path string) *gts.GtsConfig {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("warning: could not open config file: %v", err)
		return gts.DefaultGtsConfig()
	}
	defer f.Close()

	var data struct {
		EntityIDFields []string `json:"entity_id_fields"`
		TypeIDFields   []string `json:"type_id_fields"`
	}

	if err := json.NewDecoder(f).Decode(&data); err != nil {
		log.Printf("warning: could not parse config file: %v", err)
		return gts.DefaultGtsConfig()
	}

	return &gts.GtsConfig{
		EntityIDFields: data.EntityIDFields,
		TypeIDFields:   data.TypeIDFields,
	}
}

// writeJSON writes a value as JSON to stdout
func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fatalf("json encoding failed: %v", err)
	}
}

// writeJSONFile writes a value as JSON to a file
func writeJSONFile(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// fatalf prints an error message and exits with status 1
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gts: "+format+"\n", args...)
	os.Exit(1)
}
