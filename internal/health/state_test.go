package health

import "testing"

func TestUnhealthyAfterThresholdFailures(t *testing.T) {
	s := newState(2, 3) // 2 to recover, 3 to eject; starts healthy
	if flip, _ := s.recordFailure(); flip {
		t.Fatal("1 failure should not flip")
	}
	if flip, _ := s.recordFailure(); flip {
		t.Fatal("2 failures should not flip")
	}
	flip, healthy := s.recordFailure()
	if !flip || healthy {
		t.Fatalf("3rd failure should flip to unhealthy; flip=%v healthy=%v", flip, healthy)
	}
}

func TestHealthyAfterThresholdSuccesses(t *testing.T) {
	s := newState(2, 3)
	s.recordFailure()
	s.recordFailure()
	s.recordFailure() // now unhealthy

	if flip, _ := s.recordSuccess(); flip {
		t.Fatal("1 success should not flip back")
	}
	flip, healthy := s.recordSuccess()
	if !flip || !healthy {
		t.Fatalf("2nd success should flip to healthy; flip=%v healthy=%v", flip, healthy)
	}
}

func TestInterruptedStreakResets(t *testing.T) {
	s := newState(2, 3)
	s.recordFailure()
	s.recordFailure()
	s.recordSuccess() // resets the failure streak
	if flip, _ := s.recordFailure(); flip {
		t.Fatal("streak should have reset; one failure must not eject")
	}
}
