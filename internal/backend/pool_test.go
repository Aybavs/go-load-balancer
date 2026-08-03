package backend

import "testing"

func mustBackend(t *testing.T, raw string) *Backend {
	t.Helper()
	b, err := New(raw, 1)
	if err != nil {
		t.Fatalf("New(%q): %v", raw, err)
	}
	return b
}

func TestPoolHealthySnapshot(t *testing.T) {
	b1 := mustBackend(t, "http://a:1")
	b2 := mustBackend(t, "http://b:1")
	p := NewPool([]*Backend{b1, b2})

	if len(p.Healthy()) != 2 {
		t.Fatalf("Healthy len = %d, want 2", len(p.Healthy()))
	}

	b2.SetHealthy(false)
	p.RefreshHealthy()

	h := p.Healthy()
	if len(h) != 1 || h[0] != b1 {
		t.Fatalf("after ejecting b2, Healthy = %v, want [b1]", h)
	}
	if len(p.All()) != 2 {
		t.Fatal("All() must still contain both backends")
	}
}

func TestPoolReplace(t *testing.T) {
	p := NewPool([]*Backend{mustBackend(t, "http://a:1")})
	p.Replace([]*Backend{mustBackend(t, "http://b:1"), mustBackend(t, "http://c:1")})
	if len(p.All()) != 2 {
		t.Fatalf("All len = %d, want 2", len(p.All()))
	}
	if len(p.Healthy()) != 2 {
		t.Fatalf("Healthy len = %d, want 2", len(p.Healthy()))
	}
}
