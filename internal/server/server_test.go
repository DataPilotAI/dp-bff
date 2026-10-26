package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DataPilotAI/dp-bff/internal/config"
)

func newTestHandler(t *testing.T, upstreamURL string) http.Handler {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<title>app</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(config.Config{StaticDir: dir, UpstreamURL: upstream, MaxBodyBytes: 1 << 20},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestRoutes(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "api:"+r.URL.Path)
	}))
	defer upstream.Close()
	handler := newTestHandler(t, upstream.URL)

	if rec := get(handler, "/api/datasets"); rec.Body.String() != "api:/api/datasets" {
		t.Errorf("/api/ was not proxied: %q", rec.Body.String())
	}
	if rec := get(handler, "/datasets/1"); !strings.Contains(rec.Body.String(), "<title>app</title>") {
		t.Errorf("client route did not fall back to index.html: %q", rec.Body.String())
	}
	if rec := get(handler, "/healthz"); rec.Code != http.StatusNoContent {
		t.Errorf("/healthz status %d", rec.Code)
	}
}

func TestSecurityHeadersApplyToEveryRoute(t *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	defer upstream.Close()
	handler := newTestHandler(t, upstream.URL)

	for _, target := range []string{"/", "/api/anything", "/healthz", "/missing.js"} {
		if get(handler, target).Header().Get("Content-Security-Policy") == "" {
			t.Errorf("%s has no Content-Security-Policy", target)
		}
	}
}

func TestStartupFailsWithoutFrontendBundle(t *testing.T) {
	upstream, _ := url.Parse("http://localhost:8080")
	_, err := NewHandler(config.Config{StaticDir: t.TempDir(), UpstreamURL: upstream},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("expected an error when index.html is missing")
	}
}
