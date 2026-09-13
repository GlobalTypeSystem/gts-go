/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestGtsFileReader_SingleFile tests reading a single JSON file
func TestGtsFileReader_SingleFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create a JSON file with a single entity
	testFile := filepath.Join(tmpDir, "test.json")
	content := map[string]any{
		"gtsId": "gts.vendor.package.namespace.type.v0~a.b.c.d.v1",
		"name":  "Test Entity",
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create file reader
	reader := NewGtsFileReaderFromPath(testFile, nil)

	// Read entity
	entity := reader.Next()
	if entity == nil {
		t.Fatal("Expected entity, got nil")
	}

	if entity.GtsID == nil || entity.GtsID.ID != "gts.vendor.package.namespace.type.v0~a.b.c.d.v1" {
		t.Errorf("Expected GtsID 'gts.vendor.package.namespace.type.v0', got %v", entity.GtsID)
	}

	// Should be no more entities
	if reader.Next() != nil {
		t.Error("Expected no more entities")
	}
}

// TestGtsFileReader_ArrayOfEntities tests reading a JSON array
func TestGtsFileReader_ArrayOfEntities(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.json")
	content := []map[string]any{
		{
			"gtsId": "gts.vendor.package.namespace.type1.v0~",
			"name":  "Entity 1",
		},
		{
			"gtsId": "gts.vendor.package.namespace.type2.v0~",
			"name":  "Entity 2",
		},
		{
			"gtsId": "gts.vendor.package.namespace.type3.v0~",
			"name":  "Entity 3",
		},
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	reader := NewGtsFileReaderFromPath(testFile, nil)

	// Read all entities
	var entities []*JsonEntity
	for {
		entity := reader.Next()
		if entity == nil {
			break
		}
		entities = append(entities, entity)
	}

	if len(entities) != 3 {
		t.Errorf("Expected 3 entities, got %d", len(entities))
	}

	// Check that list sequence is set
	for i, entity := range entities {
		if entity.ListSequence == nil {
			t.Errorf("Entity %d has nil ListSequence", i)
		} else if *entity.ListSequence != i {
			t.Errorf("Entity %d has ListSequence %d, expected %d", i, *entity.ListSequence, i)
		}

		// Check label format
		expectedLabel := "test.json#" + string(rune('0'+i))
		if entity.Label != expectedLabel {
			t.Errorf("Entity %d has Label %q, expected %q", i, entity.Label, expectedLabel)
		}
	}
}

// TestGtsFileReader_Directory tests reading all JSON files from a directory
func TestGtsFileReader_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple JSON files
	files := []struct {
		name    string
		content map[string]any
	}{
		{
			name: "entity1.json",
			content: map[string]any{
				"gtsId": "gts.vendor.package.namespace.type1.v0~",
			},
		},
		{
			name: "entity2.json",
			content: map[string]any{
				"gtsId": "gts.vendor.package.namespace.type2.v0~",
			},
		},
		{
			name: "entity3.gts",
			content: map[string]any{
				"gtsId": "gts.vendor.package.namespace.type3.v0~",
			},
		},
	}

	for _, f := range files {
		data, err := json.Marshal(f.content)
		if err != nil {
			t.Fatalf("Failed to marshal JSON: %v", err)
		}

		filePath := filepath.Join(tmpDir, f.name)
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Create file reader for directory
	reader := NewGtsFileReaderFromPath(tmpDir, nil)

	// Read all entities
	var entities []*JsonEntity
	for {
		entity := reader.Next()
		if entity == nil {
			break
		}
		entities = append(entities, entity)
	}

	if len(entities) != 3 {
		t.Errorf("Expected 3 entities, got %d", len(entities))
	}
}

// TestGtsFileReader_ExcludeDirectories tests that excluded directories are skipped
func TestGtsFileReader_ExcludeDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid file in root
	rootFile := filepath.Join(tmpDir, "root.json")
	rootContent := map[string]any{
		"gtsId": "gts.vendor.package.namespace.root.v0~",
	}
	data, err := json.Marshal(rootContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Create node_modules directory with a file
	nodeModules := filepath.Join(tmpDir, "node_modules")
	if err := os.Mkdir(nodeModules, 0755); err != nil {
		t.Fatal(err)
	}
	nmFile := filepath.Join(nodeModules, "excluded.json")
	nmContent := map[string]any{
		"gtsId": "gts.vendor.package.namespace.excluded.v0~",
	}
	data, err = json.Marshal(nmContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nmFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Create reader
	reader := NewGtsFileReaderFromPath(tmpDir, nil)

	// Read all entities
	var entities []*JsonEntity
	for {
		entity := reader.Next()
		if entity == nil {
			break
		}
		entities = append(entities, entity)
	}

	// Should only get the root file, not the one in node_modules
	if len(entities) != 1 {
		t.Errorf("Expected 1 entity (excluding node_modules), got %d", len(entities))
	}

	if len(entities) > 0 && entities[0].GtsID.ID != "gts.vendor.package.namespace.root.v0~" {
		t.Errorf("Expected root entity, got %s", entities[0].GtsID.ID)
	}
}

// TestGtsFileReader_Reset tests resetting the reader
func TestGtsFileReader_Reset(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.json")
	content := map[string]any{
		"gtsId": "gts.vendor.package.namespace.type.v0~",
	}

	data, _ := json.Marshal(content)
	_ = os.WriteFile(testFile, data, 0644)

	reader := NewGtsFileReaderFromPath(testFile, nil)

	// Read entity
	entity1 := reader.Next()
	if entity1 == nil {
		t.Fatal("Expected entity on first read")
	}

	// Should be exhausted
	if reader.Next() != nil {
		t.Error("Expected no more entities")
	}

	// Reset and read again
	reader.Reset()
	entity2 := reader.Next()
	if entity2 == nil {
		t.Fatal("Expected entity after reset")
	}

	if entity1.GtsID.ID != entity2.GtsID.ID {
		t.Errorf("Expected same entity after reset")
	}
}

// TestGtsFileReader_MultiplePaths tests reading from multiple paths
func TestGtsFileReader_MultiplePaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two separate directories
	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	_ = os.Mkdir(dir1, 0755)
	_ = os.Mkdir(dir2, 0755)

	// Create a file in each directory
	file1 := filepath.Join(dir1, "entity1.json")
	content1 := map[string]any{
		"gtsId": "gts.vendor.package.namespace.type1.v0~",
	}
	data1, _ := json.Marshal(content1)
	_ = os.WriteFile(file1, data1, 0644)

	file2 := filepath.Join(dir2, "entity2.json")
	content2 := map[string]any{
		"gtsId": "gts.vendor.package.namespace.type2.v0~",
	}
	data2, _ := json.Marshal(content2)
	_ = os.WriteFile(file2, data2, 0644)

	// Create reader with multiple paths
	reader := NewGtsFileReader([]string{dir1, dir2}, nil)

	// Read all entities
	var entities []*JsonEntity
	for {
		entity := reader.Next()
		if entity == nil {
			break
		}
		entities = append(entities, entity)
	}

	if len(entities) != 2 {
		t.Errorf("Expected 2 entities from multiple paths, got %d", len(entities))
	}
}

// TestGtsFileReader_NoGtsID tests that entities without GTS ID are skipped
func TestGtsFileReader_NoGtsID(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.json")
	content := []map[string]any{
		{
			"name": "No GTS ID",
		},
		{
			"gtsId": "gts.vendor.package.namespace.type.v0~",
		},
	}

	data, _ := json.Marshal(content)
	_ = os.WriteFile(testFile, data, 0644)

	reader := NewGtsFileReaderFromPath(testFile, nil)

	// Should only get the entity with GTS ID
	var entities []*JsonEntity
	for {
		entity := reader.Next()
		if entity == nil {
			break
		}
		entities = append(entities, entity)
	}

	if len(entities) != 1 {
		t.Errorf("Expected 1 entity (with GTS ID), got %d", len(entities))
	}
}

// TestGtsFileReader_InvalidJSON tests that invalid JSON files are skipped
func TestGtsFileReader_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid JSON file
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	_ = os.WriteFile(invalidFile, []byte("not valid json {"), 0644)

	// Create a valid file
	validFile := filepath.Join(tmpDir, "valid.json")
	content := map[string]any{
		"gtsId": "gts.vendor.package.namespace.type.v0~",
	}
	data, _ := json.Marshal(content)
	_ = os.WriteFile(validFile, data, 0644)

	reader := NewGtsFileReaderFromPath(tmpDir, nil)

	// Should only get the valid entity
	var entities []*JsonEntity
	for {
		entity := reader.Next()
		if entity == nil {
			break
		}
		entities = append(entities, entity)
	}

	if len(entities) != 1 {
		t.Errorf("Expected 1 entity (skipping invalid JSON), got %d", len(entities))
	}
}

// TestGtsFileReader_ReadByID tests that ReadByID returns nil
func TestGtsFileReader_ReadByID(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.json")
	content := map[string]any{
		"gtsId": "gts.vendor.package.namespace.type.v0~",
	}
	data, _ := json.Marshal(content)
	_ = os.WriteFile(testFile, data, 0644)

	reader := NewGtsFileReaderFromPath(testFile, nil)

	// ReadByID should always return nil for file reader
	entity := reader.ReadByID("gts.vendor.package.namespace.type.v0~")
	if entity != nil {
		t.Error("ReadByID should return nil for file reader")
	}
}
