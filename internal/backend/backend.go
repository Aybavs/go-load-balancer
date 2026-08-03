// Package backend models a single upstream target and a pool of targets.
package backend

import (
	"errors"
	"math"
	"net/url"
	"sync/atomic"
	"time"
)

// Backend is one upstream server. URL and Weight are immutable after New;
// all other state is accessed via atomics so the request hot path is lock-free.
type Backend struct {
	URL    *url.URL
	Weight int

	inFlight        atomic.Int64
	healthy         atomic.Bool
	passiveFailures atomic.Int64
	latencyEWMA     atomic.Uint64 // math.Float64bits of the EWMA in ms; 0 = no sample
}

const ewmaAlpha = 0.2

// New parses rawURL and returns a Backend that starts healthy.
func New(rawURL string, weight int) (*Backend, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, errors.New("backend URL must include a host")
	}
	if weight <= 0 {
		weight = 1
	}
	b := &Backend{URL: u, Weight: weight}
	b.healthy.Store(true)
	return b, nil
}

func (b *Backend) IncInFlight()    { b.inFlight.Add(1) }
func (b *Backend) DecInFlight()    { b.inFlight.Add(-1) }
func (b *Backend) InFlight() int64 { return b.inFlight.Load() }

func (b *Backend) IsHealthy() bool   { return b.healthy.Load() }
func (b *Backend) SetHealthy(v bool) { b.healthy.Store(v) }

func (b *Backend) PassiveFailures() int64   { return b.passiveFailures.Load() }
func (b *Backend) AddPassiveFailure() int64 { return b.passiveFailures.Add(1) }
func (b *Backend) ResetPassiveFailures()    { b.passiveFailures.Store(0) }

// RecordLatency folds an observed request latency into the backend's EWMA
// (milliseconds). The first sample seeds the average directly so a fresh or
// idle backend does not appear artificially fast; later samples blend at
// ewmaAlpha. Lock-free via a compare-and-swap loop.
func (b *Backend) RecordLatency(d time.Duration) {
	ms := float64(d) / float64(time.Millisecond)
	if ms <= 0 {
		ms = math.SmallestNonzeroFloat64 // keep it positive; 0 is the "unset" sentinel
	}
	for {
		old := b.latencyEWMA.Load()
		var next float64
		if old == 0 {
			next = ms
		} else {
			next = ewmaAlpha*ms + (1-ewmaAlpha)*math.Float64frombits(old)
		}
		if b.latencyEWMA.CompareAndSwap(old, math.Float64bits(next)) {
			return
		}
	}
}

// LatencyEWMA returns the current latency EWMA in milliseconds (0 if no sample).
func (b *Backend) LatencyEWMA() float64 {
	return math.Float64frombits(b.latencyEWMA.Load())
}
