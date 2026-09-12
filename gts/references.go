/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"strings"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
)

// GtsReference represents a GTS ID reference found in JSON content
type GtsReference struct {
	ID         string
	SourcePath string
}

// extractGtsReferences walks through JSON content and extracts all GTS ID references
// see gts-python _extract_gts_ids_with_paths method
func extractGtsReferences(content any) []*GtsReference {
	refs := make([]*GtsReference, 0)
	seen := make(map[string]bool)

	walkAndCollectRefs(content, "", &refs, seen)
	return refs
}

// walkAndCollectRefs recursively walks JSON structure to find GTS IDs
func walkAndCollectRefs(node any, path string, refs *[]*GtsReference, seen map[string]bool) {
	if node == nil {
		return
	}

	// Check if current node is a GTS ID string
	if str, ok := node.(string); ok {
		// Strip the gts:// URI prefix (used in JSON Schema $id/$ref) before
		// validating, matching gts-python _extract_gts_ids_with_paths.
		id := strings.TrimPrefix(str, gtsid.URIPrefix)
		if gtsid.IsValid(id) {
			sourcePath := path
			if sourcePath == "" {
				sourcePath = "root"
			}
			key := id + "|" + sourcePath
			if !seen[key] {
				*refs = append(*refs, &GtsReference{
					ID:         id,
					SourcePath: sourcePath,
				})
				seen[key] = true
			}
		}
		return
	}

	// Recurse into map
	if m, ok := node.(map[string]any); ok {
		for k, v := range m {
			nextPath := k
			if path != "" {
				nextPath = path + "." + k
			}
			walkAndCollectRefs(v, nextPath, refs, seen)
		}
		return
	}

	// Recurse into slice
	if arr, ok := node.([]any); ok {
		for i, v := range arr {
			nextPath := fmt.Sprintf("[%d]", i)
			if path != "" {
				nextPath = path + nextPath
			}
			walkAndCollectRefs(v, nextPath, refs, seen)
		}
	}
}
