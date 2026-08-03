package backend

import "testing"

func TestNewStartsHealthy(t *testing.T) {
	b, err := New("http://localhost:9001", 1)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !b.IsHealthy() {
		t.Fatal("a new backend must start healthy")
	}
	if b.URL.Host != "localhost:9001" {
		t.Fatalf("URL.Host = %q, want localhost:9001", b.URL.Host)
	}
}

func TestInFlightIncDec(t *testing.T) {
	b, _ := New("http://localhost:9001", 1)
	b.IncInFlight()
	b.IncInFlight()
	if got := b.InFlight(); got != 2 {
		t.Fatalf("InFlight = %d, want 2", got)
	}
	b.DecInFlight()
	if got := b.InFlight(); got != 1 {
		t.Fatalf("InFlight = %d, want 1", got)
	}
}

func TestSetHealthy(t *testing.T) {
	b, _ := New("http://localhost:9001", 1)
	b.SetHealthy(false)
	if b.IsHealthy() {
		t.Fatal("expected unhealthy after SetHealthy(false)")
	}
}

func TestPassiveFailures(t *testing.T) {
	b, _ := New("http://localhost:9001", 1)
	if b.AddPassiveFailure() != 1 {
		t.Fatal("first AddPassiveFailure should return 1")
	}
	b.AddPassiveFailure()
	if b.PassiveFailures() != 2 {
		t.Fatalf("PassiveFailures = %d, want 2", b.PassiveFailures())
	}
	b.ResetPassiveFailures()
	if b.PassiveFailures() != 0 {
		t.Fatal("expected 0 after reset")
	}
}

func TestNewRejectsBadURL(t *testing.T) {
	if _, err := New("://bad", 1); err == nil {
		t.Fatal("expected error for invalid URL")
	}
}
