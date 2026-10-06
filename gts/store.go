/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/GlobalTypeSystem/gts-go/gtsid"
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

// EntityConflictError is returned when an entity is already registered with
// different content and entity updates are not allowed. Callers can surface this
// as an HTTP 409 Conflict.
type EntityConflictError struct {
	EntityID string
}

func (e *EntityConflictError) Error() string {
	return fmt.Sprintf("Entity '%s' is already registered with different content", e.EntityID)
}

// contentHash returns a stable SHA-256 hash of an entity's content. encoding/json
// marshals map keys in sorted order (recursively), so the serialization is
// canonical and two entities with equal content hash to the same value. This is
// used to distinguish an idempotent re-submission from a conflicting update
// without a deep structural comparison.
func validateJSONContent(content map[string]any) error {
	_, err := json.Marshal(content)
	return err
}

func contentHash(content map[string]any) string {
	b, err := json.Marshal(content)
	if err != nil {
		// Non-serializable content should never occur for JSON-sourced entities;
		// treat it as unique so the registration is conservatively rejected.
		return fmt.Sprintf("unserializable:%v", content)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func cloneJsonEntity(entity *JsonEntity) *JsonEntity {
	if entity == nil {
		return nil
	}
	clone := *entity
	if entity.Content != nil {
		clone.Content = deepCopyMap(entity.Content)
	}
	if entity.GtsID != nil {
		clone.GtsID, _ = gtsid.New(entity.GtsID.ID)
	}
	clone.GtsRefs = make([]*GtsReference, len(entity.GtsRefs))
	for i, ref := range entity.GtsRefs {
		if ref != nil {
			copied := *ref
			clone.GtsRefs[i] = &copied
		}
	}
	if entity.File != nil {
		copied := *entity.File
		copied.Content = deepCopyValue(entity.File.Content)
		clone.File = &copied
	}
	if entity.ListSequence != nil {
		sequence := *entity.ListSequence
		clone.ListSequence = &sequence
	}
	return &clone
}

// RegistryConfig configures the GtsStore behavior
type RegistryConfig struct {
	// ValidateGtsReferences enables validation of GTS references on entity registration
	ValidateGtsReferences bool
	// Verbose enables debug logging of store operations. Off by default.
	Verbose bool
	// AllowEntityUpdates permits re-registering an entity with different content.
	// When false (default), changing the content of an already-registered entity
	// is rejected with an EntityConflictError while identical re-submissions stay
	// idempotent.
	AllowEntityUpdates bool
}

// DefaultRegistryConfig returns the default registry configuration
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		ValidateGtsReferences: false,
		Verbose:               false,
		AllowEntityUpdates:    false,
	}
}

// GtsStore manages a collection of JSON entities and schemas with optional GTS reference validation.
//
// writerMu serializes mutations and registration transactions. mu protects byID;
// validation callbacks run without mu while writerMu prevents another writer from
// interleaving with snapshot/register/validate/rollback. readerMu serializes
// on-demand reads from the backing reader; it is intentionally distinct from
// writerMu so that a validation callback running inside a writerMu-held
// transaction can still fault entities in via Get without self-deadlocking.
//
// schemaCache memoizes compiled JSON schemas (see validate.go). It is cleared
// whenever byID changes so a cached schema is never reused across a mutation
// that could alter it or any schema it resolves. schemaCacheGen guards the
// cache against an invalidation that races an in-flight compile: every entry is
// tagged with the generation observed when its compile began, and a read reuses
// it only while that generation is still current (see compileSchema).
type GtsStore struct {
	writerMu sync.Mutex
	readerMu sync.Mutex
	mu       sync.RWMutex
	byID     map[string]*JsonEntity
	// staged maps a unique staging TOKEN to its entry (registry key + entity +
	// staging session). Keying by token (not by entity key) means two entries
	// that resolve to the same id - a duplicated batch entry or two concurrent
	// batches - never clobber each other, and Commit/Discard only affect the
	// entry they name.
	staged map[string]*stagedEntry
	// stagedByKey is the by-key overlay a SESSION-SCOPED read consults so a batch
	// resolves its OWN not-yet-committed siblings - and only its own. It is keyed
	// session -> key -> most-recently-staged entity, so one request's unvalidated
	// staged entries never leak into another request's validation. Recomputed
	// when a token is committed or discarded.
	stagedByKey    map[string]map[string]*JsonEntity
	stageSeq       atomic.Uint64
	reader         GtsReader
	config         *RegistryConfig
	schemaCache    sync.Map
	schemaCacheGen atomic.Uint64
}

