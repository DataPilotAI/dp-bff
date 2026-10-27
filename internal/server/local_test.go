package server

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DataPilotAI/dp-bff/internal/config"
)

const ioTimeout = 5 * time.Second

// fakeVite answers page requests and completes a websocket-style upgrade, which is what HMR needs.
func fakeVite(t *testing.T) *httptest.Server {
	t.Helper()
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "" {
			io.WriteString(w, "vite:"+r.URL.Path)
			return
		}
		conn, rw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
		rw.Flush()
		line, _ := rw.ReadString('\n')
		rw.WriteString("echo:" + line)
		rw.Flush()
	}))
	t.Cleanup(vite.Close)
	return vite
}

func newDevFront(t *testing.T, viteURL string) *httptest.Server {
	t.Helper()
	vite, _ := url.Parse(viteURL)
	api, _ := url.Parse("http://127.0.0.1:9")
	handler, err := NewHandler(config.Config{LocalFrontendURL: vite, UpstreamURL: api, MaxBodyBytes: 1 << 20},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(handler)
	t.Cleanup(front.Close)
	return front
}

func TestLocalModeProxiesPagesToViteWithoutCSP(t *testing.T) {
	front := newDevFront(t, fakeVite(t).URL)

	resp, err := (&http.Client{Timeout: ioTimeout}).Get(front.URL + "/src/main.tsx")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if string(body) != "vite:/src/main.tsx" {
		t.Errorf("body = %q", body)
	}
	if resp.Header.Get("Content-Security-Policy") != "" {
		t.Error("the production CSP would block Vite's dev client and must be off in local mode")
	}
}

func TestLocalModeCarriesHMRWebsocketUpgrade(t *testing.T) {
	front := newDevFront(t, fakeVite(t).URL)

	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(front.URL, "http://"), ioTimeout)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(ioTimeout))

	io.WriteString(conn, "GET /?token=abc HTTP/1.1\r\nHost: localhost\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(status, "101") {
		t.Fatalf("status line = %q, err = %v", status, err)
	}
	for { // skip the rest of the response headers
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}

	io.WriteString(conn, "ping\n")
	echoed, err := reader.ReadString('\n')
	if err != nil || echoed != "echo:ping\n" {
		t.Fatalf("echoed = %q, err = %v", echoed, err)
	}
}

func TestLocalModeStillProxiesAPIToTheBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "api:"+r.URL.Path)
	}))
	defer backend.Close()

	vite, _ := url.Parse(fakeVite(t).URL)
	api, _ := url.Parse(backend.URL)
	handler, err := NewHandler(config.Config{LocalFrontendURL: vite, UpstreamURL: api, MaxBodyBytes: 1 << 20},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	if body := get(handler, "/api/datasets").Body.String(); body != "api:/api/datasets" {
		t.Errorf("body = %q", body)
	}
}
