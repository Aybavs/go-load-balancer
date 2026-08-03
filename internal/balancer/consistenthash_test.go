package balancer

import (
	"net/http/httptest"
	"testing"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

func TestConsistentHashStableForSameKey(t *testing.T) {
	b1, _ := backend.New("http://a:1", 1)
	b2, _ := backend.New("http://b:1", 1)
	b3, _ := backend.New("http://c:1", 1)
	healthy := []*backend.Backend{b1, b2, b3}

	ch := NewConsistentHash()
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.5:5555"

	first := ch.Pick(healthy, req)
	for i := 0; i < 10; i++ {
		if got := ch.Pick(healthy, req); got != first {
			t.Fatalf("same key mapped to different backends: %v vs %v", got, first)
		}
	}
}

func TestConsistentHashDistributesKeys(t *testing.T) {
	b1, _ := backend.New("http://a:1", 1)
	b2, _ := backend.New("http://b:1", 1)
	healthy := []*backend.Backend{b1, b2}
	ch := NewConsistentHash()

	seen := map[*backend.Backend]bool{}
	for _, ip := range []string{"1.1.1.1:1", "2.2.2.2:2", "3.3.3.3:3", "4.4.4.4:4", "5.5.5.5:5"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip
		seen[ch.Pick(healthy, req)] = true
	}
	if len(seen) < 2 {
		t.Fatal("expected keys to spread across both backends")
	}
}

func TestConsistentHashEmpty(t *testing.T) {
	if NewConsistentHash().Pick(nil, nil) != nil {
		t.Fatal("empty snapshot must return nil")
	}
}
