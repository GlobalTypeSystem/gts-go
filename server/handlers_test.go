package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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
	post := func(body string) (int, map[string]any) {
		r := httptest.NewRequest(http.MethodPost, "/type-schemas", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, r)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}
	firstResult := func(resp map[string]any) map[string]any {
		results, _ := resp["results"].([]any)
		if len(results) == 0 {
			t.Fatalf("missing results in response: %v", resp)
		}
		first, _ := results[0].(map[string]any)
		return first
	}

	code, resp := post(`[{"$schema":"http://json-schema.org/draft-07/schema#","$id":"gts://gts.x.test._.bar.v1~","type":"object"}]`)
	if code != http.StatusOK || resp["ok"] != true {
		t.Fatalf("initial add-schema status = %d resp = %v, want 200 ok", code, resp)
	}
	if got := firstResult(resp)["type_id"]; got != "gts.x.test._.bar.v1~" {
		t.Fatalf("initial add-schema type_id = %v", got)
	}

	code, resp = post(`[{"$schema":"http://json-schema.org/draft-07/schema#","$id":"gts://gts.x.test._.bar.v1~","type":"string"}]`)
	if code != http.StatusOK {
		t.Fatalf("changed add-schema status = %d, want 200", code)
	}
	if resp["ok"] != false || firstResult(resp)["ok"] != false {
		t.Fatalf("changed add-schema should report a per-item conflict, resp = %v", resp)
	}
}

// TestAddSchemasHonorsValidate verifies POST /type-schemas applies ?validate /
// ?gts-ref-validation to every batch entry exactly like POST /entities: an
// entry whose gts:// $ref targets an unregistered type is a forward reference
// (accepted without validate, rejected with validate), and a bogus
// gts-ref-validation refuses the whole batch with 422.
func TestAddSchemasHonorsValidate(t *testing.T) {
	s := NewServer(gts.NewGtsStore(nil), "", 0, 0)
	post := func(target, body string) (int, map[string]any) {
		r := httptest.NewRequest(http.MethodPost, target, bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, r)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}
	firstOK := func(resp map[string]any) bool {
		results, _ := resp["results"].([]any)
		if len(results) == 0 {
			t.Fatalf("missing results in response: %v", resp)
		}
		first, _ := results[0].(map[string]any)
		ok, _ := first["ok"].(bool)
		return ok
	}
	schema := func(typeID string) string {
		return `[{"$schema":"http://json-schema.org/draft-07/schema#","$id":"gts://` + typeID +
			`","type":"object","properties":{"a":{"$ref":"gts://gts.x.batchval._.missing.v1~"}}}]`
	}

	// Without validate, the forward reference registers.
	code, resp := post("/type-schemas", schema("gts.x.batchval._.fwd.v1~"))
	if code != http.StatusOK || resp["ok"] != true || !firstOK(resp) {
		t.Fatalf("forward-ref batch without validate = %d %v, want 200 ok", code, resp)
	}

	// With validate=true, the unresolved reference is rejected per entry.
	code, resp = post("/type-schemas?validate=true", schema("gts.x.batchval._.needsref.v1~"))
	if code != http.StatusOK || resp["ok"] != false || firstOK(resp) {
		t.Fatalf("unresolved-ref batch with validate = %d %v, want 200 not-ok", code, resp)
	}

	// A bogus gts-ref-validation refuses the whole batch with 422.
	code, _ = post("/type-schemas?gts-ref-validation=bogus", schema("gts.x.batchval._.badmode.v1~"))
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("bogus gts-ref-validation status = %d, want 422", code)
	}
}

