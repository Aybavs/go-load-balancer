// Package server wires config into a running load balancer with graceful shutdown.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/aybavs/go-load-balancer/internal/backend"
	"github.com/aybavs/go-load-balancer/internal/balancer"
	"github.com/aybavs/go-load-balancer/internal/config"
	"github.com/aybavs/go-load-balancer/internal/health"
	"github.com/aybavs/go-load-balancer/internal/metrics"
	"github.com/aybavs/go-load-balancer/internal/proxy"
)

type Server struct {
	cfg     *config.Config
	pool    *backend.Pool
	checker *health.Checker
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

	checker := health.NewChecker(pool, health.Options{
		Path:               cfg.Health.Path,
		Interval:           cfg.Health.Interval,
		Timeout:            cfg.Health.Timeout,
		HealthyThreshold:   cfg.Health.HealthyThreshold,
		UnhealthyThreshold: cfg.Health.UnhealthyThreshold,
		PassiveThreshold:   cfg.Health.PassiveThreshold,
	})

	reg := metrics.NewRegistry(pool)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	transport := buildTransport(cfg.Transport)
	handler := proxy.NewHandler(pool, algo, cfg.Proxy.MaxRetries, checker.ReportPassiveFailure, reg, logger, transport)

	mux := http.NewServeMux()
	mux.Handle("/metrics", reg.Handler())
	mux.Handle("/", handler)

	return &Server{cfg: cfg, pool: pool, checker: checker, handler: mux}, nil
}

// buildTransport clones the default transport and applies the connection-pool
// tuning from config. The default MaxIdleConnsPerHost of 2 forces connection
// churn under concurrency; raising it materially lowers proxy overhead.
func buildTransport(cfg config.TransportConfig) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConns = cfg.MaxIdleConns
	tr.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	tr.IdleConnTimeout = cfg.IdleConnTimeout
	return tr
}

func (s *Server) Handler() http.Handler { return s.handler }
func (s *Server) Pool() *backend.Pool   { return s.pool }

// CheckOnce triggers one health probe cycle. Test/ops helper.
func (s *Server) CheckOnce() { s.checker.CheckOnce() }

// Run starts the HTTP server and the health checker and blocks until ctx is
// cancelled, then drains in-flight requests.
func (s *Server) Run(ctx context.Context) error {
	go s.checker.Start(ctx)

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
