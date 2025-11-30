/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"fmt"
	"log"
	"strings"
)

// StoreGtsObjectNotFoundError is returned when a GTS entity is not found in the store
type StoreGtsObjectNotFoundError struct {
	EntityID string
}

func (e *StoreGtsObjectNotFoundError) Error() string {
	return fmt.Sprintf("JSON object with GTS ID '%s' not found in store", e.EntityID)
}

// StoreGtsSchemaNotFoundError is returned when a GTS schema is not found in the store
type StoreGtsSchemaNotFoundError struct {
	EntityID string
}

func (e *StoreGtsSchemaNotFoundError) Error() string {
	return fmt.Sprintf("JSON schema with GTS ID '%s' not found in store", e.EntityID)
}

// StoreGtsSchemaForInstanceNotFoundError is returned when a schema ID cannot be determined for an instance
type StoreGtsSchemaForInstanceNotFoundError struct {
	EntityID string
}

func (e *StoreGtsSchemaForInstanceNotFoundError) Error() string {
	return fmt.Sprintf("Can't determine JSON schema ID for instance with GTS ID '%s'", e.EntityID)
}

// StoreGtsCastFromSchemaNotAllowedError is returned when attempting to cast from a schema ID
type StoreGtsCastFromSchemaNotAllowedError struct {
	FromID string
}

func (e *StoreGtsCastFromSchemaNotAllowedError) Error() string {
	return fmt.Sprintf("Cannot cast from schema ID '%s'. The from_id must be an instance (not ending with '~').", e.FromID)
}

// RegistryConfig configures the GtsStore behavior
type RegistryConfig struct {
	// ValidateGtsReferences enables validation of GTS references on entity registration
	ValidateGtsReferences bool
}

// DefaultRegistryConfig returns the default registry configuration
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		ValidateGtsReferences: false,
	}
}

// GtsStore manages a collection of JSON entities and schemas with optional GTS reference validation
type GtsStore struct {
	byID   map[string]*JsonEntity
	reader GtsReader
	config *RegistryConfig
}

// NewGtsStore creates a new GtsStore, optionally populating it from a reader
func NewGtsStore(reader GtsReader) *GtsStore {
	return NewGtsStoreWithConfig(reader, DefaultRegistryConfig())
}

// NewGtsStoreWithConfig creates a new GtsStore with custom configuration
func NewGtsStoreWithConfig(reader GtsReader, config *RegistryConfig) *GtsStore {
	if config == nil {
		config = DefaultRegistryConfig()
	}

	store := &GtsStore{
		byID:   make(map[string]*JsonEntity),
		reader: reader,
		config: config,
	}

	// Populate from reader if provided
	if reader != nil {
		store.populateFromReader()
	}

	log.Printf("Created GtsStore with %d entities (validation: %v)", len(store.byID), config.ValidateGtsReferences)
	return store
}

// populateFromReader loads all entities from the reader into the store
func (s *GtsStore) populateFromReader() {
	if s.reader == nil {
		return
	}

	for {
		entity := s.reader.Next()
		if entity == nil {
			break
		}
		if entity.GtsID != nil && entity.GtsID.ID != "" {
			s.byID[entity.GtsID.ID] = entity
		}
	}
}

// Register adds a JsonEntity to the store with optional GTS reference validation
func (s *GtsStore) Register(entity *JsonEntity) error {
	if entity.GtsID == nil || entity.GtsID.ID == "" {
		return fmt.Errorf("entity must have a valid gts_id")
	}

	// Perform validation if enabled
	if s.config.ValidateGtsReferences {
		if err := s.validateEntityGtsReferences(entity); err != nil {
			return fmt.Errorf("GTS reference validation failed for entity %s: %w", entity.GtsID.ID, err)
		}
	}

	s.byID[entity.GtsID.ID] = entity
	log.Printf("Registered entity: %s (schema: %v, refs: %d)", entity.GtsID.ID, entity.IsSchema, len(entity.GtsRefs))
	return nil
}

// RegisterSchema registers a schema with the given type ID
// This is a legacy method for backward compatibility
func (s *GtsStore) RegisterSchema(typeID string, schema map[string]any) error {
	if typeID[len(typeID)-1] != '~' {
		return fmt.Errorf("schema type_id must end with '~'")
	}

	// Parse to validate
	gtsID, err := NewGtsID(typeID)
	if err != nil {
		return err
	}

	entity := &JsonEntity{
		GtsID:    gtsID,
		Content:  schema,
		IsSchema: true,
	}

	s.byID[typeID] = entity
	return nil
}

// Get retrieves a JsonEntity by its ID
// If not found in cache, attempts to fetch from reader
func (s *GtsStore) Get(entityID string) *JsonEntity {
	// Check cache first
	if entity, ok := s.byID[entityID]; ok {
		return entity
	}

	// Try to fetch from reader
	if s.reader != nil {
		entity := s.reader.ReadByID(entityID)
		if entity != nil {
			s.byID[entityID] = entity
			return entity
		}
	}

	return nil
}

// GetSchemaContent retrieves schema content as a map (legacy method)
func (s *GtsStore) GetSchemaContent(typeID string) (map[string]any, error) {
	entity := s.Get(typeID)
	if entity == nil {
		return nil, fmt.Errorf("schema not found: %s", typeID)
	}
	if !entity.IsSchema {
		return nil, fmt.Errorf("entity is not a schema: %s", typeID)
	}
	return entity.Content, nil
}

// Items returns all entity ID and entity pairs
func (s *GtsStore) Items() map[string]*JsonEntity {
	return s.byID
}

