package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

const (
	dialTimeout         = 5 * time.Second
	tlsHandshakeTimeout = 10 * time.Second
	// Only the wait for response headers is bounded; the body may stream for as long as the model replies.
	responseHeaderTimeout = 60 * time.Second
	idleConnTimeout       = 90 * time.Second
	keepAlive             = 30 * time.Second
	maxIdleConns          = 100
	flushImmediately      = -1
)

// New returns a reverse proxy to upstream that is safe for server-sent events:
// responses are flushed as they arrive instead of being buffered.
func New(upstream *url.URL, logger *slog.Logger) http.Handler {
	reverseProxy := &httputil.ReverseProxy{
		// SetXForwarded replaces any X-Forwarded-* the client sent, so they cannot be spoofed.
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstream)
			r.SetXForwarded()
		},
		FlushInterval: flushImmediately,
		Transport:     newTransport(),
		// Replaced below so API responses are never cached by browsers or shared caches.
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("Cache-Control")
			return nil
		},
		ErrorHandler: errorHandler(logger),
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		reverseProxy.ServeHTTP(w, r)
	})
}

func newTransport() *http.Transport {
	return &http.Transport{
		DialContext:           (&net.Dialer{Timeout: dialTimeout, KeepAlive: keepAlive}).DialContext,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		IdleConnTimeout:       idleConnTimeout,
		MaxIdleConns:          maxIdleConns,
		ForceAttemptHTTP2:     true,
	}
}

// errorHandler never forwards upstream error text to the client; details go to the log only.
func errorHandler(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.Is(err, context.Canceled):
			// The client went away; there is nobody to answer.
		case errors.As(err, &tooLarge):
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		default:
			logger.Error("upstream request failed", "method", r.Method, "path", r.URL.Path, "error", err)
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		}
	}
}
