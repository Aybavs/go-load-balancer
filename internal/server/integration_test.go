package server

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aybavs/go-load-balancer/internal/config"
)

// TestBackendEjectedThenRecovered drives the health checker deterministically
// via CheckOnce and asserts the healthy snapshot shrinks then grows.
func TestBackendEjectedThenRecovered(t *testing.T) {
	var up atomic.Bool
	up.Store(true)
	flaky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" && !up.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer flaky.Close()

	stable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer stable.Close()

	cfg := &config.Config{
		Algorithm: "round_robin",
		Backends: []config.BackendConfig{
			{URL: flaky.URL, Weight: 1},
			{URL: stable.URL, Weight: 1},
		},
		Health: config.HealthConfig{
			Path: "/healthz", Timeout: time.Second,
			HealthyThreshold: 1, UnhealthyThreshold: 1, PassiveThreshold: 5,
		},
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if got := len(srv.Pool().Healthy()); got != 2 {
		t.Fatalf("initial healthy = %d, want 2", got)
	}

	// Flaky backend goes down; one probe cycle should eject it.
	up.Store(false)
	srv.CheckOnce()
	if got := len(srv.Pool().Healthy()); got != 1 {
		t.Fatalf("after failure healthy = %d, want 1", got)
	}

	// Flaky backend recovers; one probe cycle should re-add it.
	up.Store(true)
	srv.CheckOnce()
	if got := len(srv.Pool().Healthy()); got != 2 {
		t.Fatalf("after recovery healthy = %d, want 2", got)
	}
}
