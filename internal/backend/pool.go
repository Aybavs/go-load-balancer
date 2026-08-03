package backend

import (
	"sync"
	"sync/atomic"
)

// Pool holds the authoritative backend list and publishes an immutable
// snapshot of the currently healthy backends. Readers (the request hot path)
// call Healthy() for a single atomic load with no lock. Writers (health
// checker, config reload) are rare and rebuild the snapshot copy-on-write.
type Pool struct {
	writeMu sync.Mutex // serializes writers only; never held on the read path
	all     atomic.Pointer[[]*Backend]
	healthy atomic.Pointer[[]*Backend]
}

func NewPool(backends []*Backend) *Pool {
	p := &Pool{}
	all := append([]*Backend(nil), backends...)
	p.all.Store(&all)
	p.rebuildHealthy()
	return p
}

// All returns the full backend list snapshot.
func (p *Pool) All() []*Backend { return *p.all.Load() }

// Healthy returns the snapshot of healthy backends. Lock-free; hot path.
func (p *Pool) Healthy() []*Backend { return *p.healthy.Load() }

// RefreshHealthy recomputes the healthy snapshot from current health flags.
// Call after any backend's health changes.
func (p *Pool) RefreshHealthy() {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	p.rebuildHealthy()
}

// Replace swaps the entire backend list (config reload).
func (p *Pool) Replace(backends []*Backend) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	all := append([]*Backend(nil), backends...)
	p.all.Store(&all)
	p.rebuildHealthy()
}

func (p *Pool) rebuildHealthy() {
	all := *p.all.Load()
	h := make([]*Backend, 0, len(all))
	for _, b := range all {
		if b.IsHealthy() {
			h = append(h, b)
		}
	}
	p.healthy.Store(&h)
}
