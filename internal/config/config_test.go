package config

import (
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{EnvUpstreamURL: "http://localhost:8080"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8081" || cfg.StaticDir != "static" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.MaxBodyBytes != 55<<20 {
		t.Errorf("MaxBodyBytes = %d", cfg.MaxBodyBytes)
	}
	if cfg.UpstreamURL.Host != "localhost:8080" {
		t.Errorf("UpstreamURL = %v", cfg.UpstreamURL)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		EnvUpstreamURL: "https://api.internal",
		EnvAddr:        ":9000",
		EnvStaticDir:   "/srv/fe",
		EnvMaxBodyMB:   "10",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9000" || cfg.StaticDir != "/srv/fe" || cfg.MaxBodyBytes != 10<<20 {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	tests := map[string]map[string]string{
		"missing upstream":   {},
		"relative upstream":  {EnvUpstreamURL: "/api"},
		"unsupported scheme": {EnvUpstreamURL: "ftp://host"},
		"no host":            {EnvUpstreamURL: "http://"},
		"non numeric limit":  {EnvUpstreamURL: "http://h", EnvMaxBodyMB: "lots"},
		"zero limit":         {EnvUpstreamURL: "http://h", EnvMaxBodyMB: "0"},
		"negative limit":     {EnvUpstreamURL: "http://h", EnvMaxBodyMB: "-5"},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(env(values)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
