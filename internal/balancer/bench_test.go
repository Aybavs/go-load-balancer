package balancer

import (
	"strconv"
	"testing"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

func makeBackends(n int) []*backend.Backend {
	bs := make([]*backend.Backend, n)
	for i := range bs {
		bs[i], _ = backend.New("http://10.0.0."+strconv.Itoa(i+1)+":8080", 1)
	}
	return bs
}

func BenchmarkRoundRobinPick(b *testing.B) {
	healthy := makeBackends(8)
	rr := &RoundRobin{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rr.Pick(healthy, nil)
	}
}

func BenchmarkLeastConnectionsPick(b *testing.B) {
	healthy := makeBackends(8)
	lc := LeastConnections{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lc.Pick(healthy, nil)
	}
}

func BenchmarkConsistentHashPick(b *testing.B) {
	healthy := makeBackends(8)
	ch := NewConsistentHash()
	ch.Pick(healthy, nil) // warm the ring
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ch.Pick(healthy, nil)
	}
}
func BenchmarkP2CEWMAPick(b *testing.B) {
	healthy := makeBackends(8)
	p2c := P2CEWMA{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = p2c.Pick(healthy, nil)
	}
}
