package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aybavs/go-load-balancer/internal/backend"
	"github.com/aybavs/go-load-balancer/internal/balancer"
)

func TestHandlerLogsRequest(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer up.Close()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	b, _ := backend.New(up.URL, 1)
	pool := backend.NewPool([]*backend.Backend{b})
	h := NewHandler(pool, &balancer.RoundRobin{}, 0, nil, nil, logger)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders", nil))

	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("expected one JSON log line, got %q: %v", buf.String(), err)
	}
	if line["status"] != float64(200) {
		t.Fatalf("logged status = %v, want 200", line["status"])
	}
	if line["backend"] == nil {
		t.Fatal("log line must include the chosen backend")
	}
	if line["path"] != "/orders" {
		t.Fatalf("logged path = %v, want /orders", line["path"])
	}
}
