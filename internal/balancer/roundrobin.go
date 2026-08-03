package balancer

import (
	"net/http"
	"sync/atomic"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

// RoundRobin distributes requests evenly using a single atomic counter.
type RoundRobin struct {
	counter atomic.Uint64
}

func (rr *RoundRobin) Pick(healthy []*backend.Backend, _ *http.Request) *backend.Backend {
	n := len(healthy)
	if n == 0 {
		return nil
	}
	i := rr.counter.Add(1) - 1
	return healthy[i%uint64(n)]
}
