package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aybavs/go-load-balancer/internal/backend"
)

func TestCheckerEjectsDeadBackend(t *testing.T) {
	dead, _ := backend.New("http://127.0.0.1:1", 1) // nothing listening
	pool := backend.NewPool([]*backend.Backend{dead})

	c := NewChecker(pool, Options{
		Path: "/healthz", Timeout: 200 * time.Millisecond,
		HealthyThreshold: 1, UnhealthyThreshold: 1, PassiveThreshold: 5,
	})
	c.checkOnce()

	if dead.IsHealthy() {
		t.Fatal("dead backend should be ejected after 1 failed probe")
	}
	if len(pool.Healthy()) != 0 {
		t.Fatal("pool healthy snapshot should be empty")
	}
}

func TestCheckerRecoversBackend(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	b, _ := backend.New(up.URL, 1)
	b.SetHealthy(false) // start ejected
	pool := backend.NewPool([]*backend.Backend{b})

	c := NewChecker(pool, Options{
		Path: "/", Timeout: time.Second,
		HealthyThreshold: 1, UnhealthyThreshold: 1, PassiveThreshold: 5,
	})
	c.checkOnce()

	if !b.IsHealthy() {
		t.Fatal("backend should recover after 1 successful probe")
	}
	if len(pool.Healthy()) != 1 {
		t.Fatal("pool healthy snapshot should contain the recovered backend")
	}
}

func TestPassiveFailureEjects(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	b, _ := backend.New(up.URL, 1)
	pool := backend.NewPool([]*backend.Backend{b})
	c := NewChecker(pool, Options{
		Path: "/", Timeout: time.Second,
		HealthyThreshold: 1, UnhealthyThreshold: 3, PassiveThreshold: 2,
	})

	c.ReportPassiveFailure(b) // 1
	if !b.IsHealthy() {
		t.Fatal("should still be healthy below passive threshold")
	}
	c.ReportPassiveFailure(b) // 2 -> crosses threshold
	if b.IsHealthy() {
		t.Fatal("should be ejected once passive threshold is crossed")
	}
	if len(pool.Healthy()) != 0 {
		t.Fatal("pool healthy snapshot should be empty after passive ejection")
	}
}