// stagedEntry is one staged registration: the registry key it will publish
// under, the (already cloned) entity content, and the staging session it
// belongs to (so it is visible only to reads scoped to that same session).
type stagedEntry struct {
	key     string
	entity  *JsonEntity
	session string
}

// invalidateSchemaCache drops every memoized compiled schema and advances the
// cache generation. Bumping the generation is what makes invalidation safe
// against a concurrent compile: an entry produced from now-stale dependencies
// carries an older generation and is rejected on read even if its Store lands
// after this Clear. Callers hold no particular lock; sync.Map and atomic.Uint64
// are safe for concurrent use.
func (s *GtsStore) invalidateSchemaCache() {
	// Advance the generation before clearing so a compile that reads the new
	// generation cannot also observe a not-yet-cleared entry.
	s.schemaCacheGen.Add(1)
	s.schemaCache.Clear()
}

// forEachEntity invokes fn for every entity while holding the read lock. fn MUST
// NOT call back into the store (Get/Register/…): mu is held for reading and
// re-entrant locking can deadlock. Return false from fn to stop iteration.
func (s *GtsStore) forEachEntity(fn func(id string, entity *JsonEntity) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id, entity := range s.byID {
		if !fn(id, entity) {
			return
		}
	}
}

// entityIDs returns a snapshot of every registered entity ID. Unlike Items it
// does not clone entity content, so it is cheap for callers that only need the
// keys (e.g. wildcard matching) and then look up the few that matter. Because
// the lock is released before the caller iterates, the returned IDs are safe to
// pass back into the store (Get/…) — unlike forEachEntity, whose callback runs
// under the read lock.
func (s *GtsStore) entityIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.byID))
	for id := range s.byID {
		ids = append(ids, id)
	}
	return ids
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

	configCopy := *config
	store := &GtsStore{
		byID:        make(map[string]*JsonEntity),
		staged:      make(map[string]*stagedEntry),
		stagedByKey: make(map[string]map[string]*JsonEntity),
		reader:      reader,
		config:      &configCopy,
	}

	// Populate from reader if provided
	if reader != nil {
		store.populateFromReader()
	}

	if config.Verbose {
		log.Printf("Created GtsStore with %d entities (validation: %v)", len(store.byID), config.ValidateGtsReferences)
	}
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
		if key := entity.EffectiveID(); key != "" {
			if err := validateJSONContent(entity.Content); err != nil {
				if s.config.Verbose {
					log.Printf("Skipping entity %s with invalid JSON content: %v", key, err)
				}
				continue
			}
			s.byID[key] = cloneJsonEntity(entity)
		}
	}
}

// Register adds a JsonEntity to the store with optional GTS reference validation.
func (s *GtsStore) Register(entity *JsonEntity) error {
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	return s.registerLocked(entity)
}

// registerLocked is the writer-serialized body of Register. Callers must hold writerMu.
func (s *GtsStore) registerLocked(entity *JsonEntity) error {
	if entity == nil {
		return fmt.Errorf("entity must not be nil")
	}
	key := entity.EffectiveID()
	if key == "" {
		return fmt.Errorf("entity must have a gts_id or a non-empty id field")
	}
	if err := validateJSONContent(entity.Content); err != nil {
		return fmt.Errorf("entity content must be valid JSON: %w", err)
	}
	entity = cloneJsonEntity(entity)

	s.mu.RLock()
	previous, exists := s.byID[key]
	s.mu.RUnlock()
	if exists && !s.config.AllowEntityUpdates && contentHash(previous.Content) != contentHash(entity.Content) {
		return &EntityConflictError{EntityID: key}
	}

	if s.config.ValidateGtsReferences {
		if err := s.validateEntityGtsReferences(entity); err != nil {
			return fmt.Errorf("GTS reference validation failed for entity %s: %w", key, err)
		}
	}

	s.mu.Lock()
	s.byID[key] = entity
	s.mu.Unlock()
	s.invalidateSchemaCache()
	if s.config.Verbose {
		log.Printf("Registered entity: %s (type-schema: %v, refs: %d)", key, entity.IsTypeSchema, len(entity.GtsRefs))
	}
	return nil
}

