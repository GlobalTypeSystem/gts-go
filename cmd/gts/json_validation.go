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
	"github.com/GlobalTypeSystem/gts-go/gtsid"
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
	exclude   []string
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
		path:    path,
		cfg:     cfg,
		exclude: append([]string(nil), gts.ExcludeList...),
		issues:  make([]*GtsJsonValidationIssue, 0),
	}
}

// WithExclude overrides the directory names skipped during scanning. An empty
// list is ignored so callers keep the default exclusions.
func (v *GtsJsonValidator) WithExclude(exclude []string) *GtsJsonValidator {
	if len(exclude) > 0 {
		v.exclude = exclude
	}
	return v
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
			v.addIssue(path, "discovery", fmt.Sprintf("Traversal error: %s", err.Error()), nil)
			return nil
		}
		if info.IsDir() {
			for _, excl := range v.exclude {
				if info.Name() == excl {
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

// isGtsMarker checks raw file text for GTS markers before paying JSON parse cost.
func isGtsMarker(text string) bool {
	return strings.Contains(text, gtsid.Prefix) ||
		strings.Contains(text, gtsid.URIPrefix) ||
		strings.Contains(text, gts.KeyXGtsRef)
}

func (v *GtsJsonValidator) readFile(filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		v.addIssue(filePath, "json", err.Error(), nil)
		return
	}

	if !isGtsMarker(string(data)) {
		return
	}
	v.files++

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
		if !v.isGtsRelated(entity.Content) {
			continue
		}

		key := validatorRegistryKey(entity)
		if key == "" {
			if entity.IsTypeSchema {
				v.addIssueEntity(entity, "registry", "GTS schema has a malformed or non-GTS $id")
			}
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
	type schemaPending struct {
		depth int
		id    string
		file  string
		index *int
	}
	var pending []schemaPending

	for _, entity := range v.entities {
		if !entity.IsTypeSchema || entity.GtsID == nil {
			continue
		}
		id := entity.GtsID.ID
		if store.Get(id) == nil {
			continue
		}
		pending = append(pending, schemaPending{
			depth: len(entity.GtsID.Segments),
			id:    id,
			file:  validatorEntityFile(entity),
			index: entity.ListSequence,
		})
	}

	sort.Slice(pending, func(i, j int) bool {
		a, b := pending[i], pending[j]
		if a.depth != b.depth {
			return a.depth < b.depth
		}
		if a.id != b.id {
			return a.id < b.id
		}
		if a.file != b.file {
			return a.file < b.file
		}
		ai := -1
		if a.index != nil {
			ai = *a.index
		}
		bi := -1
		if b.index != nil {
			bi = *b.index
		}
		return ai < bi
	})

	for _, s := range pending {
		stage := "base-type"
		if s.depth > 1 {
			stage = "derived-type"
		}
		if err := store.ValidateSchema(s.id); err != nil {
			v.addIssue(s.file, stage, err.Error(), s.index)
		}
	}
}

func entityDepth(entity *gts.JsonEntity) int {
	if entity.GtsID != nil {
		return len(entity.GtsID.Segments)
	}
	if entity.TypeID != "" {
		if gid, err := gtsid.New(entity.TypeID); err == nil {
			return len(gid.Segments)
		}
	}
	return 0
}

func (v *GtsJsonValidator) validateInstances(store *gts.GtsStore) {
	type instancePending struct {
		depth       int
		id          string
		file        string
		index       *int
		registryKey string
	}
	var pending []instancePending

	for _, entity := range v.entities {
		if entity.IsTypeSchema {
			continue
		}
		key := validatorRegistryKey(entity)
		if key == "" {
			continue
		}
		// Skip rejected duplicates: only validate the registered entity
		if store.Get(key) != entity {
			continue
		}
		gtsIDStr := ""
		if entity.GtsID != nil {
			gtsIDStr = entity.GtsID.ID
		}
		pending = append(pending, instancePending{
			depth:       entityDepth(entity),
			id:          gtsIDStr,
			file:        validatorEntityFile(entity),
			index:       entity.ListSequence,
			registryKey: key,
		})
	}

	sort.Slice(pending, func(i, j int) bool {
		a, b := pending[i], pending[j]
		if a.depth != b.depth {
			return a.depth < b.depth
		}
		if a.id != b.id {
			return a.id < b.id
		}
		if a.file != b.file {
			return a.file < b.file
		}
		ai := -1
		if a.index != nil {
			ai = *a.index
		}
		bi := -1
		if b.index != nil {
			bi = *b.index
		}
		return ai < bi
	})

	for _, p := range pending {
		result := store.ValidateInstance(p.registryKey)
		if !result.OK {
			v.addIssue(p.file, "instance", result.Error, p.index)
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

// looksGts checks whether a string value looks like a GTS identifier
// (valid or potentially malformed) by checking for the bare "gts." or the
// "gts://" URI form. It delegates to the shared reference classifier so the
// grammar lives in one place.
func looksGts(v string) bool {
	switch gts.ClassifyRef(v) {
	case gts.RefBareGtsID, gts.RefGtsURI:
		return true
	default:
		return false
	}
}

// isGtsRelated checks whether a JSON object contains GTS-related identifiers
// in the configured entity or type ID fields. This avoids false positives from
// incidental "gts." mentions in arbitrary nested strings.
func (v *GtsJsonValidator) isGtsRelated(content map[string]any) bool {
	for _, f := range v.cfg.EntityIDFields {
		if val, ok := content[f]; ok {
			if s, isStr := val.(string); isStr && looksGts(s) {
				return true
			}
		}
	}
	for _, f := range v.cfg.TypeIDFields {
		if val, ok := content[f]; ok {
			if s, isStr := val.(string); isStr && looksGts(s) {
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
	if entity.TypeID != "" && gtsid.IsValid(entity.TypeID) {
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
