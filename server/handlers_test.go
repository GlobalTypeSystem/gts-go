package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GlobalTypeSystem/gts-go/gts"
)

func canonicalSchema(typeID string, content map[string]any) map[string]any {
	schema := map[string]any{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"$id":     "gts://" + typeID,
	}
	for key, value := range content {
		schema[key] = value
	}
	return schema
}

func TestValidateJSON(t *testing.T) {
	store := gts.NewGtsStore(nil)
	typeID := "gts.x.test6json._.validate_json.v1~"
	if err := store.RegisterSchema(typeID, canonicalSchema(typeID, map[string]any{
		"type":       "object",
		"required":   []any{"name"},
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	})); err != nil {
		t.Fatal(err)
	}
	s := NewServer(store, "", 0, 0)
	for _, test := range []struct {
		name string
		path string
		body string
		want int
	}{
		{"auto instance", "/validate-json", `{"type":"gts.x.test6json._.validate_json.v1~","name":"valid"}`, http.StatusOK},
		{"explicit instance", "/validate-json/gts.x.test6json._.validate_json.v1~", `{"name":"valid"}`, http.StatusOK},
		{"non-object", "/validate-json", `["invalid"]`, http.StatusUnprocessableEntity},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, test.path, bytes.NewBufferString(test.body))
			w := httptest.NewRecorder()
			s.mux.ServeHTTP(w, r)
			if w.Code != test.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, test.want, w.Body.String())
			}
		})
	}
}

const (
	testSchema        = `{"$schema":"http://json-schema.org/draft-07/schema#","$id":"gts://gts.x.test._.foo.v1~","type":"object","properties":{"name":{"type":"string"}}}`
	testSchemaChanged = `{"$schema":"http://json-schema.org/draft-07/schema#","$id":"gts://gts.x.test._.foo.v1~","type":"object","properties":{"name":{"type":"integer"}}}`
)

func postEntity(t *testing.T, s *Server, body string) int {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/entities", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)
	return w.Code
}

func entityRequest(t *testing.T, s *Server, method, path, body string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return w.Code, response
}

func TestAddEntityConflict(t *testing.T) {
	s := NewServer(gts.NewGtsStore(nil), "", 0, 0)

	if code := postEntity(t, s, testSchema); code != http.StatusOK {
		t.Fatalf("initial register status = %d, want 200", code)
	}
	// Identical re-submission stays idempotent.
	if code := postEntity(t, s, testSchema); code != http.StatusOK {
		t.Fatalf("idempotent register status = %d, want 200", code)
	}
	// Changed content is rejected with 409.
	if code := postEntity(t, s, testSchemaChanged); code != http.StatusConflict {
		t.Fatalf("changed register status = %d, want 409", code)
	}
}

func TestAddEntityAllowUpdates(t *testing.T) {
	store := gts.NewGtsStoreWithConfig(nil, &gts.RegistryConfig{AllowEntityUpdates: true})
	s := NewServer(store, "", 0, 0)

	if code := postEntity(t, s, testSchema); code != http.StatusOK {
		t.Fatalf("initial register status = %d, want 200", code)
	}
	if code := postEntity(t, s, testSchemaChanged); code != http.StatusOK {
		t.Fatalf("changed register status = %d, want 200 with updates allowed", code)
	}
}

func TestValidatedRegistrationFailureIsNotStored(t *testing.T) {
	store := gts.NewGtsStore(nil)
	s := NewServer(store, "", 0, 0)
	const id = "gts.x.server.ns.rejected.v1~"
	const target = "gts.x.server.ns.missing.v1~"
	body := `{"$schema":"http://json-schema.org/draft-07/schema#","$id":"gts://` + id + `","type":"object","properties":{"ref":{"type":"string","x-gts-ref":"` + target + `"}}}`

	status, response := entityRequest(t, s, http.MethodPost, "/entities?validate=true", body)
	errorText, _ := response["error"].(string)
	if status != http.StatusUnprocessableEntity || response["ok"] != false || !strings.Contains(errorText, target) {
		t.Fatalf("validated registration response = %d %v", status, response)
	}
	if store.Get(id) != nil {
		t.Fatal("rejected entity was stored")
	}
}

