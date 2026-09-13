/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GlobalTypeSystem/gts-go/gts"
)

func baseSchema() string {
	return `{
		"$id": "gts://gts.cli.core.test.base.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": { "name": { "type": "string" } },
		"required": ["name"]
	}`
}

func leafSchema() string {
	return `{
		"$id": "gts://gts.cli.core.test.base.v1~cli.core.test.leaf.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"allOf": [
			{ "$ref": "gts://gts.cli.core.test.base.v1~" },
			{ "properties": { "extra": { "type": "string" } } }
		]
	}`
}

func baseInstance() string {
	return `{
		"id": "gts.cli.core.test.base.v1~cli.app._.thing.v1.0",
		"name": "hello"
	}`
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func runValidator(dir string) *GtsJsonValidationResult {
	return NewGtsJsonValidator(dir, gts.DefaultGtsConfig()).Validate()
}

func TestValidSchemaAndInstanceSet(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "base.schema.json", baseSchema())
	writeTestFile(t, dir, "thing.json", baseInstance())

	result := runValidator(dir)

	if !result.OK {
		t.Fatalf("expected ok, issues: %v", result.Issues)
	}
	if result.Schemas != 1 {
		t.Errorf("expected 1 schema, got %d", result.Schemas)
	}
	if result.Instances != 1 {
		t.Errorf("expected 1 instance, got %d", result.Instances)
	}
	if result.GtsEntities != 2 {
		t.Errorf("expected 2 gts entities, got %d", result.GtsEntities)
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected no issues, got %v", result.Issues)
	}
}

func TestDerivedSchemaSetIsValid(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "base.schema.json", baseSchema())
	writeTestFile(t, dir, "leaf.schema.json", leafSchema())

	result := runValidator(dir)

	if !result.OK {
		t.Fatalf("expected ok, issues: %v", result.Issues)
	}
	if result.Schemas != 2 {
		t.Errorf("expected 2 schemas, got %d", result.Schemas)
	}
	if result.Instances != 0 {
		t.Errorf("expected 0 instances, got %d", result.Instances)
	}
}

func TestMalformedSchemaIDIsReported(t *testing.T) {
	dir := t.TempDir()
	malformed := `{
		"$id": "gts://gtx.cli.core.test.bad.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object"
	}`
	writeTestFile(t, dir, "bad.schema.json", malformed)

	result := runValidator(dir)

	if result.OK {
		t.Fatal("malformed GTS id should fail")
	}
	if result.GtsEntities != 0 {
		t.Errorf("expected 0 gts entities, got %d", result.GtsEntities)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Stage == "registry" && contains(issue.Message, "malformed") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a malformed-id diagnostic, got: %v", result.Issues)
	}
}

func TestIncidentalPrefixMentionIsNotRegistered(t *testing.T) {
	dir := t.TempDir()
	doc := `{ "description": "see gts.foo.bar for details", "value": 42 }`
	writeTestFile(t, dir, "unrelated.json", doc)

	result := runValidator(dir)

	if !result.OK {
		t.Fatalf("incidental mention should not fail: %v", result.Issues)
	}
	if result.Documents != 1 {
		t.Errorf("expected 1 document, got %d", result.Documents)
	}
	if result.GtsEntities != 0 {
		t.Errorf("expected 0 gts entities, got %d", result.GtsEntities)
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected no issues, got %v", result.Issues)
	}
}

func TestDuplicateEntityIsReported(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.schema.json", baseSchema())
	writeTestFile(t, dir, "b.schema.json", baseSchema())

	result := runValidator(dir)

	if result.OK {
		t.Fatal("duplicate ids should fail")
	}
	found := false
	for _, issue := range result.Issues {
		if contains(issue.Message, "Duplicate") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a duplicate diagnostic, got: %v", result.Issues)
	}
}

func TestNonGtsFilesAreIgnored(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "base.schema.json", baseSchema())
	writeTestFile(t, dir, "package.json", `{ "name": "pkg", "version": "1.0.0" }`)

	nm := filepath.Join(dir, "node_modules")
	if err := os.Mkdir(nm, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, nm, "broken.json", "{ this is not json ")

	result := runValidator(dir)

	if !result.OK {
		t.Fatalf("non-GTS files must be ignored: %v", result.Issues)
	}
	if result.Files != 1 {
		t.Errorf("only the GTS schema should be processed, got %d files", result.Files)
	}
	if result.Schemas != 1 {
		t.Errorf("expected 1 schema, got %d", result.Schemas)
	}
}

