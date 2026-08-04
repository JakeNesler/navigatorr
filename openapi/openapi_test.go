package openapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A parameter schema with no "type" is legal OpenAPI and appears in real specs
// (oneOf/anyOf/$ref-only). Parsing one used to panic, which killed the server
// at startup because LoadAll has no recover.
func TestParseTypelessParameterSchema(t *testing.T) {
	const spec = `{
  "openapi": "3.0.0",
  "info": {"title": "t", "version": "1"},
  "paths": {
    "/thing": {
      "get": {
        "summary": "get a thing",
        "parameters": [
          {"name": "id", "in": "query", "schema": {"oneOf": [{"type": "string"}, {"type": "integer"}]}},
          {"name": "typed", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`

	idx, err := Parse(context.Background(), "demo", []byte(spec))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	detail, err := idx.GetDetail("/thing", "GET")
	if err != nil {
		t.Fatalf("GetDetail: %v", err)
	}
	if len(detail.Parameters) != 2 {
		t.Fatalf("got %d parameters, want 2", len(detail.Parameters))
	}
	byName := map[string]string{}
	for _, p := range detail.Parameters {
		byName[p.Name] = p.Type
	}
	if byName["id"] != "" {
		t.Errorf("typeless param should have empty Type, got %q", byName["id"])
	}
	if byName["typed"] != "string" {
		t.Errorf("typed param Type = %q, want string", byName["typed"])
	}
}

func testIndex() *Index {
	mk := func(method, path string) *EndpointDetail {
		return &EndpointDetail{Service: "sonarr", Method: method, Path: path, Summary: method + " " + path}
	}
	return &Index{
		Service: "sonarr",
		Endpoints: map[string]map[string]*EndpointDetail{
			"/series":            {"GET": mk("GET", "/series"), "POST": mk("POST", "/series")},
			"/series/{id}":       {"GET": mk("GET", "/series/{id}"), "PUT": mk("PUT", "/series/{id}")},
			"/series/lookup":     {"GET": mk("GET", "/series/lookup")},
			"/importlist/series": {"GET": mk("GET", "/importlist/series")},
			"/queue/bulk":        {"DELETE": mk("DELETE", "/queue/bulk")},
		},
	}
}

// The fallback path match walks a map, so without sorting the same query
// returns a different endpoint on each call.
func TestGetDetailIsDeterministic(t *testing.T) {
	idx := testIndex()

	tests := []struct {
		query      string
		method     string
		wantPath   string
		wantMethod string
	}{
		{"/series", "GET", "/series", "GET"},             // exact wins
		{"/series", "POST", "/series", "POST"},           // exact method
		{"/series/", "GET", "/series/{id}", "GET"},       // prefix match, shortest candidate
		{"/series/l", "GET", "/series/lookup", "GET"},    // prefix match, only candidate
		{"/lookup", "GET", "/series/lookup", "GET"},      // no prefix match, falls to suffix
		{"/queue/bulk", "GET", "/queue/bulk", "DELETE"},  // method fallback
		{"/series/{id}", "PATCH", "/series/{id}", "GET"}, // method fallback sorts
	}

	for _, tt := range tests {
		for i := 0; i < 50; i++ {
			got, err := idx.GetDetail(tt.query, tt.method)
			if err != nil {
				t.Fatalf("GetDetail(%q, %q): %v", tt.query, tt.method, err)
			}
			if got.Path != tt.wantPath || got.Method != tt.wantMethod {
				t.Fatalf("GetDetail(%q, %q) = %s %s, want %s %s (iteration %d)",
					tt.query, tt.method, got.Method, got.Path, tt.wantMethod, tt.wantPath, i)
			}
		}
	}

	if _, err := idx.GetDetail("/nonexistent-zzz", "GET"); err == nil {
		t.Error("expected an error for a path matching nothing")
	}
}

// Search and Filter walk maps too; their output order must be stable or the
// same query renders differently every call.
func TestSearchAndFilterAreOrdered(t *testing.T) {
	idx := testIndex()

	var first string
	for i := 0; i < 50; i++ {
		results := idx.Search("series")
		if len(results) == 0 {
			t.Fatal("expected search results")
		}
		var key string
		for _, r := range results {
			key += r.Method + " " + r.Path + ";"
		}
		if i == 0 {
			first = key
			continue
		}
		if key != first {
			t.Fatalf("Search order changed between calls:\n  %s\n  %s", first, key)
		}
	}

	first = ""
	for i := 0; i < 50; i++ {
		results := idx.Filter("", "GET")
		var key string
		for _, r := range results {
			key += r.Method + " " + r.Path + ";"
			if r.Method != "GET" {
				t.Fatalf("Filter(method=GET) returned %s", r.Method)
			}
		}
		if i == 0 {
			first = key
			continue
		}
		if key != first {
			t.Fatalf("Filter order changed between calls:\n  %s\n  %s", first, key)
		}
	}
}

func TestCacheRoundTripAndAtomicity(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir)
	const url = "https://example.com/spec.json"

	if got := c.Get(url); got != nil {
		t.Fatal("expected a miss on an empty cache")
	}

	want := []byte(`{"openapi":"3.0.0"}`)
	if err := c.Put(url, want); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if got := c.Get(url); string(got) != string(want) {
		t.Fatalf("Get = %q, want %q", got, want)
	}

	// The temp file used for the atomic rename must not survive as a stray
	// entry in the cache directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("cache dir holds %v, want exactly the one cache file", names)
	}

	// Overwriting must fully replace, never leave a mix of old and new bytes.
	shorter := []byte(`{}`)
	if err := c.Put(url, shorter); err != nil {
		t.Fatalf("Put overwrite: %v", err)
	}
	if got := c.Get(url); string(got) != string(shorter) {
		t.Fatalf("after overwrite Get = %q, want %q", got, shorter)
	}

	c.Invalidate(url)
	if got := c.Get(url); got != nil {
		t.Error("expected a miss after Invalidate")
	}
	if _, err := os.Stat(filepath.Join(dir, "nonexistent")); !os.IsNotExist(err) {
		t.Error("sanity check failed")
	}
}
