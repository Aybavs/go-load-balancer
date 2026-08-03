package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/aybavs/go-load-balancer/internal/config"
)

// TestConcurrentRequestsWhileHealthFlips hammers the hot path from many
// goroutines while another goroutine continuously flips backend health and
// rebuilds the snapshot. It must be `-race` clean and never panic.
func TestConcurrentRequestsWhileHealthFlips(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer up.Close()

	cfg := &config.Config{
		Algorithm: "least_connections",
		Backends: []config.BackendConfig{
			{URL: up.URL, Weight: 1},
			{URL: up.URL, Weight: 1},
			{URL: up.URL, Weight: 1},
		},
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(srv.Handler())
	defer front.Close()

	var load sync.WaitGroup

	// Load: many concurrent clients.
	const clients, perClient = 40, 50
	for i := 0; i < clients; i++ {
		load.Add(1)
		go func() {
			defer load.Done()
			c := &http.Client{}
			for j := 0; j < perClient; j++ {
				resp, err := c.Get(front.URL)
				if err == nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}
		}()
	}

	// Flipper: continuously eject/recover backends until the load finishes.
	stop := make(chan struct{})
	var flipper sync.WaitGroup
	flipper.Add(1)
	go func() {
		defer flipper.Done()
		all := srv.Pool().All()
		for {
			select {
			case <-stop:
				return
			default:
				for _, b := range all {
					b.SetHealthy(!b.IsHealthy())
				}
				srv.Pool().RefreshHealthy()
			}
		}
	}()

	load.Wait()
	close(stop)
	flipper.Wait()
}
