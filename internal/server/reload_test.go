package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aybavs/go-load-balancer/internal/config"
)

func nameServer(name string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, name)
	}))
}

func getBody(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestReloadSwapsBackends(t *testing.T) {
	a := nameServer("a")
	defer a.Close()
	b := nameServer("b")
	defer b.Close()

	srv, err := New(&config.Config{
		Algorithm: "round_robin",
		Backends:  []config.BackendConfig{{URL: a.URL, Weight: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(srv.Handler())
	defer front.Close()

	if got := getBody(t, front.URL); got != "a" {
		t.Fatalf("before reload got %q, want a", got)
	}

	if err := srv.Reload(&config.Config{
		Algorithm: "round_robin",
		Backends:  []config.BackendConfig{{URL: b.URL, Weight: 1}},
	}); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if got := getBody(t, front.URL); got != "b" {
		t.Fatalf("after reload got %q, want b", got)
	}
	if n := len(srv.Pool().Healthy()); n != 1 {
		t.Fatalf("healthy = %d, want 1 after reload", n)
	}
}

func TestReloadRejectsInvalidConfigAndKeepsServing(t *testing.T) {
	a := nameServer("a")
	defer a.Close()

	srv, err := New(&config.Config{
		Algorithm: "round_robin",
		Backends:  []config.BackendConfig{{URL: a.URL, Weight: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(srv.Handler())
	defer front.Close()

	if err := srv.Reload(&config.Config{
		Algorithm: "does_not_exist",
		Backends:  []config.BackendConfig{{URL: a.URL, Weight: 1}},
	}); err == nil {
		t.Fatal("expected Reload to reject an invalid algorithm")
	}
	if got := getBody(t, front.URL); got != "a" {
		t.Fatalf("after failed reload got %q, want still a", got)
	}
}
