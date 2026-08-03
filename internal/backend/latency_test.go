package backend

import (
	"math"
	"testing"
	"time"
)

func TestLatencyEWMAZeroBeforeSample(t *testing.T) {
	b, _ := New("http://a:1", 1)
	if got := b.LatencyEWMA(); got != 0 {
		t.Fatalf("LatencyEWMA = %v, want 0 before any sample", got)
	}
}

func TestRecordLatencyFirstSampleSetsDirectly(t *testing.T) {
	b, _ := New("http://a:1", 1)
	b.RecordLatency(10 * time.Millisecond)
	if got := b.LatencyEWMA(); math.Abs(got-10) > 1e-6 {
		t.Fatalf("first sample: LatencyEWMA = %v, want 10", got)
	}
}

func TestRecordLatencyBlends(t *testing.T) {
	b, _ := New("http://a:1", 1)
	b.RecordLatency(10 * time.Millisecond) // ewma = 10
	b.RecordLatency(20 * time.Millisecond) // ewma = 0.2*20 + 0.8*10 = 12
	if got := b.LatencyEWMA(); math.Abs(got-12) > 1e-6 {
		t.Fatalf("blended: LatencyEWMA = %v, want 12", got)
	}
}
