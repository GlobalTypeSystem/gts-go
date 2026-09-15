package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GlobalTypeSystem/gts-go/gts"
)

func TestValidateJSON(t *testing.T) {
	store := gts.NewGtsStore(nil)
	typeID := "gts.x.test6json._.validate_json.v1~"
	if err := store.RegisterSchema(typeID, map[string]any{
		"type":       "object",
		"required":   []any{"name"},
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	}); err != nil {
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

func TestAddSchemaConflict(t *testing.T) {
	s := NewServer(gts.NewGtsStore(nil), "", 0, 0)
	post := func(schema string) int {
		r := httptest.NewRequest(http.MethodPost, "/type-schemas", bytes.NewBufferString(schema))
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, r)
		return w.Code
	}

	if code := post(`{"type_id":"gts.x.test._.bar.v1~","schema":{"type":"object"}}`); code != http.StatusOK {
		t.Fatalf("initial add-schema status = %d, want 200", code)
	}
	if code := post(`{"type_id":"gts.x.test._.bar.v1~","schema":{"type":"string"}}`); code != http.StatusConflict {
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