// RegisterWithValidation registers entity, runs validate, and rolls back on failure.
// writerMu prevents concurrent writers from interleaving with the transaction while
// validation performs synchronized reads through the regular store API. The callback
// must not call writer methods.
func (s *GtsStore) RegisterWithValidation(entity *JsonEntity, validate func(id string) error) error {
	if entity == nil {
		return fmt.Errorf("entity must not be nil")
	}
	s.writerMu.Lock()
	defer s.writerMu.Unlock()

	key := entity.EffectiveID()
	s.mu.RLock()
	previous, hadPrevious := s.byID[key]
	s.mu.RUnlock()
	if err := s.registerLocked(entity); err != nil {
		return err
	}

	if err := validate(key); err != nil {
		s.mu.Lock()
		if hadPrevious {
			s.byID[key] = previous
		} else {
			delete(s.byID, key)
		}
		s.mu.Unlock()
		s.invalidateSchemaCache()
		return err
	}
	return nil
}

// RegisterSchema registers a schema with the given type ID
// This is a legacy method for backward compatibility
func (s *GtsStore) RegisterSchema(typeID string, schema map[string]any) error {
	if !gtsid.IsTypeID(typeID) {
		return fmt.Errorf("schema type_id must end with '~'")
	}
	if err := validateJSONContent(schema); err != nil {
		return fmt.Errorf("schema content must be valid JSON: %w", err)
	}

	dialect, ok := schema["$schema"].(string)
	if !ok || strings.TrimSpace(dialect) == "" {
		return fmt.Errorf("GTS Type Schema must contain a top-level $schema field")
	}
	embeddedID, ok := schema["$id"].(string)
	if !ok || !strings.HasPrefix(embeddedID, gtsid.URIPrefix+gtsid.Prefix) {
		return fmt.Errorf("GTS Type Schema must contain a top-level $id in gts:// form")
	}
	normalizedID := gtsid.NormalizeID(embeddedID)
	if !gtsid.IsValid(normalizedID) || !gtsid.IsTypeID(normalizedID) {
		return fmt.Errorf("invalid GTS Type Schema $id: %q", embeddedID)
	}
	if normalizedID != typeID {
		return fmt.Errorf("embedded $id %q must match external type_id %q", embeddedID, typeID)
	}

	// Parse to validate
	gtsID, err := gtsid.New(typeID)
	if err != nil {
		return err
	}

	entity := &JsonEntity{
		GtsID:        gtsID,
		Content:      deepCopyMap(schema),
		IsTypeSchema: true,
	}

	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.byID[typeID]; ok && !s.config.AllowEntityUpdates &&
		contentHash(previous.Content) != contentHash(schema) {
		return &EntityConflictError{EntityID: typeID}
	}
	s.byID[typeID] = entity
	s.invalidateSchemaCache()
	return nil
}

// Get retrieves a JsonEntity by its ID for INTERNAL use (validation, $ref and
// chain resolution) using the COMMITTED store only. It never consults the
// staging overlay, so one request's not-yet-validated staged entries can never
// affect another's resolution. A validation that must resolve its own batch's
// staged siblings uses getScoped with its staging session. Public read
// endpoints use GetCommitted so uncommitted entities are never exposed.
func (s *GtsStore) Get(entityID string) *JsonEntity {
	return s.getScoped(entityID, "")
}

// getScoped resolves entityID against the committed store overlaid with ONLY the
// staged entries of the given staging session. An empty session means
// committed-only (identical to a public read), so an entry staged by another
// request is invisible here.
func (s *GtsStore) getScoped(entityID string, session string) *JsonEntity {
	if session != "" {
		s.mu.RLock()
		if overlay := s.stagedByKey[session]; overlay != nil {
			if staged := overlay[entityID]; staged != nil {
				s.mu.RUnlock()
				return cloneJsonEntity(staged)
			}
		}
		s.mu.RUnlock()
	}
	return s.GetCommitted(entityID)
}

// GetCommitted retrieves a committed JsonEntity by its ID, never consulting the
// staging overlay. This is the read path for public/API consumers: a
// staged-but-not-yet-committed entity is invisible here, so clients never
// observe an entity that has not passed validation.
// If not found in cache, attempts to fetch from reader.
func (s *GtsStore) GetCommitted(entityID string) *JsonEntity {
	s.mu.RLock()
	entity := s.byID[entityID]
	s.mu.RUnlock()
	if entity != nil {
		return cloneJsonEntity(entity)
	}

	if s.reader == nil {
		return nil
	}

	// readerMu (not writerMu) serializes on-demand reader faults. Using a
	// dedicated lock keeps Get callable from validation callbacks that already
	// hold writerMu, avoiding a self-deadlock.
	s.readerMu.Lock()
	defer s.readerMu.Unlock()
	s.mu.RLock()
	entity = s.byID[entityID]
	s.mu.RUnlock()
	if entity != nil {
		return cloneJsonEntity(entity)
	}

	entity = s.reader.ReadByID(entityID)
	if entity == nil {
		return nil
	}
	stored := cloneJsonEntity(entity)
	s.mu.Lock()
	s.byID[entityID] = stored
	s.mu.Unlock()
	// Only a newly-visible type schema can affect a compiled result; faulting in
	// an instance cannot, so it must not flush every memoized schema.
	if stored.IsTypeSchema {
		s.invalidateSchemaCache()
	}
	return cloneJsonEntity(stored)
}

