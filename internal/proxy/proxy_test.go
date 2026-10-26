package proxy

import (
	"bufio"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DataPilotAI/dp-bff/internal/middleware"
)

const clientTimeout = 5 * time.Second

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func newFront(t *testing.T, upstream string, wrap func(http.Handler) http.Handler) *httptest.Server {
	t.Helper()
	handler := New(mustParse(t, upstream), discardLogger())
	if wrap != nil {
		handler = wrap(handler)
	}
	front := httptest.NewServer(handler)
	t.Cleanup(front.Close)
	return front
}

func TestForwardsRequestAndOverridesForwardedHeaders(t *testing.T) {
	var gotURI, gotMethod, gotBody, gotForwardedFor, gotForwardedHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotURI, gotMethod, gotBody = r.URL.RequestURI(), r.Method, string(body)
		gotForwardedFor, gotForwardedHost = r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Forwarded-Host")
		w.Header().Set("Cache-Control", "public, max-age=999")
		io.WriteString(w, "ok")
	}))
	defer upstream.Close()
	front := newFront(t, upstream.URL, nil)

	req, _ := http.NewRequest(http.MethodPost, front.URL+"/api/datasets/7/chat?x=1", strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("X-Forwarded-For", "6.6.6.6")
	resp, err := (&http.Client{Timeout: clientTimeout}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if gotURI != "/api/datasets/7/chat?x=1" || gotMethod != http.MethodPost || gotBody != `{"message":"hi"}` {
		t.Errorf("upstream saw %s %s %q", gotMethod, gotURI, gotBody)
	}
	if gotForwardedFor != "127.0.0.1" {
		t.Errorf("X-Forwarded-For = %q, the client-supplied value must not pass through", gotForwardedFor)
	}
	if gotForwardedHost == "" {
		t.Error("X-Forwarded-Host was not set")
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestStreamsResponsesWithoutBuffering(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseUpstream := func() { releaseOnce.Do(func() { close(release) }) }

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: one\n\n")
		w.(http.Flusher).Flush()
		<-release
		io.WriteString(w, "data: two\n\n")
	}))
	defer upstream.Close()
	defer releaseUpstream() // runs before upstream.Close so the handler can finish

	front := newFront(t, upstream.URL, nil)
	resp, err := (&http.Client{Timeout: clientTimeout}).Get(front.URL + "/api/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// The second event is still held back, so this only succeeds if the proxy flushed the first one.
	reader := bufio.NewReader(resp.Body)
	first, err := reader.ReadString('\n')
	if err != nil || first != "data: one\n" {
		t.Fatalf("first event = %q, err = %v", first, err)
	}

	releaseUpstream()
	rest, _ := io.ReadAll(reader)
	if !strings.Contains(string(rest), "data: two") {
		t.Fatalf("remaining stream = %q", rest)
	}
}

func TestUnreachableUpstreamIs502WithoutDetails(t *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	upstreamURL := upstream.URL
	upstream.Close()
	front := newFront(t, upstreamURL, nil)

	resp, err := (&http.Client{Timeout: clientTimeout}).Get(front.URL + "/api/x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if strings.Contains(string(body), "127.0.0.1") || strings.Contains(string(body), "refused") {
		t.Errorf("body leaks upstream details: %q", body)
	}
}

func TestOversizedStreamedBodyIs413(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
	}))
	defer upstream.Close()
	front := newFront(t, upstream.URL, func(next http.Handler) http.Handler { return middleware.LimitBody(10, next) })

	// NopCloser hides the length, so the body goes out chunked and only the streaming limit can catch it.
	body := io.NopCloser(strings.NewReader(strings.Repeat("x", 1000)))
	resp, err := (&http.Client{Timeout: clientTimeout}).Post(front.URL+"/api/upload", "text/plain", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
