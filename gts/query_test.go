/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"testing"
)

// setupQueryTestStore creates a store with test entities
func setupQueryTestStore() *GtsStore {
	store := NewGtsStore(nil)

	// Entity 1
	entity1 := NewJsonEntity(map[string]any{
		"gtsId":    "gts.x.test10.query.event.v1.0~a.b.c.d.v1",
		"type":     "gts.x.test10.query.event.v1.0~",
		"eventId":  "evt-001",
		"status":   "active",
		"category": "order",
	}, DefaultGtsConfig())
	_ = store.Register(entity1)

	// Entity 2
	entity2 := NewJsonEntity(map[string]any{
		"gtsId":    "gts.x.test10.query.event.v1.1~a.b.c.d.v2",
		"type":     "gts.x.test10.query.event.v1.1~",
		"eventId":  "evt-002",
		"status":   "inactive",
		"category": "payment",
	}, DefaultGtsConfig())
	_ = store.Register(entity2)

	// Entity 3
	entity3 := NewJsonEntity(map[string]any{
		"gtsId":    "gts.x.test10.query.event.v2.2~a.b.c.d.v1~a.b.c.d.v2",
		"type":     "gts.x.test10.query.event.v2.2~a.b.c.d.v1~",
		"eventId":  "evt-003",
		"status":   "active",
		"category": "email",
	}, DefaultGtsConfig())
	_ = store.Register(entity3)

	// Entity 4
	entity4 := NewJsonEntity(map[string]any{
		"gtsId":    "gts.x.test10.other_namespace.notification.v1.0~a.b.c.d.v1",
		"type":     "gts.x.test10.other_namespace.notification.v1.0~",
		"eventId":  "evt-003",
		"status":   "some",
		"category": "email",
	}, DefaultGtsConfig())
	_ = store.Register(entity4)

	// Entity 5
	entity5 := NewJsonEntity(map[string]any{
		"gtsId":    "gts.x.test10_2.commerce.order.v2.0~a.b.c.d.v1",
		"type":     "gts.x.test10_2.commerce.order.v2.0~",
		"eventId":  "evt-004",
		"status":   "active",
		"category": "order",
	}, DefaultGtsConfig())
	_ = store.Register(entity5)

	return store
}

// Test 1: Exact match
func TestQuery_ExactMatch(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.query.event.v1.0~a.b.c.d.v1", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 1 {
		t.Errorf("Expected count 1, got: %d", result.Count)
	}

	if len(result.Results) != 1 {
		t.Errorf("Expected 1 result, got: %d", len(result.Results))
	}

	if result.Results[0]["gtsId"] != "gts.x.test10.query.event.v1.0~a.b.c.d.v1" {
		t.Errorf("Expected specific gtsId, got: %v", result.Results[0]["gtsId"])
	}
}

// Test 2: Invalid query (partial GTS ID without wildcard)
func TestQuery_InvalidQuery_PartialID(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.query", 100)

	if result.Error == "" {
		t.Error("Expected error for partial GTS ID without wildcard")
	}

	if !containsString(result.Error, "Invalid query") {
		t.Errorf("Expected 'Invalid query' in error, got: %s", result.Error)
	}
}

// Test 3: Invalid query (missing namespace - double dots)
func TestQuery_InvalidQuery_MissingNamespace(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10..query.v1", 100)

	if result.Error == "" {
		t.Error("Expected error for missing namespace")
	}

	if !containsString(result.Error, "Invalid query") {
		t.Errorf("Expected 'Invalid query' in error, got: %s", result.Error)
	}
}

// Test 4: Invalid query (incomplete version)
func TestQuery_InvalidQuery_IncompleteVersion(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gtsa.x.test10._.query.v", 100)

	if result.Error == "" {
		t.Error("Expected error for incomplete version")
	}

	if !containsString(result.Error, "Invalid query") {
		t.Errorf("Expected 'Invalid query' in error, got: %s", result.Error)
	}
}

// Test 5: Invalid query (no version in instance ID)
func TestQuery_InvalidQuery_NoVersion(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gtsa.x.test10._.query.v1~a.b.c.d", 100)

	if result.Error == "" {
		t.Error("Expected error for no version in instance ID")
	}

	if !containsString(result.Error, "Invalid query") {
		t.Errorf("Expected 'Invalid query' in error, got: %s", result.Error)
	}
}

// Test 6: Wildcard package match
func TestQuery_WildcardPackage(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.*", 50)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 4 {
		t.Errorf("Expected count 4, got: %d", result.Count)
	}

	if result.Limit != 50 {
		t.Errorf("Expected limit 50, got: %d", result.Limit)
	}
}

// Test 7: Wildcard package with limit
func TestQuery_WildcardPackageWithLimit(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.*", 2)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 2 {
		t.Errorf("Expected count 2, got: %d", result.Count)
	}

	if result.Limit != 2 {
		t.Errorf("Expected limit 2, got: %d", result.Limit)
	}
}

// Test 8: Wildcard namespace match
func TestQuery_WildcardNamespace(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.query.*", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 3 {
		t.Errorf("Expected count 3, got: %d", result.Count)
	}
}

// Test 9: Wildcard type match
func TestQuery_WildcardType(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.query.event.*", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 3 {
		t.Errorf("Expected count 3, got: %d", result.Count)
	}
}

// Test 10: Wildcard major version match
func TestQuery_WildcardMajorVersion(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.query.event.v2.*", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 1 {
		t.Errorf("Expected count 1, got: %d", result.Count)
	}
}

