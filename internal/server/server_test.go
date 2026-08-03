package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aybavs/go-load-balancer/internal/config"
)

func TestServerDistributesAcrossBackends(t *testing.T) {
	hits := map[string]int{}
	mk := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits[name]++
			io.WriteString(w, name)
		}))
	}
	a := mk("a")
	defer a.Close()
	b := mk("b")
	defer b.Close()

	cfg := &config.Config{
		Listen:    ":0",
		Algorithm: "round_robin",
		Backends: []config.BackendConfig{
			{URL: a.URL, Weight: 1},
			{URL: b.URL, Weight: 1},
		},
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	front := httptest.NewServer(srv.Handler())
	defer front.Close()

	for i := 0; i < 4; i++ {
		resp, err := http.Get(front.URL)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if hits["a"] != 2 || hits["b"] != 2 {
		t.Fatalf("uneven distribution: a=%d b=%d, want 2/2", hits["a"], hits["b"])
	}
}

func TestNewRejectsBadAlgorithm(t *testing.T) {
	cfg := &config.Config{Algorithm: "nope", Backends: []config.BackendConfig{{URL: "http://a:1"}}}
	if _, err := New(cfg); err == nil {
		t.Fatal("expected error for bad algorithm")
	}
}
