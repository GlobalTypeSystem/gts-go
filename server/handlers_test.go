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

func TestServerClosesConnections(t *testing.T) {
	s := NewServer(nil, "", 0, 0)
	testServer := httptest.NewServer(s.mux)
	defer testServer.Close()

	response, err := http.Get(testServer.URL + "/validate-id?gts_id=gts.x.test1.events.type.v1~")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if !response.Close {
		t.Fatal("response connection remains open")
	}
}