// Count returns the number of entities in the store
func (s *GtsStore) Count() int {
	return len(s.byID)
}

// EntityInfo represents basic information about an entity
type EntityInfo struct {
	ID       string `json:"id"`
	SchemaID string `json:"schema_id"`
	IsSchema bool   `json:"is_schema"`
}

// ListResult represents the result of listing entities
type ListResult struct {
	Entities []EntityInfo `json:"entities"`
	Count    int          `json:"count"`
	Total    int          `json:"total"`
}

// List returns a list of entities up to the specified limit
func (s *GtsStore) List(limit int) *ListResult {
	total := len(s.byID)
	entities := []EntityInfo{}

	count := 0
	for id, entity := range s.byID {
		if count >= limit {
			break
		}
		entities = append(entities, EntityInfo{
			ID:       id,
			SchemaID: entity.SchemaID,
			IsSchema: entity.IsSchema,
		})
		count++
	}

	return &ListResult{
		Entities: entities,
		Count:    count,
		Total:    total,
	}
}

// validateEntityGtsReferences validates all GTS references in an entity
func (s *GtsStore) validateEntityGtsReferences(entity *JsonEntity) error {
	if entity == nil || len(entity.GtsRefs) == 0 {
		return nil
	}

	var errors []string

	for _, ref := range entity.GtsRefs {
		if ref.ID == entity.GtsID.ID {
			// Skip self-references
			continue
		}

		// Skip JSON Schema meta-schema references
		if strings.HasPrefix(ref.ID, "http://json-schema.org") ||
			strings.HasPrefix(ref.ID, "https://json-schema.org") {
			continue
		}

		// Check if the referenced entity exists in the store
		referencedEntity := s.Get(ref.ID)
		if referencedEntity == nil {
			errors = append(errors, fmt.Sprintf("referenced entity not found: %s (at %s)", ref.ID, ref.SourcePath))
			continue
		}

		// Additional validation for schema references
		if entity.IsSchema {
			if strings.Contains(ref.SourcePath, "$ref") {
				// This is a schema reference - the referenced entity should be a schema
				if !referencedEntity.IsSchema {
					errors = append(errors, fmt.Sprintf("schema reference points to non-schema entity: %s (at %s)", ref.ID, ref.SourcePath))
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("GTS reference validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// ValidateSchema validates a schema including JSON Schema meta-schema and GTS reference validation
func (s *GtsStore) ValidateSchema(gtsID string) error {
	if !strings.HasSuffix(gtsID, "~") {
		return fmt.Errorf("ID '%s' is not a schema (must end with '~')", gtsID)
	}

	entity := s.Get(gtsID)
	if entity == nil {
		return &StoreGtsSchemaNotFoundError{EntityID: gtsID}
	}

	if !entity.IsSchema {
		return fmt.Errorf("entity '%s' is not a schema", gtsID)
	}

	log.Printf("Validating schema %s", gtsID)

	// Validate JSON Schema meta-schema (basic check)
	if entity.Content == nil {
		return fmt.Errorf("schema content is nil")
	}

	// Validate x-gts-ref constraints in the schema
	xGtsRefValidator := NewXGtsRefValidator(s)
	xGtsRefErrors := xGtsRefValidator.ValidateSchema(entity.Content, "", nil)
	if len(xGtsRefErrors) > 0 {
		var errorMsgs []string
		for _, err := range xGtsRefErrors {
			errorMsgs = append(errorMsgs, err.Error())
		}
		return fmt.Errorf("x-gts-ref validation failed: %s", strings.Join(errorMsgs, "; "))
	}

	// Validate GTS references in the schema
	if err := s.validateEntityGtsReferences(entity); err != nil {
		return fmt.Errorf("schema GTS reference validation failed: %w", err)
	}

	log.Printf("Schema %s passed validation", gtsID)
	return nil
}

// ValidateInstanceWithXGtsRef validates an instance against its schema including x-gts-ref constraints
func (s *GtsStore) ValidateInstanceWithXGtsRef(instanceID string) error {
	instance := s.Get(instanceID)
	if instance == nil {
		return &StoreGtsObjectNotFoundError{EntityID: instanceID}
	}

	if instance.IsSchema {
		return fmt.Errorf("entity '%s' is a schema, not an instance", instanceID)
	}

	// Get the schema for this instance
	if instance.SchemaID == "" {
		return &StoreGtsSchemaForInstanceNotFoundError{EntityID: instanceID}
	}

	schema := s.Get(instance.SchemaID)
	if schema == nil {
		return &StoreGtsSchemaNotFoundError{EntityID: instance.SchemaID}
	}

	if !schema.IsSchema {
		return fmt.Errorf("schema entity '%s' is not marked as schema", instance.SchemaID)
	}

	log.Printf("Validating instance %s against schema %s", instanceID, instance.SchemaID)

	// Validate x-gts-ref constraints
	xGtsRefValidator := NewXGtsRefValidator(s)
	xGtsRefErrors := xGtsRefValidator.ValidateInstance(instance.Content, schema.Content, "")
	if len(xGtsRefErrors) > 0 {
		var errorMsgs []string
		for _, err := range xGtsRefErrors {
			errorMsgs = append(errorMsgs, err.Error())
		}
		return fmt.Errorf("x-gts-ref validation failed: %s", strings.Join(errorMsgs, "; "))
	}

	// Validate GTS references in the instance
	if err := s.validateEntityGtsReferences(instance); err != nil {
		return fmt.Errorf("instance GTS reference validation failed: %w", err)
	}

	log.Printf("Instance %s passed validation", instanceID)
	return nil
}
