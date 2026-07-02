package server

import (
	"testing"
	"time"
)

func TestShouldExit(t *testing.T) {
	const grace = 90 * time.Second
	const idle = 15 * time.Second
	now := time.Now()

	cases := []struct {
		name              string
		started, lastBeat time.Time
		seen              bool
		want              bool
	}{
		{"startup grace, no beat yet", now.Add(-10 * time.Second), now.Add(-10 * time.Second), false, false},
		{"grace expired, never seen", now.Add(-2 * time.Minute), now.Add(-2 * time.Minute), false, true},
		{"seen and recently active", now.Add(-time.Minute), now.Add(-2 * time.Second), true, false},
		{"seen but idle too long", now.Add(-time.Minute), now.Add(-30 * time.Second), true, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &Server{started: c.started, lastBeat: c.lastBeat, beatSeen: c.seen}
			if got := s.shouldExit(now, grace, idle); got != c.want {
				t.Fatalf("shouldExit = %v, want %v", got, c.want)
			}
		})
	}
}
