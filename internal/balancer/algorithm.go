// Package balancer selects a backend from a healthy snapshot.
package balancer

import (
	"net/http"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

// Algorithm picks one backend from the given healthy snapshot.
// It must be safe for concurrent use and must return nil when healthy is empty.
type Algorithm interface {
	Pick(healthy []*backend.Backend, r *http.Request) *backend.Backend
}
