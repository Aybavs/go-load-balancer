package proxy

import (
	"net/http"
	"testing"

	"github.com/aybavs/go-load-balancer/internal/backend"
	"github.com/aybavs/go-load-balancer/internal/balancer"
)

func TestHandlerSharesProvidedTransport(t *testing.T) {
	b1, _ := backend.New("http://a:1", 1)
	b2, _ := backend.New("http://b:1", 1)
	pool := backend.NewPool([]*backend.Backend{b1, b2})

	tr := &http.Transport{MaxIdleConnsPerHost: 77}
	h := NewHandler(pool, &balancer.RoundRobin{}, 0, nil, nil, nil, tr)

	for _, rp := range h.proxies {
		if rp.Transport != tr {
			t.Fatal("every reverse proxy must use the provided shared transport")
		}
	}
}
