package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

const (
	EnvAddr        = "DP_BFF_ADDR"
	EnvStaticDir   = "DP_BFF_STATIC_DIR"
	EnvUpstreamURL = "DP_BFF_UPSTREAM_URL"
	EnvMaxBodyMB   = "DP_BFF_MAX_BODY_MB"

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
}

// Load reads configuration through getenv so tests do not touch the process environment.
func Load(getenv func(string) string) (Config, error) {
	upstream, err := parseUpstream(getenv(EnvUpstreamURL))
	if err != nil {
		return Config{}, err
	}
	maxBodyMB, err := parsePositive(getenv(EnvMaxBodyMB), defaultMaxBodyMB)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", EnvMaxBodyMB, err)
	}

	return Config{
		Addr:         valueOr(getenv(EnvAddr), defaultAddr),
		StaticDir:    valueOr(getenv(EnvStaticDir), defaultStaticDir),
		UpstreamURL:  upstream,
		MaxBodyBytes: maxBodyMB * bytesPerMB,
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

func parseUpstream(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("%s is required", EnvUpstreamURL)
	}
	upstream, err := url.Parse(raw)
	if err != nil || upstream.Host == "" || (upstream.Scheme != "http" && upstream.Scheme != "https") {
		return nil, fmt.Errorf("%s must be an absolute http(s) URL", EnvUpstreamURL)
	}
	return upstream, nil
}