func TestMarkerHeuristicMatchesExpectedCombinations(t *testing.T) {
	if !isGtsMarker(`{ "id": "gts.x.y.z.t.v1~a.b.c.d.v1.0" }`) {
		t.Error("should match gts. prefix")
	}
	if !isGtsMarker(`{ "$id": "gts://gts.x.y.z.t.v1~" }`) {
		t.Error("should match gts:// URI")
	}
	if !isGtsMarker(`{ "properties": { "p": { "x-gts-ref": "..." } } }`) {
		t.Error("should match x-gts-ref keyword")
	}
	if isGtsMarker(`{ "name": "widgets", "version": "1.0.0" }`) {
		t.Error("should not match non-GTS content")
	}
}

func TestSchemaErrorsOrderedByDepthThenGtsID(t *testing.T) {
	dir := t.TempDir()
	// Valid base schema
	writeTestFile(t, dir, "base.schema.json", baseSchema())

	// Derived schema with invalid x-gts-ref that triggers validation error
	invalidLeaf := `{
		"$id": "gts://gts.cli.core.test.base.v1~cli.core.test.leaf.v1~",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": { "ref": { "x-gts-ref": 42 } }
	}`
	writeTestFile(t, dir, "a_leaf.schema.json", invalidLeaf)

	result := runValidator(dir)

	var schemaIssues []*GtsJsonValidationIssue
	for _, issue := range result.Issues {
		if issue.Stage == "base-type" || issue.Stage == "derived-type" {
			schemaIssues = append(schemaIssues, issue)
		}
	}

	// The derived schema should produce a derived-type error (chain incompatibility)
	derivedIssues := filterByStage(schemaIssues, "derived-type")
	if len(derivedIssues) == 0 {
		t.Fatalf("expected at least 1 derived-type issue, got %d: %v", len(derivedIssues), issuesSummary(result.Issues))
	}

	// All base-type issues (if any) should come before derived-type issues
	baseIssues := filterByStage(schemaIssues, "base-type")
	if len(baseIssues) > 0 && len(derivedIssues) > 0 {
		lastBaseIdx := indexOfIssue(result.Issues, baseIssues[len(baseIssues)-1])
		firstDerivedIdx := indexOfIssue(result.Issues, derivedIssues[0])
		if lastBaseIdx > firstDerivedIdx {
			t.Errorf("base-type issues should come before derived-type issues")
		}
	}
}

func TestInstanceErrorsOrderedByGtsID(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "base.schema.json", baseSchema())

	invalidInstance := func(seg string) string {
		return fmt.Sprintf(`{ "id": "gts.cli.core.test.base.v1~cli.app._.%s.v1.0" }`, seg)
	}

	writeTestFile(t, dir, "z_alpha.json", invalidInstance("alpha"))
	writeTestFile(t, dir, "a_zeta.json", invalidInstance("zeta"))

	result := runValidator(dir)

	var instanceIssues []*GtsJsonValidationIssue
	for _, issue := range result.Issues {
		if issue.Stage == "instance" {
			instanceIssues = append(instanceIssues, issue)
		}
	}

	if len(instanceIssues) != 2 {
		t.Fatalf("expected 2 instance issues, got %d: %v", len(instanceIssues), issuesSummary(result.Issues))
	}
	if !endsWith(instanceIssues[0].File, "z_alpha.json") {
		t.Errorf("alpha should be first: %s", instanceIssues[0].File)
	}
	if !endsWith(instanceIssues[1].File, "a_zeta.json") {
		t.Errorf("zeta should be second: %s", instanceIssues[1].File)
	}
}

// Helpers

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func filterByStage(issues []*GtsJsonValidationIssue, stage string) []*GtsJsonValidationIssue {
	var filtered []*GtsJsonValidationIssue
	for _, issue := range issues {
		if issue.Stage == stage {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func indexOfIssue(all []*GtsJsonValidationIssue, target *GtsJsonValidationIssue) int {
	for i, issue := range all {
		if issue == target {
			return i
		}
	}
	return -1
}

func issuesSummary(issues []*GtsJsonValidationIssue) string {
	b, _ := json.MarshalIndent(issues, "", "  ")
	return string(b)
}
