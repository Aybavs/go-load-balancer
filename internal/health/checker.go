package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

type Options struct {
	Path               string
	Interval           time.Duration
	Timeout            time.Duration
	HealthyThreshold   int
	UnhealthyThreshold int
	PassiveThreshold   int
}

type Checker struct {
	pool   *backend.Pool
	opts   Options
	client *http.Client

	mu     sync.Mutex
	states map[*backend.Backend]*state
}

func NewChecker(pool *backend.Pool, opts Options) *Checker {
	c := &Checker{
		pool:   pool,
		opts:   opts,
		client: &http.Client{Timeout: opts.Timeout},
		states: make(map[*backend.Backend]*state),
	}
	for _, b := range pool.All() {
		st := newState(opts.HealthyThreshold, opts.UnhealthyThreshold)
		st.healthy = b.IsHealthy()
		c.states[b] = st
	}
	return c
}

// Start runs active probing until ctx is cancelled.
func (c *Checker) Start(ctx context.Context) {
	ticker := time.NewTicker(c.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkOnce()
		}
	}
}

// CheckOnce runs a single probe cycle. Exported for tests and manual triggers.
func (c *Checker) CheckOnce() { c.checkOnce() }

// checkOnce probes every backend once, concurrently, and applies results.
func (c *Checker) checkOnce() {
	var wg sync.WaitGroup
	var changedMu sync.Mutex
	changed := false

	for _, b := range c.pool.All() {
		wg.Add(1)
		go func(b *backend.Backend) {
			defer wg.Done()
			ok := c.probe(b)
			if c.apply(b, ok) {
				changedMu.Lock()
				changed = true
				changedMu.Unlock()
			}
		}(b)
	}
	wg.Wait()

	if changed {
		c.pool.RefreshHealthy()
	}
}

func (c *Checker) probe(b *backend.Backend) bool {
	target := *b.URL
	target.Path = c.opts.Path
	resp, err := c.client.Get(target.String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// apply records the probe result and returns whether health flipped.
func (c *Checker) apply(b *backend.Backend, ok bool) bool {
	c.mu.Lock()
	st := c.states[b]
	var flip, healthy bool
	if ok {
		flip, healthy = st.recordSuccess()
	} else {
		flip, healthy = st.recordFailure()
	}
	c.mu.Unlock()

	if flip {
		b.SetHealthy(healthy)
		if healthy {
			b.ResetPassiveFailures()
		}
	}
	return flip
}

// ReportPassiveFailure is called from the proxy path when a request to b fails
// at the transport level. It ejects b immediately once the passive threshold
// is crossed, without waiting for the next active probe.
func (c *Checker) ReportPassiveFailure(b *backend.Backend) {
	if b.AddPassiveFailure() < int64(c.opts.PassiveThreshold) {
		return
	}
	c.mu.Lock()
	st := c.states[b]
	flip := false
	var healthy bool
	if st != nil && st.healthy {
		st.healthy = false
		st.failureStreak = 0
		flip, healthy = true, false
	}
	c.mu.Unlock()

	if flip {
		b.SetHealthy(healthy)
		c.pool.RefreshHealthy()
	}
}
