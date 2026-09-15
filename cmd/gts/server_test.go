/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import "testing"

func TestServerEntityUpdatesDisabledByDefault(t *testing.T) {
	allowEntityUpdates = false
	if err := cmdServer.Flag.Parse([]string{}); err != nil {
		t.Fatalf("parse without flag failed: %v", err)
	}
	if allowEntityUpdates {
		t.Fatal("allowEntityUpdates should default to false")
	}

	if err := cmdServer.Flag.Parse([]string{"-allow-entity-updates"}); err != nil {
		t.Fatalf("parse with flag failed: %v", err)
	}
	if !allowEntityUpdates {
		t.Fatal("allowEntityUpdates should be true when -allow-entity-updates is set")
	}
	allowEntityUpdates = false
}