// GetSchemaContent retrieves schema content as a map (legacy method)
func (s *GtsStore) GetSchemaContent(typeID string) (map[string]any, error) {
	entity := s.Get(typeID)
	if entity == nil {
		return nil, fmt.Errorf("schema not found: %s", typeID)
	}
	if !entity.IsTypeSchema {
		return nil, fmt.Errorf("entity is not a type-schema: %s", typeID)
	}
	return entity.Content, nil
}

// Items returns all entity ID and entity pairs
func (s *GtsStore) Items() map[string]*JsonEntity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make(map[string]*JsonEntity, len(s.byID))
	for id, entity := range s.byID {
		items[id] = cloneJsonEntity(entity)
	}
	return items
}

// Unregister removes an entity from the store by its effective ID.
// Used to roll back registrations that fail post-register validation.
func (s *GtsStore) Unregister(entityID string) {
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	s.mu.Lock()
	delete(s.byID, entityID)
	s.mu.Unlock()
	s.invalidateSchemaCache()
}

// Stage places an entity into the staging overlay WITHOUT publishing it. A
// staged entity is visible to internal validation (via Get) so a batch can
// resolve intra-batch references regardless of entry order, but it is invisible
// to public reads (GetCommitted / List / Query) until Commit. This is how a
// validate=true registration avoids ever exposing an entity that has not yet
// passed validation, without holding a store-wide write lock across the
// (potentially slow) validation. It returns a unique staging token identifying
// this entry, or an EntityConflictError when a committed entity with different
// content already holds the id and updates are not allowed. Callers MUST
// eventually Commit or Discard the returned token. The entry is associated with
// the given staging session (a batch id); pass "" for a private per-token
// session that is visible to no snapshot.
func (s *GtsStore) Stage(entity *JsonEntity, session string) (string, error) {
	return s.stage(entity, session, s.config.AllowEntityUpdates)
}

// stage also supports transient validation of a candidate under an existing ID.
// Such candidates may shadow committed content, but must always be discarded.
// Commit independently enforces the registry's update policy.
func (s *GtsStore) stage(entity *JsonEntity, session string, allowUpdates bool) (string, error) {
	if entity == nil {
		return "", fmt.Errorf("entity must not be nil")
	}
	key := entity.EffectiveID()
	if key == "" {
		return "", fmt.Errorf("entity must have a gts_id or a non-empty id field")
	}
	if err := validateJSONContent(entity.Content); err != nil {
		return "", fmt.Errorf("entity content must be valid JSON: %w", err)
	}
	staged := cloneJsonEntity(entity)
	token := "stg-" + strconv.FormatUint(s.stageSeq.Add(1), 10)
	if session == "" {
		session = token
	}

	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	s.mu.Lock()
	previous, exists := s.byID[key]
	s.mu.Unlock()
	if exists && !allowUpdates && contentHash(previous.Content) != contentHash(entity.Content) {
		return "", &EntityConflictError{EntityID: key}
	}

	s.mu.Lock()
	s.staged[token] = &stagedEntry{key: key, entity: staged, session: session}
	s.setStagedByKeyLocked(session, key, staged)
	s.mu.Unlock()
	// A staged type schema changes what a validating sibling resolves, so any
	// memoized compilation that predates it must be discarded.
	s.invalidateSchemaCache()
	return token, nil
}

// NewStagingSession returns a unique staging-session id. Stage entries created
// with the same session id share one overlay visible only to reads scoped to
// that session, so a validate=true batch resolves its own siblings while another
// request's staged entries stay invisible.
func (s *GtsStore) NewStagingSession() string {
	return "ses-" + strconv.FormatUint(s.stageSeq.Add(1), 10)
}

