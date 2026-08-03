// Package metrics tracks request counters and exposes a JSON snapshot.
package metrics

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

type backendCounters struct {
	requests atomic.Int64
	errors   atomic.Int64
	totalNs  atomic.Int64
}

type Registry struct {
	pool          *backend.Pool
	totalRequests atomic.Int64
	totalErrors   atomic.Int64

	mu       sync.RWMutex
	counters map[*backend.Backend]*backendCounters
}

func NewRegistry(pool *backend.Pool) *Registry {
	m := &Registry{pool: pool, counters: make(map[*backend.Backend]*backendCounters)}
	for _, b := range pool.All() {
		m.counters[b] = &backendCounters{}
	}
	return m
}

// Observe records one completed request. statusClass is the HTTP status / 100
// (e.g. 2 for 2xx, 5 for 5xx). A class >= 5 counts as an error.
func (m *Registry) Observe(b *backend.Backend, statusClass int, dur time.Duration) {
	m.totalRequests.Add(1)
	if statusClass >= 5 {
		m.totalErrors.Add(1)
	}
	m.mu.RLock()
	c := m.counters[b]
	m.mu.RUnlock()
	if c == nil {
		return
	}
	c.requests.Add(1)
	if statusClass >= 5 {
		c.errors.Add(1)
	}
	c.totalNs.Add(dur.Nanoseconds())
}

type backendSnapshot struct {
	URL          string  `json:"url"`
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	InFlight     int64   `json:"in_flight"`
	Healthy      bool    `json:"healthy"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

type snapshot struct {
	TotalRequests int64             `json:"total_requests"`
	TotalErrors   int64             `json:"total_errors"`
	Backends      []backendSnapshot `json:"backends"`
}

func (m *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snap := snapshot{
			TotalRequests: m.totalRequests.Load(),
			TotalErrors:   m.totalErrors.Load(),
		}
		for _, b := range m.pool.All() {
			m.mu.RLock()
			c := m.counters[b]
			m.mu.RUnlock()
			bs := backendSnapshot{URL: b.URL.String(), InFlight: b.InFlight(), Healthy: b.IsHealthy()}
			if c != nil {
				reqs := c.requests.Load()
				bs.Requests = reqs
				bs.Errors = c.errors.Load()
				if reqs > 0 {
					bs.AvgLatencyMs = float64(c.totalNs.Load()) / float64(reqs) / 1e6
				}
			}
			snap.Backends = append(snap.Backends, bs)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snap)
	})
}
