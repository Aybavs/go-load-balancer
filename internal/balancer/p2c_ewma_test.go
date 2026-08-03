package balancer

import (
	"testing"
	"time"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

func TestP2CEmptyAndSingle(t *testing.T) {
	p := &P2CEWMA{}
	if p.Pick(nil, nil) != nil {
		t.Fatal("empty snapshot must return nil")
	}
	only, _ := backend.New("http://a:1", 1)
	if got := p.Pick([]*backend.Backend{only}, nil); got != only {
		t.Fatal("single snapshot must return the only backend")
	}
}

// With exactly two backends P2C compares both, so selection is deterministic.
func TestP2CPrefersLowerCostByLatency(t *testing.T) {
	slow, _ := backend.New("http://slow:1", 1)
	fast, _ := backend.New("http://fast:1", 1)
	slow.RecordLatency(100 * time.Millisecond)
	fast.RecordLatency(5 * time.Millisecond)

	p := &P2CEWMA{}
	for i := 0; i < 20; i++ {
		if got := p.Pick([]*backend.Backend{slow, fast}, nil); got != fast {
			t.Fatalf("iteration %d picked the slow backend", i)
		}
	}
}

func TestP2CPrefersFewerInFlightWhenLatencyEqual(t *testing.T) {
	a, _ := backend.New("http://a:1", 1)
	b, _ := backend.New("http://b:1", 1)
	a.RecordLatency(10 * time.Millisecond)
	b.RecordLatency(10 * time.Millisecond)
	a.IncInFlight()
	a.IncInFlight() // a has more load

	p := &P2CEWMA{}
	for i := 0; i < 20; i++ {
		if got := p.Pick([]*backend.Backend{a, b}, nil); got != b {
			t.Fatalf("iteration %d picked the busier backend", i)
		}
	}
}

func TestP2CDistributesAcrossManyBackends(t *testing.T) {
	bs := make([]*backend.Backend, 5)
	for i := range bs {
		bs[i], _ = backend.New("http://x:1", 1)
	}
	p := &P2CEWMA{}
	seen := map[*backend.Backend]bool{}
	for i := 0; i < 200; i++ {
		seen[p.Pick(bs, nil)] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected spread across backends, saw %d", len(seen))
	}
}