// TestAddSchemasValidateStagingOrderIndependentAndAtomic verifies the two-phase
// staged batch: a validated batch resolves an intra-batch reference even when
// the referrer precedes its target (order-independent), and a mixed batch
// commits only the entries that pass - the invalid one is never published, so a
// follow-up GET for it returns not-found.
func TestAddSchemasValidateStagingOrderIndependentAndAtomic(t *testing.T) {
	s := NewServer(gts.NewGtsStore(nil), "", 0, 0)
	do := func(method, target, body string) (int, map[string]any) {
		r := httptest.NewRequest(method, target, bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, r)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}
	itemOK := func(resp map[string]any, i int) bool {
		results, _ := resp["results"].([]any)
		if i >= len(results) {
			t.Fatalf("missing results[%d] in %v", i, resp)
		}
		item, _ := results[i].(map[string]any)
		ok, _ := item["ok"].(bool)
		return ok
	}
	d7 := `"$schema":"http://json-schema.org/draft-07/schema#"`

	// Referrer BEFORE target in one validated batch: both must register.
	_, resp := do(http.MethodPost, "/type-schemas?validate=true", `[`+
		`{`+d7+`,"$id":"gts://gts.x.border._.referrer.v1~","type":"object","properties":{"child":{"$ref":"gts://gts.x.border._.target.v1~"}}},`+
		`{`+d7+`,"$id":"gts://gts.x.border._.target.v1~","type":"object"}]`)
	if resp["ok"] != true || !itemOK(resp, 0) || !itemOK(resp, 1) {
		t.Fatalf("order-independent batch should fully register, got %v", resp)
	}

	// Mixed batch: valid commits, invalid (unresolved ref) does not.
	_, resp = do(http.MethodPost, "/type-schemas?validate=true", `[`+
		`{`+d7+`,"$id":"gts://gts.x.border._.good.v1~","type":"object"},`+
		`{`+d7+`,"$id":"gts://gts.x.border._.bad.v1~","type":"object","properties":{"a":{"$ref":"gts://gts.x.border._.missing.v1~"}}}]`)
	if resp["ok"] != false || !itemOK(resp, 0) || itemOK(resp, 1) {
		t.Fatalf("mixed batch should commit only the valid entry, got %v", resp)
	}
	if code, good := do(http.MethodGet, "/entities/gts.x.border._.good.v1~", ""); code != http.StatusOK || good["ok"] != true {
		t.Fatalf("valid entry must be retrievable, got %d %v", code, good)
	}
	if _, bad := do(http.MethodGet, "/entities/gts.x.border._.bad.v1~", ""); bad["ok"] != false {
		t.Fatalf("invalid entry must not be registered, got %v", bad)
	}
}

// TestAddSchemasValidateDoesNotCommitDependentOfDiscarded verifies a survivor
// is never published when a sibling it depends on is itself discarded. Under
// any-present ref validation, entry B carries an x-gts-ref to A, and A carries
// an x-gts-ref to a type that is never registered. Validated against the fully
// staged set, B passes (A is present) while A fails (its target is missing) -
// a single-pass implementation would then commit B with a dangling reference to
// the discarded A. The iterative discard-then-revalidate must reject B too, so
// neither is retrievable afterwards.
func TestAddSchemasValidateDoesNotCommitDependentOfDiscarded(t *testing.T) {
	s := NewServer(gts.NewGtsStore(nil), "", 0, 0)
	do := func(method, target, body string) (int, map[string]any) {
		r := httptest.NewRequest(method, target, bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, r)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}
	itemOK := func(resp map[string]any, i int) bool {
		results, _ := resp["results"].([]any)
		item, _ := results[i].(map[string]any)
		ok, _ := item["ok"].(bool)
		return ok
	}
	d7 := `"$schema":"http://json-schema.org/draft-07/schema#"`

	// A (index 0) is invalid: its x-gts-ref target is never registered.
	// B (index 1) is structurally valid but x-gts-refs A.
	_, resp := do(http.MethodPost, "/type-schemas?validate=true&gts-ref-validation=any-present", `[`+
		`{`+d7+`,"$id":"gts://gts.x.dep._.a.v1~","type":"object","properties":{"r":{"type":"string","x-gts-ref":"gts.x.dep._.missing.v1~"}}},`+
		`{`+d7+`,"$id":"gts://gts.x.dep._.b.v1~","type":"object","properties":{"x":{"type":"string","x-gts-ref":"gts.x.dep._.a.v1~"}}}]`)
	if resp["ok"] != false || itemOK(resp, 0) || itemOK(resp, 1) {
		t.Fatalf("neither the invalid A nor its dependent B may be committed, got %v", resp)
	}
	if _, a := do(http.MethodGet, "/entities/gts.x.dep._.a.v1~", ""); a["ok"] != false {
		t.Fatalf("invalid A must not be registered, got %v", a)
	}
	if _, b := do(http.MethodGet, "/entities/gts.x.dep._.b.v1~", ""); b["ok"] != false {
		t.Fatalf("B (dependent on a discarded sibling) must not be registered, got %v", b)
	}
}

