// Package backend models a single upstream target and a pool of targets.
package backend

import (
	"errors"
	"net/url"
	"sync/atomic"
)

// Backend is one upstream server. URL and Weight are immutable after New;
// all other state is accessed via atomics so the request hot path is lock-free.
type Backend struct {
	URL    *url.URL
	Weight int

	inFlight        atomic.Int64
	healthy         atomic.Bool
	passiveFailures atomic.Int64
}

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
