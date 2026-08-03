package metrics

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

func TestObserveAndSnapshot(t *testing.T) {
	b, _ := backend.New("http://a:1", 1)
	pool := backend.NewPool([]*backend.Backend{b})
	m := NewRegistry(pool)

	m.Observe(b, 2, 10*time.Millisecond) // 2xx
	m.Observe(b, 2, 20*time.Millisecond)
	m.Observe(b, 5, 5*time.Millisecond) // 5xx

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	m.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var out struct {
		TotalRequests int64 `json:"total_requests"`
		TotalErrors   int64 `json:"total_errors"`
		Backends      []struct {
			URL      string `json:"url"`
			Requests int64  `json:"requests"`
			Healthy  bool   `json:"healthy"`
		} `json:"backends"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if out.TotalRequests != 3 {
		t.Fatalf("total_requests = %d, want 3", out.TotalRequests)
	}
	if out.TotalErrors != 1 {
		t.Fatalf("total_errors = %d, want 1", out.TotalErrors)
	}
	if len(out.Backends) != 1 || out.Backends[0].Requests != 3 {
		t.Fatalf("backend requests = %+v", out.Backends)
	}
}
