// Package health tracks backend health via active probes and passive failures.
package health

type state struct {
	healthyThreshold   int
	unhealthyThreshold int
	healthy            bool
	successStreak      int
	failureStreak      int
}

func newState(healthyThreshold, unhealthyThreshold int) *state {
	return &state{
		healthyThreshold:   healthyThreshold,
		unhealthyThreshold: unhealthyThreshold,
		healthy:            true,
	}
}

func (s *state) recordSuccess() (flip bool, healthy bool) {
	s.failureStreak = 0
	if s.healthy {
		return false, true
	}
	s.successStreak++
	if s.successStreak >= s.healthyThreshold {
		s.healthy = true
		s.successStreak = 0
		return true, true
	}
	return false, false
}

func (s *state) recordFailure() (flip bool, healthy bool) {
	s.successStreak = 0
	if !s.healthy {
		return false, false
	}
	s.failureStreak++
	if s.failureStreak >= s.unhealthyThreshold {
		s.healthy = false
		s.failureStreak = 0
		return true, false
	}
	return false, true
}
