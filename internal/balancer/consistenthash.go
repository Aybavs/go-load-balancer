package balancer

import (
	"hash/fnv"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

const virtualNodes = 100

// ConsistentHash maps a request key (client IP) onto a hash ring of backends,
// so the same client tends to reach the same backend (cache affinity).
//
// The ring is published via an atomic pointer, so Pick reads it lock-free on
// the hot path. It is rebuilt (under a mutex) only when the healthy set size
// changes — a V1 heuristic; a production ring would key rebuilds on set
// identity, not just size.
type ConsistentHash struct {
	mu   sync.Mutex
	ring atomic.Pointer[[]ringPoint]
}

type ringPoint struct {
	hash uint32
	b    *backend.Backend
}

func NewConsistentHash() *ConsistentHash { return &ConsistentHash{} }

func hashKey(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

func (c *ConsistentHash) rebuild(healthy []*backend.Backend) []ringPoint {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine may have rebuilt while we waited.
	if cur := c.ring.Load(); cur != nil && len(*cur) == len(healthy)*virtualNodes {
		return *cur
	}

	ring := make([]ringPoint, 0, len(healthy)*virtualNodes)
	for _, b := range healthy {
		for i := 0; i < virtualNodes; i++ {
			ring = append(ring, ringPoint{
				hash: hashKey(b.URL.String() + "#" + strconv.Itoa(i)),
				b:    b,
			})
		}
	}
	sort.Slice(ring, func(i, j int) bool { return ring[i].hash < ring[j].hash })
	c.ring.Store(&ring)
	return ring
}

func (c *ConsistentHash) Pick(healthy []*backend.Backend, r *http.Request) *backend.Backend {
	if len(healthy) == 0 {
		return nil
	}
	rp := c.ring.Load()
	var ring []ringPoint
	if rp == nil || len(*rp) != len(healthy)*virtualNodes {
		ring = c.rebuild(healthy)
	} else {
		ring = *rp
	}

	key := "default"
	if r != nil {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			key = host
		} else {
			key = r.RemoteAddr
		}
	}
	target := hashKey(key)
	idx := sort.Search(len(ring), func(i int) bool { return ring[i].hash >= target })
	if idx == len(ring) {
		idx = 0
	}
	return ring[idx].b
}
