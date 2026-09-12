/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var (
	// ExcludeList is the default set of directory names to exclude during file
	// scanning. The CLI --exclude option overrides it per invocation.
	ExcludeList = []string{"node_modules", "dist", "build", ".git", "target"}
)

// GtsFileReader reads JSON entities from files and directories
type GtsFileReader struct {
	paths               []string
	cfg                 *GtsConfig
	exclude             []string
	files               []string
	currentIndex        int
	currentFileEntities []*JsonEntity
	currentEntityIndex  int
	initialized         bool
}

// NewGtsFileReader creates a new file reader with the given paths
func NewGtsFileReader(paths []string, cfg *GtsConfig) *GtsFileReader {
	if cfg == nil {
		cfg = DefaultGtsConfig()
	}

	// Expand home directory in paths
	expandedPaths := make([]string, len(paths))
	for i, p := range paths {
		if strings.HasPrefix(p, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				p = filepath.Join(home, p[2:])
			}
		}
		expandedPaths[i] = p
	}

	return &GtsFileReader{
		paths:   expandedPaths,
		cfg:     cfg,
		exclude: append([]string(nil), ExcludeList...),
	}
}

// WithExclude overrides the directory names skipped during scanning. An empty
// list is ignored so callers keep the default exclusions.
func (r *GtsFileReader) WithExclude(exclude []string) *GtsFileReader {
	if len(exclude) > 0 {
		r.exclude = exclude
	}
	return r
}

// NewGtsFileReaderFromPath creates a new file reader from a single path
func NewGtsFileReaderFromPath(path string, cfg *GtsConfig) *GtsFileReader {
	return NewGtsFileReader([]string{path}, cfg)
}

// collectFiles collects all JSON files from the specified paths
func (r *GtsFileReader) collectFiles() {
	validExtensions := map[string]bool{
		".json":  true,
		".jsonc": true,
		".gts":   true,
	}

	seen := make(map[string]bool)
	var collected []string

	for _, path := range r.paths {
		// Resolve path
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			// Walk directory recursively
			err := filepath.Walk(absPath, func(filePath string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // Skip files with errors
				}

				// Skip excluded directories
				if info.IsDir() {
					if slices.Contains(r.exclude, info.Name()) {
						return filepath.SkipDir
					}
					return nil
				}

				// Check if file has valid extension
				ext := strings.ToLower(filepath.Ext(filePath))
				if validExtensions[ext] {
					realPath, err := filepath.EvalSymlinks(filePath)
					if err != nil {
						realPath = filePath
					}

					if !seen[realPath] {
						seen[realPath] = true
						collected = append(collected, realPath)
					}
				}

				return nil
			})
			if err != nil {
				continue
			}
		} else {
			// Single file
			ext := strings.ToLower(filepath.Ext(absPath))
			if validExtensions[ext] {
				realPath, err := filepath.EvalSymlinks(absPath)
				if err != nil {
					realPath = absPath
				}

				if !seen[realPath] {
					seen[realPath] = true
					collected = append(collected, realPath)
				}
			}
		}
	}

	r.files = collected
}

// loadJSONFile loads JSON content from a file
func (r *GtsFileReader) loadJSONFile(filePath string) (any, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var content any
	if err := json.Unmarshal(data, &content); err != nil {
		return nil, err
	}

	return content, nil
}

// processFile processes a single JSON file and returns list of JsonEntity objects
func (r *GtsFileReader) processFile(filePath string) []*JsonEntity {
	var entities []*JsonEntity

	content, err := r.loadJSONFile(filePath)
	if err != nil {
		return entities
	}

	jsonFile := &JsonFile{
		Path:    filePath,
		Name:    filepath.Base(filePath),
		Content: content,
	}

	// Handle both single objects and arrays
	switch v := content.(type) {
	case []any:
		// Array of items
		for idx, item := range v {
			if itemMap, ok := item.(map[string]any); ok {
				entity := NewJsonEntityWithFile(itemMap, r.cfg, jsonFile, &idx)
				if entity.GtsID != nil {
					entities = append(entities, entity)
				}
			}
		}
	case map[string]any:
		// Single object
		entity := NewJsonEntityWithFile(v, r.cfg, jsonFile, nil)
		if entity.GtsID != nil {
			entities = append(entities, entity)
		}
	}

	return entities
}

// Next returns the next JsonEntity or nil when exhausted
func (r *GtsFileReader) Next() *JsonEntity {
	if !r.initialized {
		r.collectFiles()
		r.initialized = true
	}

	// If we have entities from current file, return next one
	if r.currentEntityIndex < len(r.currentFileEntities) {
		entity := r.currentFileEntities[r.currentEntityIndex]
		r.currentEntityIndex++
		return entity
	}

	// Move to next file
	for r.currentIndex < len(r.files) {
		r.currentFileEntities = r.processFile(r.files[r.currentIndex])
		r.currentIndex++
		r.currentEntityIndex = 0

		if len(r.currentFileEntities) > 0 {
			entity := r.currentFileEntities[r.currentEntityIndex]
			r.currentEntityIndex++
			return entity
		}
	}

	return nil
}

// ReadByID reads a JsonEntity by its ID
// For FileReader, this returns nil as we don't support random access by ID
func (r *GtsFileReader) ReadByID(entityID string) *JsonEntity {
	return nil
}

// Reset resets the iterator to start from the beginning
func (r *GtsFileReader) Reset() {
	r.currentIndex = 0
	r.currentFileEntities = nil
	r.currentEntityIndex = 0
	r.initialized = false
}
