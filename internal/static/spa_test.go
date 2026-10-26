package static

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

const indexHTML = "<!doctype html><title>app</title>"

func newTestSPA() http.Handler {
	return NewSPA(fstest.MapFS{
		"index.html":       {Data: []byte(indexHTML)},
		"favicon.svg":      {Data: []byte("<svg/>")},
		"assets/app-1.js":  {Data: []byte("console.log(1)")},
		"assets/app-1.css": {Data: []byte("body{}")},
	})
}

func serve(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestServesIndexAtRoot(t *testing.T) {
	rec := serve(newTestSPA(), http.MethodGet, "/")
	if rec.Code != http.StatusOK || rec.Body.String() != indexHTML {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q", got)
	}
}

func TestFallsBackToIndexForClientRoutes(t *testing.T) {
	for _, target := range []string{"/datasets/42", "/assets", "/v1.2/dashboard"} {
		t.Run(target, func(t *testing.T) {
			rec := serve(newTestSPA(), http.MethodGet, target)
			if rec.Code != http.StatusOK || rec.Body.String() != indexHTML {
				t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMissingFilesAre404NotIndex(t *testing.T) {
	for _, target := range []string{"/assets/missing.js", "/nope.png"} {
		t.Run(target, func(t *testing.T) {
			if rec := serve(newTestSPA(), http.MethodGet, target); rec.Code != http.StatusNotFound {
				t.Fatalf("status %d", rec.Code)
			}
		})
	}
}

func TestCacheHeaders(t *testing.T) {
	tests := map[string]string{
		"/assets/app-1.js":  "public, max-age=31536000, immutable",
		"/assets/app-1.css": "public, max-age=31536000, immutable",
		"/favicon.svg":      "no-cache",
	}
	for target, want := range tests {
		if got := serve(newTestSPA(), http.MethodGet, target).Header().Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control = %q, want %q", target, got, want)
		}
	}
}

func TestOnlyReadMethodsAreAllowed(t *testing.T) {
	rec := serve(newTestSPA(), http.MethodPost, "/")
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("status %d Allow %q", rec.Code, rec.Header().Get("Allow"))
	}
	if rec := serve(newTestSPA(), http.MethodHead, "/"); rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("HEAD: status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestPathTraversalCannotEscapeRoot(t *testing.T) {
	for _, target := range []string{"/../../etc/passwd", "/assets/../../secret", "/%2e%2e/%2e%2e/etc/passwd"} {
		t.Run(target, func(t *testing.T) {
			rec := serve(newTestSPA(), http.MethodGet, target)
			if strings.Contains(rec.Body.String(), "root:") {
				t.Fatal("leaked a file outside the root")
			}
		})
	}
}
