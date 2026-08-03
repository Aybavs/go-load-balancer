// Package server wires config into a running load balancer with graceful
// shutdown and atomic config hot-reload.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/aybavs/go-load-balancer/internal/backend"
	"github.com/aybavs/go-load-balancer/internal/balancer"
	"github.com/aybavs/go-load-balancer/internal/config"
	"github.com/aybavs/go-load-balancer/internal/health"
	"github.com/aybavs/go-load-balancer/internal/metrics"
	"github.com/aybavs/go-load-balancer/internal/proxy"
)

// generation is one immutable set of serving components built from a config.
// Reload swaps the current generation atomically; in-flight requests keep using
// the generation they started on, so removed backends drain cleanly.
type generation struct {
	cfg     *config.Config
	pool    *backend.Pool
	checker *health.Checker
	handler http.Handler
	cancel  context.CancelFunc // cancels this generation's checker; nil until started
}

type Server struct {
	logger  *slog.Logger
	outer   http.Handler
	current atomic.Pointer[generation]

	mu      sync.Mutex      // serializes Reload
	baseCtx context.Context // set by Run; nil means "not running"
}

func New(cfg *config.Config) (*Server, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	g, err := buildGeneration(cfg, logger)
	if err != nil {
		return nil, err
	}
	s := &Server{logger: logger}
	s.current.Store(g)
	// Stable outer handler: every request delegates to the current generation.
	s.outer = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.current.Load().handler.ServeHTTP(w, r)
	})
	return s, nil
}

func buildGeneration(cfg *config.Config, logger *slog.Logger) (*generation, error) {
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
	transport := buildTransport(cfg.Transport)
	handler := proxy.NewHandler(pool, algo, cfg.Proxy.MaxRetries, checker.ReportPassiveFailure, reg, logger, transport)

	mux := http.NewServeMux()
	mux.Handle("/metrics", reg.Handler())
	mux.Handle("/", handler)

	return &generation{cfg: cfg, pool: pool, checker: checker, handler: mux}, nil
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

func (s *Server) Handler() http.Handler { return s.outer }
func (s *Server) Pool() *backend.Pool   { return s.current.Load().pool }

// CheckOnce triggers one health probe cycle on the current generation.
func (s *Server) CheckOnce() { s.current.Load().checker.CheckOnce() }

// startChecker starts a generation's health checker under a child of the base
// context and records its cancel func. No-op if the server is not running yet.
func (s *Server) startChecker(g *generation) {
	if s.baseCtx == nil {
		return
	}
	ctx, cancel := context.WithCancel(s.baseCtx)
	g.cancel = cancel
	go g.checker.Start(ctx)
}

// Reload builds a new generation from cfg and swaps it in atomically. In-flight
// requests finish on the previous generation; backends dropped from cfg drain.
// Everything is rebuilt except the bound listener. On an invalid config the old
// generation keeps serving and an error is returned.
func (s *Server) Reload(cfg *config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next, err := buildGeneration(cfg, s.logger)
	if err != nil {
		return err
	}
	s.startChecker(next)
	old := s.current.Swap(next)
	if old != nil && old.cancel != nil {
		old.cancel()
	}
	return nil
}

// Run starts the HTTP server and the current generation's health checker, then
// blocks until ctx is cancelled and drains in-flight requests.
func (s *Server) Run(ctx context.Context) error {
	s.mu.Lock()
	s.baseCtx = ctx
	s.startChecker(s.current.Load())
	s.mu.Unlock()

	httpSrv := &http.Server{Addr: s.current.Load().cfg.Listen, Handler: s.outer}

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), s.current.Load().cfg.ShutdownTimeout)
		defer cancel()
		return httpSrv.Shutdown(shutCtx)
	}
}
