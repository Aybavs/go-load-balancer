package balancer

import (
	"testing"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

func TestLeastConnectionsPicksMin(t *testing.T) {
	b1, _ := backend.New("http://a:1", 1)
	b2, _ := backend.New("http://b:1", 1)
	b3, _ := backend.New("http://c:1", 1)
	b1.IncInFlight()
	b1.IncInFlight() // b1=2
	b2.IncInFlight() // b2=1
	// b3=0

	got := LeastConnections{}.Pick([]*backend.Backend{b1, b2, b3}, nil)
	if got != b3 {
		t.Fatalf("picked %v, want b3 (fewest in-flight)", got)
	}
}

func TestLeastConnectionsEmpty(t *testing.T) {
	if (LeastConnections{}).Pick(nil, nil) != nil {
		t.Fatal("empty snapshot must return nil")
	}
}