// TestAddSchemasValidateDuplicateIDInBatch verifies a batch that carries the
// same $id twice with different content does not silently keep only the last
// entry: the conflicting second commit is reported as not-ok rather than
// overwriting the first.
func TestAddSchemasValidateDuplicateIDInBatch(t *testing.T) {
	s := NewServer(gts.NewGtsStore(nil), "", 0, 0)
	do := func(method, target, body string) (int, map[string]any) {
		r := httptest.NewRequest(method, target, bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, r)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}
	itemOK := func(resp map[string]any, i int) bool {
		results, _ := resp["results"].([]any)
		item, _ := results[i].(map[string]any)
		ok, _ := item["ok"].(bool)
		return ok
	}
	d7 := `"$schema":"http://json-schema.org/draft-07/schema#"`

	_, resp := do(http.MethodPost, "/type-schemas?validate=true", `[`+
		`{`+d7+`,"$id":"gts://gts.x.dup._.t.v1~","type":"object","title":"a"},`+
		`{`+d7+`,"$id":"gts://gts.x.dup._.t.v1~","type":"object","title":"b"}]`)
	// A batch carrying the same $id twice with different content is internally
	// inconsistent, so the atomic publish keeps NEITHER entry (no silent
	// last-wins): the batch is rejected as a whole and nothing is committed.
	if resp["ok"] != false {
		t.Fatalf("a batch with a conflicting duplicate id must report ok=false, got %v", resp)
	}
	if itemOK(resp, 0) || itemOK(resp, 1) {
		t.Fatalf("all-or-nothing: neither duplicate entry may commit, got %v", resp)
	}
	// Nothing from the inconsistent batch is published.
	if _, getResp := do(http.MethodGet, "/entities/gts.x.dup._.t.v1~", ""); getResp["ok"] == true {
		t.Fatalf("no entity should be committed from the rejected duplicate batch, got %v", getResp)
	}
}

// TestAddSchemasValidateStagingConcurrentReadsNeverSeeInvalid is a concurrency
// probe: while a validate=true batch containing a deliberately-invalid entry is
// in flight, several goroutines hammer GET /entities/<invalid-id> and assert
// the invalid entity is NEVER observable. A staged (not-yet-committed) or a
// register-then-rollback entry would be caught here. The cycle is repeated many
// times to widen the window. The store uses a read/write lock with a staging
// overlay, so these reads run genuinely concurrently with the batch writer.
func TestAddSchemasValidateStagingConcurrentReadsNeverSeeInvalid(t *testing.T) {
	s := NewServer(gts.NewGtsStore(nil), "", 0, 0)
	ts := httptest.NewServer(s.mux)
	defer ts.Close()
	client := ts.Client()

	const cycles = 10
	const probes = 4
	const validPerBatch = 120
	const d7 = `"$schema":"http://json-schema.org/draft-07/schema#"`

	getOK := func(url string) bool {
		resp, err := client.Get(url)
		if err != nil {
			return false
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		var parsed map[string]any
		if json.Unmarshal(body, &parsed) != nil {
			return false
		}
		ok, _ := parsed["ok"].(bool)
		return ok
	}

	var leaks int64
	for cycle := 0; cycle < cycles; cycle++ {
		ns := fmt.Sprintf("gts.x.goconc%d._", cycle)
		var sb strings.Builder
		sb.WriteString("[")
		for i := 0; i < validPerBatch; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, `{%s,"$id":"gts://%s.t%d.v1~","type":"object","properties":{"p":{"type":"string"}}}`, d7, ns, i)
		}
		invalidID := fmt.Sprintf("%s.invalid.v1~", ns)
		fmt.Fprintf(&sb, `,{%s,"$id":"gts://%s","type":"object","properties":{"a":{"$ref":"gts://%s.never.v1~"}}}]`, d7, invalidID, ns)
		batch := sb.String()
		invalidURL := ts.URL + "/entities/" + invalidID

		stop := make(chan struct{})
		var wg sync.WaitGroup
		for p := 0; p < probes; p++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						if getOK(invalidURL) {
							atomic.AddInt64(&leaks, 1)
						}
					}
				}
			}()
		}

		resp, err := client.Post(ts.URL+"/type-schemas?validate=true", "application/json", strings.NewReader(batch))
		close(stop)
		wg.Wait()
		if err != nil {
			t.Fatalf("cycle %d POST failed: %v", cycle, err)
		}
		var body map[string]any
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		_ = json.Unmarshal(respBody, &body)
		if body["ok"] != false {
			t.Fatalf("cycle %d: batch with an invalid entry must report ok=false, got %v", cycle, body)
		}
		if getOK(invalidURL) {
			t.Fatalf("cycle %d: invalid entry must not be registered after the batch", cycle)
		}
		if !getOK(ts.URL + "/entities/" + ns + ".t0.v1~") {
			t.Fatalf("cycle %d: a valid batch entry must be registered", cycle)
		}
	}

	if n := atomic.LoadInt64(&leaks); n != 0 {
		t.Fatalf("an uncommitted/invalid entity was exposed to a concurrent reader %d time(s)", n)
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
