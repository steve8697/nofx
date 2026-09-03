package utils

import "time"

// TimeProvider defines an interface for getting the current time.
// This allows for mocking time in backtests.
type TimeProvider interface {
	Now() time.Time
}

// RealTimeProvider implements TimeProvider using the actual system time.
type RealTimeProvider struct{}

func (rtp *RealTimeProvider) Now() time.Time {
	return time.Now()
}
