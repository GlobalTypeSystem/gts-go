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
	"sort"
	"strings"

	"github.com/GlobalTypeSystem/gts-go/gts"
	"github.com/google/uuid"
)

// GtsJsonValidationIssue represents a single issue found during JSON validation
type GtsJsonValidationIssue struct {
	File    string `json:"file"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
	Index   *int   `json:"index,omitempty"`
}

// GtsJsonValidationResult represents the result of validating JSON files
type GtsJsonValidationResult struct {
	OK          bool                      `json:"ok"`
	Files       int                       `json:"files"`
	Documents   int                       `json:"documents"`
	GtsEntities int                       `json:"gts_entities"`
	Schemas     int                       `json:"schemas"`
	Instances   int                       `json:"instances"`
	Issues      []*GtsJsonValidationIssue `json:"issues"`
}

// GtsJsonValidator validates all JSON documents in a file or directory
type GtsJsonValidator struct {
	path      string
	cfg       *gts.GtsConfig
	files     int
	documents int
	entities  []*gts.JsonEntity
	issues    []*GtsJsonValidationIssue
}

// NewGtsJsonValidator creates a new JSON validator
func NewGtsJsonValidator(path string, cfg *gts.GtsConfig) *GtsJsonValidator {
	if cfg == nil {
		cfg = gts.DefaultGtsConfig()
	}
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	return &GtsJsonValidator{
		path:   path,
		cfg:    cfg,
		issues: make([]*GtsJsonValidationIssue, 0),
	}
}

// Validate performs the full JSON validation workflow
func (v *GtsJsonValidator) Validate() *GtsJsonValidationResult {
	jsonFiles := v.collectJSONFiles()
	for _, filePath := range jsonFiles {
		v.readFile(filePath)
	}
	v.validateJSONSchemas()
	store := v.registerGtsEntities()
	schemasCount, instancesCount := v.countSchemaInstance()
	v.validateSchemas(store)
	v.validateInstances(store)

	return &GtsJsonValidationResult{
		OK:          len(v.issues) == 0,
		Files:       v.files,
		Documents:   v.documents,
		GtsEntities: schemasCount + instancesCount,
		Schemas:     schemasCount,
		Instances:   instancesCount,
		Issues:      v.issues,
	}
}

func (v *GtsJsonValidator) collectJSONFiles() []string {
	resolved, err := filepath.Abs(v.path)
	if err != nil {
		resolved = v.path
	}

	info, err := os.Stat(resolved)
	if err != nil {
		v.addIssue(resolved, "discovery", "Path does not exist or is not accessible", nil)
		return nil
	}

	if !info.IsDir() {
		if strings.EqualFold(filepath.Ext(resolved), ".json") {
			return []string{resolved}
		}
		v.addIssue(resolved, "discovery", "Expected a .json file", nil)
		return nil
	}

	seen := make(map[string]bool)
	var files []string

	err = filepath.Walk(resolved, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			for _, excluded := range gts.ExcludeList {
				if name == excluded {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".json") {
			abs, err := filepath.Abs(path)
			if err != nil {
				abs = path
			}
			if !seen[abs] {
				seen[abs] = true
				files = append(files, abs)
			}
		}
		return nil
	})
	if err != nil {
		v.addIssue(resolved, "discovery", err.Error(), nil)
	}

	sort.Strings(files)
	return files
}

func (v *GtsJsonValidator) readFile(filePath string) {
	v.files++

	data, err := os.ReadFile(filePath)
	if err != nil {
		v.addIssue(filePath, "json", err.Error(), nil)
		return
	}

	var content any
	if err := json.Unmarshal(data, &content); err != nil {
		v.addIssue(filePath, "json", err.Error(), nil)
		return
	}

	fileName := filepath.Base(filePath)
	file := &gts.JsonFile{
		Path:    filePath,
		Name:    fileName,
		Content: content,
	}

	switch val := content.(type) {
	case []any:
		for i, item := range val {
			v.documents++
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			idx := i
			entity := gts.NewJsonEntityWithFile(obj, v.cfg, file, &idx)
			v.entities = append(v.entities, entity)
		}
	case map[string]any:
		v.documents++
		entity := gts.NewJsonEntityWithFile(val, v.cfg, file, nil)
		v.entities = append(v.entities, entity)
	}
}

func (v *GtsJsonValidator) validateJSONSchemas() {
	for _, entity := range v.entities {
		if entity.Content == nil {
			continue
		}
		schemaVal, hasSchema := entity.Content["$schema"]
		if !hasSchema {
			continue
		}
		if _, ok := schemaVal.(string); !ok {
			v.addIssueEntity(entity, "json-schema", "$schema must be a string")
		}
	}
}

func (v *GtsJsonValidator) registerGtsEntities() *gts.GtsStore {
	store := gts.NewGtsStore(nil)
	keys := make(map[string]bool)

	for _, entity := range v.entities {
		if !isGtsRelated(entity.Content) {
			continue
		}

		key := validatorRegistryKey(entity)
		if key == "" {
			v.addIssueEntity(entity, "registry", "GTS-related document has no registrable GTS ID")
			continue
		}

		// For non-schema entities without a selected entity field, generate UUID from raw id
		if !entity.IsTypeSchema && entity.SelectedEntityField == "" {
			if entity.File != nil {
				id := entity.File.Path
				if entity.ListSequence != nil {
					id = fmt.Sprintf("%s#%d", id, *entity.ListSequence)
				}
				ns := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8") // URL namespace
				key = uuid.NewSHA1(ns, []byte(id)).String()
			}
		}

		if keys[key] {
			v.addIssueEntity(entity, "registry", fmt.Sprintf("Duplicate GTS entity ID '%s'", key))
			continue
		}

		keys[key] = true
		_ = store.Register(entity)
	}

	return store
}

func (v *GtsJsonValidator) countSchemaInstance() (int, int) {
	schemas := 0
	instances := 0
	for _, entity := range v.entities {
		if !isGtsRelated(entity.Content) {
			continue
		}
		if validatorRegistryKey(entity) == "" {
			continue
		}
		if entity.IsTypeSchema {
			schemas++
		} else {
			instances++
		}
	}
	return schemas, instances
}

func (v *GtsJsonValidator) validateSchemas(store *gts.GtsStore) {
	type schemaEntry struct {
		id    string
		depth int
	}
	var schemaIDs []schemaEntry

	for _, entity := range v.entities {
		if !entity.IsTypeSchema || entity.GtsID == nil {
			continue
		}
		id := entity.GtsID.ID
		if store.Get(id) == nil {
			continue
		}
		depth := len(entity.GtsID.Segments)
		schemaIDs = append(schemaIDs, schemaEntry{id: id, depth: depth})
	}

	sort.Slice(schemaIDs, func(i, j int) bool {
		return schemaIDs[i].depth < schemaIDs[j].depth
	})

	// Validate base types first (depth 1)
	for _, entry := range schemaIDs {
		if entry.depth != 1 {
			continue
		}
		if err := store.ValidateSchema(entry.id); err != nil {
			entity := store.Get(entry.id)
			file, index := validatorEntityFileInfo(entity)
			v.addIssue(file, "base-type", err.Error(), index)
		}
	}

	// Then derived types (depth > 1)
	for _, entry := range schemaIDs {
		if entry.depth <= 1 {
			continue
		}
		if err := store.ValidateSchema(entry.id); err != nil {
			entity := store.Get(entry.id)
			file, index := validatorEntityFileInfo(entity)
			v.addIssue(file, "derived-type", err.Error(), index)
		}
	}
}

func (v *GtsJsonValidator) validateInstances(store *gts.GtsStore) {
	for _, entity := range v.entities {
		if entity.IsTypeSchema || !isGtsRelated(entity.Content) {
			continue
		}
		key := validatorRegistryKey(entity)
		if key == "" {
			continue
		}
		result := store.ValidateInstance(key)
		if !result.OK {
			v.addIssueEntity(entity, "instance", result.Error)
		}
	}
}

func (v *GtsJsonValidator) addIssue(file, stage, message string, index *int) {
	v.issues = append(v.issues, &GtsJsonValidationIssue{
		File:    file,
		Stage:   stage,
		Message: message,
		Index:   index,
	})
}

func (v *GtsJsonValidator) addIssueEntity(entity *gts.JsonEntity, stage, message string) {
	file := validatorEntityFile(entity)
	v.issues = append(v.issues, &GtsJsonValidationIssue{
		File:    file,
		Stage:   stage,
		Message: message,
		Index:   entity.ListSequence,
	})
}

// Helper functions

func isGtsRelated(content map[string]any) bool {
	return isGtsRelatedValue(content)
}

func isGtsRelatedValue(val any) bool {
	switch v := val.(type) {
	case string:
		return strings.Contains(v, "gts.")
	case map[string]any:
		for _, value := range v {
			if isGtsRelatedValue(value) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if isGtsRelatedValue(item) {
				return true
			}
		}
	}
	return false
}

func validatorRegistryKey(entity *gts.JsonEntity) string {
	if entity.IsTypeSchema {
		if entity.GtsID != nil {
			return entity.GtsID.ID
		}
		return ""
	}
	// For instances: need an effective ID AND either a gts_id or a valid type_id
	effectiveID := entity.EffectiveID()
	if effectiveID == "" {
		return ""
	}
	if entity.GtsID != nil {
		return effectiveID
	}
	if entity.TypeID != "" && gts.IsValidGtsID(entity.TypeID) {
		return effectiveID
	}
	return ""
}

func validatorEntityFile(entity *gts.JsonEntity) string {
	if entity.File != nil {
		return entity.File.Path
	}
	return entity.Label
}

func validatorEntityFileInfo(entity *gts.JsonEntity) (string, *int) {
	if entity == nil {
		return "", nil
	}
	return validatorEntityFile(entity), entity.ListSequence
}