// setStagedByKeyLocked points session's overlay at entity for key. Callers must
// hold mu for writing.
func (s *GtsStore) setStagedByKeyLocked(session, key string, entity *JsonEntity) {
	overlay := s.stagedByKey[session]
	if overlay == nil {
		overlay = make(map[string]*JsonEntity)
		s.stagedByKey[session] = overlay
	}
	overlay[key] = entity
}

// removeStagedLocked drops the staged entry named by token and rebuilds its
// session's by-key overlay for that key from any other tokens that still target
// it within the same session. Callers must hold mu for writing.
func (s *GtsStore) removeStagedLocked(token string, entry *stagedEntry) {
	delete(s.staged, token)
	overlay := s.stagedByKey[entry.session]
	if overlay == nil {
		return
	}
	delete(overlay, entry.key)
	for _, other := range s.staged {
		if other.session == entry.session && other.key == entry.key {
			overlay[entry.key] = other.entity
		}
	}
	if len(overlay) == 0 {
		delete(s.stagedByKey, entry.session)
	}
}

// Commit publishes the entity named by token, making it visible to public
// reads, and clears it from the staging overlay. The publish is atomic with a
// conflict check against the committed store (the same compare-and-swap a
// direct save uses): it returns an EntityConflictError when a different entity
// already holds the id and updates are not allowed, so a concurrent commit of
// the same id, or an intra-batch duplicate, never silently overwrites committed
// content. An unknown/already-resolved token also yields a conflict.
func (s *GtsStore) Commit(token string) error {
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.staged[token]
	if !ok {
		return &EntityConflictError{EntityID: token}
	}
	s.removeStagedLocked(token, entry)
	if existing, exists := s.byID[entry.key]; exists &&
		!s.config.AllowEntityUpdates && contentHash(existing.Content) != contentHash(entry.entity.Content) {
		return &EntityConflictError{EntityID: entry.key}
	}
	s.byID[entry.key] = entry.entity
	s.invalidateSchemaCache()
	return nil
}

// CommitBatch atomically publishes a whole set of staged tokens as one
// all-or-nothing unit. Every token's target is checked against the committed
// store (and against its batch siblings) under a single lock; if ANY would
// conflict - a concurrent commit won the id with different content, an
// intra-batch duplicate disagrees, or a token is unknown - NOTHING is published.
// This closes the gap where a per-entry commit loop could publish a dependent
// schema against a target whose content changed after the dependent was
// validated. Returns one error per token, positionally aligned with tokens (nil
// on success); on a batch conflict every entry reports an EntityConflictError
// and the tokens remain staged for the caller to discard.
func (s *GtsStore) CommitBatch(tokens []string) []error {
	errs := make([]error, len(tokens))
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()

	entries := make([]*stagedEntry, len(tokens))
	pendingByKey := make(map[string]*JsonEntity)
	anyConflict := false
	for i, token := range tokens {
		entry, ok := s.staged[token]
		if !ok {
			errs[i] = &EntityConflictError{EntityID: token}
			anyConflict = true
			continue
		}
		entries[i] = entry
		existing, exists := pendingByKey[entry.key]
		if !exists {
			existing, exists = s.byID[entry.key]
		}
		if exists && !s.config.AllowEntityUpdates && contentHash(existing.Content) != contentHash(entry.entity.Content) {
			errs[i] = &EntityConflictError{EntityID: entry.key}
			anyConflict = true
		}
		pendingByKey[entry.key] = entry.entity
	}

	// All-or-nothing: if any target conflicts, publish none and report every
	// entry as a conflict. The tokens stay staged for the caller to discard.
	if anyConflict {
		for i := range tokens {
			if errs[i] == nil {
				errs[i] = &EntityConflictError{EntityID: entries[i].key}
			}
		}
		return errs
	}

	for i, token := range tokens {
		entry := entries[i]
		s.removeStagedLocked(token, entry)
		s.byID[entry.key] = entry.entity
	}
	s.invalidateSchemaCache()
	return errs
}

// Discard drops the staged entity named by token that failed validation. The
// committed state is untouched, so a client never observes the discarded
// (invalid) entity and any prior committed version under the same id is
// preserved.
func (s *GtsStore) Discard(token string) {
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	s.mu.Lock()
	if entry, ok := s.staged[token]; ok {
		s.removeStagedLocked(token, entry)
	}
	s.mu.Unlock()
	s.invalidateSchemaCache()
}

// Count returns the number of entities in the store
func (s *GtsStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byID)
}

// EntityInfo represents basic information about an entity
type EntityInfo struct {
	ID           string `json:"id"`
	TypeID       string `json:"type_id"`
	IsTypeSchema bool   `json:"is_type_schema"`
}

