package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadValid(t *testing.T) {
	path := writeTemp(t, `
listen: ":8080"
algorithm: round_robin
shutdown_timeout: 5s
backends:
  - url: http://localhost:9001
    weight: 1
  - url: http://localhost:9002
health:
  path: /healthz
  interval: 2s
  timeout: 1s
  healthy_threshold: 2
  unhealthy_threshold: 3
proxy:
  max_retries: 1
`)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Listen != ":8080" {
		t.Fatalf("Listen = %q", c.Listen)
	}
	if len(c.Backends) != 2 {
		t.Fatalf("Backends len = %d, want 2", len(c.Backends))
	}
	if c.Backends[1].Weight != 1 {
		t.Fatalf("default weight = %d, want 1", c.Backends[1].Weight)
	}
	if c.Health.Interval != 2*time.Second {
		t.Fatalf("Health.Interval = %v", c.Health.Interval)
	}
	if c.ShutdownTimeout != 5*time.Second {
		t.Fatalf("ShutdownTimeout = %v", c.ShutdownTimeout)
	}
}

func TestValidateRejectsNoBackends(t *testing.T) {
	path := writeTemp(t, `listen: ":8080"`+"\n"+`algorithm: round_robin`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error when no backends configured")
	}
}

func TestValidateRejectsUnknownAlgorithm(t *testing.T) {
	path := writeTemp(t, `
listen: ":8080"
algorithm: magic
backends:
  - url: http://localhost:9001
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for unknown algorithm")
	}
}
