package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/DataPilotAI/dp-bff/internal/config"
	"github.com/DataPilotAI/dp-bff/internal/middleware"
	"github.com/DataPilotAI/dp-bff/internal/proxy"
	"github.com/DataPilotAI/dp-bff/internal/static"
)

const (
	apiPrefix         = "/api/"
	healthPath        = "GET /healthz"
	indexFile         = "index.html"
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 120 * time.Second
)

// NewHandler wires the routes: /api/ to the upstream, /healthz, and the SPA for everything else.
func NewHandler(cfg config.Config, logger *slog.Logger) (http.Handler, error) {
	// Fail at startup rather than serving 404s if the frontend bundle is missing.
	if _, err := os.Stat(filepath.Join(cfg.StaticDir, indexFile)); err != nil {
		return nil, fmt.Errorf("frontend bundle not found in %q: %w", cfg.StaticDir, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle(apiPrefix, middleware.LimitBody(cfg.MaxBodyBytes, proxy.New(cfg.UpstreamURL, logger)))
	mux.Handle("/", static.NewSPA(os.DirFS(cfg.StaticDir)))

	return middleware.SecurityHeaders(middleware.AccessLog(logger, middleware.Recover(logger, mux))), nil
}

// New builds the HTTP server. There is no read or write timeout on purpose: uploads
// and chat streams can legitimately run long, so only header and idle time are bounded.
func New(cfg config.Config, logger *slog.Logger) (*http.Server, error) {
	handler, err := NewHandler(cfg, logger)
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}, nil
}
