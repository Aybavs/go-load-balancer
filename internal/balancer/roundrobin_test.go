package balancer

import (
	"testing"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

func TestRoundRobinCyclesInOrder(t *testing.T) {
	b1, _ := backend.New("http://a:1", 1)
	b2, _ := backend.New("http://b:1", 1)
	b3, _ := backend.New("http://c:1", 1)
	healthy := []*backend.Backend{b1, b2, b3}

	rr := &RoundRobin{}
	got := []*backend.Backend{
		rr.Pick(healthy, nil),
		rr.Pick(healthy, nil),
		rr.Pick(healthy, nil),
		rr.Pick(healthy, nil),
	}
	want := []*backend.Backend{b1, b2, b3, b1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pick %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestRoundRobinEmpty(t *testing.T) {
	rr := &RoundRobin{}
	if rr.Pick(nil, nil) != nil {
		t.Fatal("Pick on empty snapshot must return nil")
	}
}
