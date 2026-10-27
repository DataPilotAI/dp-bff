package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

const (
	EnvAddr             = "DP_BFF_ADDR"
	EnvStaticDir        = "DP_BFF_STATIC_DIR"
	EnvUpstreamURL      = "DP_BFF_UPSTREAM_URL"
	EnvMaxBodyMB        = "DP_BFF_MAX_BODY_MB"
	EnvLocalFrontendURL = "DP_BFF_LOCAL_FRONTEND_URL"

	defaultAddr      = ":8081"
	defaultStaticDir = "static"
	// Slightly above the frontend's 50 MB upload limit so multipart overhead still fits.
	defaultMaxBodyMB = 55
	bytesPerMB       = 1 << 20
)

type Config struct {
	Addr         string
	StaticDir    string
	UpstreamURL  *url.URL
	MaxBodyBytes int64
	// LocalFrontendURL, when set, replaces the static bundle with a local Vite dev server (hot reload).
	LocalFrontendURL *url.URL
}

// Load reads configuration through getenv so tests do not touch the process environment.
func Load(getenv func(string) string) (Config, error) {
	upstream, err := parseHTTPURL(EnvUpstreamURL, getenv(EnvUpstreamURL))
	if err != nil {
		return Config{}, err
	}
	devFrontend, err := parseLocalFrontend(getenv(EnvLocalFrontendURL))
	if err != nil {
		return Config{}, err
	}
	maxBodyMB, err := parsePositive(getenv(EnvMaxBodyMB), defaultMaxBodyMB)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", EnvMaxBodyMB, err)
	}

	return Config{
		Addr:             valueOr(getenv(EnvAddr), defaultAddr),
		StaticDir:        valueOr(getenv(EnvStaticDir), defaultStaticDir),
		UpstreamURL:      upstream,
		MaxBodyBytes:     maxBodyMB * bytesPerMB,
		LocalFrontendURL: devFrontend,
	}, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func parsePositive(raw string, fallback int64) (int64, error) {
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("must be a positive integer")
	}
	return parsed, nil
}

func parseHTTPURL(envName, raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("%s is required", envName)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("%s must be an absolute http(s) URL", envName)
	}
	return parsed, nil
}

// Local mode turns off the Content-Security-Policy, so it is limited to a loopback host
// to keep it from being pointed at something remote by mistake in a deployed environment.
func parseLocalFrontend(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := parseHTTPURL(EnvLocalFrontendURL, raw)
	if err != nil {
		return nil, err
	}
	if host := parsed.Hostname(); host != "localhost" && !isLoopbackIP(host) {
		return nil, fmt.Errorf("%s must point at localhost or a loopback address", EnvLocalFrontendURL)
	}
	return parsed, nil
}

func isLoopbackIP(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
