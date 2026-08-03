package balancer

import (
	"math/rand/v2"
	"net/http"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

// P2CEWMA implements the "power of two choices" strategy scored by a latency
// EWMA weighted by in-flight requests. It samples two random backends and
// picks the one with the lower cost, avoiding the herd behaviour of always
// choosing the global minimum while reacting to latency, not just connections.
type P2CEWMA struct{}

// cost is lower for faster, less-loaded backends. With no latency samples yet
// (EWMA == 0) it reduces to InFlight()+1, i.e. least-connections behaviour.
func cost(b *backend.Backend) float64 {
	return (b.LatencyEWMA() + 1) * float64(b.InFlight()+1)
}

func (P2CEWMA) Pick(healthy []*backend.Backend, _ *http.Request) *backend.Backend {
	n := len(healthy)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return healthy[0]
	}
	i := rand.IntN(n)
	j := rand.IntN(n - 1)
	if j >= i {
		j++ // ensure j != i, uniformly
	}
	a, b := healthy[i], healthy[j]
	if cost(b) < cost(a) {
		return b
	}
	return a
}