// ListResult represents the result of listing entities
type ListResult struct {
	Entities []EntityInfo `json:"entities"`
	Count    int          `json:"count"`
	Total    int          `json:"total"`
}

// List returns a list of entities up to the specified limit
func (s *GtsStore) List(limit int) *ListResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := len(s.byID)
	entities := []EntityInfo{}

	count := 0
	for id, entity := range s.byID {
		if count >= limit {
			break
		}
		entities = append(entities, EntityInfo{
			ID:           id,
			TypeID:       entity.TypeID,
			IsTypeSchema: entity.IsTypeSchema,
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
		if entity.GtsID != nil && ref.ID == entity.GtsID.ID {
			// Skip self-references
			continue
		}

		// Skip JSON Schema meta-schema references
		if strings.HasPrefix(ref.ID, "http://json-schema.org") ||
			strings.HasPrefix(ref.ID, "https://json-schema.org") {
			continue
		}

		// Skip wildcard patterns — they are x-gts-ref constraints, not concrete entity IDs
		if gtsid.HasWildcard(ref.ID) {
			continue
		}

		// Check if the referenced entity exists in the store
		referencedEntity := s.Get(ref.ID)
		if referencedEntity == nil {
			errors = append(errors, fmt.Sprintf("referenced entity not found: %s (at %s)", ref.ID, ref.SourcePath))
			continue
		}

		// Additional validation for type-schema references
		if entity.IsTypeSchema {
			if strings.Contains(ref.SourcePath, "$ref") {
				// This is a type-schema reference - the referenced entity should be a type-schema
				if !referencedEntity.IsTypeSchema {
					errors = append(errors, fmt.Sprintf("type-schema reference points to non-type-schema entity: %s (at %s)", ref.ID, ref.SourcePath))
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
	if !gtsid.IsTypeID(gtsID) {
		return fmt.Errorf("ID '%s' is not a type-schema ID (must end with '~')", gtsID)
	}

	entity := s.Get(gtsID)
	if entity == nil {
		return &StoreGtsSchemaNotFoundError{EntityID: gtsID}
	}

	if !entity.IsTypeSchema {
		return fmt.Errorf("entity '%s' is not a type-schema", gtsID)
	}

	if s.config.Verbose {
		log.Printf("Validating schema %s", gtsID)
	}

	// Validate JSON Schema meta-schema (basic check)
	if entity.Content == nil {
		return fmt.Errorf("schema content is nil")
	}

	// Validate $ref constraints in the schema
	refValidator := NewRefValidator()
	refErrors := refValidator.ValidateSchemaRefs(entity.Content, "")
	if len(refErrors) > 0 {
		var errorMsgs []string
		for _, err := range refErrors {
			errorMsgs = append(errorMsgs, err.Error())
		}
		return fmt.Errorf("$ref validation failed: %s", strings.Join(errorMsgs, "; "))
	}

	// Validate x-gts-ref constraints in the schema
	xGtsRefValidator := NewXGtsRefValidator(s)
	xGtsRefErrors := xGtsRefValidator.ValidateSchema(entity.Content, "")
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

	if s.config.Verbose {
		log.Printf("Schema %s passed validation", gtsID)
	}
	return nil
}

// ValidateInstanceWithXGtsRef validates an instance against its schema including x-gts-ref constraints
func (s *GtsStore) ValidateInstanceWithXGtsRef(instanceID string) error {
	instance := s.Get(instanceID)
	if instance == nil {
		return &StoreGtsObjectNotFoundError{EntityID: instanceID}
	}

	if instance.IsTypeSchema {
		return fmt.Errorf("entity '%s' is a type-schema, not an instance", instanceID)
	}

	// Get the type-schema for this instance
	if instance.TypeID == "" {
		return &StoreGtsSchemaForInstanceNotFoundError{EntityID: instanceID}
	}

	schema := s.Get(instance.TypeID)
	if schema == nil {
		return &StoreGtsSchemaNotFoundError{EntityID: instance.TypeID}
	}

	if !schema.IsTypeSchema {
		return fmt.Errorf("type-schema entity '%s' is not marked as type-schema", instance.TypeID)
	}

	if s.config.Verbose {
		log.Printf("Validating instance %s against type-schema %s", instanceID, instance.TypeID)
	}

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

	if s.config.Verbose {
		log.Printf("Instance %s passed validation", instanceID)
	}
	return nil
}
