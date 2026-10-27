package config

import "testing"

func TestLoadLocalFrontend(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		EnvUpstreamURL:      "http://localhost:8080",
		EnvLocalFrontendURL: "http://localhost:5173",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LocalFrontendURL == nil || cfg.LocalFrontendURL.Host != "localhost:5173" {
		t.Errorf("DevFrontendURL = %v", cfg.LocalFrontendURL)
	}
}

func TestLocalFrontendIsOffByDefault(t *testing.T) {
	cfg, err := Load(env(map[string]string{EnvUpstreamURL: "http://localhost:8080"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LocalFrontendURL != nil {
		t.Errorf("DevFrontendURL = %v, want nil", cfg.LocalFrontendURL)
	}
}

func TestLocalFrontendAcceptsLoopbackHosts(t *testing.T) {
	for _, raw := range []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://[::1]:5173"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := Load(env(map[string]string{EnvUpstreamURL: "http://h", EnvLocalFrontendURL: raw})); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLocalFrontendRejectsNonLocalHosts(t *testing.T) {
	for _, raw := range []string{"http://example.com:5173", "http://10.0.0.5:5173", "5173", "ftp://localhost"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := Load(env(map[string]string{EnvUpstreamURL: "http://h", EnvLocalFrontendURL: raw})); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
