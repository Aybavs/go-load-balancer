// Package server wires config into a running load balancer with graceful shutdown.
package server

import (
	"context"
	"net/http"

	"github.com/aybavs/go-load-balancer/internal/backend"
	"github.com/aybavs/go-load-balancer/internal/balancer"
	"github.com/aybavs/go-load-balancer/internal/config"
	"github.com/aybavs/go-load-balancer/internal/proxy"
)

type Server struct {
	cfg     *config.Config
	pool    *backend.Pool
	handler http.Handler
}

func New(cfg *config.Config) (*Server, error) {
	algo, err := balancer.NewFromName(cfg.Algorithm)
	if err != nil {
		return nil, err
	}
	backends := make([]*backend.Backend, 0, len(cfg.Backends))
	for _, bc := range cfg.Backends {
		b, err := backend.New(bc.URL, bc.Weight)
		if err != nil {
			return nil, err
		}
		backends = append(backends, b)
	}
	pool := backend.NewPool(backends)

	// onFailure records a passive failure; the health checker consumes it later.
	handler := proxy.NewHandler(pool, algo, cfg.Proxy.MaxRetries, func(b *backend.Backend) {
		b.AddPassiveFailure()
	})

	return &Server{cfg: cfg, pool: pool, handler: handler}, nil
}

func (s *Server) Handler() http.Handler { return s.handler }
func (s *Server) Pool() *backend.Pool   { return s.pool }

// Run starts the HTTP server and blocks until ctx is cancelled, then drains.
func (s *Server) Run(ctx context.Context) error {
	httpSrv := &http.Server{Addr: s.cfg.Listen, Handler: s.handler}

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		return httpSrv.Shutdown(shutCtx)
	}
}