// Test 11: Wildcard minor version match
func TestQuery_WildcardMinorVersion(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.query.event.v1.1~*", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 1 {
		t.Errorf("Expected count 1, got: %d", result.Count)
	}
}

// Test 12: Wildcard and filter by attribute
func TestQuery_WildcardAndFilterByAttribute(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.*[status=active]", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 2 {
		t.Errorf("Expected count 2, got: %d", result.Count)
	}
}

// Test 13: Multiple filters
func TestQuery_MultipleFilters(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.*[status=active, category=order]", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 1 {
		t.Errorf("Expected count 1, got: %d", result.Count)
	}
}

// Test 14: Multiple filters with quotes
func TestQuery_MultipleFiltersWithQuotes(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query(`gts.x.test10.*[status="active", category="order"]`, 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 1 {
		t.Errorf("Expected count 1, got: %d", result.Count)
	}
}

// Test 15: Multiple filters with wildcard value
func TestQuery_MultipleFiltersWithWildcard(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.*[status=active, category=*]", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 2 {
		t.Errorf("Expected count 2, got: %d", result.Count)
	}
}

// Test 16: Invalid filter by attribute (filter after tilde)
func TestQuery_InvalidFilterByAttribute(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.*~[status=active]", 100)

	if result.Error == "" {
		t.Error("Expected error for invalid filter syntax")
	}

	if !containsString(result.Error, "Invalid query") {
		t.Errorf("Expected 'Invalid query' in error, got: %s", result.Error)
	}
}

// Test 17: No matches
func TestQuery_NoMatches(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.nonexistent.*", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 0 {
		t.Errorf("Expected count 0, got: %d", result.Count)
	}

	if len(result.Results) != 0 {
		t.Errorf("Expected 0 results, got: %d", len(result.Results))
	}
}

// Test 18: Filter no matches
func TestQuery_FilterNoMatches(t *testing.T) {
	store := setupQueryTestStore()

	result := store.Query("gts.x.test10.*[status=nonexisting, category=order]", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 0 {
		t.Errorf("Expected count 0, got: %d", result.Count)
	}
}

// setupWildcardUseCaseStore creates a store for wildcard use case tests
func setupWildcardUseCaseStore() *GtsStore {
	store := NewGtsStore(nil)

	// Base schema v1.0
	entity1 := NewJsonEntity(map[string]any{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"$id":         "gts.x.test10_llm.chat.message.v1.0~",
		"type":        "object",
		"description": "Base chat message v1.0",
	}, DefaultGtsConfig())
	_ = store.Register(entity1)

	// Derived schema from v1.0
	entity2 := NewJsonEntity(map[string]any{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"$id":         "gts.x.test10_llm.chat.message.v1.0~x.test10_llm._.system_message.v1.0~",
		"type":        "object",
		"description": "System message derived from v1.0",
		"allOf": []any{
			map[string]any{
				"$ref": "gts.x.test10_llm.chat.message.v1.0~",
			},
		},
	}, DefaultGtsConfig())
	_ = store.Register(entity2)

	// Base schema v1.1
	entity3 := NewJsonEntity(map[string]any{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"$id":         "gts.x.test10_llm.chat.message.v1.1~",
		"type":        "object",
		"description": "Base chat message v1.1",
	}, DefaultGtsConfig())
	_ = store.Register(entity3)

	// Derived schema from v1.1
	entity4 := NewJsonEntity(map[string]any{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"$id":         "gts.x.test10_llm.chat.message.v1.1~x.test10_llm._.user_message.v1.1~",
		"type":        "object",
		"description": "User message derived from v1.1",
		"allOf": []any{
			map[string]any{
				"$ref": "gts.x.test10_llm.chat.message.v1.1~",
			},
		},
	}, DefaultGtsConfig())
	_ = store.Register(entity4)

	return store
}

// Test 19: Use Case 1 - Find all derived types from v1.0 base schema
func TestQuery_UseCase1_DerivedFromV10(t *testing.T) {
	store := setupWildcardUseCaseStore()

	result := store.Query("gts.x.test10_llm.chat.message.v1.0~*", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 1 {
		t.Errorf("Expected count 1 (only derived from v1.0), got: %d", result.Count)
	}
}

// Test 20: Use Case 2 - Find all base schemas and derived schemas
func TestQuery_UseCase2_AllVersions(t *testing.T) {
	store := setupWildcardUseCaseStore()

	result := store.Query("gts.x.test10_llm.chat.message.*", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 4 {
		t.Errorf("Expected count 4 (all base and derived), got: %d", result.Count)
	}
}

// Test 21: Use Case 3 - Find all derived types from v1 (any minor)
func TestQuery_UseCase3_DerivedFromV1AnyMinor(t *testing.T) {
	store := setupWildcardUseCaseStore()

	result := store.Query("gts.x.test10_llm.chat.message.v1~*", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 2 {
		t.Errorf("Expected count 2 (derived from v1.0 and v1.1), got: %d", result.Count)
	}
}

// Test 22: Use Case 4 - Find all base and derived from v1 (any minor)
func TestQuery_UseCase4_AllV1BaseAndDerived(t *testing.T) {
	store := setupWildcardUseCaseStore()

	result := store.Query("gts.x.test10_llm.chat.message.v1.*", 100)

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Count != 4 {
		t.Errorf("Expected count 4 (all v1.x base and derived), got: %d", result.Count)
	}
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