func TestRejectedRevalidationPreservesStoredSchema(t *testing.T) {
	store := gts.NewGtsStore(nil)
	s := NewServer(store, "", 0, 0)
	const id = "gts.x.server.ns.preserved.v1~"
	const target = "gts.x.server.ns.missing.v1~"
	body := `{"$schema":"http://json-schema.org/draft-07/schema#","$id":"gts://` + id + `","type":"object","properties":{"ref":{"type":"string","x-gts-ref":"` + target + `"}}}`

	if status, _ := entityRequest(t, s, http.MethodPost, "/entities", body); status != http.StatusOK {
		t.Fatalf("initial registration status = %d", status)
	}
	status, response := entityRequest(t, s, http.MethodPost, "/entities?validate=true", body)
	if status != http.StatusUnprocessableEntity || response["ok"] != false {
		t.Fatalf("revalidation response = %d %v", status, response)
	}
	stored := store.Get(id)
	if stored == nil || stored.Content["type"] != "object" {
		t.Fatalf("stored schema = %#v", stored)
	}
	status, response = entityRequest(t, s, http.MethodGet, "/entities/"+id, "")
	if status != http.StatusOK || response["ok"] != true {
		t.Fatalf("get response = %d %v", status, response)
	}
}

func TestAddSchemaConflict(t *testing.T) {
	s := NewServer(gts.NewGtsStore(nil), "", 0, 0)
	post := func(schema string) int {
		r := httptest.NewRequest(http.MethodPost, "/type-schemas", bytes.NewBufferString(schema))
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, r)
		return w.Code
	}

	if code := post(`{"type_id":"gts.x.test._.bar.v1~","schema":{"$schema":"http://json-schema.org/draft-07/schema#","$id":"gts://gts.x.test._.bar.v1~","type":"object"}}`); code != http.StatusOK {
		t.Fatalf("initial add-schema status = %d, want 200", code)
	}
	if code := post(`{"type_id":"gts.x.test._.bar.v1~","schema":{"$schema":"http://json-schema.org/draft-07/schema#","$id":"gts://gts.x.test._.bar.v1~","type":"string"}}`); code != http.StatusConflict {
		t.Fatalf("changed add-schema status = %d, want 409", code)
	}
}

func TestServerClosesConnections(t *testing.T) {
	s := NewServer(nil, "", 0, 0)
	testServer := httptest.NewServer(s.mux)
	defer testServer.Close()

	response, err := http.Get(testServer.URL + "/validate-id?gts_id=gts.x.test1.events.type.v1~")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()

	if !response.Close {
		t.Fatal("response connection remains open")
	}
}

func TestValidateSchemaRejectsInstanceID(t *testing.T) {
	store := gts.NewGtsStore(nil)
	typeID := "gts.x.server.ns.type.v1~"
	if err := store.RegisterSchema(typeID, canonicalSchema(typeID, map[string]any{"type": "object"})); err != nil {
		t.Fatal(err)
	}
	instanceID := "gts.x.server.ns.type.v1~x.server._.instance.v1"
	if err := store.Register(gts.NewJsonEntity(map[string]any{
		"gts_id": instanceID,
		"type":   typeID,
	}, gts.DefaultGtsConfig())); err != nil {
		t.Fatal(err)
	}

	s := NewServer(store, "", 0, 0)
	r := httptest.NewRequest(http.MethodPost, "/validate-type-schema", strings.NewReader(`{"type_id":"`+instanceID+`"}`))
	w := httptest.NewRecorder()
	s.handleValidateSchema(w, r)

	if !strings.Contains(w.Body.String(), "type_id does not identify a type schema") {
		t.Fatalf("response = %s", w.Body.String())
	}
}
