package balancer

import (
	"net/http"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

// LeastConnections picks the healthy backend with the fewest in-flight requests.
type LeastConnections struct{}

func (LeastConnections) Pick(healthy []*backend.Backend, _ *http.Request) *backend.Backend {
	if len(healthy) == 0 {
		return nil
	}
	best := healthy[0]
	bestN := best.InFlight()
	for _, b := range healthy[1:] {
		if n := b.InFlight(); n < bestN {
			best, bestN = b, n
		}
	}
	return best
}
