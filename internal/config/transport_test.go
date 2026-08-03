package config

import (
	"testing"
	"time"
)

func TestTransportDefaults(t *testing.T) {
	path := writeTemp(t, `
algorithm: round_robin
backends:
  - url: http://localhost:9001
`)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Transport.MaxIdleConns != 100 {
		t.Fatalf("MaxIdleConns = %d, want default 100", c.Transport.MaxIdleConns)
	}
	if c.Transport.MaxIdleConnsPerHost != 100 {
		t.Fatalf("MaxIdleConnsPerHost = %d, want default 100", c.Transport.MaxIdleConnsPerHost)
	}
	if c.Transport.IdleConnTimeout != 90*time.Second {
		t.Fatalf("IdleConnTimeout = %v, want default 90s", c.Transport.IdleConnTimeout)
	}
}

func TestTransportParsed(t *testing.T) {
	path := writeTemp(t, `
algorithm: round_robin
backends:
  - url: http://localhost:9001
transport:
  max_idle_conns: 512
  max_idle_conns_per_host: 256
  idle_conn_timeout: 30s
`)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Transport.MaxIdleConns != 512 || c.Transport.MaxIdleConnsPerHost != 256 {
		t.Fatalf("transport idle conns = %d/%d", c.Transport.MaxIdleConns, c.Transport.MaxIdleConnsPerHost)
	}
	if c.Transport.IdleConnTimeout != 30*time.Second {
		t.Fatalf("IdleConnTimeout = %v, want 30s", c.Transport.IdleConnTimeout)
	}
}
