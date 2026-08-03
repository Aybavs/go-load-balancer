package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aybavs/go-load-balancer/internal/backend"
	"github.com/aybavs/go-load-balancer/internal/balancer"
)

func TestServeHTTPProxiesToBackend(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-From", "upstream")
		io.WriteString(w, "hello")
	}))
	defer upstream.Close()

	b, _ := backend.New(upstream.URL, 1)
	pool := backend.NewPool([]*backend.Backend{b})
	h := NewHandler(pool, &balancer.RoundRobin{}, 0, nil, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("body = %q, want hello", rec.Body.String())
	}
	if rec.Header().Get("X-From") != "upstream" {
		t.Fatal("expected upstream header to pass through")
	}
}

func TestServeHTTPNoHealthyBackends(t *testing.T) {
	b, _ := backend.New("http://127.0.0.1:1", 1)
	b.SetHealthy(false)
	pool := backend.NewPool([]*backend.Backend{b})
	h := NewHandler(pool, &balancer.RoundRobin{}, 0, nil, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestServeHTTPRetriesOnDeadBackend(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer good.Close()

	dead, _ := backend.New("http://127.0.0.1:1", 1) // nothing listening
	goodB, _ := backend.New(good.URL, 1)
	pool := backend.NewPool([]*backend.Backend{dead, goodB})

	var failed []*backend.Backend
	h := NewHandler(pool, &balancer.RoundRobin{}, 1, func(b *backend.Backend) {
		failed = append(failed, b)
	}, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("expected retry to reach good backend; got %d %q", rec.Code, rec.Body.String())
	}
	if len(failed) == 0 || failed[0] != dead {
		t.Fatal("expected onFailure to be called for the dead backend")
	}
}
