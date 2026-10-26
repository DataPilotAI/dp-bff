package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func ok(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	SecurityHeaders(http.HandlerFunc(ok)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	for header, want := range map[string]string{
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
		"Referrer-Policy":            "no-referrer",
		"Cross-Origin-Opener-Policy": "same-origin",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"default-src 'self'", "object-src 'none'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP %q is missing %q", csp, directive)
		}
	}
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS must not be sent over plain HTTP")
	}
}

func TestSecurityHeadersSendsHSTSOverTLS(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	rec := httptest.NewRecorder()
	SecurityHeaders(http.HandlerFunc(ok)).ServeHTTP(rec, req)
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected HSTS over TLS")
	}
}

func TestLimitBodyRejectsKnownLength(t *testing.T) {
	called := false
	handler := LimitBody(10, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 11))))

	if rec.Code != http.StatusRequestEntityTooLarge || called {
		t.Fatalf("status %d, handler called: %v", rec.Code, called)
	}
}

func TestLimitBodyStopsReadingUnknownLength(t *testing.T) {
	var readErr error
	handler := LimitBody(10, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 50)))
	req.ContentLength = -1
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if readErr == nil {
		t.Fatal("expected a read error once the limit is exceeded")
	}
}

func TestLimitBodyAllowsBodiesWithinTheLimit(t *testing.T) {
	var got []byte
	handler := LimitBody(10, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789")))

	if string(got) != "0123456789" {
		t.Fatalf("body = %q", got)
	}
}

func TestRecoverReturns500(t *testing.T) {
	handler := Recover(discardLogger(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestRecoverRepanicsAbortHandler(t *testing.T) {
	handler := Recover(discardLogger(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	defer func() {
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", recovered)
		}
	}()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestAccessLogOmitsQueryString(t *testing.T) {
	var out bytes.Buffer
	handler := AccessLog(slog.New(slog.NewTextHandler(&out, nil)), http.HandlerFunc(ok))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/x?token=secret", nil))

	line := out.String()
	if !strings.Contains(line, "path=/api/x") || !strings.Contains(line, "status=200") {
		t.Errorf("unexpected log line: %s", line)
	}
	if strings.Contains(line, "secret") {
		t.Errorf("log leaked the query string: %s", line)
	}
}
