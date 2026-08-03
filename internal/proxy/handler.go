// Package proxy selects a backend and forwards the request to it, retrying
// idempotent requests on transport failure and reporting failures upstream.
package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/aybavs/go-load-balancer/internal/backend"
	"github.com/aybavs/go-load-balancer/internal/balancer"
)

type ctxKey int

const stateKey ctxKey = 0

type reqState struct{ failed bool }

// Observer records a completed request (status class = HTTP status / 100).
type Observer interface {
	Observe(b *backend.Backend, statusClass int, dur time.Duration)
}

// failTrackingWriter records whether any response bytes/headers were written
// (so we know whether a retry is still possible) and the status code.
type failTrackingWriter struct {
	http.ResponseWriter
	wrote  bool
	status int
}

func (w *failTrackingWriter) WriteHeader(code int) {
	w.wrote = true
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *failTrackingWriter) Write(b []byte) (int, error) {
	w.wrote = true
	return w.ResponseWriter.Write(b)
}

type Handler struct {
	pool       *backend.Pool
	algo       balancer.Algorithm
	maxRetries int
	onFailure  func(*backend.Backend)
	observer   Observer
	proxies    map[*backend.Backend]*httputil.ReverseProxy
}

func NewHandler(pool *backend.Pool, algo balancer.Algorithm, maxRetries int, onFailure func(*backend.Backend), observer Observer) *Handler {
	h := &Handler{
		pool:       pool,
		algo:       algo,
		maxRetries: maxRetries,
		onFailure:  onFailure,
		observer:   observer,
		proxies:    make(map[*backend.Backend]*httputil.ReverseProxy),
	}
	for _, b := range pool.All() {
		h.proxies[b] = h.newReverseProxy(b)
	}
	return h
}

func (h *Handler) newReverseProxy(b *backend.Backend) *httputil.ReverseProxy {
	rp := httputil.NewSingleHostReverseProxy(b.URL)
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if st, ok := r.Context().Value(stateKey).(*reqState); ok {
			st.failed = true
		}
	}
	return rp
}

func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	healthy := h.pool.Healthy()
	if len(healthy) == 0 {
		http.Error(w, "no healthy backends", http.StatusServiceUnavailable)
		return
	}

	tried := make(map[*backend.Backend]bool, len(healthy))
	ftw := &failTrackingWriter{ResponseWriter: w, status: http.StatusOK}

	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		b := h.pickUntried(healthy, r, tried)
		if b == nil {
			break
		}
		tried[b] = true

		rp := h.proxies[b]
		if rp == nil {
			continue
		}

		st := &reqState{}
		ctx := context.WithValue(r.Context(), stateKey, st)
		start := time.Now()
		b.IncInFlight()
		rp.ServeHTTP(ftw, r.WithContext(ctx))
		b.DecInFlight()

		if !st.failed {
			if h.observer != nil {
				h.observer.Observe(b, ftw.status/100, time.Since(start))
			}
			return
		}
		if h.onFailure != nil {
			h.onFailure(b)
		}
		// If bytes already went to the client, we cannot retry safely.
		if ftw.wrote || !isIdempotent(r.Method) {
			break
		}
	}

	if !ftw.wrote {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}
}

func (h *Handler) pickUntried(healthy []*backend.Backend, r *http.Request, tried map[*backend.Backend]bool) *backend.Backend {
	if b := h.algo.Pick(healthy, r); b != nil && !tried[b] {
		return b
	}
	for _, b := range healthy {
		if !tried[b] {
			return b
		}
	}
	return nil
}
